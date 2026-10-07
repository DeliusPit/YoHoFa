package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"hohoho/internal/config"
	"hohoho/internal/store"
)

func coreTestHandler(t *testing.T, cfg config.Runtime) http.Handler {
	t.Helper()
	db, err := store.OpenCore(context.Background(), filepath.Join(t.TempDir(), "core.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return NewCore(CoreDependencies{Config: cfg, Store: db})
}

func coreRequest(t *testing.T, handler http.Handler, method, path, body string, status int) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
	if response.Code != status {
		t.Fatalf("%s %s returned %d: %s; want %d", method, path, response.Code, response.Body.String(), status)
	}
	return response
}

func coreMultipartRequest(t *testing.T, handler http.Handler, path string, fields map[string]string, filename, content string, status int) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != status {
		t.Fatalf("POST %s returned %d: %s; want %d", path, response.Code, response.Body.String(), status)
	}
	return response
}

func TestCoreAPIFlow(t *testing.T) {
	handler := coreTestHandler(t, config.Runtime{})
	response := coreRequest(t, handler, "GET", "/api/v1/financial-objects", "", 200)
	if strings.TrimSpace(response.Body.String()) != "[]" {
		t.Fatal("initial objects must be empty")
	}
	coreRequest(t, handler, "GET", "/healthz", "", 200)
	coreRequest(t, handler, "GET", "/readyz", "", 503)
	for _, body := range []string{
		`{"objectType":"asset","name":"Test","unknown":1}`,
		`{"objectType":"asset","name":"Test"} {}`,
		`{"objectType":"income","name":"Test"}`,
		`{"objectType":"asset","name":" "}`,
	} {
		coreRequest(t, handler, "POST", "/api/v1/financial-objects", body, 400)
	}
	var asset, debt store.FinancialObject
	response = coreRequest(t, handler, "POST", "/api/v1/financial-objects", `{"objectType":"asset","name":"Woning"}`, 201)
	if err := json.Unmarshal(response.Body.Bytes(), &asset); err != nil {
		t.Fatal(err)
	}
	response = coreRequest(t, handler, "POST", "/api/v1/financial-objects", `{"objectType":"debt","name":"Hypotheek"}`, 201)
	if err := json.Unmarshal(response.Body.Bytes(), &debt); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/financial-objects/" + strconv.FormatInt(asset.ID, 10)
	coreRequest(t, handler, "PUT", path, `{"objectType":"asset","name":"Woning Sögel","description":"Eigen woning"}`, 200)
	coreRequest(t, handler, "POST", path+"/values", `{"valueDate":"2026-10-01"}`, 400)
	coreRequest(t, handler, "POST", path+"/values", `{"valueDate":"2026-10-01","valueCents":59850000}`, 201)
	response = coreRequest(t, handler, "GET", path, "", 200)
	if err := json.Unmarshal(response.Body.Bytes(), &asset); err != nil || asset.CurrentValue == nil || asset.CurrentValue.ValueCents != 59850000 {
		t.Fatalf("latest value missing: %s, %v", response.Body.String(), err)
	}
	relation, _ := json.Marshal(store.ObjectRelation{SourceObjectID: debt.ID, TargetObjectID: asset.ID, RelationType: "finances"})
	coreRequest(t, handler, "POST", "/api/v1/object-relations", string(relation), 201)
	coreRequest(t, handler, "POST", "/api/v1/object-relations", string(relation), 409)
	response = coreRequest(t, handler, "GET", path+"/relations", "", 200)
	if !strings.Contains(response.Body.String(), "finances") {
		t.Fatal("incoming relation missing")
	}
	coreRequest(t, handler, "DELETE", path, "", 409)
	coreRequest(t, handler, "DELETE", "/api/v1/object-relations", string(relation), 204)
	coreRequest(t, handler, "DELETE", path+"/values/"+strconv.FormatInt(asset.CurrentValue.ID, 10), "", 204)
	coreRequest(t, handler, "DELETE", path, "", 204)
	coreRequest(t, handler, "GET", path, "", 404)
	coreRequest(t, handler, "GET", "/api/v1/financial-objects/-1", "", 400)
	response = coreRequest(t, handler, "GET", "/api/v1/summary", "", 200)
	var summary store.CoreSummary
	if err := json.Unmarshal(response.Body.Bytes(), &summary); err != nil || summary.FinancialObjects != 1 || summary.ObjectValues != 0 || summary.Transactions != 0 {
		t.Fatalf("unexpected summary: %s, %v", response.Body.String(), err)
	}
	response = coreRequest(t, handler, "GET", "/api/v1/system/configuration", "", 200)
	if body := response.Body.String(); !strings.Contains(body, `"backend":"sqlite"`) || !strings.Contains(body, `"secretKeySet":false`) || strings.Contains(body, "S3_SECRET") {
		t.Fatalf("configuration response must be sanitized and v3-specific: %s", body)
	}
}

