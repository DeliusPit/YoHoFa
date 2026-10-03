package documenttranslation

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hohoho/internal/store"
)

func TestProcessStoresAnalysisFromPDFAIService(t *testing.T) {
	var uploaded string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/document/analyse" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("missing multipart file: %v", err)
		}
		defer file.Close()
		if header.Filename != "test.pdf" {
			t.Fatalf("unexpected filename %q", header.Filename)
		}
		data, _ := io.ReadAll(file)
		uploaded = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"filename":"test.pdf","analysis":{"documentType":"factuur","amount":"12,34"},"text":"Volledige OCR tekst"}`))
	}))
	defer server.Close()
	repository := &translationRepository{document: store.Document{ID: 1, OriginalFilename: "test.pdf", MIMEType: "application/pdf", ObjectKey: strings.Repeat("a", 64)}}
	service := New(repository, translationObjects{"%PDF-1.7"}, server.URL, slog.New(slog.NewTextHandler(io.Discard, nil)))

	service.Process(1)

	if uploaded != "%PDF-1.7" {
		t.Fatalf("unexpected uploaded data %q", uploaded)
	}
	if repository.status != "completed" || repository.text != "Volledige OCR tekst" {
		t.Fatalf("unexpected stored analysis: status=%q text=%q", repository.status, repository.text)
	}
	var analysis map[string]string
	if err := json.Unmarshal([]byte(repository.analysisJSON), &analysis); err != nil {
		t.Fatalf("invalid analysis JSON: %v", err)
	}
	if analysis["documentType"] != "factuur" || analysis["amount"] != "12,34" {
		t.Fatalf("unexpected analysis: %+v", analysis)
	}
}

func TestProcessMarksNonPDFUnsupportedWithoutCallingPDFAI(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer server.Close()
	repository := &translationRepository{document: store.Document{ID: 1, MIMEType: "text/plain", ObjectKey: strings.Repeat("a", 64)}}
	New(repository, translationObjects{"not read"}, server.URL, slog.New(slog.NewTextHandler(io.Discard, nil))).Process(1)
	if called {
		t.Fatal("PDF-AI service was called for a non-PDF document")
	}
	if repository.status != "unsupported" || repository.text != "" || repository.analysisJSON != "" {
		t.Fatalf("unexpected unsupported result: %+v", repository)
	}
}

func TestProcessStoresPDFAIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "ocr failed", http.StatusBadGateway)
	}))
	defer server.Close()
	repository := &translationRepository{document: store.Document{ID: 1, OriginalFilename: "test.pdf", MIMEType: "application/pdf", ObjectKey: strings.Repeat("a", 64)}}
	New(repository, translationObjects{"%PDF-1.7"}, server.URL, slog.New(slog.NewTextHandler(io.Discard, nil))).Process(1)
	if repository.status != "failed" || !strings.Contains(repository.text, "ocr failed") {
		t.Fatalf("unexpected failed result: %+v", repository)
	}
}

type translationRepository struct {
	document     store.Document
	status       string
	analysisJSON string
	text         string
}

func (r *translationRepository) CreateDocument(context.Context, store.CreateDocument) (store.Document, error) {
	return store.Document{}, nil
}
func (r *translationRepository) Document(context.Context, int64) (store.Document, error) {
	return r.document, nil
}
func (r *translationRepository) Documents(context.Context) ([]store.Document, error)      { return nil, nil }
func (r *translationRepository) LinkDocument(context.Context, int64, string, int64) error { return nil }
func (r *translationRepository) DocumentsForObject(context.Context, string, int64) ([]store.Document, error) {
	return nil, nil
}
func (r *translationRepository) UnlinkDocument(context.Context, int64, string, int64) error {
	return nil
}
func (r *translationRepository) UpdateDocumentTranslation(_ context.Context, _ int64, status string, text string) error {
	r.status, r.text = status, text
	return nil
}
func (r *translationRepository) UpdateDocumentAnalysis(_ context.Context, _ int64, status string, analysisJSON string, text string) error {
	r.status, r.analysisJSON, r.text = status, analysisJSON, text
	return nil
}

type translationObjects struct{ text string }

func (o translationObjects) Ping(context.Context) error                        { return nil }
func (o translationObjects) Put(context.Context, string, []byte, string) error { return nil }
func (o translationObjects) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(o.text)), nil
}
func (o translationObjects) Delete(context.Context, string) error { return nil }
