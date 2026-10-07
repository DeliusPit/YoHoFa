package config

import "testing"

func TestRuntimeIgnoresLegacyStorageSettings(t *testing.T) {
	clearStorageEnv(t)
	t.Setenv("STORE_BACKEND", "postgres")
	t.Setenv("DATABASE_BACKEND", "mysql")
	t.Setenv("CONFIG_PATH", "old-storage.json")
	t.Setenv("OBJECT_BACKEND", "filesystem")
	cfg := RuntimeFromEnv()
	if cfg.DatabasePath != "data/v3/hohoho.db" || cfg.S3Bucket != "hohoho-v3-documents" || cfg.PDFAIURL != "" || cfg.S3CreateBucket {
		t.Fatalf("runtime must use empty-start defaults: path=%s bucket=%s", cfg.DatabasePath, cfg.S3Bucket)
	}
	t.Setenv("DATABASE_PATH", "/tmp/custom.db")
	t.Setenv("S3_ENDPOINT", "garage:3900")
	cfg = RuntimeFromEnv()
	if cfg.DatabasePath != "/tmp/custom.db" || cfg.S3Endpoint != "garage:3900" {
		t.Fatal("runtime settings were ignored")
	}
}

func TestRuntimeProductionAuthentication(t *testing.T) {
	for _, cfg := range []Runtime{{AppEnv: "production"}, {AuthUsername: "user"}, {AuthPassword: "password"}} {
		if cfg.Validate() == nil {
			t.Fatal("missing authentication must be rejected")
		}
	}
	if err := (Runtime{AppEnv: "production", AuthUsername: "user", AuthPassword: "password"}).Validate(); err != nil {
		t.Fatal(err)
	}
}
