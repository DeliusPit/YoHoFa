package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hohoho/internal/config"
	"hohoho/internal/store"
)

type coreConfigurationView struct {
	Runtime         coreRuntimeView        `json:"runtime"`
	Database        coreDatabaseView       `json:"database"`
	ObjectStorage   coreObjectStorageView  `json:"objectStorage"`
	BackupRecovery  coreBackupRecoveryView `json:"backupRecovery"`
	Editable        bool                   `json:"editable"`
	RestartRequired bool                   `json:"restartRequired"`
}

type coreRuntimeView struct {
	Environment             string   `json:"environment"`
	HTTPAddress             string   `json:"httpAddress"`
	WebDirectory            string   `json:"webDirectory"`
	AuthenticationEnabled   bool     `json:"authenticationEnabled"`
	CORSConfigured          bool     `json:"corsConfigured"`
	PDFAIConfigured         bool     `json:"pdfAiConfigured"`
	PDFAIURL                string   `json:"pdfAiUrl"`
	PDFAIModel              string   `json:"pdfAiModel"`
	TransactionAIConfigured bool     `json:"transactionAiConfigured"`
	TransactionAIURL        string   `json:"transactionAiUrl"`
	TransactionAIModel      string   `json:"transactionAiModel"`
	Language                string   `json:"language"`
	AvailableLanguages      []string `json:"availableLanguages"`
}

type translationsView struct {
	Language  string            `json:"language"`
	Available []string          `json:"available"`
	Messages  map[string]string `json:"messages"`
}

type coreDatabaseView struct {
	Backend            string   `json:"backend"`
	Path               string   `json:"path"`
	Directory          string   `json:"directory"`
	Filename           string   `json:"filename"`
	AvailableDatabases []string `json:"availableDatabases"`
	Status             string   `json:"status"`
	SchemaVersion      int      `json:"schemaVersion"`
	SizeBytes          int64    `json:"sizeBytes,omitempty"`
	Message            string   `json:"message,omitempty"`
}

type coreObjectStorageView struct {
	Backend       string `json:"backend"`
	Endpoint      string `json:"endpoint,omitempty"`
	Bucket        string `json:"bucket,omitempty"`
	Region        string `json:"region,omitempty"`
	TLS           bool   `json:"tls"`
	PathStyle     bool   `json:"pathStyle"`
	CreateBucket  bool   `json:"createBucket"`
	Configured    bool   `json:"configured"`
	Ready         bool   `json:"ready"`
	AccessKeySet  bool   `json:"accessKeySet"`
	SecretKeySet  bool   `json:"secretKeySet"`
	StatusMessage string `json:"statusMessage,omitempty"`
}

type coreBackupRecoveryView struct {
	Available bool   `json:"available"`
	Backend   string `json:"backend"`
	Message   string `json:"message"`
}

type transactionAIModelsView struct {
	Models []string `json:"models"`
}

type aiDocsConfigView struct {
	Path    string   `json:"path"`
	RawYAML string   `json:"rawYaml"`
	Status  yamlView `json:"status"`
}

type yamlView struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

type saveAIDocsConfigInput struct {
	RawYAML string `json:"rawYaml"`
}

