package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	drivermysql "github.com/go-sql-driver/mysql"
)

func TestEnvironmentConfigurationIsReadOnly(t *testing.T) {
	clearStorageEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:password@db:5432/money?sslmode=disable")
	cfg := testConfig(t)
	cfg.ConfigEditable = true

	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if view := manager.View(); view.Source != "environment" || view.Editable {
		t.Fatalf("unexpected environment view: %+v", view)
	}
	if err := manager.Save(validUpdate()); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Save returned %v, want ErrReadOnly", err)
	}
}

func TestEditableConfigurationWithoutAuthStaysReadOnly(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	cfg.ConfigEditable = true
	cfg.HTTPAddr = ":8080"
	cfg.AuthUsername = ""
	cfg.AuthPassword = ""

	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if view := manager.View(); view.Editable || !strings.Contains(view.ReadOnlyReason, "AUTH_USERNAME") {
		t.Fatalf("expected readable authentication reason: %+v", view)
	}
}

func TestProductionLoopbackStillRequiresAuth(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	cfg.AppEnv = "production"
	cfg.ConfigEditable = true
	cfg.AuthUsername = ""
	cfg.AuthPassword = ""
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if manager.View().Editable {
		t.Fatal("production configuration became editable without authentication")
	}
}

func TestSaveMasksAndRetainsSecretsAndBecomesPending(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	cfg.ConfigEditable = true
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err := manager.Save(validUpdate()); err != nil {
		t.Fatal(err)
	}
	view := manager.View()
	if !view.RestartRequired || view.Pending == nil {
		t.Fatalf("saved configuration is not shown as pending: %+v", view)
	}
	if view.Effective.Database.Host != "old-db" || view.Pending.Database.Host != "new-db" {
		t.Fatalf("effective configuration changed before restart: %+v", view)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"db-password", "s3-access", "s3-secret", "postgres://"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("safe view contains secret configuration")
		}
	}
	if !view.Pending.Database.PasswordSet || !view.Pending.Objects.AccessKeySet || !view.Pending.Objects.SecretKeySet {
		t.Fatal("safe view did not report retained secrets")
	}

	info, err := os.Stat(cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("configuration mode is %o, want 600", info.Mode().Perm())
	}
	persisted, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"db-password", "s3-access", "s3-secret"} {
		if !strings.Contains(string(persisted), secret) {
			t.Fatal("persisted configuration did not retain an omitted secret")
		}
	}

	restarted, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Effective().S3Endpoint != "new-s3:3900" || restarted.Effective().DatabaseURL == cfg.DatabaseURL {
		t.Fatal("persisted configuration was not applied on restart")
	}
}

func TestPersistedConfigurationLoadsReadOnly(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	cfg.ConfigEditable = true
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Save(validUpdate()); err != nil {
		t.Fatal(err)
	}

	cfg.ConfigEditable = false
	restarted, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	view := restarted.View()
	if view.Source != "file" || view.Editable || restarted.Effective().S3Endpoint != "new-s3:3900" {
		t.Fatalf("persisted configuration was not loaded read-only: %+v", view)
	}
}

