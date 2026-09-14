package config

import "testing"

func TestFromEnvLegacyMinIODefaults(t *testing.T) {
	clearStorageEnv(t)
	t.Setenv("MINIO_ENDPOINT", "garage:3900")
	t.Setenv("MINIO_ACCESS_KEY", "legacy-access")
	t.Setenv("MINIO_SECRET_KEY", "legacy-secret")
	t.Setenv("MINIO_BUCKET", "documents")
	t.Setenv("MINIO_USE_SSL", "true")

	cfg := FromEnv()
	if cfg.S3Endpoint != "garage:3900" || cfg.S3AccessKey != "legacy-access" || cfg.S3SecretKey != "legacy-secret" || cfg.S3Bucket != "documents" || !cfg.S3UseTLS {
		t.Fatal("legacy MINIO_* values were not mapped to S3 configuration")
	}
	if !cfg.S3CreateBucket || !cfg.S3PathStyle || cfg.S3Region != "us-east-1" {
		t.Fatalf("unexpected legacy S3 defaults: create=%v pathStyle=%v region=%q", cfg.S3CreateBucket, cfg.S3PathStyle, cfg.S3Region)
	}
}

func TestFromEnvPrefersProviderNeutralS3Values(t *testing.T) {
	clearStorageEnv(t)
	t.Setenv("MINIO_ENDPOINT", "legacy:9000")
	t.Setenv("S3_ENDPOINT", "s3.example.test")
	t.Setenv("S3_CREATE_BUCKET", "false")
	t.Setenv("S3_PATH_STYLE", "false")

	cfg := FromEnv()
	if cfg.S3Endpoint != "s3.example.test" || cfg.S3CreateBucket || cfg.S3PathStyle {
		t.Fatalf("provider-neutral values did not take precedence: %+v", cfg)
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
	for _, key := range []string{"STORAGE_PROFILE", "DATABASE_BACKEND", "STORE_BACKEND", "DATABASE_URL", "DATABASE_PATH", "MYSQL_HOST", "MYSQL_PORT", "MYSQL_DATABASE", "MYSQL_USER", "MYSQL_PASSWORD", "MYSQL_TLS_MODE", "OBJECT_BACKEND", "OBJECT_FILESYSTEM_ROOT", "S3_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY", "S3_BUCKET", "S3_USE_TLS", "S3_REGION", "S3_PATH_STYLE", "S3_CREATE_BUCKET", "MINIO_ENDPOINT", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY", "MINIO_BUCKET", "MINIO_USE_SSL"} {
		t.Setenv(key, "")
	}
}