func registerCoreConfigurationRoutes(mux *http.ServeMux, deps CoreDependencies) {
	mux.HandleFunc("GET /api/v1/system/configuration", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		displayConfig, saved := config.SavedRuntime(deps.Config.ConfigPath, deps.Config)
		languages := availableTranslationLanguages(deps.Config.TranslationDir)
		language := displayConfig.Language
		if !containsString(languages, language) && len(languages) > 0 {
			language = languages[0]
		}
		databaseDirectory := filepath.Dir(displayConfig.DatabasePath)
		databaseFilename := filepath.Base(displayConfig.DatabasePath)
		view := coreConfigurationView{
			Editable:        true,
			RestartRequired: saved && runtimeSettingsDiffer(deps.Config, displayConfig),
			Runtime: coreRuntimeView{
				Environment: displayConfig.AppEnv, HTTPAddress: displayConfig.HTTPAddr, WebDirectory: displayConfig.WebDir,
				AuthenticationEnabled:   displayConfig.AuthUsername != "" && displayConfig.AuthPassword != "",
				CORSConfigured:          displayConfig.CORSOrigin != "",
				PDFAIConfigured:         displayConfig.PDFAIURL != "",
				PDFAIURL:                displayConfig.PDFAIURL,
				PDFAIModel:              displayConfig.PDFAIModel,
				TransactionAIConfigured: displayConfig.TransactionAIURL != "",
				TransactionAIURL:        displayConfig.TransactionAIURL,
				TransactionAIModel:      displayConfig.TransactionAIModel,
				Language:                language,
				AvailableLanguages:      languages,
			},
			Database: coreDatabaseView{Backend: "sqlite", Path: displayConfig.DatabasePath, Directory: databaseDirectory, Filename: databaseFilename, AvailableDatabases: availableDatabaseFiles(databaseDirectory, databaseFilename), Status: "ok", SchemaVersion: store.CoreSchemaVersion},
			ObjectStorage: coreObjectStorageView{
				Backend: "garage/s3", Endpoint: displayConfig.S3Endpoint, Bucket: displayConfig.S3Bucket, Region: displayConfig.S3Region,
				TLS: displayConfig.S3UseTLS, PathStyle: displayConfig.S3PathStyle, CreateBucket: displayConfig.S3CreateBucket,
				Configured:   displayConfig.S3Endpoint != "" && displayConfig.S3AccessKey != "" && displayConfig.S3SecretKey != "" && displayConfig.S3Bucket != "",
				AccessKeySet: displayConfig.S3AccessKey != "", SecretKeySet: displayConfig.S3SecretKey != "",
			},
			BackupRecovery: coreBackupRecoveryView{Available: deps.SystemBackup != nil, Backend: "restic/rclone", Message: "Configureer een restic repository, rclone config en restic wachtwoord. Backups bevatten SQLite, Garage/S3-objecten en configuratie."},
		}
		if stat, err := os.Stat(deps.Config.DatabasePath); err == nil {
			view.Database.SizeBytes = stat.Size()
		}
		if err := deps.Store.Ping(ctx); err != nil {
			view.Database.Status = "error"
			view.Database.Message = "SQLite is niet bereikbaar"
		}
		if err := deps.Objects.Ping(ctx); err != nil {
			if view.ObjectStorage.Configured {
				view.ObjectStorage.StatusMessage = "S3/Garage is geconfigureerd maar niet bereikbaar"
			} else {
				view.ObjectStorage.StatusMessage = "S3/Garage is nog niet geconfigureerd"
			}
		} else {
			view.ObjectStorage.Ready = true
			view.ObjectStorage.StatusMessage = "S3/Garage is bereikbaar"
		}
		writeJSON(w, http.StatusOK, view)
	})
	mux.HandleFunc("GET /api/v1/system/translations", func(w http.ResponseWriter, r *http.Request) {
		displayConfig, _ := config.SavedRuntime(deps.Config.ConfigPath, deps.Config)
		languages := availableTranslationLanguages(deps.Config.TranslationDir)
		language := strings.TrimSpace(r.URL.Query().Get("language"))
		if language == "" {
			language = displayConfig.Language
		}
		if !containsString(languages, language) {
			if len(languages) == 0 {
				writeJSON(w, http.StatusOK, translationsView{Language: "", Available: []string{}, Messages: map[string]string{}})
				return
			}
			language = languages[0]
		}
		messages, err := readTranslationMessages(deps.Config.TranslationDir, language)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Vertaling kon niet worden geladen"})
			return
		}
		writeJSON(w, http.StatusOK, translationsView{Language: language, Available: languages, Messages: messages})
	})
	mux.HandleFunc("GET /api/v1/system/ai-docs-config", func(w http.ResponseWriter, r *http.Request) {
		view, err := readAIDocsConfig(deps.Config.AIDocsConfigPath)
		if err != nil {
			deps.Logger.Error("Document-AI configuratie laden mislukt", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Document-AI configuratie kon niet worden geladen"})
			return
		}
		writeJSON(w, http.StatusOK, view)
	})
	mux.HandleFunc("PUT /api/v1/system/ai-docs-config", func(w http.ResponseWriter, r *http.Request) {
		var input saveAIDocsConfigInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Ongeldige JSON-invoer"})
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Verwacht één JSON-object"})
			return
		}
		status := validateAIDocsYAML(input.RawYAML)
		if !status.Valid {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Document-AI configuratie is ongeldig", "status": status})
			return
		}
		if err := writeAIDocsConfig(deps.Config.AIDocsConfigPath, input.RawYAML); err != nil {
			deps.Logger.Error("Document-AI configuratie opslaan mislukt", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Document-AI configuratie kon niet worden opgeslagen"})
			return
		}
		view, err := readAIDocsConfig(deps.Config.AIDocsConfigPath)
		if err != nil {
			writeJSON(w, http.StatusOK, aiDocsConfigView{Path: deps.Config.AIDocsConfigPath, RawYAML: strings.TrimSpace(input.RawYAML) + "\n", Status: status})
			return
		}
		writeJSON(w, http.StatusOK, view)
	})
	mux.HandleFunc("GET /api/v1/system/transaction-ai-models", func(w http.ResponseWriter, r *http.Request) {
		inputURL := strings.TrimSpace(r.URL.Query().Get("url"))
		if inputURL == "" {
			displayConfig, _ := config.SavedRuntime(deps.Config.ConfigPath, deps.Config)
			inputURL = displayConfig.TransactionAIURL
		}
		modelsURL, err := transactionAIModelsURL(inputURL)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Transactie-AI URL is ongeldig"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Transactie-AI URL is ongeldig"})
			return
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Transactie-AI modellen konden niet worden opgehaald"})
			return
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode > 299 {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Transactie-AI modellen konden niet worden opgehaald"})
			return
		}
		var payload struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Transactie-AI modellenantwoord is ongeldig"})
			return
		}
		models := make([]string, 0, len(payload.Data))
		seen := map[string]bool{}
		for _, item := range payload.Data {
			id := strings.TrimSpace(item.ID)
			if id != "" && !seen[id] {
				seen[id] = true
				models = append(models, id)
			}
		}
		sort.Strings(models)
		writeJSON(w, http.StatusOK, transactionAIModelsView{Models: models})
	})
	mux.HandleFunc("PUT /api/v1/system/configuration", func(w http.ResponseWriter, r *http.Request) {
		var input config.RuntimeUpdate
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		baseConfig, _ := config.SavedRuntime(deps.Config.ConfigPath, deps.Config)
		currentDatabasePath := baseConfig.DatabasePath
		if currentDatabasePath == "" {
			currentDatabasePath = deps.Config.DatabasePath
		}
		if input.DatabasePath == "" {
			input.DatabasePath = currentDatabasePath
		}
		if input.DatabasePath != "" && !validDatabasePath(currentDatabasePath, input.DatabasePath) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Database moet in dezelfde datamap staan en op .db eindigen"})
			return
		}
		if input.S3Bucket == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "S3 bucket is verplicht"})
			return
		}
		if input.S3Region == "" {
			input.S3Region = "garage"
		}
		languages := availableTranslationLanguages(deps.Config.TranslationDir)
		if input.Language == "" && len(languages) > 0 {
			input.Language = languages[0]
		}
		if input.Language != "" && !containsString(languages, input.Language) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Taal is niet beschikbaar"})
			return
		}
		if err := config.SaveRuntimeUpdate(deps.Config.ConfigPath, baseConfig, input); err != nil {
			deps.Logger.Error("Configuratie opslaan mislukt", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Configuratie kon niet worden opgeslagen"})
			return
		}
		accessKeySet := baseConfig.S3AccessKey != ""
		secretKeySet := baseConfig.S3SecretKey != ""
		if input.S3AccessKey != nil {
			accessKeySet = *input.S3AccessKey != ""
		}
		if input.S3SecretKey != nil {
			secretKeySet = *input.S3SecretKey != ""
		}
		savedConfig, _ := config.SavedRuntime(deps.Config.ConfigPath, deps.Config)
		statusMessage := "Wijzigingen opgeslagen."
		ready := false
		if !accessKeySet || !secretKeySet || input.S3Endpoint == "" {
			deps.Objects.Swap(nil)
			statusMessage = "S3/Garage is onvolledig ingevuld; documentopslag staat uit."
		} else if client, err := OpenCoreObjectStore(r.Context(), savedConfig); err != nil {
			deps.Logger.Error("S3 heropenen na configuratiewijziging mislukt", "error", err)
			deps.Objects.Swap(nil)
			statusMessage = "Configuratie opgeslagen, maar S3/Garage is niet bereikbaar."
		} else {
			deps.Objects.Swap(client)
			ready = true
			statusMessage = "Configuratie opgeslagen en S3/Garage geopend."
		}
		databaseDirectory := filepath.Dir(input.DatabasePath)
		databaseFilename := filepath.Base(input.DatabasePath)
		view := coreConfigurationView{Editable: true, RestartRequired: input.DatabasePath != deps.Config.DatabasePath, Runtime: coreRuntimeView{Environment: deps.Config.AppEnv, HTTPAddress: deps.Config.HTTPAddr, WebDirectory: deps.Config.WebDir, AuthenticationEnabled: deps.Config.AuthUsername != "" && deps.Config.AuthPassword != "", CORSConfigured: deps.Config.CORSOrigin != "", PDFAIConfigured: input.PDFAIURL != "", PDFAIURL: input.PDFAIURL, PDFAIModel: input.PDFAIModel, TransactionAIConfigured: input.TransactionAIURL != "", TransactionAIURL: input.TransactionAIURL, TransactionAIModel: input.TransactionAIModel, Language: input.Language, AvailableLanguages: languages}, Database: coreDatabaseView{Backend: "sqlite", Path: input.DatabasePath, Directory: databaseDirectory, Filename: databaseFilename, AvailableDatabases: availableDatabaseFiles(databaseDirectory, databaseFilename), Status: "ok", SchemaVersion: store.CoreSchemaVersion}, ObjectStorage: coreObjectStorageView{Backend: "garage/s3", Endpoint: input.S3Endpoint, Bucket: input.S3Bucket, Region: input.S3Region, TLS: input.S3UseTLS, PathStyle: input.S3PathStyle, CreateBucket: input.S3CreateBucket, Configured: input.S3Endpoint != "" && accessKeySet && secretKeySet && input.S3Bucket != "", Ready: ready, AccessKeySet: accessKeySet, SecretKeySet: secretKeySet, StatusMessage: statusMessage}, BackupRecovery: coreBackupRecoveryView{Available: deps.SystemBackup != nil, Backend: "restic/rclone", Message: "Backupconfiguratie blijft beschikbaar. Herstart na S3-wijzigingen voordat je een backup valideert."}}
		writeJSON(w, http.StatusOK, view)
	})
}