func TestCoreAuthAndLegacyRoutes(t *testing.T) {
	webDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("v3-browser"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := coreTestHandler(t, config.Runtime{WebDir: webDir, AuthUsername: "user", AuthPassword: "password"})
	for _, path := range []string{"/", "/healthz", "/api/v1/financial-objects", "/api/v1/transactions", "/api/v1/dashboard"} {
		coreRequest(t, handler, "GET", path, "", 401)
	}
	for _, path := range []string{"/api/v1/accounts", "/api/v1/categories", "/api/v1/cases"} {
		request := httptest.NewRequest("GET", path, nil)
		request.SetBasicAuth("user", "password")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 404 || !strings.Contains(response.Header().Get("Content-Type"), "application/json") {
			t.Fatalf("old route must not serve SPA: %s %d %s", path, response.Code, response.Body.String())
		}
	}
	request := httptest.NewRequest("GET", "/", nil)
	request.SetBasicAuth("user", "password")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 || response.Body.String() != "v3-browser" {
		t.Fatal("authenticated browser must receive frontend")
	}
}

func TestCoreConfigurationSaveIsSanitized(t *testing.T) {
	cfg := config.Runtime{ConfigPath: filepath.Join(t.TempDir(), "runtime.json"), S3AccessKey: "old-access", S3SecretKey: "old-secret"}
	handler := coreTestHandler(t, cfg)
	response := coreRequest(t, handler, "PUT", "/api/v1/system/configuration", `{"s3Endpoint":"garage:3900","s3AccessKey":"new-access","s3SecretKey":"new-secret","s3Bucket":"hohoho-documents","s3Region":"garage","s3UseTLS":false,"s3PathStyle":true,"s3CreateBucket":false,"pdfAiUrl":"http://pdf-ai:8090","transactionAiUrl":"http://transaction-ai:8091","transactionAiModel":"Qwen3.6-35B-A3B-Q4_K_M"}`, 200)
	body := response.Body.String()
	if strings.Contains(body, "new-access") || strings.Contains(body, "new-secret") || !strings.Contains(body, `"restartRequired":false`) || !strings.Contains(body, `"accessKeySet":true`) || !strings.Contains(body, `"transactionAiUrl":"http://transaction-ai:8091"`) || !strings.Contains(body, `"transactionAiModel":"Qwen3.6-35B-A3B-Q4_K_M"`) {
		t.Fatalf("configuration response leaked secrets or missed live-swap marker: %s", body)
	}
	response = coreRequest(t, handler, "PUT", "/api/v1/system/configuration", `{"s3Endpoint":"garage:3900","s3Bucket":"hohoho-documents","s3Region":"garage","s3UseTLS":false,"s3PathStyle":true,"s3CreateBucket":false,"pdfAiUrl":"http://pdf-ai:8090","transactionAiUrl":"http://transaction-ai:8092/v1/chat/completions","transactionAiModel":"a-model"}`, 200)
	body = response.Body.String()
	if !strings.Contains(body, `"accessKeySet":true`) || !strings.Contains(body, `"secretKeySet":true`) || !strings.Contains(body, `"transactionAiModel":"a-model"`) {
		t.Fatalf("configuration update did not preserve saved values: %s", body)
	}
	response = coreRequest(t, handler, "GET", "/api/v1/system/configuration", "", 200)
	body = response.Body.String()
	if !strings.Contains(body, `"accessKeySet":true`) || !strings.Contains(body, `"secretKeySet":true`) || !strings.Contains(body, `"transactionAiUrl":"http://transaction-ai:8092/v1/chat/completions"`) {
		t.Fatalf("saved configuration not shown consistently: %s", body)
	}
}

func TestCoreTransactionAIModelsEndpoint(t *testing.T) {
	models := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected models path: %s", r.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": []map[string]string{{"id": "z-model"}, {"id": "a-model"}}})
	}))
	defer models.Close()
	handler := coreTestHandler(t, config.Runtime{ConfigPath: filepath.Join(t.TempDir(), "runtime.json")})
	response := coreRequest(t, handler, "GET", "/api/v1/system/transaction-ai-models?url="+url.QueryEscape(models.URL+"/v1/chat/completions"), "", 200)
	if text := response.Body.String(); !strings.Contains(text, `"models":["a-model","z-model"]`) {
		t.Fatalf("unexpected models response: %s", text)
	}
}

