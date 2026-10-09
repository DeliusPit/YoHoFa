package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Runtime is the fixed SQLite/S3 configuration used by cmd/api in v3.
// Legacy storage profiles and config/storage.json do not participate.
type Runtime struct {
	AppEnv             string
	HTTPAddr           string
	WebDir             string
	CORSOrigin         string
	AuthUsername       string
	AuthPassword       string
	DatabasePath       string
	S3Endpoint         string
	S3AccessKey        string
	S3SecretKey        string
	S3Bucket           string
	S3UseTLS           bool
	S3Region           string
	S3PathStyle        bool
	S3CreateBucket     bool
	PDFAIURL           string
	PDFAIModel         string
	TransactionAIURL   string
	TransactionAIModel string
	ConfigPath         string
	BackupWorkRoot     string
	BankConfigDir      string
	TranslationDir     string
	AIDocsConfigPath   string
	Language           string
}

type RuntimeUpdate struct {
	DatabasePath       string  `json:"databasePath"`
	S3Endpoint         string  `json:"s3Endpoint"`
	S3AccessKey        *string `json:"s3AccessKey,omitempty"`
	S3SecretKey        *string `json:"s3SecretKey,omitempty"`
	S3Bucket           string  `json:"s3Bucket"`
	S3UseTLS           bool    `json:"s3UseTLS"`
	S3Region           string  `json:"s3Region"`
	S3PathStyle        bool    `json:"s3PathStyle"`
	S3CreateBucket     bool    `json:"s3CreateBucket"`
	PDFAIURL           string  `json:"pdfAiUrl"`
	PDFAIModel         string  `json:"pdfAiModel"`
	TransactionAIURL   string  `json:"transactionAiUrl"`
	TransactionAIModel string  `json:"transactionAiModel"`
	Language           string  `json:"language"`
}

type runtimeDisk struct {
	Version            int    `json:"version"`
	DatabasePath       string `json:"databasePath"`
	S3Endpoint         string `json:"s3Endpoint"`
	S3AccessKey        string `json:"s3AccessKey"`
	S3SecretKey        string `json:"s3SecretKey"`
	S3Bucket           string `json:"s3Bucket"`
	S3UseTLS           bool   `json:"s3UseTLS"`
	S3Region           string `json:"s3Region"`
	S3PathStyle        bool   `json:"s3PathStyle"`
	S3CreateBucket     bool   `json:"s3CreateBucket"`
	PDFAIURL           string `json:"pdfAiUrl"`
	PDFAIModel         string `json:"pdfAiModel"`
	TransactionAIURL   string `json:"transactionAiUrl"`
	TransactionAIModel string `json:"transactionAiModel"`
	Language           string `json:"language"`
}

func RuntimeFromEnv() Runtime {
	cfg := Runtime{
		AppEnv: env("APP_ENV", "development"), HTTPAddr: env("HTTP_ADDR", ":8080"),
		WebDir: env("WEB_DIR", "web/dist"), CORSOrigin: env("CORS_ORIGIN", ""),
		AuthUsername: authEnv("AUTH_USERNAME", "AUTH_USERNAME_BASE64"),
		AuthPassword: authEnv("AUTH_PASSWORD", "AUTH_PASSWORD_BASE64"),
		DatabasePath: env("DATABASE_PATH", "data/v3/hohoho.db"),
		S3Endpoint:   env("S3_ENDPOINT", ""), S3AccessKey: env("S3_ACCESS_KEY", ""), S3SecretKey: env("S3_SECRET_KEY", ""),
		S3Bucket: env("S3_BUCKET", "hohoho-v3-documents"), S3Region: env("S3_REGION", "garage"),
		S3UseTLS: boolEnv("S3_USE_TLS", false), S3PathStyle: boolEnv("S3_PATH_STYLE", true),
		S3CreateBucket: boolEnv("S3_CREATE_BUCKET", false), PDFAIURL: env("PDF_AI_URL", ""), PDFAIModel: env("PDF_AI_MODEL", "Qwen3.8-Flash-Next-UD-Q4_K_XL"), TransactionAIURL: env("TRANSACTION_AI_URL", ""), TransactionAIModel: env("TRANSACTION_AI_MODEL", "Qwen3.6-35B-A3B-Q4_K_M"),
		ConfigPath: env("RUNTIME_CONFIG_PATH", "data/v3/runtime.json"), BackupWorkRoot: env("BACKUP_WORK_ROOT", "data/v3/backup-work"),
		BankConfigDir: env("BANK_CONFIG_DIR", "config/banks"), TranslationDir: env("TRANSLATION_DIR", "config/translations"), AIDocsConfigPath: env("AI_DOCS_CONFIG_PATH", "config/ai-docs.yaml"), Language: env("APP_LANGUAGE", "nl"),
	}
	if disk, ok := readRuntimeDisk(cfg.ConfigPath); ok {
		applyRuntimeDisk(&cfg, disk)
	}
	applyRuntimeEnv(&cfg)
	return cfg
}

func (cfg Runtime) Validate() error {
	if (cfg.AuthUsername == "") != (cfg.AuthPassword == "") {
		return fmt.Errorf("Vul zowel AUTH_USERNAME als AUTH_PASSWORD in")
	}
	if cfg.AppEnv == "production" && cfg.AuthUsername == "" {
		return fmt.Errorf("Authenticatie is verplicht in productie")
	}
	return nil
}

func RuntimeUpdateFromCurrent(cfg Runtime) RuntimeUpdate {
	return RuntimeUpdate{DatabasePath: cfg.DatabasePath, S3Endpoint: cfg.S3Endpoint, S3Bucket: cfg.S3Bucket, S3UseTLS: cfg.S3UseTLS, S3Region: cfg.S3Region, S3PathStyle: cfg.S3PathStyle, S3CreateBucket: cfg.S3CreateBucket, PDFAIURL: cfg.PDFAIURL, PDFAIModel: cfg.PDFAIModel, TransactionAIURL: cfg.TransactionAIURL, TransactionAIModel: cfg.TransactionAIModel, Language: cfg.Language}
}