func TestPersistedConfigurationWinsOverStorageEnvironment(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	fileDatabaseURL := cfg.DatabaseURL
	fileS3Endpoint := cfg.S3Endpoint
	if err := writeAtomic(cfg.ConfigPath, diskFromConfig(cfg)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", "postgres://env:secret@env-db:5432/env?sslmode=disable")
	t.Setenv("S3_ENDPOINT", "env-s3:9000")
	cfg.DatabaseURL = "postgres://env:secret@env-db:5432/env?sslmode=disable"
	cfg.S3Endpoint = "env-s3:9000"

	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := manager.Effective(); got.DatabaseURL != fileDatabaseURL || got.S3Endpoint != fileS3Endpoint {
		t.Fatalf("environment overrode file configuration: %+v", got)
	}
}

func TestBootstrapCreatesAuthoritativeFile(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	cfg.ConfigBootstrapFromEnv = true

	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if manager.View().Source != "file" {
		t.Fatalf("bootstrap source is %q, want file", manager.View().Source)
	}
	info, err := os.Stat(cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("bootstrapped configuration mode is %o, want 600", info.Mode().Perm())
	}

	cfg.S3Endpoint = "changed-by-env:9000"
	restarted, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Effective().S3Endpoint != "old-s3:9000" {
		t.Fatal("bootstrapped file was not authoritative after restart")
	}
}

func TestBootstrapRejectsInvalidEnvironment(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	cfg.ConfigBootstrapFromEnv = true
	cfg.S3SecretKey = ""
	if _, err := NewManager(cfg); !errors.Is(err, ErrInvalid) {
		t.Fatalf("NewManager returned %v, want ErrInvalid", err)
	}
	if _, err := os.Stat(cfg.ConfigPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid bootstrap created a file: %v", err)
	}
}

func TestFilesystemConfigurationPersistsAndSkipsS3Validation(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	cfg.ConfigEditable = true
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	input := validUpdate()
	input.Objects = ObjectInput{Backend: "filesystem", Path: "/app/data/objects", Endpoint: "https://ignored.invalid"}
	if err := manager.Save(input); err != nil {
		t.Fatal(err)
	}

	restarted, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := restarted.Effective(); got.ObjectBackend != "filesystem" || got.ObjectFilesystemRoot != "/app/data/objects" {
		t.Fatalf("filesystem configuration was not restored: %+v", got)
	}
}

func TestObjectBackendSettingsAreRetainedWhenSwitching(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	cfg.ConfigEditable = true
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	filesystem := validUpdate()
	filesystem.Objects = ObjectInput{Backend: "filesystem", Path: "/app/data/objects"}
	if err := manager.Save(filesystem); err != nil {
		t.Fatal(err)
	}
	if pending := manager.pending; pending.S3Endpoint != cfg.S3Endpoint || pending.S3Bucket != cfg.S3Bucket {
		t.Fatal("switching to filesystem discarded S3 settings")
	}

	s3 := validUpdate()
	if err := manager.Save(s3); err != nil {
		t.Fatal(err)
	}
	if pending := manager.pending; pending.ObjectFilesystemRoot != "/app/data/objects" {
		t.Fatal("switching to S3 discarded filesystem settings")
	}
}

func TestFilesystemConfigurationRejectsRootAndRelativePaths(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	cfg.ConfigEditable = true
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", ".", "objects", "/"} {
		input := validUpdate()
		input.Objects = ObjectInput{Backend: "filesystem", Path: path}
		if err := manager.Save(input); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Save with filesystem path %q returned %v, want ErrInvalid", path, err)
		}
	}
}

func TestExistingConfigurationPermissionsAreRestricted(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	if err := writeAtomic(cfg.ConfigPath, diskFromConfig(cfg)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cfg.ConfigPath, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewManager(cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("configuration mode is %o, want 600", info.Mode().Perm())
	}
}

func TestSaveRetainsPostgresConnectionParameters(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	cfg.ConfigEditable = true
	cfg.DatabaseURL += "&connect_timeout=10&application_name=geldzaken"
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Save(validUpdate()); err != nil {
		t.Fatal(err)
	}

	pending := manager.pending.DatabaseURL
	if !strings.Contains(pending, "connect_timeout=10") || !strings.Contains(pending, "application_name=geldzaken") || !strings.Contains(pending, "sslmode=require") {
		t.Fatalf("Postgres connection parameters were not retained: %s", pending)
	}
}

func TestSaveRejectsUnsupportedCombination(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	cfg.ConfigEditable = true
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	input := validUpdate()
	input.Profile = "local"
	if err := manager.Save(input); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Save returned %v, want ErrInvalid", err)
	}
	if _, err := os.Stat(cfg.ConfigPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid configuration was persisted: %v", err)
	}
}