func TestCoreBankConfigRoutesSyncProfile(t *testing.T) {
	cfg := config.Runtime{BankConfigDir: t.TempDir()}
	handler := coreTestHandler(t, cfg)
	body := `{"slug":"sparda","rawYaml":"bank:\n  name: \"Sparda-Bank West\"\n\nsource:\n  format: csv\n  delimiter: \";\"\n  encoding: utf-8\n\nmapping:\n  transaction_date: Buchungstag\n  amount_cents: Betrag\n  description: Verwendungszweck\n  counterparty: Auftraggeber/Empfänger\n\nformats:\n  transaction_date: \"02.01.2006\"\n  decimal_separator: \",\"\n"}`
	response := coreRequest(t, handler, "PUT", "/api/v1/bank-configs/sparda", body, 200)
	if text := response.Body.String(); !strings.Contains(text, `"slug":"sparda"`) || !strings.Contains(text, `"profileId":1`) {
		t.Fatalf("bank config response did not include profile sync: %s", text)
	}
	response = coreRequest(t, handler, "GET", "/api/v1/summary", "", 200)
	if !strings.Contains(response.Body.String(), `"importProfiles":1`) {
		t.Fatalf("profile not synchronized into summary: %s", response.Body.String())
	}
}

func TestCoreBankImportPreviewAndImport(t *testing.T) {
	cfg := config.Runtime{BankConfigDir: t.TempDir()}
	handler := coreTestHandler(t, cfg)
	body := `{"slug":"sparda","rawYaml":"bank:\n  name: \"Sparda-Bank West\"\n\nsource:\n  format: csv\n  delimiter: \";\"\n  encoding: utf-8\n\nmapping:\n  transaction_date: Buchungstag\n  amount_cents: Betrag\n  description: Verwendungszweck\n  counterparty: Auftraggeber/Empfänger\n\nformats:\n  transaction_date: \"02.01.2006\"\n  decimal_separator: \",\"\n"}`
	coreRequest(t, handler, "PUT", "/api/v1/bank-configs/sparda", body, 200)
	csv := "Buchungstag;Betrag;Verwendungszweck;Auftraggeber/Empfänger\n04.10.2026;-83,42;ALDI NORD;ALDI\n05.10.2026;12,34;Loon;Werkgever\n"
	response := coreMultipartRequest(t, handler, "/api/v1/bank-imports/preview", map[string]string{"bankConfig": "sparda"}, "sparda.csv", csv, 200)
	if text := response.Body.String(); !strings.Contains(text, `"total":2`) || !strings.Contains(text, `"amount_cents":-8342`) {
		t.Fatalf("unexpected preview: %s", text)
	}
	response = coreMultipartRequest(t, handler, "/api/v1/bank-imports", map[string]string{"bankConfig": "sparda"}, "sparda.csv", csv, 201)
	if text := response.Body.String(); !strings.Contains(text, `"imported":2`) {
		t.Fatalf("unexpected import result: %s", text)
	}
	response = coreMultipartRequest(t, handler, "/api/v1/bank-imports", map[string]string{"bankConfig": "sparda"}, "sparda.csv", csv, 201)
	if text := response.Body.String(); !strings.Contains(text, `"imported":0`) || !strings.Contains(text, `"skipped":2`) {
		t.Fatalf("duplicate import was not skipped: %s", text)
	}
	response = coreRequest(t, handler, "GET", "/api/v1/transactions?limit=10", "", 200)
	if text := response.Body.String(); !strings.Contains(text, `"total":2`) || !strings.Contains(text, "ALDI NORD") || !strings.Contains(text, "Werkgever") || !strings.Contains(text, `"status":"I"`) {
		t.Fatalf("transactions not imported: %s", text)
	}
}