func availableTranslationLanguages(dir string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	languages := make([]string, 0, len(matches))
	for _, match := range matches {
		language := strings.TrimSuffix(filepath.Base(match), ".json")
		if language != "" && !strings.ContainsAny(language, `/\`) {
			languages = append(languages, language)
		}
	}
	sort.Strings(languages)
	return languages
}

func readTranslationMessages(dir string, language string) (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(dir, language+".json"))
	if err != nil {
		return nil, err
	}
	messages := map[string]string{}
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func readAIDocsConfig(path string) (aiDocsConfigView, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		raw := defaultAIDocsYAML()
		return aiDocsConfigView{Path: path, RawYAML: raw, Status: validateAIDocsYAML(raw)}, nil
	}
	if err != nil {
		return aiDocsConfigView{}, err
	}
	raw := string(data)
	return aiDocsConfigView{Path: path, RawYAML: raw, Status: validateAIDocsYAML(raw)}, nil
}

func availableDatabaseFiles(dir string, current string) []string {
	seen := map[string]bool{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name != "" && strings.HasSuffix(name, ".db") && !strings.ContainsAny(name, `/\`) {
			seen[name] = true
		}
	}
	add("hohoho.db")
	add("example.db")
	add(current)
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				add(entry.Name())
			}
		}
	}
	items := make([]string, 0, len(seen))
	for item := range seen {
		items = append(items, item)
	}
	sort.Strings(items)
	return items
}

func validDatabasePath(currentPath string, nextPath string) bool {
	if filepath.Dir(currentPath) != filepath.Dir(nextPath) {
		return false
	}
	name := filepath.Base(nextPath)
	return name != "" && strings.HasSuffix(name, ".db") && !strings.ContainsAny(name, `/\`)
}

func writeAIDocsConfig(path string, raw string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".ai-docs-*.yaml")
	if err != nil {
		return err
	}
	name := temporary.Name()
	if _, err := temporary.WriteString(strings.TrimSpace(raw) + "\n"); err != nil {
		temporary.Close()
		os.Remove(name)
		return err
	}
	if err := temporary.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

func validateAIDocsYAML(raw string) yamlView {
	trimmed := strings.TrimSpace(raw)
	status := yamlView{Valid: true}
	if trimmed == "" {
		status.Valid = false
		status.Errors = append(status.Errors, "Configuratie is leeg")
		return status
	}
	for _, section := range []string{"processing:", "translation:", "analysis:"} {
		if !containsYAMLLine(trimmed, section) {
			status.Valid = false
			status.Errors = append(status.Errors, "Sectie ontbreekt: "+strings.TrimSuffix(section, ":"))
		}
	}
	for _, field := range []string{"prompt:"} {
		if !strings.Contains(trimmed, field) {
			status.Valid = false
			status.Errors = append(status.Errors, "Veld ontbreekt: "+strings.TrimSuffix(field, ":"))
		}
	}
	return status
}

func containsYAMLLine(raw string, needle string) bool {
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == needle {
			return true
		}
	}
	return false
}

func defaultAIDocsYAML() string {
	return `processing:
  max_text_chars: 50000
  min_text_chars: 80
  min_alpha_chars: 30
  command_timeout: 300
  ocr_languages: "nld+deu+eng"

translation:
  target_language: "Nederlands"
  prompt: |
    Vertaal het volledige onderstaande administratieve document nauwkeurig
    naar het Nederlands.

    Regels:
    - behoud namen exact;
    - behoud adressen exact;
    - behoud bedragen exact;
    - behoud datums exact;
    - behoud rekeningnummers exact;
    - behoud contract-, factuur-, polis- en referentienummers exact;
    - verzin niets;
    - corrigeer geen onzekere OCR-resultaten door te gokken;
    - markeer onleesbare of onzekere tekst als [onleesbaar];
    - behoud waar mogelijk de structuur van het document;
    - geef uitsluitend de Nederlandse vertaling terug.

analysis:
  prompt: |
    Analyseer het onderstaande administratieve document voor het
    huishoudelijk administratiesysteem YoHoFa.

    Gebruik uitsluitend informatie die daadwerkelijk uit het document blijkt.
    Verzin ontbrekende waarden niet.
    Gebruik alleen bestaande financiële objecten wanneer die door YoHoFa worden aangeboden.
    Geef uitsluitend geldige JSON terug.
`
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func transactionAIModelsURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("invalid transaction AI URL")
	}
	path := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(path, "/chat/completions") {
		path = strings.TrimSuffix(path, "/chat/completions") + "/models"
	} else if strings.HasSuffix(path, "/completions") {
		path = strings.TrimSuffix(path, "/completions") + "/models"
	} else if strings.HasSuffix(path, "/models") {
		path = strings.TrimRight(path, "/")
	} else {
		path = path + "/models"
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func runtimeSettingsDiffer(left, right config.Runtime) bool {
	return left.S3Endpoint != right.S3Endpoint || left.S3AccessKey != right.S3AccessKey || left.S3SecretKey != right.S3SecretKey ||
		left.S3Bucket != right.S3Bucket || left.S3UseTLS != right.S3UseTLS || left.S3Region != right.S3Region ||
		left.S3PathStyle != right.S3PathStyle || left.S3CreateBucket != right.S3CreateBucket ||
		left.PDFAIURL != right.PDFAIURL || left.PDFAIModel != right.PDFAIModel || left.TransactionAIURL != right.TransactionAIURL || left.TransactionAIModel != right.TransactionAIModel
}
