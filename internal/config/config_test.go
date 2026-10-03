package config

import "testing"

func TestFromEnvS3Defaults(t *testing.T) {
	clearStorageEnv(t)

	cfg := FromEnv()
	if cfg.S3Endpoint != "localhost:9000" || cfg.S3AccessKey != "" || cfg.S3SecretKey != "" || cfg.S3Bucket != "hohoho-documents" || cfg.S3UseTLS {
		t.Fatal("unexpected S3 defaults")
	}
	if cfg.PDFAIURL != "http://192.168.178.215:8090" {
		t.Fatalf("unexpected PDF AI default: %q", cfg.PDFAIURL)
	}
	if !cfg.S3CreateBucket || !cfg.S3PathStyle || cfg.S3Region != "us-east-1" {
		t.Fatalf("unexpected S3 defaults: create=%v pathStyle=%v region=%q", cfg.S3CreateBucket, cfg.S3PathStyle, cfg.S3Region)
	}
}

func TestFromEnvUsesS3Values(t *testing.T) {
	clearStorageEnv(t)
	t.Setenv("S3_ENDPOINT", "s3.example.test")
	t.Setenv("S3_CREATE_BUCKET", "false")
	t.Setenv("S3_PATH_STYLE", "false")

	cfg := FromEnv()
	if cfg.S3Endpoint != "s3.example.test" || cfg.S3CreateBucket || cfg.S3PathStyle {
		t.Fatalf("S3 values were not used: %+v", cfg)
	}
}

func TestFromEnvBuildsMySQLConfiguration(t *testing.T) {
	clearStorageEnv(t)
	t.Setenv("MYSQL_HOST", "mysql")
	t.Setenv("MYSQL_PORT", "3307")
	t.Setenv("MYSQL_DATABASE", "money")
	t.Setenv("MYSQL_USER", "money")
	t.Setenv("MYSQL_PASSWORD", "secret")
	t.Setenv("MYSQL_TLS_MODE", "disabled")

	cfg := FromEnv()
	if cfg.MySQLHost != "mysql" || cfg.MySQLPort != 3307 || cfg.MySQLName != "money" || cfg.MySQLUser != "money" || cfg.MySQLDSN == "" {
		t.Fatalf("MySQL environment was not mapped: %+v", safeView(cfg).Database.MySQL)
	}
}

func clearStorageEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"STORAGE_PROFILE", "DATABASE_BACKEND", "STORE_BACKEND", "DATABASE_URL", "DATABASE_PATH", "MYSQL_HOST", "MYSQL_PORT", "MYSQL_DATABASE", "MYSQL_USER", "MYSQL_PASSWORD", "MYSQL_TLS_MODE", "OBJECT_BACKEND", "OBJECT_FILESYSTEM_ROOT", "S3_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY", "S3_BUCKET", "S3_USE_TLS", "S3_REGION", "S3_PATH_STYLE", "S3_CREATE_BUCKET", "LLM_ENDPOINT", "PDF_AI_URL", "LLM_DOCUMENT_METADATA", "LLM_TRANSACTION_CLASSIFY"} {
		t.Setenv(key, "")
	}
}