func TestCoreBankImportReportsAIErrorBody(t *testing.T) {
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/running" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "context length exceeded"})
	}))
	defer ai.Close()
	handler := coreTestHandler(t, config.Runtime{BankConfigDir: t.TempDir(), TransactionAIURL: ai.URL, TransactionAIModel: "test-model"})
	coreRequest(t, handler, "POST", "/api/v1/financial-objects", `{"objectType":"expense","name":"Bankimport"}`, 201)
	body := `{"slug":"sparda","rawYaml":"bank:\n  name: \"Sparda-Bank West\"\n\nsource:\n  format: csv\n  delimiter: \";\"\n  encoding: utf-8\n\nmapping:\n  transaction_date: Buchungstag\n  amount_cents: Betrag\n  description: Verwendungszweck\n  counterparty: Tegenpartij\n\nformats:\n  transaction_date: \"02.01.2006\"\n  decimal_separator: \",\"\n"}`
	coreRequest(t, handler, "PUT", "/api/v1/bank-configs/sparda", body, 200)
	csv := "Buchungstag;Betrag;Verwendungszweck;Tegenpartij\n04.10.2026;-83,42;ALDI NORD;ALDI\n"
	coreMultipartRequest(t, handler, "/api/v1/bank-imports", map[string]string{"bankConfig": "sparda"}, "sparda.csv", csv, 201)
	response := coreRequest(t, handler, "POST", "/api/v1/transactions/classify-ai", `{}`, 502)
	if text := response.Body.String(); !strings.Contains(text, "context length exceeded") || !strings.Contains(text, "status 400") {
		t.Fatalf("AI error body not reported: %s", text)
	}
}

func TestCoreBankImportUsesSavedAIConfigurationWithoutRestart(t *testing.T) {
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/running" {
			http.NotFound(w, r)
			return
		}
		var input struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Model != "saved-model" {
			t.Fatalf("unexpected AI request: %#v err=%v", input, err)
		}
		writeJSON(w, http.StatusOK, map[string]any{"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": `{"transactions":[{"row_index":1,"category_id":"1","confidence":0.99}]}`}}}})
	}))
	defer ai.Close()
	handler := coreTestHandler(t, config.Runtime{ConfigPath: filepath.Join(t.TempDir(), "runtime.json"), BankConfigDir: t.TempDir()})
	coreRequest(t, handler, "PUT", "/api/v1/system/configuration", `{"s3Endpoint":"garage:3900","s3Bucket":"hohoho-documents","s3Region":"garage","s3UseTLS":false,"s3PathStyle":true,"s3CreateBucket":false,"pdfAiUrl":"","transactionAiUrl":"`+ai.URL+`","transactionAiModel":"saved-model"}`, 200)
	coreRequest(t, handler, "POST", "/api/v1/financial-objects", `{"objectType":"expense","name":"Bankimport"}`, 201)
	body := `{"slug":"sparda","rawYaml":"bank:\n  name: \"Sparda-Bank West\"\n\nsource:\n  format: csv\n  delimiter: \";\"\n  encoding: utf-8\n\nmapping:\n  transaction_date: Buchungstag\n  amount_cents: Betrag\n  description: Verwendungszweck\n  counterparty: Tegenpartij\n\nformats:\n  transaction_date: \"02.01.2006\"\n  decimal_separator: \",\"\n"}`
	coreRequest(t, handler, "PUT", "/api/v1/bank-configs/sparda", body, 200)
	csv := "Buchungstag;Betrag;Verwendungszweck;Tegenpartij\n04.10.2026;-83,42;ALDI NORD;ALDI\n"
	coreMultipartRequest(t, handler, "/api/v1/bank-imports", map[string]string{"bankConfig": "sparda"}, "sparda.csv", csv, 201)
	response := coreRequest(t, handler, "POST", "/api/v1/transactions/classify-ai", `{}`, 200)
	if text := response.Body.String(); !strings.Contains(text, `"classified":1`) {
		t.Fatalf("classification did not use saved AI configuration: %s", text)
	}
}

