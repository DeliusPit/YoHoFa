package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"hohoho/internal/bankconfig"
	"hohoho/internal/config"
	"hohoho/internal/store"
)

const defaultTransactionAIModel = "Qwen3.6-35B-A3B-Q4_K_M"
const transactionAIBatchSize = 25
const transactionAITimeout = 10 * time.Minute
const transactionAIWarmupTimeout = 5 * time.Minute

type coreBankImportPreview struct {
	BankConfig string                 `json:"bankConfig"`
	BankName   string                 `json:"bankName"`
	Rows       []bankconfig.ParsedRow `json:"rows"`
	Total      int                    `json:"total"`
}

type bankImportAICategory struct {
	ID          string `json:"id"`
	ObjectType  string `json:"objectType"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type chatCompletionRequest struct {
	Model              string             `json:"model"`
	Temperature        float64            `json:"temperature"`
	MaxTokens          int                `json:"max_tokens"`
	Stream             bool               `json:"stream"`
	ChatTemplateKwargs chatTemplateKwargs `json:"chat_template_kwargs,omitempty"`
	ResponseFormat     responseFormat     `json:"response_format,omitempty"`
	Messages           []chatMessage      `json:"messages"`
}

type chatTemplateKwargs struct {
	EnableThinking bool `json:"enable_thinking"`
}

type responseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *jsonSchema `json:"json_schema,omitempty"`
}

type jsonSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

type runningModelStatus int

const (
	runningModelUnsupported runningModelStatus = iota
	runningModelMismatch
	runningModelMatch
)

type bankImportAIResponse struct {
	Classifications []bankImportAIClassification `json:"classifications"`
}

type bankImportAIClassification struct {
	Row                     int     `json:"row"`
	RowIndex                int     `json:"row_index"`
	FinancialObjectID       int64   `json:"financialObjectId"`
	FinancialObjectIDLegacy int64   `json:"financial_object_id"`
	CategoryID              string  `json:"category_id"`
	AmountCents             *int64  `json:"amountCents,omitempty"`
	Confidence              float64 `json:"confidence,omitempty"`
	Reason                  string  `json:"reason,omitempty"`
}

func registerCoreBankImportRoutes(mux *http.ServeMux, deps CoreDependencies) {
	mux.HandleFunc("POST /api/v1/bank-imports/preview", func(w http.ResponseWriter, r *http.Request) {
		item, rows, ok := parseBankImportUpload(w, r, deps, 50)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, coreBankImportPreview{BankConfig: item.Slug, BankName: item.Bank.Name, Rows: rows, Total: len(rows)})
	})
	mux.HandleFunc("POST /api/v1/bank-imports", func(w http.ResponseWriter, r *http.Request) {
		item, rows, ok := parseBankImportUpload(w, r, deps, 0)
		if !ok {
			return
		}
		configuration, err := bankconfig.ConfigurationJSON(item)
		if err != nil {
			coreRespond(w, r, deps, 0, nil, store.ErrCoreInvalid)
			return
		}
		profile, err := deps.Store.UpsertBankImportProfile(r.Context(), item.Bank.Name, item.Source.Format, configuration)
		if err != nil {
			coreRespond(w, r, deps, 0, nil, err)
			return
		}
		importRows := make([]store.CoreBankImportRow, 0, len(rows))
		for _, row := range rows {
			importRows = append(importRows, store.CoreBankImportRow{Parsed: row})
		}
		result, err := deps.Store.ImportBankTransactions(r.Context(), profile, importRows)
		coreRespond(w, r, deps, http.StatusCreated, result, err)
	})
}

func classifyTransactionsWithAI(ctx context.Context, deps CoreDependencies, transactions []store.CoreTransaction) (map[int64]store.TransactionObjectInput, map[int64]bool, error) {
	runtimeConfig, _ := config.SavedRuntime(deps.Config.ConfigPath, deps.Config)
	if runtimeConfig.TransactionAIURL == "" {
		return nil, nil, fmt.Errorf("Transactie-AI URL is niet geconfigureerd")
	}
	objects, err := deps.Store.FinancialObjects(ctx, "")
	if err != nil {
		return nil, nil, err
	}
	categories := make([]bankImportAICategory, 0, len(objects))
	validObjects := map[int64]bool{}
	for _, object := range objects {
		categories = append(categories, bankImportAICategory{ID: fmt.Sprint(object.ID), ObjectType: object.ObjectType, Name: object.Name, Description: object.Description})
		validObjects[object.ID] = true
	}
	model := runtimeConfig.TransactionAIModel
	if model == "" {
		model = defaultTransactionAIModel
	}
	if err := prepareTransactionAIModel(ctx, runtimeConfig.TransactionAIURL, model); err != nil {
		return nil, nil, err
	}
	allocationsByTransaction := map[int64]store.TransactionObjectInput{}
	failed := map[int64]bool{}
	for start := 0; start < len(transactions); start += transactionAIBatchSize {
		end := start + transactionAIBatchSize
		if end > len(transactions) {
			end = len(transactions)
		}
		batch := transactions[start:end]
		output, err := classifyTransactionBatch(ctx, runtimeConfig.TransactionAIURL, categories, batch, model)
		if err != nil {
			return nil, nil, err
		}
		seenRows := map[int]bool{}
		for _, classification := range output.Classifications {
			rowIndex := classification.Row
			if rowIndex == 0 {
				rowIndex = classification.RowIndex
			}
			financialObjectID := classification.FinancialObjectID
			if financialObjectID == 0 {
				financialObjectID = classification.FinancialObjectIDLegacy
			}
			if financialObjectID == 0 && classification.CategoryID != "" {
				parsedID, err := strconv.ParseInt(strings.TrimSpace(classification.CategoryID), 10, 64)
				if err != nil {
					continue
				}
				financialObjectID = parsedID
			}
			if !validObjects[financialObjectID] {
				continue
			}
			if rowIndex < 1 || rowIndex > len(batch) {
				continue
			}
			transaction := batch[rowIndex-1]
			amount := transaction.AmountCents
			seenRows[rowIndex] = true
			allocationsByTransaction[transaction.ID] = store.TransactionObjectInput{FinancialObjectID: financialObjectID, AmountCents: &amount}
		}
		for i, transaction := range batch {
			if !seenRows[i+1] {
				failed[transaction.ID] = true
			}
		}
	}
	return allocationsByTransaction, failed, nil
}

func prepareTransactionAIModel(ctx context.Context, transactionAIURL, model string) error {
	runningURL, err := llamaSwapRunningURL(transactionAIURL)
	if err != nil {
		return err
	}
	status, running, err := runningModel(ctx, runningURL, model)
	if err != nil {
		return fmt.Errorf("Transactie-AI status kon niet worden gecontroleerd: %s", err)
	}
	if status == runningModelUnsupported || status == runningModelMatch {
		return nil
	}
	if err := warmupTransactionAIModel(ctx, transactionAIURL, model); err != nil {
		return err
	}
	deadline := time.Now().Add(transactionAIWarmupTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
		status, running, err = runningModel(ctx, runningURL, model)
		if err != nil {
			return fmt.Errorf("Transactie-AI status kon niet worden gecontroleerd: %s", err)
		}
		if status == runningModelUnsupported || status == runningModelMatch {
			return nil
		}
	}
	if running == "" {
		running = "geen model"
	}
	return fmt.Errorf("Transactie-AI model %q werd niet actief binnen %s; llama-swap meldt %q", model, transactionAIWarmupTimeout, running)
}

func warmupTransactionAIModel(ctx context.Context, transactionAIURL, model string) error {
	body, err := json.Marshal(chatCompletionRequest{Model: model, Temperature: 0, MaxTokens: 1, Stream: false, ChatTemplateKwargs: chatTemplateKwargs{EnableThinking: false}, Messages: []chatMessage{{Role: "system", Content: "Antwoord kort."}, {Role: "user", Content: "ping"}}})
	if err != nil {
		return err
	}
	requestCtx, cancel := context.WithTimeout(ctx, transactionAIWarmupTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, transactionAIURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		if isTimeoutError(err) {
			return fmt.Errorf("Transactie-AI model %q kon niet binnen %s worden geladen via llama-swap: %s", model, transactionAIWarmupTimeout, networkErrorMessage(err))
		}
		return fmt.Errorf("Transactie-AI model %q kon niet worden aangevraagd via llama-swap: %s", model, networkErrorMessage(err))
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		detail := limitedResponseText(response.Body, 2048)
		if detail != "" {
			return fmt.Errorf("Transactie-AI model %q kon niet worden aangevraagd; status %d: %s", model, response.StatusCode, detail)
		}
		return fmt.Errorf("Transactie-AI model %q kon niet worden aangevraagd; status %d", model, response.StatusCode)
	}
	return nil
}

func llamaSwapRunningURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("Transactie-AI URL is ongeldig")
	}
	parsed.Path = "/running"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func runningModel(ctx context.Context, runningURL, expected string) (runningModelStatus, string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, runningURL, nil)
	if err != nil {
		return runningModelUnsupported, "", err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return runningModelUnsupported, "", err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusMethodNotAllowed {
		return runningModelUnsupported, "", nil
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return runningModelUnsupported, "", fmt.Errorf("/running gaf status %d", response.StatusCode)
	}
	data, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	running, recognized := parseRunningModel(data)
	if !recognized {
		return runningModelUnsupported, "", nil
	}
	if running == expected {
		return runningModelMatch, running, nil
	}
	return runningModelMismatch, running, nil
}

func parseRunningModel(data []byte) (string, bool) {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" || text == "{}" {
		return "", true
	}
	var single string
	if json.Unmarshal(data, &single) == nil {
		return strings.TrimSpace(single), true
	}
	var list []string
	if json.Unmarshal(data, &list) == nil {
		if len(list) == 0 {
			return "", true
		}
		return strings.TrimSpace(list[0]), strings.TrimSpace(list[0]) != ""
	}
	var objectList []map[string]any
	if json.Unmarshal(data, &objectList) == nil {
		return runningModelFromObjectList(objectList)
	}
	var object map[string]any
	if json.Unmarshal(data, &object) == nil {
		for _, key := range []string{"model", "model_name", "id", "name", "running", "current_model", "current"} {
			if value, ok := object[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value), true
			}
			if values, ok := object[key].([]any); ok {
				if len(values) == 0 {
					return "", true
				}
				if value, ok := values[0].(string); ok && strings.TrimSpace(value) != "" {
					return strings.TrimSpace(value), true
				}
				if value, ok := values[0].(map[string]any); ok {
					if model, ok := runningModelFromObject(value); ok {
						return model, true
					}
				}
			}
		}
		return "", false
	}
	return text, true
}

func runningModelFromObjectList(values []map[string]any) (string, bool) {
	if len(values) == 0 {
		return "", true
	}
	return runningModelFromObject(values[0])
}

func runningModelFromObject(value map[string]any) (string, bool) {
	for _, key := range []string{"model", "model_name", "id", "name", "current_model", "current"} {
		if model, ok := value[key].(string); ok && strings.TrimSpace(model) != "" {
			return strings.TrimSpace(model), true
		}
	}
	return "", false
}

func classifyBankImportBatch(ctx context.Context, transactionAIURL string, item bankconfig.Config, categories []bankImportAICategory, rows []bankconfig.ParsedRow, model string) (bankImportAIResponse, error) {
	requestBody, err := json.Marshal(bankImportAIChatRequest(model, item, categories, rows))
	if err != nil {
		return bankImportAIResponse{}, err
	}
	return doTransactionAIRequest(ctx, transactionAIURL, requestBody, len(rows))
}

func classifyTransactionBatch(ctx context.Context, transactionAIURL string, categories []bankImportAICategory, transactions []store.CoreTransaction, model string) (bankImportAIResponse, error) {
	requestBody, err := json.Marshal(transactionAIChatRequest(model, categories, transactions))
	if err != nil {
		return bankImportAIResponse{}, err
	}
	return doTransactionAIRequest(ctx, transactionAIURL, requestBody, len(transactions))
}

func doTransactionAIRequest(ctx context.Context, transactionAIURL string, requestBody []byte, rowCount int) (bankImportAIResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, transactionAITimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, transactionAIURL, bytes.NewReader(requestBody))
	if err != nil {
		return bankImportAIResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		if isTimeoutError(err) {
			return bankImportAIResponse{}, fmt.Errorf("Transactie-AI antwoordde niet binnen %s voor %d regels (%d bytes request). Bij llama-swap betekent dit meestal dat het model nog laadt of de completion te langzaam is: %s", transactionAITimeout, rowCount, len(requestBody), networkErrorMessage(err))
		}
		return bankImportAIResponse{}, fmt.Errorf("Transactie-AI is niet bereikbaar voor %d regels (%d bytes request): %s", rowCount, len(requestBody), networkErrorMessage(err))
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		detail := limitedResponseText(response.Body, 2048)
		if detail != "" {
			return bankImportAIResponse{}, fmt.Errorf("Transactie-AI gaf status %d voor %d regels (%d bytes request): %s", response.StatusCode, rowCount, len(requestBody), detail)
		}
		return bankImportAIResponse{}, fmt.Errorf("Transactie-AI gaf status %d voor %d regels (%d bytes request)", response.StatusCode, rowCount, len(requestBody))
	}
	var chatOutput chatCompletionResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&chatOutput); err != nil || len(chatOutput.Choices) == 0 {
		return bankImportAIResponse{}, fmt.Errorf("Transactie-AI antwoord is ongeldig")
	}
	var output bankImportAIResponse
	if err := parseBankImportAIContent(chatOutput.Choices[0].Message.Content, &output); err != nil {
		return bankImportAIResponse{}, fmt.Errorf("Transactie-AI JSON is ongeldig: %s", previewText(chatOutput.Choices[0].Message.Content, 1000))
	}
	return output, nil
}

func parseBankImportAIContent(content string, output *bankImportAIResponse) error {
	jsonText := extractJSONText(content)
	if jsonText == "" {
		return errors.New("empty AI JSON")
	}
	if json.Unmarshal([]byte(jsonText), output) == nil && len(output.Classifications) > 0 {
		return nil
	}
	var wrapper struct {
		Classifications []bankImportAIClassification `json:"classifications"`
		Transactions    []bankImportAIClassification `json:"transactions"`
		Items           []bankImportAIClassification `json:"items"`
		Results         []bankImportAIClassification `json:"results"`
	}
	if err := json.Unmarshal([]byte(jsonText), &wrapper); err == nil {
		switch {
		case len(wrapper.Classifications) > 0:
			output.Classifications = wrapper.Classifications
			return nil
		case len(wrapper.Transactions) > 0:
			output.Classifications = wrapper.Transactions
			return nil
		case len(wrapper.Items) > 0:
			output.Classifications = wrapper.Items
			return nil
		case len(wrapper.Results) > 0:
			output.Classifications = wrapper.Results
			return nil
		}
	}
	var list []bankImportAIClassification
	if err := json.Unmarshal([]byte(jsonText), &list); err == nil && len(list) > 0 {
		output.Classifications = list
		return nil
	}
	var single bankImportAIClassification
	if err := json.Unmarshal([]byte(jsonText), &single); err == nil && single.Row != 0 {
		output.Classifications = []bankImportAIClassification{single}
		return nil
	}
	return errors.New("unsupported AI JSON shape")
}

func extractJSONText(content string) string {
	text := strings.TrimSpace(content)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		return text
	}
	objectStart := strings.Index(text, "{")
	arrayStart := strings.Index(text, "[")
	start := -1
	if objectStart >= 0 && arrayStart >= 0 {
		if objectStart < arrayStart {
			start = objectStart
		} else {
			start = arrayStart
		}
	} else if objectStart >= 0 {
		start = objectStart
	} else {
		start = arrayStart
	}
	if start < 0 {
		return ""
	}
	end := strings.LastIndexAny(text, "}]")
	if end <= start {
		return ""
	}
	return strings.TrimSpace(text[start : end+1])
}

func previewText(value string, limit int) string {
	text := strings.Join(strings.Fields(value), " ")
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "..."
}

func limitedResponseText(body io.Reader, limit int64) string {
	data, _ := io.ReadAll(io.LimitReader(body, limit))
	return strings.Join(strings.Fields(string(data)), " ")
}

func networkErrorMessage(err error) string {
	message := strings.Join(strings.Fields(err.Error()), " ")
	if parsed, parseErr := url.Parse(message); parseErr == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return parsed.String()
	}
	return strings.ReplaceAll(message, "?", " ?")
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func bankImportAIChatRequest(model string, item bankconfig.Config, categories []bankImportAICategory, rows []bankconfig.ParsedRow) chatCompletionRequest {
	return chatCompletionRequest{
		Model:              model,
		Temperature:        0.1,
		MaxTokens:          2000,
		Stream:             false,
		ChatTemplateKwargs: chatTemplateKwargs{EnableThinking: false},
		ResponseFormat:     responseFormat{Type: "json_schema", JSONSchema: &jsonSchema{Name: "categorized_transactions", Strict: true, Schema: bankImportAIJSONSchema()}},
		Messages: []chatMessage{
			{Role: "system", Content: "Je categoriseert Nederlandse banktransacties. Kies uitsluitend een category_id uit de opgegeven categorieen. Categoriseer iedere transactieregel exact een keer. Verander geen bedragen of transactiedata. Gebruik row_index om naar de invoerregel te verwijzen. Geef uitsluitend JSON terug volgens het schema."},
			{Role: "user", Content: bankImportAIPrompt(item, categories, rows)},
		},
	}
}

func transactionAIChatRequest(model string, categories []bankImportAICategory, transactions []store.CoreTransaction) chatCompletionRequest {
	return chatCompletionRequest{
		Model:              model,
		Temperature:        0.1,
		MaxTokens:          2000,
		Stream:             false,
		ChatTemplateKwargs: chatTemplateKwargs{EnableThinking: false},
		ResponseFormat:     responseFormat{Type: "json_schema", JSONSchema: &jsonSchema{Name: "categorized_transactions", Strict: true, Schema: bankImportAIJSONSchema()}},
		Messages: []chatMessage{
			{Role: "system", Content: "Je categoriseert Nederlandse banktransacties. Kies uitsluitend een category_id uit de opgegeven categorieen. Categoriseer iedere transactieregel exact een keer. Verander geen bedragen of transactiedata. Gebruik row_index om naar de invoerregel te verwijzen. Geef uitsluitend JSON terug volgens het schema."},
			{Role: "user", Content: transactionAIPrompt(categories, transactions)},
		},
	}
}

func bankImportAIJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"transactions": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"row_index":   map[string]any{"type": "integer"},
						"category_id": map[string]any{"type": "string"},
						"confidence":  map[string]any{"type": "number"},
					},
					"required":             []string{"row_index", "category_id", "confidence"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"transactions"},
		"additionalProperties": false,
	}
}

func bankImportAIPrompt(item bankconfig.Config, categories []bankImportAICategory, rows []bankconfig.ParsedRow) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Bankconfiguratie: %s\nBank: %s\n\n", item.Slug, item.Bank.Name)
	categoriesJSON, _ := json.Marshal(categories)
	builder.WriteString("CATEGORIEEN:\n")
	builder.Write(categoriesJSON)
	builder.WriteString("\n\nCSV-kolommen:\nrow_index,datum,bedrag_centen,tegenpartij,omschrijving\n\nTRANSACTIES:\n")
	for _, row := range rows {
		fmt.Fprintf(&builder, "%d,%q,%d,%q,%q\n", row.Row, row.Transaction.TransactionDate, row.Transaction.AmountCents, row.Transaction.Counterparty, row.Transaction.Description)
	}
	builder.WriteString("\nGeef exact deze JSON-vorm terug, met iedere row_index exact een keer:\n")
	builder.WriteString(`{"transactions":[{"row_index":1,"category_id":"1","confidence":0.99}]}`)
	return builder.String()
}

func transactionAIPrompt(categories []bankImportAICategory, transactions []store.CoreTransaction) string {
	var builder strings.Builder
	categoriesJSON, _ := json.Marshal(categories)
	builder.WriteString("CATEGORIEEN:\n")
	builder.Write(categoriesJSON)
	builder.WriteString("\n\nCSV-kolommen:\nrow_index,datum,bedrag_centen,tegenpartij,omschrijving\n\nTRANSACTIES:\n")
	for index, transaction := range transactions {
		fmt.Fprintf(&builder, "%d,%q,%d,%q,%q\n", index+1, transaction.TransactionDate, transaction.AmountCents, transaction.Counterparty, transaction.Description)
	}
	builder.WriteString("\nGeef exact deze JSON-vorm terug, met iedere row_index exact een keer:\n")
	builder.WriteString(`{"transactions":[{"row_index":1,"category_id":"1","confidence":0.99}]}`)
	return builder.String()
}

func parseBankImportUpload(w http.ResponseWriter, r *http.Request, deps CoreDependencies, limit int) (bankconfig.Config, []bankconfig.ParsedRow, bool) {
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Upload kon niet worden gelezen"})
		return bankconfig.Config{}, nil, false
	}
	slug := r.FormValue("bankConfig")
	item, err := bankconfig.Read(deps.Config.BankConfigDir, slug)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Bankconfiguratie kon niet worden geladen"})
		return bankconfig.Config{}, nil, false
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Kies een bankbestand"})
		return bankconfig.Config{}, nil, false
	}
	defer file.Close()
	rows, err := bankconfig.ParseCSVRows(item, file, limit)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return bankconfig.Config{}, nil, false
	}
	if len(rows) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Het bestand bevat geen transactieregels"})
		return bankconfig.Config{}, nil, false
	}
	return item, rows, true
}
