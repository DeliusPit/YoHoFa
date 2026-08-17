package config

import (
	"encoding/base64"
	"os"
)

type Config struct {
	AppEnv         string
	HTTPAddr       string
	StoreBackend   string
	DatabaseURL    string
	DatabasePath   string
	CORSOrigin     string
	WebDir         string
	AuthUsername   string
	AuthPassword   string
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
	MinIOUseSSL    bool
}

func FromEnv() Config {
	return Config{
		AppEnv:         env("APP_ENV", "development"),
		HTTPAddr:       env("HTTP_ADDR", ":8080"),
		StoreBackend:   env("STORE_BACKEND", "postgres"),
		DatabaseURL:    env("DATABASE_URL", "postgres://geldzaken:geldzaken@localhost:5432/geldzaken?sslmode=disable"),
		DatabasePath:   env("DATABASE_PATH", "data/geldzaken.db"),
		CORSOrigin:     env("CORS_ORIGIN", "http://localhost:5173"),
		WebDir:         env("WEB_DIR", ""),
		AuthUsername:   authEnv("AUTH_USERNAME", "AUTH_USERNAME_BASE64"),
		AuthPassword:   authEnv("AUTH_PASSWORD", "AUTH_PASSWORD_BASE64"),
		MinIOEndpoint:  env("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: env("MINIO_ACCESS_KEY", ""),
		MinIOSecretKey: env("MINIO_SECRET_KEY", ""),
		MinIOBucket:    env("MINIO_BUCKET", "geldzaken-documents"),
		MinIOUseSSL:    env("MINIO_USE_SSL", "false") == "true",
	}
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
