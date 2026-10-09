package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"hohoho/internal/config"
	"hohoho/internal/httpserver"
	"hohoho/internal/store"
)

const defaultDocumentAIModel = "Qwen3.8-Flash-Next-UD-Q4_K_XL"

func documentAIAnalyzer(cfg config.Runtime, objects *httpserver.CoreObjectStorage, coreStore *store.Core) httpserver.CoreDocumentAnalyzer {
	if objects == nil {
		return nil
	}
	return &documentAIService{config: cfg, objects: objects, store: coreStore, client: &http.Client{Timeout: 30 * time.Minute}}
}

type documentAIService struct {
	config  config.Runtime
	objects *httpserver.CoreObjectStorage
	store   *store.Core
	client  *http.Client
}

type documentAIConfig struct {
	MaxTextChars    int
	MinTextChars    int
	MinAlphaChars   int
	CommandTimeout  time.Duration
	OCRLanguages    string
	TranslateTo     string
	TranslatePrompt string
	AnalysisPrompt  string
}

func (s *documentAIService) AnalyzeDocument(ctx context.Context, document store.CoreDocument) (httpserver.CoreDocumentAnalysisResult, error) {
	runtimeConfig, _ := config.SavedRuntime(s.config.ConfigPath, s.config)
	endpoint, err := documentAIChatURL(runtimeConfig.PDFAIURL)
	if err != nil {
		return httpserver.CoreDocumentAnalysisResult{}, fmt.Errorf("Document-AI URL is niet geconfigureerd")
	}
	model := strings.TrimSpace(runtimeConfig.PDFAIModel)
	if model == "" {
		model = defaultDocumentAIModel
	}
	aiConfig := readDocumentAIConfig(runtimeConfig.AIDocsConfigPath)

	object, err := s.objects.Get(ctx, document.ObjectKeys)
	if err != nil {
		return httpserver.CoreDocumentAnalysisResult{}, err
	}
	defer object.Close()
	data, err := io.ReadAll(io.LimitReader(object, maxCoreDocumentAnalysisBytes+1))
	if err != nil {
		return httpserver.CoreDocumentAnalysisResult{}, err
	}
	if len(data) > maxCoreDocumentAnalysisBytes {
		return httpserver.CoreDocumentAnalysisResult{}, fmt.Errorf("document is te groot voor analyse")
	}

	extracted, err := extractDocumentText(ctx, document, data, aiConfig)
	if err != nil {
		return httpserver.CoreDocumentAnalysisResult{}, err
	}
	extracted = trimToRunes(strings.TrimSpace(extracted), aiConfig.MaxTextChars)
	if !enoughText(extracted, aiConfig) {
		return httpserver.CoreDocumentAnalysisResult{}, fmt.Errorf("te weinig tekst gevonden voor documentanalyse")
	}

	translated, err := s.completeText(ctx, endpoint, model, aiConfig.TranslatePrompt, extracted, false)
	if err != nil {
		return httpserver.CoreDocumentAnalysisResult{ExtractedText: extracted}, err
	}
	analysisPrompt, validObjectIDs, err := s.analysisPrompt(ctx, aiConfig.AnalysisPrompt)
	if err != nil {
		return httpserver.CoreDocumentAnalysisResult{ExtractedText: extracted, TranslatedText: translated}, err
	}
	analysis, err := s.completeText(ctx, endpoint, model, analysisPrompt, translated, true)
	if err != nil {
		return httpserver.CoreDocumentAnalysisResult{ExtractedText: extracted, TranslatedText: translated}, err
	}
	jsonText := extractJSONText(analysis)
	if jsonText == "" || !json.Valid([]byte(jsonText)) {
		return httpserver.CoreDocumentAnalysisResult{ExtractedText: extracted, TranslatedText: translated}, fmt.Errorf("Document-AI gaf geen geldige JSON terug")
	}
	return httpserver.CoreDocumentAnalysisResult{ExtractedText: extracted, TranslatedText: strings.TrimSpace(translated), AnalysisJSON: jsonText, FinancialObjectIDs: financialObjectIDsFromAnalysis(jsonText, validObjectIDs)}, nil
}

const maxCoreDocumentAnalysisBytes = 50 << 20