func TestCoreBankImportWarmsUpLlamaSwapModel(t *testing.T) {
	warmups := 0
	chatCalls := 0
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/running" {
			if warmups == 0 {
				writeJSON(w, http.StatusOK, map[string]string{"model": "other-model"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"model": "target-model"})
			return
		}
		var input struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Model != "target-model" {
			t.Fatalf("unexpected AI request: %#v err=%v", input, err)
		}
		if input.MaxTokens == 1 {
			warmups++
			writeJSON(w, http.StatusOK, map[string]any{"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": `ok`}}}})
			return
		}
		chatCalls++
		writeJSON(w, http.StatusOK, map[string]any{"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": `{"transactions":[{"row_index":1,"category_id":"1","confidence":0.99}]}`}}}})
	}))
	defer ai.Close()
	handler := coreTestHandler(t, config.Runtime{BankConfigDir: t.TempDir(), TransactionAIURL: ai.URL, TransactionAIModel: "target-model"})
	coreRequest(t, handler, "POST", "/api/v1/financial-objects", `{"objectType":"expense","name":"Bankimport"}`, 201)
	body := `{"slug":"sparda","rawYaml":"bank:\n  name: \"Sparda-Bank West\"\n\nsource:\n  format: csv\n  delimiter: \";\"\n  encoding: utf-8\n\nmapping:\n  transaction_date: Buchungstag\n  amount_cents: Betrag\n  description: Verwendungszweck\n  counterparty: Tegenpartij\n\nformats:\n  transaction_date: \"02.01.2006\"\n  decimal_separator: \",\"\n"}`
	coreRequest(t, handler, "PUT", "/api/v1/bank-configs/sparda", body, 200)
	csv := "Buchungstag;Betrag;Verwendungszweck;Tegenpartij\n04.10.2026;-83,42;ALDI NORD;ALDI\n"
	coreMultipartRequest(t, handler, "/api/v1/bank-imports", map[string]string{"bankConfig": "sparda"}, "sparda.csv", csv, 201)
	response := coreRequest(t, handler, "POST", "/api/v1/transactions/classify-ai", `{}`, 200)
	if text := response.Body.String(); !strings.Contains(text, `"classified":1`) || warmups != 1 || chatCalls != 1 {
		t.Fatalf("unexpected import/warmup result: body=%s warmups=%d chatCalls=%d", text, warmups, chatCalls)
	}
}

func TestTransactionAIRunningHelpers(t *testing.T) {
	got, err := llamaSwapRunningURL("http://192.168.178.215:8081/v1/chat/completions")
	if err != nil || got != "http://192.168.178.215:8081/running" {
		t.Fatalf("unexpected running url: %q err=%v", got, err)
	}
	for _, input := range [][]byte{[]byte(`"model-a"`), []byte(`{"model":"model-a"}`), []byte(`{"current_model":"model-a"}`), []byte(`{"running":[{"model":"model-a","state":"ready"}]}`), []byte(`model-a`)} {
		got, ok := parseRunningModel(input)
		if !ok || got != "model-a" {
			t.Fatalf("unexpected running model for %s: %q", input, got)
		}
	}
}

func TestParseBankImportAIContentVariants(t *testing.T) {
	for _, input := range []string{
		`{"classifications":[{"row":2,"financialObjectId":1}]}`,
		"```json\n{\"transactions\":[{\"row_index\":2,\"category_id\":\"1\",\"confidence\":0.9}]}\n```",
		`[{"row":2,"financialObjectId":1}]`,
		`Hier is de JSON: {"results":[{"row":2,"financialObjectId":1}]}`,
	} {
		var output bankImportAIResponse
		err := parseBankImportAIContent(input, &output)
		row := 0
		if len(output.Classifications) == 1 {
			row = output.Classifications[0].Row
			if row == 0 {
				row = output.Classifications[0].RowIndex
			}
		}
		if err != nil || len(output.Classifications) != 1 || row != 2 {
			t.Fatalf("unexpected parse result for %q: %#v err=%v", input, output, err)
		}
	}
}