func TestSaveRejectsInvalidS3Endpoint(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	cfg.ConfigEditable = true
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	for _, endpoint := range []string{"https://garage.example.test:3900", "garage.example.test/path", "garage.example.test:70000", "user@garage.example.test"} {
		input := validUpdate()
		input.Objects.Endpoint = endpoint
		if err := manager.Save(input); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Save with endpoint %q returned %v, want ErrInvalid", endpoint, err)
		}
	}
}

func TestLegacyConfigurationLoadsWithoutRewriteAndSavesAsV2(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	cfg.ConfigEditable = true
	legacy := `{
  "storageProfile": "server",
  "database": {"backend": "postgres", "url": "postgres://legacy:secret@legacy-db:5432/money?sslmode=require"},
  "object": {
    "backend": "s3",
    "filesystem": {"path": "/app/data/objects"},
    "s3": {"endpoint": "garage:3900", "accessKey": "access", "secretKey": "secret", "bucket": "documents", "useTLS": true, "region": "garage", "pathStyle": true, "createBucket": false}
  }
}
`
	if err := os.MkdirAll(filepath.Dir(cfg.ConfigPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.ConfigPath, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}

	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	unchanged, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(unchanged) != legacy || manager.Effective().DatabaseURL != "postgres://legacy:secret@legacy-db:5432/money?sslmode=require" {
		t.Fatal("legacy configuration was rewritten or not normalized at load")
	}
	if err := manager.Save(validUpdate()); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), `"version": 2`) || strings.Contains(string(saved), `"url": "postgres://legacy`) {
		t.Fatalf("legacy configuration was not upgraded on save: %s", saved)
	}
}

func TestV2RoundTripRetainsIndependentDatabaseSecretsAndMasksThem(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	cfg.ConfigEditable = true
	cfg.DatabaseDataRoot = "/app/data/sqlite"
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	postgresPassword := "new-postgres-secret"
	mysqlPassword := "mysql-secret"
	input := validUpdate()
	input.Database = DatabaseInput{
		Backend:  "postgres",
		SQLite:   &SQLiteInput{Path: "/app/data/sqlite/money.db"},
		Postgres: &PostgresInput{Host: "new-db", Port: 5432, Name: "money", User: "money", TLSMode: "require", Password: &postgresPassword},
		MySQL:    &MySQLInput{Host: "mysql", Port: 3306, Name: "money", User: "money", TLSMode: "required", Password: &mysqlPassword},
	}
	if err := manager.Save(input); err != nil {
		t.Fatal(err)
	}

	input.Database.Postgres.Password = nil
	input.Database.MySQL.Password = nil
	input.Database.MySQL.Host = "mysql-new"
	if err := manager.Save(input); err != nil {
		t.Fatal(err)
	}
	viewJSON, err := json.Marshal(manager.View())
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{postgresPassword, mysqlPassword, "postgres://", "@tcp("} {
		if strings.Contains(string(viewJSON), secret) {
			t.Fatalf("safe view leaked %q", secret)
		}
	}
	if pending := manager.View().Pending; pending == nil || !pending.Database.Postgres.PasswordSet || !pending.Database.MySQL.PasswordSet || pending.Database.MySQL.Host != "mysql-new" {
		t.Fatalf("safe view did not retain independent database state: %+v", pending)
	}

	restarted, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := restarted.Effective()
	if got.DatabasePath != "/app/data/sqlite/money.db" || databasePassword(got.PostgresURL) != postgresPassword || got.MySQLPassword != mysqlPassword || got.MySQLHost != "mysql-new" {
		t.Fatalf("v2 database settings did not round-trip: %+v", got)
	}
}

func TestSQLitePathSafety(t *testing.T) {
	for _, path := range []string{"money.db", "/", "file:/app/data/sqlite/money.db", "sqlite:///app/data/sqlite/money.db", "/app/data/other/money.db", "/app/data/sqlite/money.db?mode=memory", "/app/data/sqlite/money.db#fragment"} {
		if err := validateSQLitePath(path, "/app/data/sqlite"); !errors.Is(err, ErrInvalid) {
			t.Fatalf("validateSQLitePath(%q) returned %v, want ErrInvalid", path, err)
		}
	}
	if err := validateSQLitePath("/app/data/sqlite/nested/money.db", "/app/data/sqlite"); err != nil {
		t.Fatalf("safe SQLite path was rejected: %v", err)
	}
}

