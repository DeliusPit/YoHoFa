package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"geldzaken/internal/bankimport"
	"geldzaken/internal/config"
	"geldzaken/internal/store"
)

func TestImportedTransactionsKeepsRepeatedTransactionsDistinct(t *testing.T) {
	transaction := bankimport.Transaction{
		BookedOn:    "2026-01-01",
		Description: "Dezelfde betaling",
		AmountCents: -1000,
	}

	first := importedTransactions(1, []bankimport.Transaction{transaction, transaction})
	if first[0].ImportFingerprint == first[1].ImportFingerprint {
		t.Fatal("repeated transactions must have distinct fingerprints")
	}

	second := importedTransactions(1, []bankimport.Transaction{transaction, transaction})
	if first[0].ImportFingerprint != second[0].ImportFingerprint || first[1].ImportFingerprint != second[1].ImportFingerprint {
		t.Fatal("fingerprints must remain stable when the same file is imported again")
	}
}

func TestConfigurationGETMasksSecrets(t *testing.T) {
	manager := editableConfigurationManager(t)
	handler := New(Dependencies{
		Config:        manager.Effective(),
		Configuration: manager,
		Store:         store.OpenMemory(),
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/system/configuration", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET returned %d, want %d", response.Code, http.StatusOK)
	}
	for _, secret := range []string{"db-secret", "s3-access", "s3-secret", "postgres://"} {
		if bytes.Contains(response.Body.Bytes(), []byte(secret)) {
			t.Fatal("GET response leaked secret configuration")
		}
	}
	var view config.View
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if !view.Editable || !view.Effective.Database.PasswordSet || !view.Effective.Objects.AccessKeySet || !view.Effective.Objects.SecretKeySet {
		t.Fatalf("unexpected safe view: %+v", view)
	}
}

func TestConfigurationPUTIsStrictAndDoesNotLeakSecrets(t *testing.T) {
	manager := editableConfigurationManager(t)
	handler := New(Dependencies{
		Config:        manager.Effective(),
		Configuration: manager,
		Store:         store.OpenMemory(),
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	tests := []string{
		`{"storageProfile":`,
		`{"unknown":"db-secret"}`,
		validConfigurationJSON() + ` {}`,
		strings.Replace(validConfigurationJSON(), `"central"`, `"local"`, 1),
	}
	for _, body := range tests {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/v1/system/configuration", strings.NewReader(body)))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("PUT %q returned %d, want %d", body, response.Code, http.StatusBadRequest)
		}
		if strings.Contains(response.Body.String(), "db-secret") {
			t.Fatal("PUT error response leaked request content")
		}
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/v1/system/configuration", strings.NewReader(validConfigurationJSON())))
	if response.Code != http.StatusOK {
		t.Fatalf("valid PUT returned %d: %s", response.Code, response.Body.String())
	}
	if !manager.View().RestartRequired || manager.Effective().S3Endpoint != "old-s3:9000" {
		t.Fatal("PUT did not remain pending until restart")
	}
}

func TestConfigurationPUTRejectsEnvironmentOwnedConfiguration(t *testing.T) {
	clearStorageEnvironment(t)
	t.Setenv("MINIO_ENDPOINT", "environment-owned:9000")
	cfg := configurationTestConfig(t)
	cfg.ConfigEditable = true
	manager, err := config.NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(Dependencies{Config: cfg, Configuration: manager, Store: store.OpenMemory(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/v1/system/configuration", strings.NewReader(validConfigurationJSON())))
	if response.Code != http.StatusForbidden || response.Body.String() != "{\"error\":\"configuration_read_only\"}\n" {
		t.Fatalf("unexpected read-only response: %d %s", response.Code, response.Body.String())
	}
}

func editableConfigurationManager(t *testing.T) *config.Manager {
	t.Helper()
	clearStorageEnvironment(t)
	cfg := configurationTestConfig(t)
	cfg.ConfigEditable = true
	manager, err := config.NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func configurationTestConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		HTTPAddr:       "127.0.0.1:8080",
		StorageProfile: "server",
		StoreBackend:   "postgres",
		DatabaseURL:    "postgres://money:db-secret@old-db:5432/money?sslmode=disable",
		ObjectBackend:  "s3",
		S3Endpoint:     "old-s3:9000",
		S3AccessKey:    "s3-access",
		S3SecretKey:    "s3-secret",
		S3Bucket:       "documents",
		S3Region:       "us-east-1",
		S3PathStyle:    true,
		S3CreateBucket: true,
		ConfigPath:     filepath.Join(t.TempDir(), "config.json"),
	}
}

func clearStorageEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"STORAGE_PROFILE", "DATABASE_BACKEND", "STORE_BACKEND", "DATABASE_URL", "DATABASE_PATH", "OBJECT_BACKEND", "S3_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY", "S3_BUCKET", "S3_USE_TLS", "S3_REGION", "S3_PATH_STYLE", "S3_CREATE_BUCKET", "MINIO_ENDPOINT", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY", "MINIO_BUCKET", "MINIO_USE_SSL"} {
		t.Setenv(key, "")
	}
}

func validConfigurationJSON() string {
	return `{"profile":"central","database":{"backend":"postgres","host":"new-db","port":5432,"name":"money","user":"money","sslMode":"require"},"objects":{"backend":"s3","endpoint":"new-s3:3900","bucket":"documents","useTLS":false,"region":"garage","pathStyle":true,"createBucket":false}}`
}

func TestBasicAuthProtectsAPI(t *testing.T) {
	handler := New(Dependencies{
		Config: config.Config{AuthUsername: "user", AuthPassword: "secret"},
		Store:  store.OpenMemory(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/summary", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("request without credentials returned %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/summary", nil)
	request.SetBasicAuth("user", "secret")
	authorized := httptest.NewRecorder()
	handler.ServeHTTP(authorized, request)
	if authorized.Code != http.StatusOK {
		t.Fatalf("request with credentials returned %d, want %d", authorized.Code, http.StatusOK)
	}
}

func TestHealthIsUnhealthyWhenConfiguredObjectStoreIsUnavailable(t *testing.T) {
	handler := New(Dependencies{
		Store:               store.OpenMemory(),
		ObjectStoreRequired: true,
		Logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("health returned %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestDomainEventsRequirePostgres(t *testing.T) {
	handler := New(Dependencies{
		Store:  store.OpenMemory(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/assets/1/events", nil))
	if response.Code != http.StatusNotImplemented {
		t.Fatalf("events returned %d, want %d", response.Code, http.StatusNotImplemented)
	}
	if response.Body.String() != "{\"error\":\"domain_events_require_postgres\"}\n" {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}
