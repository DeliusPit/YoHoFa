package documenttranslation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"hohoho/internal/objectstore"
	"hohoho/internal/store"
)

type Service struct {
	documents store.DocumentRepository
	objects   objectstore.Store
	endpoint  string
	client    *http.Client
	logger    *slog.Logger
	worker    chan struct{}
}

func New(documents store.DocumentRepository, objects objectstore.Store, endpoint string, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{documents: documents, objects: objects, endpoint: strings.TrimRight(strings.TrimSpace(endpoint), "/"), client: &http.Client{Timeout: 30 * time.Minute}, logger: logger, worker: make(chan struct{}, 1)}
}

func (s *Service) Enabled() bool {
	return s != nil && s.documents != nil && s.objects != nil && s.endpoint != ""
}

// Start marks the document before returning and performs the potentially slow work separately.
func (s *Service) Start(documentID int64) {
	if !s.Enabled() {
		return
	}
	if err := s.documents.UpdateDocumentAnalysis(context.Background(), documentID, "pending", "", ""); err != nil {
		s.logger.Error("mark document translation pending", "document_id", documentID, "error", err)
		return
	}
	go s.Process(documentID)
}

func (s *Service) Process(documentID int64) {
	s.worker <- struct{}{}
	defer func() { <-s.worker }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	document, err := s.documents.Document(ctx, documentID)
	if err != nil {
		s.logger.Error("load document for translation", "document_id", documentID, "error", err)
		return
	}
	if document.MIMEType != "application/pdf" {
		s.finish(ctx, documentID, "unsupported", "", "", nil)
		return
	}
	analysisJSON, text, err := s.analyzePDF(ctx, document)
	if err != nil {
		s.finish(ctx, documentID, "failed", "", err.Error(), err)
		return
	}
	s.finish(ctx, documentID, "completed", analysisJSON, text, nil)
}

func (s *Service) finish(ctx context.Context, documentID int64, status string, analysisJSON string, text string, cause error) {
	if cause != nil {
		s.logger.Error("document analysis failed", "document_id", documentID, "error", cause)
	}
	if err := s.documents.UpdateDocumentAnalysis(ctx, documentID, status, analysisJSON, text); err != nil {
		s.logger.Error("store document analysis", "document_id", documentID, "error", err)
	}
}

func (s *Service) analyzePDF(ctx context.Context, document store.Document) (string, string, error) {
	object, err := s.objects.Get(ctx, document.ObjectKey)
	if err != nil {
		return "", "", err
	}
	defer object.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", document.OriginalFilename)
	if err != nil {
		return "", "", err
	}
	if _, err := io.Copy(part, object); err != nil {
		return "", "", err
	}
	if err := writer.Close(); err != nil {
		return "", "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL("api/v1/document/analyse"), &body)
	if err != nil {
		return "", "", err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := s.client.Do(request)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		message := strings.TrimSpace(string(detail))
		if message == "" {
			message = response.Status
		}
		return "", "", fmt.Errorf("PDF-AI-service gaf %s: %s", response.Status, message)
	}
	var result struct {
		Filename string          `json:"filename"`
		Analysis json.RawMessage `json:"analysis"`
		Text     string          `json:"text"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 100<<20)).Decode(&result); err != nil {
		return "", "", err
	}
	if len(result.Analysis) == 0 || string(result.Analysis) == "null" {
		return "", "", fmt.Errorf("PDF-AI-service gaf geen analysis terug")
	}
	return string(result.Analysis), strings.TrimSpace(result.Text), nil
}

func (s *Service) apiURL(resource string) string {
	base, err := url.Parse(s.endpoint)
	if err != nil {
		return strings.TrimRight(s.endpoint, "/") + "/" + strings.TrimLeft(resource, "/")
	}
	joined, err := url.JoinPath(base.String(), resource)
	if err != nil {
		base.Path = path.Join(base.Path, resource)
		return base.String()
	}
	return joined
}
