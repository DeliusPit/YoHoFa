package systembackup

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"hohoho/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSettingsValidation(t *testing.T) {
	valid := settings{Repository: "archive:hohoho", RcloneConfig: "[archive]\ntype = local\n", ResticPassword: "secret"}
	if err := valid.valid(); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}
	for name, mutate := range map[string]func(*settings){
		"missing remote": func(value *settings) { value.Repository = "hohoho" },
		"unknown remote": func(value *settings) { value.Repository = "other:hohoho" },
		"empty path":     func(value *settings) { value.Repository = "archive:" },
		"reserved remote": func(value *settings) {
			value.RcloneConfig += "[__hohoho_source]\ntype = local\n"
		},
		"missing password": func(value *settings) { value.ResticPassword = "" },
	} {
		t.Run(name, func(t *testing.T) {
			value := valid
			mutate(&value)
			if err := value.valid(); err == nil {
				t.Fatal("invalid settings accepted")
			}
		})
	}
}

func TestWriteSettingsUsesPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "backup.json")
	value := settings{Version: 1, Repository: "archive:hohoho", RcloneConfig: "[archive]\ntype = local\n", ResticPassword: "secret"}
	if err := writeSettings(path, value); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"resticPassword": "secret"`) {
		t.Fatal("settings were not persisted")
	}
}

func TestSnapshotID(t *testing.T) {
	output := []byte("{\"message_type\":\"status\"}\n{\"message_type\":\"summary\",\"snapshot_id\":\"abc123\"}\n")
	if got := snapshotID(output); got != "abc123" {
		t.Fatalf("snapshotID() = %q", got)
	}
}

func TestClassifyCommandError(t *testing.T) {
	tests := map[string]struct {
		command string
		output  string
		want    string
	}{
		"proton domain":  {"rclone", "Are you sure this is the correct domain? (Code=8002)", "proton_username_invalid"},
		"two factor":     {"rclone", "2FA code is required", "proton_2fa_required"},
		"obscure":        {"rclone", "failed to reveal password: base64 decode failed", "rclone_password_format"},
		"restic key":     {"restic", "no key found in repository", "restic_password_invalid"},
		"generic rclone": {"rclone", "network unavailable", "rclone_destination_failed"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := classifyCommandError(test.command, test.output); got != test.want {
				t.Fatalf("classifyCommandError() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRecoveryPayloadFindsAndValidatesManifest(t *testing.T) {
	root := t.TempDir()
	payload := filepath.Join(root, "nested", "backup-123")
	if err := os.MkdirAll(payload, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(payload, "manifest.json"), []byte(`{"version":1,"databaseBackend":"sqlite","objectBackend":"filesystem"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, manifest, err := recoveryPayload(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != payload || manifest.DatabaseBackend != "sqlite" || manifest.ObjectBackend != "filesystem" {
		t.Fatalf("payload=%q manifest=%+v", got, manifest)
	}
}

func TestSQLiteRecoveryRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic is not installed")
	}
	if _, err := exec.LookPath("rclone"); err != nil {
		t.Skip("rclone is not installed")
	}
	root := t.TempDir()
	configDirectory := filepath.Join(root, "config")
	workRoot := filepath.Join(root, "work")
	objectRoot := filepath.Join(root, "objects")
	repositoryRoot := filepath.Join(root, "repository")
	databasePath := filepath.Join(root, "database", "hohoho.db")
	configPath := filepath.Join(configDirectory, "storage.json")
	for _, directory := range []string{configDirectory, workRoot, objectRoot, repositoryRoot, filepath.Dir(databasePath)} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(configPath, []byte("original config"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(objectRoot, "document.txt"), []byte("original object"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`create table recovery_test(value text); insert into recovery_test(value) values ('original')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	backupSettings := settings{Version: 1, Repository: "archive:" + repositoryRoot, RcloneConfig: "[archive]\ntype = local\n", ResticPassword: "recovery-test-password"}
	if err := writeSettings(filepath.Join(configDirectory, "backup.json"), backupSettings); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{ConfigPath: configPath, BackupWorkRoot: workRoot, StoreBackend: "sqlite", DatabasePath: databasePath, ObjectBackend: "filesystem", ObjectFilesystemRoot: objectRoot}
	service, err := New(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.validateRepository(context.Background(), backupSettings); err != nil {
		t.Fatal(err)
	}
	snapshotID, err := service.run(context.Background(), backupSettings)
	if err != nil {
		t.Fatal(err)
	}

	db, err = sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`update recovery_test set value='changed'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(objectRoot, "document.txt"), []byte("changed object"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("changed config"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONAtomic(service.recoveryRequestPath(), recoveryRequest{SnapshotID: snapshotID}); err != nil {
		t.Fatal(err)
	}
	recovered, err := RunPendingRecovery(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil || !recovered {
		t.Fatal(err)
	}
	if _, err := os.Stat(service.recoveryRequestPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recovery request was not removed: %v", err)
	}

	db, err = sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var value string
	if err := db.QueryRow(`select value from recovery_test`).Scan(&value); err != nil || value != "original" {
		t.Fatalf("database value=%q err=%v", value, err)
	}
	objectData, err := os.ReadFile(filepath.Join(objectRoot, "document.txt"))
	if err != nil || string(objectData) != "original object" {
		t.Fatalf("object=%q err=%v", objectData, err)
	}
	configData, err := os.ReadFile(configPath)
	if err != nil || string(configData) != "original config" {
		t.Fatalf("config=%q err=%v", configData, err)
	}
}

func TestPostgresRecoveryRoundTrip(t *testing.T) {
	databaseURL := os.Getenv("TEST_RECOVERY_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("TEST_RECOVERY_POSTGRES_URL is not set")
	}
	for _, command := range []string{"restic", "rclone", "pg_dump", "pg_restore"} {
		if _, err := exec.LookPath(command); err != nil {
			t.Skip(command + " is not installed")
		}
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `drop table if exists recovery_test; create table recovery_test(value text); insert into recovery_test(value) values ('original')`); err != nil {
		t.Fatal(err)
	}
	pool.Close()

	root := t.TempDir()
	configDirectory := filepath.Join(root, "config")
	workRoot := filepath.Join(root, "work")
	objectRoot := filepath.Join(root, "objects")
	repositoryRoot := filepath.Join(root, "repository")
	configPath := filepath.Join(configDirectory, "storage.json")
	for _, directory := range []string{configDirectory, workRoot, objectRoot, repositoryRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(configPath, []byte("original config"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(objectRoot, "document.txt"), []byte("original object"), 0o600); err != nil {
		t.Fatal(err)
	}
	backupSettings := settings{Version: 1, Repository: "archive:" + repositoryRoot, RcloneConfig: "[archive]\ntype = local\n", ResticPassword: "recovery-test-password"}
	if err := writeSettings(filepath.Join(configDirectory, "backup.json"), backupSettings); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{ConfigPath: configPath, BackupWorkRoot: workRoot, StoreBackend: "postgres", PostgresURL: databaseURL, ObjectBackend: "filesystem", ObjectFilesystemRoot: objectRoot}
	service, err := New(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.validateRepository(ctx, backupSettings); err != nil {
		t.Fatal(err)
	}
	snapshotID, err := service.run(ctx, backupSettings)
	if err != nil {
		var commandFailure commandError
		if errors.As(err, &commandFailure) {
			t.Fatalf("%v: %s", err, commandFailure.detail)
		}
		t.Fatal(err)
	}

	pool, err = pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `update recovery_test set value='changed'`); err != nil {
		t.Fatal(err)
	}
	pool.Close()
	if err := os.WriteFile(filepath.Join(objectRoot, "document.txt"), []byte("changed object"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.RecoverNow(ctx, snapshotID, false); err != nil {
		var commandFailure commandError
		if errors.As(err, &commandFailure) {
			t.Fatalf("%v: %s", err, commandFailure.detail)
		}
		t.Fatal(err)
	}

	pool, err = pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var value string
	if err := pool.QueryRow(ctx, `select value from recovery_test`).Scan(&value); err != nil || value != "original" {
		t.Fatalf("database value=%q err=%v", value, err)
	}
}

func TestMySQLRecoveryRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_RECOVERY_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_RECOVERY_MYSQL_DSN is not set")
	}
	for _, command := range []string{"restic", "rclone", "mariadb-dump", "mariadb"} {
		if _, err := exec.LookPath(command); err != nil {
			t.Skip(command + " is not installed")
		}
	}
	ctx := context.Background()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{`drop table if exists recovery_test`, `create table recovery_test(value varchar(64))`, `insert into recovery_test(value) values ('original')`} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	configDirectory := filepath.Join(root, "config")
	workRoot := filepath.Join(root, "work")
	objectRoot := filepath.Join(root, "objects")
	repositoryRoot := filepath.Join(root, "repository")
	configPath := filepath.Join(configDirectory, "storage.json")
	for _, directory := range []string{configDirectory, workRoot, objectRoot, repositoryRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(configPath, []byte("original config"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(objectRoot, "document.txt"), []byte("original object"), 0o600); err != nil {
		t.Fatal(err)
	}
	backupSettings := settings{Version: 1, Repository: "archive:" + repositoryRoot, RcloneConfig: "[archive]\ntype = local\n", ResticPassword: "recovery-test-password"}
	if err := writeSettings(filepath.Join(configDirectory, "backup.json"), backupSettings); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{ConfigPath: configPath, BackupWorkRoot: workRoot, StoreBackend: "mysql", MySQLDSN: dsn, MySQLTLSMode: "disabled", ObjectBackend: "filesystem", ObjectFilesystemRoot: objectRoot}
	service, err := New(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.validateRepository(ctx, backupSettings); err != nil {
		t.Fatal(err)
	}
	snapshotID, err := service.run(ctx, backupSettings)
	if err != nil {
		var commandFailure commandError
		if errors.As(err, &commandFailure) {
			t.Fatalf("%v: %s", err, commandFailure.detail)
		}
		t.Fatal(err)
	}

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `update recovery_test set value='changed'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(objectRoot, "document.txt"), []byte("changed object"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.RecoverNow(ctx, snapshotID, false); err != nil {
		var commandFailure commandError
		if errors.As(err, &commandFailure) {
			t.Fatalf("%v: %s", err, commandFailure.detail)
		}
		t.Fatal(err)
	}

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var value string
	if err := db.QueryRowContext(ctx, `select value from recovery_test`).Scan(&value); err != nil || value != "original" {
		t.Fatalf("database value=%q err=%v", value, err)
	}
}