func extractDocumentText(ctx context.Context, document store.CoreDocument, data []byte, cfg documentAIConfig) (string, error) {
	dir, err := os.MkdirTemp("", "hohoho-doc-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	ext := strings.ToLower(filepath.Ext(document.Filename))
	if ext == "" {
		ext = extensionForMime(document.MIMEType)
	}
	inputPath := filepath.Join(dir, "input"+ext)
	if err := os.WriteFile(inputPath, data, 0o600); err != nil {
		return "", err
	}
	switch {
	case document.MIMEType == "application/pdf" || ext == ".pdf":
		text, _ := runCommand(ctx, cfg.CommandTimeout, "pdftotext", "-layout", inputPath, "-")
		if enoughText(text, cfg) {
			return text, nil
		}
		sidecar := filepath.Join(dir, "ocr.txt")
		outputPDF := filepath.Join(dir, "ocr.pdf")
		if _, err := runCommand(ctx, cfg.CommandTimeout, "ocrmypdf", "--sidecar", sidecar, "-l", cfg.OCRLanguages, "--skip-text", inputPath, outputPDF); err != nil {
			return text, fmt.Errorf("OCR op PDF mislukt: %w", err)
		}
		ocrText, err := os.ReadFile(sidecar)
		if err != nil {
			return text, err
		}
		return string(ocrText), nil
	case strings.HasPrefix(document.MIMEType, "image/"):
		return runCommand(ctx, cfg.CommandTimeout, "tesseract", inputPath, "stdout", "-l", cfg.OCRLanguages)
	case strings.HasPrefix(document.MIMEType, "text/"):
		return string(data), nil
	default:
		return "", fmt.Errorf("bestandstype %q wordt nog niet ondersteund voor documentanalyse", document.MIMEType)
	}
}

func runCommand(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(commandCtx.Err(), context.DeadlineExceeded) {
			return stdout.String(), fmt.Errorf("%s duurde langer dan %s", name, timeout)
		}
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return stdout.String(), fmt.Errorf("%s: %s", err, detail)
		}
		return stdout.String(), err
	}
	return stdout.String(), nil
}

func (s *documentAIService) completeText(ctx context.Context, endpoint string, model string, prompt string, text string, jsonOutput bool) (string, error) {
	requestBody := chatCompletionRequest{
		Model: model, Temperature: 0, MaxTokens: 4096, Stream: false,
		ChatTemplateKwargs: chatTemplateKwargs{EnableThinking: false},
		Messages:           []chatMessage{{Role: "system", Content: prompt}, {Role: "user", Content: text}},
	}
	if jsonOutput {
		requestBody.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("Document-AI is niet bereikbaar: %s", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return "", fmt.Errorf("Document-AI gaf status %d: %s", response.StatusCode, strings.TrimSpace(string(detail)))
	}
	var output chatCompletionResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&output); err != nil || len(output.Choices) == 0 {
		return "", fmt.Errorf("Document-AI antwoord is ongeldig")
	}
	return strings.TrimSpace(output.Choices[0].Message.Content), nil
}

func (s *documentAIService) analysisPrompt(ctx context.Context, base string) (string, map[int64]bool, error) {
	if s.store == nil {
		return base, nil, nil
	}
	objects, err := s.store.FinancialObjects(ctx, "")
	if err != nil {
		return "", nil, err
	}
	valid := map[int64]bool{}
	var builder strings.Builder
	builder.WriteString(base)
	builder.WriteString("\n\nBeschikbare financiële objecten. Gebruik uitsluitend deze IDs als het document bij objecten hoort:\n")
	builder.WriteString("[\n")
	for i, object := range objects {
		valid[object.ID] = true
		if i > 0 {
			builder.WriteString(",\n")
		}
		line, _ := json.Marshal(map[string]any{"id": object.ID, "type": object.ObjectType, "naam": object.Name, "omschrijving": object.Description})
		builder.Write(line)
	}
	builder.WriteString("\n]\n\n")
	builder.WriteString("Neem in de JSON altijd financialObjectIds op als array met alleen passende bestaande object-ID's. Gebruik een lege array als geen object past.")
	return builder.String(), valid, nil
}

func financialObjectIDsFromAnalysis(raw string, valid map[int64]bool) []int64 {
	var value map[string]any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return nil
	}
	seen := map[int64]bool{}
	var ids []int64
	for _, key := range []string{"financialObjectIds", "financial_object_ids", "financialObjectIDs", "objectIds", "object_ids"} {
		addDocumentObjectIDValue(value[key], valid, seen, &ids)
	}
	for _, key := range []string{"financialObjectId", "financial_object_id", "objectId", "object_id"} {
		addDocumentObjectIDValue(value[key], valid, seen, &ids)
	}
	for _, key := range []string{"financialObjects", "financial_objects", "objects", "objecten"} {
		if list, ok := value[key].([]any); ok {
			for _, item := range list {
				if object, ok := item.(map[string]any); ok {
					for _, idKey := range []string{"financialObjectId", "financial_object_id", "objectId", "object_id", "id"} {
						addDocumentObjectIDValue(object[idKey], valid, seen, &ids)
					}
				}
			}
		}
	}
	return ids
}

func addDocumentObjectIDValue(value any, valid map[int64]bool, seen map[int64]bool, ids *[]int64) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			addDocumentObjectIDValue(item, valid, seen, ids)
		}
	case float64:
		addFinancialObjectID(int64(typed), valid, seen, ids)
	case string:
		var id int64
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &id); err == nil {
			addFinancialObjectID(id, valid, seen, ids)
		}
	}
}