func SavedRuntime(path string, current Runtime) (Runtime, bool) {
	disk, ok := readRuntimeDisk(path)
	if !ok {
		return current, false
	}
	next := current
	applyRuntimeDisk(&next, disk)
	return next, true
}

func SaveRuntimeUpdate(path string, current Runtime, input RuntimeUpdate) error {
	accessKey := current.S3AccessKey
	secretKey := current.S3SecretKey
	if input.S3AccessKey != nil {
		accessKey = *input.S3AccessKey
	}
	if input.S3SecretKey != nil {
		secretKey = *input.S3SecretKey
	}
	model := input.TransactionAIModel
	if model == "" {
		model = current.TransactionAIModel
	}
	pdfModel := input.PDFAIModel
	if pdfModel == "" {
		pdfModel = current.PDFAIModel
	}
	language := input.Language
	if language == "" {
		language = current.Language
	}
	databasePath := input.DatabasePath
	if databasePath == "" {
		databasePath = current.DatabasePath
	}
	disk := runtimeDisk{Version: 1, DatabasePath: databasePath, S3Endpoint: input.S3Endpoint, S3AccessKey: accessKey, S3SecretKey: secretKey, S3Bucket: input.S3Bucket, S3UseTLS: input.S3UseTLS, S3Region: input.S3Region, S3PathStyle: input.S3PathStyle, S3CreateBucket: input.S3CreateBucket, PDFAIURL: input.PDFAIURL, PDFAIModel: pdfModel, TransactionAIURL: input.TransactionAIURL, TransactionAIModel: model, Language: language}
	data, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".runtime-*.json")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		temporary.Close()
		os.Remove(temporaryName)
		return err
	}
	if err := temporary.Close(); err != nil {
		os.Remove(temporaryName)
		return err
	}
	if err := os.Chmod(temporaryName, 0o600); err != nil {
		os.Remove(temporaryName)
		return err
	}
	return os.Rename(temporaryName, path)
}

func readRuntimeDisk(path string) (runtimeDisk, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return runtimeDisk{}, false
	}
	var disk runtimeDisk
	if json.Unmarshal(data, &disk) != nil || disk.Version != 1 {
		return runtimeDisk{}, false
	}
	return disk, true
}

func applyRuntimeDisk(cfg *Runtime, disk runtimeDisk) {
	if disk.DatabasePath != "" {
		cfg.DatabasePath = disk.DatabasePath
	}
	cfg.S3Endpoint = disk.S3Endpoint
	cfg.S3AccessKey = disk.S3AccessKey
	cfg.S3SecretKey = disk.S3SecretKey
	cfg.S3Bucket = disk.S3Bucket
	cfg.S3UseTLS = disk.S3UseTLS
	cfg.S3Region = disk.S3Region
	cfg.S3PathStyle = disk.S3PathStyle
	cfg.S3CreateBucket = disk.S3CreateBucket
	cfg.PDFAIURL = disk.PDFAIURL
	if disk.PDFAIModel != "" {
		cfg.PDFAIModel = disk.PDFAIModel
	}
	cfg.TransactionAIURL = disk.TransactionAIURL
	if disk.TransactionAIModel != "" {
		cfg.TransactionAIModel = disk.TransactionAIModel
	}
	if disk.Language != "" {
		cfg.Language = disk.Language
	}
}

func applyRuntimeEnv(cfg *Runtime) {
	if value := os.Getenv("S3_ENDPOINT"); value != "" {
		cfg.S3Endpoint = value
	}
	if value := os.Getenv("S3_ACCESS_KEY"); value != "" {
		cfg.S3AccessKey = value
	}
	if value := os.Getenv("S3_SECRET_KEY"); value != "" {
		cfg.S3SecretKey = value
	}
	if value := os.Getenv("S3_BUCKET"); value != "" {
		cfg.S3Bucket = value
	}
	if value := os.Getenv("S3_REGION"); value != "" {
		cfg.S3Region = value
	}
	if value := os.Getenv("S3_USE_TLS"); value != "" {
		cfg.S3UseTLS = value == "true"
	}
	if value := os.Getenv("S3_PATH_STYLE"); value != "" {
		cfg.S3PathStyle = value == "true"
	}
	if value := os.Getenv("S3_CREATE_BUCKET"); value != "" {
		cfg.S3CreateBucket = value == "true"
	}
	if value := os.Getenv("PDF_AI_URL"); value != "" {
		cfg.PDFAIURL = value
	}
	if value := os.Getenv("PDF_AI_MODEL"); value != "" {
		cfg.PDFAIModel = value
	}
	if value := os.Getenv("TRANSACTION_AI_URL"); value != "" {
		cfg.TransactionAIURL = value
	}
	if value := os.Getenv("TRANSACTION_AI_MODEL"); value != "" {
		cfg.TransactionAIModel = value
	}
	if value := os.Getenv("APP_LANGUAGE"); value != "" {
		cfg.Language = value
	}
}

// Environment helpers live with Runtime so it remains independent of legacy
// configuration when config.go is eventually removed.
func authEnv(plainKey string, base64Key string) string {
	if value := os.Getenv(plainKey); value != "" {
		return value
	}
	encoded := os.Getenv(base64Key)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	return string(decoded)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envFallback(key, legacyKey, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return env(legacyKey, fallback)
}

func boolEnv(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value == "true"
}

func intEnv(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value == 0 {
		return fallback
	}
	return value
}
