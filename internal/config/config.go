package config

import (
	"encoding/base64"
	"os"
	"strconv"
)

type Config struct {
	AppEnv                 string
	HTTPAddr               string
	CORSOrigin             string
	WebDir                 string
	AuthUsername           string
	AuthPassword           string
	StorageProfile         string
	StoreBackend           string
	DatabaseURL            string
	DatabasePath           string
	PostgresURL            string
	MySQLDSN               string
	MySQLHost              string
	MySQLPort              int
	MySQLName              string
	MySQLUser              string
	MySQLPassword          string
	MySQLTLSMode           string
	DatabaseDataRoot       string
	ObjectBackend          string
	S3Endpoint             string
	S3AccessKey            string
	S3SecretKey            string
	S3Bucket               string
	S3UseTLS               bool
	S3Region               string
	S3PathStyle            bool
	S3CreateBucket         bool
	ObjectFilesystemRoot   string
	LLMEndpoint            string
	LLMDocumentMetadata    bool
	LLMTransactionClassify bool
	ConfigPath             string
	ConfigEditable         bool
	ConfigBootstrapFromEnv bool
	BackupWorkRoot         string
}

func FromEnv() Config {
	databaseURL := env("DATABASE_URL", "postgres://hohoho:hohoho@localhost:5432/hohoho?sslmode=disable")
	cfg := Config{
		AppEnv:                 env("APP_ENV", "development"),
		HTTPAddr:               env("HTTP_ADDR", ":8080"),
		CORSOrigin:             env("CORS_ORIGIN", "http://localhost:5173"),
		WebDir:                 env("WEB_DIR", ""),
		AuthUsername:           authEnv("AUTH_USERNAME", "AUTH_USERNAME_BASE64"),
		AuthPassword:           authEnv("AUTH_PASSWORD", "AUTH_PASSWORD_BASE64"),
		StorageProfile:         env("STORAGE_PROFILE", "server"),
		StoreBackend:           envFallback("DATABASE_BACKEND", "STORE_BACKEND", "postgres"),
		DatabaseURL:            databaseURL,
		DatabasePath:           env("DATABASE_PATH", "data/hohoho.db"),
		PostgresURL:            databaseURL,
		MySQLHost:              env("MYSQL_HOST", ""),
		MySQLPort:              intEnv("MYSQL_PORT", 3306),
		MySQLName:              env("MYSQL_DATABASE", ""),
		MySQLUser:              env("MYSQL_USER", ""),
		MySQLPassword:          env("MYSQL_PASSWORD", ""),
		MySQLTLSMode:           env("MYSQL_TLS_MODE", "required"),
		DatabaseDataRoot:       env("DATABASE_DATA_ROOT", ""),
		ObjectBackend:          env("OBJECT_BACKEND", "s3"),
		S3Endpoint:             env("S3_ENDPOINT", "localhost:9000"),
		S3AccessKey:            env("S3_ACCESS_KEY", ""),
		S3SecretKey:            env("S3_SECRET_KEY", ""),
		S3Bucket:               env("S3_BUCKET", "hohoho-documents"),
		S3UseTLS:               boolEnv("S3_USE_TLS", false),
		S3Region:               env("S3_REGION", "us-east-1"),
		S3PathStyle:            boolEnv("S3_PATH_STYLE", true),
		S3CreateBucket:         boolEnv("S3_CREATE_BUCKET", true),
		ObjectFilesystemRoot:   env("OBJECT_FILESYSTEM_ROOT", "/app/data/objects"),
		LLMEndpoint:            env("LLM_ENDPOINT", ""),
		LLMDocumentMetadata:    boolEnv("LLM_DOCUMENT_METADATA", false),
		LLMTransactionClassify: boolEnv("LLM_TRANSACTION_CLASSIFY", false),
		ConfigPath:             env("CONFIG_PATH", "config/storage.json"),
		ConfigEditable:         boolEnv("CONFIG_EDITABLE", false),
		ConfigBootstrapFromEnv: boolEnv("CONFIG_BOOTSTRAP_FROM_ENV", false),
		BackupWorkRoot:         env("BACKUP_WORK_ROOT", "/app/data/backup-work"),
	}
	if cfg.MySQLHost != "" {
		cfg.MySQLDSN = buildMySQLDSN(cfg)
	}
	return cfg
}

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
