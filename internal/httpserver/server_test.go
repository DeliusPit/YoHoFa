package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
