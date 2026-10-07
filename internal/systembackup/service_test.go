package systembackup

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hohoho/internal/config"
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

func TestPolicyValidation(t *testing.T) {
	if err := defaultPolicy().valid(); err != nil {
		t.Fatalf("default policy rejected: %v", err)
	}
	for _, policy := range []Policy{
		{Schedule: "", KeepDaily: 1, KeepWeekly: 1, KeepMonthly: 1},
		{Schedule: "25:00", KeepDaily: 1, KeepWeekly: 1, KeepMonthly: 1},
		{Schedule: "02:30", KeepDaily: 0, KeepWeekly: 1, KeepMonthly: 1},
	} {
		if err := policy.valid(); err == nil {
			t.Fatalf("invalid policy accepted: %+v", policy)
		}
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
	if err := os.WriteFile(filepath.Join(payload, "manifest.json"), []byte(`{"version":2,"databaseBackend":"sqlite","objectBackend":"s3"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, manifest, err := recoveryPayload(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != payload || manifest.DatabaseBackend != "sqlite" || manifest.ObjectBackend != "s3" {
		t.Fatalf("payload=%q manifest=%+v", got, manifest)
	}
}

func TestNewUsesRuntimeConfiguration(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	service, err := New(ctx, config.Runtime{
		ConfigPath:     filepath.Join(root, "runtime.json"),
		BackupWorkRoot: filepath.Join(root, "backup-work"),
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if service.path != filepath.Join(root, "backup.json") {
		t.Fatalf("backup settings path = %q", service.path)
	}
}
