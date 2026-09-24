package documenttranslation

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hohoho/internal/store"
)

func TestProcessStoresTranslationFromLoadedModel(t *testing.T) {
	requests := []string{}
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"local-model","status":{"value":"loaded"}}]}`))
		case "/v1/chat/completions":
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Hallo wereld"}}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer llm.Close()
	repository := &translationRepository{document: store.Document{ID: 1, MIMEType: "text/plain", ObjectKey: strings.Repeat("a", 64)}}
	service := New(repository, translationObjects{"Hello world"}, llm.URL, slog.New(slog.NewTextHandler(io.Discard, nil)))

	service.Process(1)

	if repository.status != "completed" || repository.text != "Hallo wereld" {
		t.Fatalf("unexpected stored translation: status=%q text=%q", repository.status, repository.text)
	}
	if strings.Join(requests, ",") != "/v1/models,/v1/chat/completions" {
		t.Fatalf("unexpected LLM requests: %v", requests)
	}
}

func TestProcessStoresSummaryForDutchText(t *testing.T) {
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			_, _ = w.Write([]byte(`{"data":[{"id":"local-model","status":{"value":"loaded"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Samenvatting in het Nederlands"}}]}`))
	}))
	defer llm.Close()
	repository := &translationRepository{document: store.Document{ID: 1, MIMEType: "text/plain", ObjectKey: strings.Repeat("a", 64)}}
	New(repository, translationObjects{"Dit is al Nederlands."}, llm.URL+"/v1", slog.New(slog.NewTextHandler(io.Discard, nil))).Process(1)
	if repository.status != "completed" || repository.text != "Samenvatting in het Nederlands" {
		t.Fatalf("Dutch text must be summarized: %+v", repository)
	}
}

func TestProcessMarksImagesUnsupportedWithoutCallingLLM(t *testing.T) {
	repository := &translationRepository{document: store.Document{ID: 1, MIMEType: "image/png", ObjectKey: strings.Repeat("a", 64)}}
	New(repository, translationObjects{"not read"}, "http://127.0.0.1:1", slog.New(slog.NewTextHandler(io.Discard, nil))).Process(1)
	if repository.status != "unsupported" || repository.text != "" {
		t.Fatalf("unexpected unsupported result: %+v", repository)
	}
}

type translationRepository struct {
	document store.Document
	status   string
	text     string
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

type translationObjects struct{ text string }

func (o translationObjects) Ping(context.Context) error                        { return nil }
func (o translationObjects) Put(context.Context, string, []byte, string) error { return nil }
func (o translationObjects) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(o.text)), nil
}
func (o translationObjects) Delete(context.Context, string) error { return nil }
