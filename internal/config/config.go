package config

import (
	"encoding/base64"
	"os"
)

type Config struct {
	AppEnv       string
	HTTPAddr     string
	StoreBackend string
	DatabaseURL  string
	DatabasePath string
	CORSOrigin   string
	WebDir       string
	AuthUsername string
	AuthPassword string
}

func FromEnv() Config {
	return Config{
		AppEnv:       env("APP_ENV", "development"),
		HTTPAddr:     env("HTTP_ADDR", ":8080"),
		StoreBackend: env("STORE_BACKEND", "sqlite"),
		DatabaseURL:  env("DATABASE_URL", "postgres://geldzaken:geldzaken@localhost:5432/geldzaken?sslmode=disable"),
		DatabasePath: env("DATABASE_PATH", "data/geldzaken.db"),
		CORSOrigin:   env("CORS_ORIGIN", "http://localhost:5173"),
		WebDir:       env("WEB_DIR", ""),
		AuthUsername: authEnv("AUTH_USERNAME", "AUTH_USERNAME_BASE64"),
		AuthPassword: authEnv("AUTH_PASSWORD", "AUTH_PASSWORD_BASE64"),
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