func addFinancialObjectID(id int64, valid map[int64]bool, seen map[int64]bool, ids *[]int64) {
	if id <= 0 || seen[id] || (valid != nil && !valid[id]) {
		return
	}
	seen[id] = true
	*ids = append(*ids, id)
}

func documentAIChatURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("invalid document AI URL")
	}
	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case path == "":
		path = "/v1/chat/completions"
	case strings.HasSuffix(path, "/chat/completions"):
	case strings.HasSuffix(path, "/models"):
		path = strings.TrimSuffix(path, "/models") + "/chat/completions"
	case strings.HasSuffix(path, "/v1"):
		path += "/chat/completions"
	default:
		path += "/v1/chat/completions"
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func readDocumentAIConfig(path string) documentAIConfig {
	cfg := documentAIConfig{MaxTextChars: 50000, MinTextChars: 80, MinAlphaChars: 30, CommandTimeout: 300 * time.Second, OCRLanguages: "nld+deu+eng", TranslateTo: "Nederlands", TranslatePrompt: "Vertaal het document naar het Nederlands. Geef uitsluitend de vertaling terug.", AnalysisPrompt: "Analyseer het document en geef uitsluitend geldige JSON terug."}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	section := ""
	blockKey := ""
	var block []string
	flush := func() {
		value := strings.TrimRight(strings.Join(block, "\n"), "\n")
		if blockKey == "translation.prompt" && strings.TrimSpace(value) != "" {
			cfg.TranslatePrompt = value
		} else if blockKey == "analysis.prompt" && strings.TrimSpace(value) != "" {
			cfg.AnalysisPrompt = value
		}
		blockKey = ""
		block = nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		if blockKey != "" {
			if strings.HasPrefix(line, "    ") || strings.TrimSpace(line) == "" {
				block = append(block, strings.TrimPrefix(line, "    "))
				continue
			}
			flush()
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && strings.HasSuffix(trimmed, ":") {
			section = strings.TrimSuffix(trimmed, ":")
			continue
		}
		if strings.HasPrefix(line, "  ") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), `"`)
			if value == "|" {
				blockKey = section + "." + key
				continue
			}
			applyDocumentAIConfigValue(&cfg, section, key, value)
		}
	}
	if blockKey != "" {
		flush()
	}
	return cfg
}

func applyDocumentAIConfigValue(cfg *documentAIConfig, section string, key string, value string) {
	switch section + "." + key {
	case "processing.max_text_chars":
		fmt.Sscanf(value, "%d", &cfg.MaxTextChars)
	case "processing.min_text_chars":
		fmt.Sscanf(value, "%d", &cfg.MinTextChars)
	case "processing.min_alpha_chars":
		fmt.Sscanf(value, "%d", &cfg.MinAlphaChars)
	case "processing.command_timeout":
		var seconds int
		if _, err := fmt.Sscanf(value, "%d", &seconds); err == nil && seconds > 0 {
			cfg.CommandTimeout = time.Duration(seconds) * time.Second
		}
	case "processing.ocr_languages":
		if value != "" {
			cfg.OCRLanguages = value
		}
	case "translation.target_language":
		if value != "" {
			cfg.TranslateTo = value
		}
	}
}

func enoughText(text string, cfg documentAIConfig) bool {
	if len([]rune(strings.TrimSpace(text))) < cfg.MinTextChars {
		return false
	}
	alpha := 0
	for _, char := range text {
		if unicode.IsLetter(char) {
			alpha++
		}
	}
	return alpha >= cfg.MinAlphaChars
}

func trimToRunes(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func extensionForMime(mimeType string) string {
	switch mimeType {
	case "application/pdf":
		return ".pdf"
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/tiff":
		return ".tif"
	case "image/webp":
		return ".webp"
	default:
		return ".bin"
	}
}

type chatCompletionRequest struct {
	Model              string             `json:"model"`
	Temperature        float64            `json:"temperature"`
	MaxTokens          int                `json:"max_tokens"`
	Stream             bool               `json:"stream"`
	ChatTemplateKwargs chatTemplateKwargs `json:"chat_template_kwargs,omitempty"`
	ResponseFormat     *responseFormat    `json:"response_format,omitempty"`
	Messages           []chatMessage      `json:"messages"`
}

type chatTemplateKwargs struct {
	EnableThinking bool `json:"enable_thinking"`
}

type responseFormat struct {
	Type string `json:"type"`
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

func extractJSONText(content string) string {
	text := strings.TrimSpace(content)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimPrefix(strings.TrimSpace(text), "json")
		text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "```"))
	}
	start := strings.IndexAny(text, "{[")
	if start < 0 {
		return ""
	}
	end := strings.LastIndexAny(text, "}]")
	if end < start {
		return ""
	}
	return strings.TrimSpace(text[start : end+1])
}