func TestPostgreSQLURLAliasIsAccepted(t *testing.T) {
	if err := validatePostgresURL("postgresql://money:secret@postgres:5432/money?sslmode=require"); err != nil {
		t.Fatalf("postgresql URL alias was rejected: %v", err)
	}
}

func TestMySQLFieldsAndDatabaseCapabilities(t *testing.T) {
	cfg := testConfig(t)
	cfg.MySQLHost = "mysql"
	cfg.MySQLPort = 3306
	cfg.MySQLName = "money"
	cfg.MySQLUser = "money"
	cfg.MySQLPassword = "secret"
	cfg.MySQLTLSMode = "required"
	if err := validateMySQL(cfg); err != nil {
		t.Fatalf("valid MySQL fields were rejected: %v", err)
	}
	parsed, err := drivermysql.ParseDSN(buildMySQLDSN(cfg))
	if err != nil || parsed.User != cfg.MySQLUser || parsed.Passwd != cfg.MySQLPassword || parsed.Addr != "mysql:3306" || parsed.DBName != cfg.MySQLName || parsed.TLSConfig != "true" {
		t.Fatalf("unexpected MySQL DSN round-trip: parsed=%+v err=%v", parsed, err)
	}
	cfg.MySQLTLSMode = "disabled"
	parsed, err = drivermysql.ParseDSN(buildMySQLDSN(cfg))
	if err != nil || parsed.TLSConfig != "false" {
		t.Fatalf("explicit local MySQL plaintext mode did not round-trip: parsed=%+v err=%v", parsed, err)
	}
	cfg.MySQLTLSMode = "verify-ca"
	if err := validateMySQL(cfg); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unverified MySQL TLS mode returned %v, want ErrInvalid", err)
	}

	clearStorageEnv(t)
	manager, err := NewManager(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	capabilities := manager.View().Capabilities
	if len(capabilities.Databases) != 3 || !capabilities.Databases[1].Supported || !capabilities.Databases[2].Supported || !capabilities.Profiles[0].Supported {
		t.Fatalf("unexpected database capabilities: %+v", capabilities)
	}
}

