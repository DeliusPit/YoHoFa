package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
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
	Environment             string `json:"environment"`
	HTTPAddress             string `json:"httpAddress"`
	WebDirectory            string `json:"webDirectory"`
	AuthenticationEnabled   bool   `json:"authenticationEnabled"`
	CORSConfigured          bool   `json:"corsConfigured"`
	PDFAIConfigured         bool   `json:"pdfAiConfigured"`
	PDFAIURL                string `json:"pdfAiUrl"`
	TransactionAIConfigured bool   `json:"transactionAiConfigured"`
	TransactionAIURL        string `json:"transactionAiUrl"`
	TransactionAIModel      string `json:"transactionAiModel"`
}

type coreDatabaseView struct {
	Backend       string `json:"backend"`
	Path          string `json:"path"`
	Status        string `json:"status"`
	SchemaVersion int    `json:"schemaVersion"`
	SizeBytes     int64  `json:"sizeBytes,omitempty"`
	Message       string `json:"message,omitempty"`
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

func registerCoreConfigurationRoutes(mux *http.ServeMux, deps CoreDependencies) {
	mux.HandleFunc("GET /api/v1/system/configuration", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		displayConfig, saved := config.SavedRuntime(deps.Config.ConfigPath, deps.Config)
		view := coreConfigurationView{
			Editable:        true,
			RestartRequired: saved && runtimeSettingsDiffer(deps.Config, displayConfig),
			Runtime: coreRuntimeView{
				Environment: displayConfig.AppEnv, HTTPAddress: displayConfig.HTTPAddr, WebDirectory: displayConfig.WebDir,
				AuthenticationEnabled:   displayConfig.AuthUsername != "" && displayConfig.AuthPassword != "",
				CORSConfigured:          displayConfig.CORSOrigin != "",
				PDFAIConfigured:         displayConfig.PDFAIURL != "",
				PDFAIURL:                displayConfig.PDFAIURL,
				TransactionAIConfigured: displayConfig.TransactionAIURL != "",
				TransactionAIURL:        displayConfig.TransactionAIURL,
				TransactionAIModel:      displayConfig.TransactionAIModel,
			},
			Database: coreDatabaseView{Backend: "sqlite", Path: deps.Config.DatabasePath, Status: "ok", SchemaVersion: store.CoreSchemaVersion},
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
		if input.S3Bucket == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "S3 bucket is verplicht"})
			return
		}
		if input.S3Region == "" {
			input.S3Region = "garage"
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
		view := coreConfigurationView{Editable: true, RestartRequired: false, Runtime: coreRuntimeView{Environment: deps.Config.AppEnv, HTTPAddress: deps.Config.HTTPAddr, WebDirectory: deps.Config.WebDir, AuthenticationEnabled: deps.Config.AuthUsername != "" && deps.Config.AuthPassword != "", CORSConfigured: deps.Config.CORSOrigin != "", PDFAIConfigured: input.PDFAIURL != "", PDFAIURL: input.PDFAIURL, TransactionAIConfigured: input.TransactionAIURL != "", TransactionAIURL: input.TransactionAIURL, TransactionAIModel: input.TransactionAIModel}, Database: coreDatabaseView{Backend: "sqlite", Path: deps.Config.DatabasePath, Status: "ok", SchemaVersion: store.CoreSchemaVersion}, ObjectStorage: coreObjectStorageView{Backend: "garage/s3", Endpoint: input.S3Endpoint, Bucket: input.S3Bucket, Region: input.S3Region, TLS: input.S3UseTLS, PathStyle: input.S3PathStyle, CreateBucket: input.S3CreateBucket, Configured: input.S3Endpoint != "" && accessKeySet && secretKeySet && input.S3Bucket != "", Ready: ready, AccessKeySet: accessKeySet, SecretKeySet: secretKeySet, StatusMessage: statusMessage}, BackupRecovery: coreBackupRecoveryView{Available: deps.SystemBackup != nil, Backend: "restic/rclone", Message: "Backupconfiguratie blijft beschikbaar. Herstart na S3-wijzigingen voordat je een backup valideert."}}
		writeJSON(w, http.StatusOK, view)
	})
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
		left.PDFAIURL != right.PDFAIURL || left.TransactionAIURL != right.TransactionAIURL || left.TransactionAIModel != right.TransactionAIModel
}
