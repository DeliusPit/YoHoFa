package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"hohoho/internal/config"
	"hohoho/internal/httpserver"
	"hohoho/internal/store"
)

// pdfAIAnalyzer verbindt de configureerbare PDF-AI-service met de losse
// CoreDocumentAnalyzer-hook. Zonder URL of zonder S3 is er geen analyzer;
// uploaden en beheer werken dan gewoon door.
func pdfAIAnalyzer(cfg config.Runtime, objects *httpserver.CoreObjectStorage) httpserver.CoreDocumentAnalyzer {
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.PDFAIURL), "/")
	if endpoint == "" || objects == nil {
		return nil
	}
	return &pdfAIService{objects: objects, endpoint: endpoint, client: &http.Client{Timeout: 30 * time.Minute}}
}

type pdfAIService struct {
	objects  *httpserver.CoreObjectStorage
	endpoint string
	client   *http.Client
}

func (s *pdfAIService) AnalyzeDocument(ctx context.Context, document store.CoreDocument) (string, string, string, error) {
	if document.MIMEType != "application/pdf" {
		return "", "", "", fmt.Errorf("alleen PDF-documenten worden geanalyseerd")
	}
	object, err := s.objects.Get(ctx, document.ObjectKeys)
	if err != nil {
		return "", "", "", err
	}
	defer object.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", document.Filename)
	if err != nil {
		return "", "", "", err
	}
	if _, err := io.Copy(part, object); err != nil {
		return "", "", "", err
	}
	if err := writer.Close(); err != nil {
		return "", "", "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL("api/v1/document/analyse"), &body)
	if err != nil {
		return "", "", "", err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := s.client.Do(request)
	if err != nil {
		return "", "", "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		message := strings.TrimSpace(string(detail))
		if message == "" {
			message = response.Status
		}
		return "", "", "", fmt.Errorf("PDF-AI-service gaf %s: %s", response.Status, message)
	}
	var result struct {
		Analysis       json.RawMessage `json:"analysis"`
		Text           string          `json:"text"`
		TranslatedText string          `json:"translatedText"`
		TranslatedText2 string         `json:"translated_text"`
		Translation    string          `json:"translation"`
		Vertaling      string          `json:"vertaling"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 100<<20)).Decode(&result); err != nil {
		return "", "", "", err
	}
	if len(result.Analysis) == 0 || string(result.Analysis) == "null" {
		return "", "", "", fmt.Errorf("PDF-AI-service gaf geen analysis terug")
	}
	translatedText := firstNonEmpty(result.TranslatedText, result.TranslatedText2, result.Translation, result.Vertaling)
	return strings.TrimSpace(result.Text), translatedText, string(result.Analysis), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (s *pdfAIService) apiURL(resource string) string {
	base, err := url.Parse(s.endpoint)
	if err != nil {
		return s.endpoint + "/" + strings.TrimLeft(resource, "/")
	}
	joined, err := url.JoinPath(base.String(), resource)
	if err != nil {
		base.Path = path.Join(base.Path, resource)
		return base.String()
	}
	return joined
}