func TestMySQLDSNFormatsDelimitersAndRejectsUnsafeIdentifiers(t *testing.T) {
	cfg := testConfig(t)
	cfg.MySQLHost = "2001:db8::1"
	cfg.MySQLPort = 3306
	cfg.MySQLName = "money/archive?2026"
	cfg.MySQLUser = "money@home"
	cfg.MySQLPassword = "p@ss:word)/?&"
	cfg.MySQLTLSMode = "required"
	if err := validateMySQL(cfg); err != nil {
		t.Fatalf("safe delimiter-heavy MySQL settings were rejected: %v", err)
	}
	parsed, err := drivermysql.ParseDSN(buildMySQLDSN(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.User != cfg.MySQLUser || parsed.Passwd != cfg.MySQLPassword || parsed.DBName != cfg.MySQLName || parsed.Addr != "[2001:db8::1]:3306" {
		t.Fatalf("driver formatter did not preserve MySQL settings: %+v", parsed)
	}

	for _, change := range []func(*Config){
		func(value *Config) { value.MySQLHost = "mysql)/other" },
		func(value *Config) { value.MySQLHost = "user@mysql" },
		func(value *Config) { value.MySQLHost = "mysql:3307" },
		func(value *Config) { value.MySQLUser = "money:admin" },
		func(value *Config) { value.MySQLName = "money\nother" },
	} {
		invalid := cfg
		change(&invalid)
		if err := validateMySQL(invalid); !errors.Is(err, ErrInvalid) {
			t.Fatalf("unsafe MySQL settings returned %v, want ErrInvalid: %+v", err, invalid)
		}
	}
}

func TestV2InactiveDatabaseSectionsAreNormalizedAndValidated(t *testing.T) {
	clearStorageEnv(t)
	cfg := testConfig(t)
	persisted := diskFromConfig(cfg)
	persisted.Database.SQLite.Path = "data/money.db"
	persisted.Database.MySQL.Host = "mysql"
	persisted.Database.MySQL.Port = 3306
	persisted.Database.MySQL.Name = "money"
	persisted.Database.MySQL.User = "money"
	persisted.Database.MySQL.TLSMode = "preferred"
	if err := writeAtomic(cfg.ConfigPath, persisted); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := manager.Effective()
	if got.DatabasePath != "" || got.MySQLTLSMode != "required" {
		t.Fatalf("inactive v2 settings were not safely normalized: %+v", got)
	}
	parsed, err := drivermysql.ParseDSN(got.MySQLDSN)
	if err != nil || parsed.TLSConfig != "true" {
		t.Fatalf("normalized inactive MySQL DSN is not TLS-required: parsed=%+v err=%v", parsed, err)
	}

	malformed := diskFromConfig(cfg)
	malformed.Database.MySQL.Host = "mysql"
	if err := writeAtomic(cfg.ConfigPath, malformed); err != nil {
		t.Fatal(err)
	}
	if _, err := NewManager(cfg); !errors.Is(err, ErrInvalid) {
		t.Fatalf("malformed inactive MySQL section returned %v, want ErrInvalid", err)
	}
}

func TestProfileBackendCombinationsAreValidated(t *testing.T) {
	cfg := testConfig(t)
	cfg.StorageProfile = "local"
	cfg.StoreBackend = "sqlite"
	cfg.DatabasePath = "/app/data/sqlite/money.db"
	cfg.DatabaseDataRoot = "/app/data/sqlite"
	cfg.ObjectBackend = "filesystem"
	if err := validateCombination(cfg); err != nil {
		t.Fatalf("local SQLite/filesystem mapping was rejected: %v", err)
	}
	if err := validate(cfg); err != nil {
		t.Fatalf("valid local configuration was rejected: %v", err)
	}

	cfg.StorageProfile = "server"
	cfg.StoreBackend = "mysql"
	cfg.MySQLHost = "mysql"
	cfg.MySQLPort = 3306
	cfg.MySQLName = "money"
	cfg.MySQLUser = "money"
	cfg.MySQLTLSMode = "disabled"
	if err := validateCombination(cfg); err != nil {
		t.Fatalf("server MySQL/filesystem mapping was rejected: %v", err)
	}
	if err := validate(cfg); err != nil {
		t.Fatalf("valid MySQL configuration was rejected: %v", err)
	}
	cfg.StoreBackend = "sqlite"
	if err := validateCombination(cfg); !errors.Is(err, ErrInvalid) {
		t.Fatalf("server SQLite mapping returned %v, want ErrInvalid", err)
	}
}

func testConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		HTTPAddr:             "127.0.0.1:8080",
		StorageProfile:       "server",
		StoreBackend:         "postgres",
		DatabaseURL:          "postgres://money:db-password@old-db:5432/money?sslmode=disable",
		ObjectBackend:        "s3",
		ObjectFilesystemRoot: "/app/data/objects",
		S3Endpoint:           "old-s3:9000",
		S3AccessKey:          "s3-access",
		S3SecretKey:          "s3-secret",
		S3Bucket:             "documents",
		S3Region:             "us-east-1",
		S3PathStyle:          true,
		S3CreateBucket:       true,
		ConfigPath:           filepath.Join(t.TempDir(), "nested", "config.json"),
	}
}

func validUpdate() Update {
	return Update{
		Profile:  "central",
		Database: DatabaseInput{Backend: "postgres", Host: "new-db", Port: 5432, Name: "money", User: "money", SSLMode: "require"},
		Objects:  ObjectInput{Backend: "s3", Endpoint: "new-s3:3900", Bucket: "documents", Region: "garage", PathStyle: true},
	}
}
