package systembackup

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"hohoho/internal/config"

	_ "modernc.org/sqlite"
)

var (
	ErrInvalid          = errors.New("invalid backup configuration")
	ErrNotReady         = errors.New("backup configuration is not validated")
	ErrRunning          = errors.New("backup is already running")
	ErrConfirmation     = errors.New("invalid recovery confirmation")
	ErrSnapshotNotFound = errors.New("snapshot not found")
	remoteNameExpr      = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	snapshotIDExpr      = regexp.MustCompile(`^[a-fA-F0-9]{8,64}$`)
)

const (
	backupTag        = "hohoho-system"
	sourceRemoteName = "__hohoho_source"
)

type SettingsInput struct {
	Repository     string  `json:"repository"`
	RcloneConfig   *string `json:"rcloneConfig,omitempty"`
	ResticPassword *string `json:"resticPassword,omitempty"`
	Policy         *Policy `json:"policy,omitempty"`
}

// Policy keeps a small, predictable retention set for a single household.
type Policy struct {
	Schedule    string `json:"schedule"`
	KeepDaily   int    `json:"keepDaily"`
	KeepWeekly  int    `json:"keepWeekly"`
	KeepMonthly int    `json:"keepMonthly"`
}

func defaultPolicy() Policy {
	return Policy{Schedule: "02:30", KeepDaily: 14, KeepWeekly: 8, KeepMonthly: 24}
}

func (p Policy) valid() error {
	if _, err := time.Parse("15:04", p.Schedule); err != nil || p.KeepDaily < 1 || p.KeepWeekly < 1 || p.KeepMonthly < 1 {
		return ErrInvalid
	}
	return nil
}

type commandError struct {
	code   string
	detail string
}

func (e commandError) Error() string { return "command_failed" }

func ErrorCode(err error) string {
	var commandFailure commandError
	if errors.As(err, &commandFailure) {
		return commandFailure.code
	}
	if errors.Is(err, ErrInvalid) {
		return "backup_invalid_configuration"
	}
	if errors.Is(err, ErrRunning) {
		return "backup_running"
	}
	if errors.Is(err, ErrConfirmation) {
		return "invalid_confirmation"
	}
	if errors.Is(err, ErrSnapshotNotFound) {
		return "snapshot_not_found"
	}
	return "backup_validation_failed"
}

type View struct {
	Repository         string `json:"repository"`
	RcloneConfigSet    bool   `json:"rcloneConfigSet"`
	ResticPasswordSet  bool   `json:"resticPasswordSet"`
	Configured         bool   `json:"configured"`
	Validated          bool   `json:"validated"`
	Running            bool   `json:"running"`
	LastStartedAt      string `json:"lastStartedAt,omitempty"`
	LastFinishedAt     string `json:"lastFinishedAt,omitempty"`
	LastSuccessAt      string `json:"lastSuccessAt,omitempty"`
	LastSnapshotID     string `json:"lastSnapshotId,omitempty"`
	LastError          string `json:"lastError,omitempty"`
	RecoveryState      string `json:"recoveryState,omitempty"`
	RecoverySnapshot   string `json:"recoverySnapshot,omitempty"`
	RecoveryFinishedAt string `json:"recoveryFinishedAt,omitempty"`
	RecoveryError      string `json:"recoveryError,omitempty"`
	Policy             Policy `json:"policy"`
	LastValidatedAt    string `json:"lastValidatedAt,omitempty"`
	LastDrillAt        string `json:"lastDrillAt,omitempty"`
}

type Snapshot struct {
	ID      string `json:"id"`
	ShortID string `json:"shortId"`
	Time    string `json:"time"`
}

type RecoveryInput struct {
	SnapshotID   string `json:"snapshotId"`
	Confirmation string `json:"confirmation"`
}

type recoveryRequest struct {
	SnapshotID string `json:"snapshotId"`
}

type recoveryStatus struct {
	State      string `json:"state"`
	SnapshotID string `json:"snapshotId"`
	FinishedAt string `json:"finishedAt,omitempty"`
	Error      string `json:"error,omitempty"`
}

type settings struct {
	Version        int    `json:"version"`
	Repository     string `json:"repository"`
	RcloneConfig   string `json:"rcloneConfig"`
	ResticPassword string `json:"resticPassword"`
	Policy         Policy `json:"policy"`
}

type Service struct {
	mu           sync.Mutex
	cfg          config.Runtime
	path         string
	workRoot     string
	settings     settings
	validated    bool
	configuring  bool
	running      bool
	status       View
	ctx          context.Context
	logger       *slog.Logger
	shutdown     func()
	quiesce      func() func()
	lastSchedule string
}

func New(ctx context.Context, cfg config.Runtime, logger *slog.Logger) (*Service, error) {
	configPath, err := filepath.Abs(cfg.ConfigPath)
	if err != nil {
		return nil, err
	}
	cfg.ConfigPath = configPath
	workRoot := cfg.BackupWorkRoot
	if workRoot == "" {
		workRoot = "/app/data/backup-work"
	}
	workRoot, err = filepath.Abs(workRoot)
	if err != nil {
		return nil, err
	}
	cfg.BackupWorkRoot = workRoot
	service := &Service{cfg: cfg, path: filepath.Join(filepath.Dir(cfg.ConfigPath), "backup.json"), workRoot: workRoot, ctx: ctx, logger: logger}
	if workRoot == string(filepath.Separator) {
		return nil, fmt.Errorf("invalid backup work directory")
	}
	data, err := os.ReadFile(service.path)
	if errors.Is(err, os.ErrNotExist) {
		service.startScheduler()
		return service, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &service.settings); err != nil {
		return nil, fmt.Errorf("read backup configuration: %w", err)
	}
	if service.settings.Policy.Schedule == "" {
		service.settings.Policy = defaultPolicy()
	}
	if err := service.settings.valid(); err != nil {
		return nil, err
	}
	service.validated = true
	service.loadRecoveryStatus()
	service.startScheduler()
	return service, nil
}

func (s *Service) SetQuiesce(quiesce func() func()) {
	s.mu.Lock()
	s.quiesce = quiesce
	s.mu.Unlock()
}

func (s *Service) SetShutdown(shutdown func()) {
	s.mu.Lock()
	s.shutdown = shutdown
	s.mu.Unlock()
}

func (s *Service) View() View {
	s.mu.Lock()
	defer s.mu.Unlock()
	view := s.status
	view.Repository = s.settings.Repository
	view.RcloneConfigSet = s.settings.RcloneConfig != ""
	view.ResticPasswordSet = s.settings.ResticPassword != ""
	view.Configured = view.Repository != "" && view.RcloneConfigSet && view.ResticPasswordSet
	view.Validated = s.validated
	view.Running = s.running
	view.Policy = s.settings.Policy
	return view
}

func (s *Service) SaveAndValidate(ctx context.Context, input SettingsInput) (View, error) {
	s.mu.Lock()
	if s.running || s.configuring {
		s.mu.Unlock()
		return s.View(), ErrRunning
	}
	s.configuring = true
	s.validated = false
	next := s.settings
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.configuring = false
		s.mu.Unlock()
	}()
	next.Version = 1
	next.Repository = strings.TrimSpace(input.Repository)
	if input.RcloneConfig != nil && *input.RcloneConfig != "" {
		next.RcloneConfig = strings.TrimSpace(*input.RcloneConfig) + "\n"
	}
	if input.ResticPassword != nil && *input.ResticPassword != "" {
		next.ResticPassword = *input.ResticPassword
	}
	if input.Policy != nil {
		next.Policy = *input.Policy
	}
	if next.Policy.Schedule == "" {
		next.Policy = defaultPolicy()
	}
	if err := next.valid(); err != nil {
		return s.View(), err
	}
	if s.destinationOverlapsSource(next) {
		return s.View(), ErrInvalid
	}
	if err := s.validateRepository(ctx, next); err != nil {
		return s.View(), err
	}
	if err := writeSettings(s.path, next); err != nil {
		return s.View(), err
	}
	s.mu.Lock()
	s.settings = next
	s.validated = true
	s.status.LastError = ""
	s.mu.Unlock()
	return s.View(), nil
}

func (s *Service) startScheduler() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case now := <-ticker.C:
				s.startIfDue(now.UTC())
			}
		}
	}()
}

func (s *Service) startIfDue(now time.Time) {
	s.mu.Lock()
	policy := s.settings.Policy
	due := policy.Schedule == now.Format("15:04") && s.lastSchedule != now.Format("2006-01-02") && s.validated && !s.running && !s.configuring
	if due {
		s.lastSchedule = now.Format("2006-01-02")
	}
	s.mu.Unlock()
	if due {
		if err := s.Start(); err != nil {
			s.logger.Error("geplande backup kon niet starten", "error", safeStage(err))
		}
	}
}

func (s *Service) Validate(ctx context.Context) (View, error) {
	s.mu.Lock()
	if s.running || s.configuring {
		s.mu.Unlock()
		return s.View(), ErrRunning
	}
	s.configuring = true
	s.validated = false
	current := s.settings
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.configuring = false
		s.mu.Unlock()
	}()
	if err := current.valid(); err != nil {
		return s.View(), err
	}
	if s.destinationOverlapsSource(current) {
		return s.View(), ErrInvalid
	}
	if err := s.validateRepository(ctx, current); err != nil {
		return s.View(), err
	}
	s.mu.Lock()
	s.validated = true
	s.status.LastError = ""
	s.mu.Unlock()
	return s.View(), nil
}

func (s *Service) Start() error {
	s.mu.Lock()
	if !s.validated {
		s.mu.Unlock()
		return ErrNotReady
	}
	if s.configuring {
		s.mu.Unlock()
		return ErrRunning
	}
	if s.running {
		s.mu.Unlock()
		return ErrRunning
	}
	current := s.settings
	s.running = true
	s.status.LastStartedAt = time.Now().UTC().Format(time.RFC3339)
	s.status.LastFinishedAt = ""
	s.status.LastError = ""
	s.mu.Unlock()

	go func() {
		snapshotID, err := s.run(s.ctx, current)
		now := time.Now().UTC().Format(time.RFC3339)
		s.mu.Lock()
		s.running = false
		s.status.LastFinishedAt = now
		if err != nil {
			s.status.LastError = "backup_failed"
			s.logger.Error("system backup failed", "stage", safeStage(err))
		} else {
			s.status.LastSuccessAt = now
			s.status.LastSnapshotID = snapshotID
		}
		s.mu.Unlock()
	}()
	return nil
}

func (s *Service) Snapshots(ctx context.Context) ([]Snapshot, error) {
	s.mu.Lock()
	if s.running || s.configuring {
		s.mu.Unlock()
		return nil, ErrRunning
	}
	current := s.settings
	s.mu.Unlock()
	if err := current.valid(); err != nil {
		return nil, err
	}
	runtime, cleanup, err := s.runtimeFiles(current, false)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	var stored []struct {
		ID      string    `json:"id"`
		ShortID string    `json:"short_id"`
		Time    time.Time `json:"time"`
	}
	output, err := commandOutput(ctx, runtime.env, "restic", "snapshots", "--tag", backupTag, "--json")
	if err != nil {
		return nil, fmt.Errorf("repository: %w", err)
	}
	if err := json.Unmarshal(output, &stored); err != nil {
		return nil, fmt.Errorf("repository: invalid snapshot list")
	}
	items := make([]Snapshot, 0, len(stored))
	for i := len(stored) - 1; i >= 0; i-- {
		items = append(items, Snapshot{ID: stored[i].ID, ShortID: stored[i].ShortID, Time: stored[i].Time.UTC().Format(time.RFC3339)})
	}
	return items, nil
}

func (s *Service) ScheduleRecovery(ctx context.Context, input RecoveryInput) error {
	if input.Confirmation != "HERSTEL" {
		return ErrConfirmation
	}
	if !snapshotIDExpr.MatchString(input.SnapshotID) {
		return ErrInvalid
	}
	snapshots, err := s.Snapshots(ctx)
	if err != nil {
		return err
	}
	found := false
	for _, snapshot := range snapshots {
		if snapshot.ID == input.SnapshotID || snapshot.ShortID == input.SnapshotID {
			input.SnapshotID = snapshot.ID
			found = true
			break
		}
	}
	if !found {
		return ErrSnapshotNotFound
	}
	request := recoveryRequest{SnapshotID: input.SnapshotID}
	if err := writeJSONAtomic(s.recoveryRequestPath(), request); err != nil {
		return err
	}
	if err := s.writeRecoveryStatus(recoveryStatus{State: "scheduled", SnapshotID: input.SnapshotID}); err != nil {
		_ = os.Remove(s.recoveryRequestPath())
		return err
	}
	s.mu.Lock()
	shutdown := s.shutdown
	s.mu.Unlock()
	if shutdown == nil {
		_ = os.Remove(s.recoveryRequestPath())
		return fmt.Errorf("recovery restart is unavailable")
	}
	go func() {
		time.Sleep(500 * time.Millisecond)
		shutdown()
	}()
	return nil
}

func (s *Service) recoveryRequestPath() string {
	return filepath.Join(filepath.Dir(s.path), "recovery.json")
}

func (s *Service) recoveryStatusPath() string {
	return filepath.Join(filepath.Dir(s.path), "recovery-status.json")
}

func (s *Service) loadRecoveryStatus() {
	data, err := os.ReadFile(s.recoveryStatusPath())
	if err != nil {
		return
	}
	var status recoveryStatus
	if json.Unmarshal(data, &status) != nil {
		return
	}
	s.status.RecoveryState = status.State
	s.status.RecoverySnapshot = status.SnapshotID
	s.status.RecoveryFinishedAt = status.FinishedAt
	s.status.RecoveryError = status.Error
}

func (s *Service) writeRecoveryStatus(status recoveryStatus) error {
	if err := writeJSONAtomic(s.recoveryStatusPath(), status); err != nil {
		return err
	}
	s.mu.Lock()
	s.status.RecoveryState = status.State
	s.status.RecoverySnapshot = status.SnapshotID
	s.status.RecoveryFinishedAt = status.FinishedAt
	s.status.RecoveryError = status.Error
	s.mu.Unlock()
	return nil
}

func (v settings) valid() error {
	if v.Repository == "" || v.RcloneConfig == "" || v.ResticPassword == "" || strings.ContainsAny(v.Repository, "\r\n\x00") {
		return ErrInvalid
	}
	remote, path, ok := strings.Cut(v.Repository, ":")
	if !ok || !remoteNameExpr.MatchString(remote) || strings.Trim(path, "/") == "" || strings.Contains(v.RcloneConfig, "["+sourceRemoteName+"]") {
		return ErrInvalid
	}
	if !strings.Contains(v.RcloneConfig, "["+remote+"]") {
		return ErrInvalid
	}
	if v.Policy.Schedule == "" {
		v.Policy = defaultPolicy()
	}
	if err := v.Policy.valid(); err != nil {
		return err
	}
	return nil
}

func (s *Service) destinationOverlapsSource(value settings) bool {
	remote, repositoryPath, _ := strings.Cut(value.Repository, ":")
	remoteType := rcloneRemoteValue(value.RcloneConfig, remote, "type")
	if remoteType == "local" && filepath.IsAbs(repositoryPath) {
		return pathsOverlap(repositoryPath, s.workRoot) || pathsOverlap(repositoryPath, filepath.Dir(s.cfg.ConfigPath))
	}
	if remoteType != "s3" {
		return false
	}
	endpoint := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(rcloneRemoteValue(value.RcloneConfig, remote, "endpoint"), "https://"), "http://"), "/")
	bucket := strings.Split(strings.Trim(repositoryPath, "/"), "/")[0]
	return strings.EqualFold(endpoint, strings.TrimSuffix(s.cfg.S3Endpoint, "/")) && bucket == s.cfg.S3Bucket
}

func rcloneRemoteValue(configuration, remote, key string) string {
	section := ""
	for _, line := range strings.Split(configuration, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		if section != remote || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(name) == key {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func pathsOverlap(left, right string) bool {
	left, leftErr := filepath.Abs(filepath.Clean(left))
	right, rightErr := filepath.Abs(filepath.Clean(right))
	if leftErr != nil || rightErr != nil {
		return true
	}
	relative, err := filepath.Rel(left, right)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return true
	}
	relative, err = filepath.Rel(right, left)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func writeSettings(path string, value settings) error {
	return writeJSONAtomic(path, value)
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".backup-*.json")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}

func (s *Service) validateRepository(ctx context.Context, current settings) error {
	runtime, cleanup, err := s.runtimeFiles(current, false)
	if err != nil {
		return err
	}
	defer cleanup()
	if _, err := commandOutput(ctx, runtime.env, "rclone", "mkdir", current.Repository); err != nil {
		return fmt.Errorf("destination: %w", err)
	}
	output, err := commandOutput(ctx, runtime.env, "rclone", "lsf", current.Repository, "--max-depth", "1")
	if err != nil {
		return fmt.Errorf("destination: %w", err)
	}
	if strings.TrimSpace(string(output)) == "" {
		if _, err := commandOutput(ctx, runtime.env, "restic", "init"); err != nil {
			return fmt.Errorf("repository: %w", err)
		}
		return nil
	}
	if _, err := commandOutput(ctx, runtime.env, "restic", "snapshots", "--json"); err != nil {
		return fmt.Errorf("repository: %w", err)
	}
	return nil
}

type runtimeFiles struct {
	dir        string
	rclonePath string
	env        []string
}

func (s *Service) runtimeFiles(current settings, includeSource bool) (runtimeFiles, func(), error) {
	if err := os.MkdirAll(s.workRoot, 0o700); err != nil {
		return runtimeFiles{}, func() {}, err
	}
	dir, err := os.MkdirTemp(s.workRoot, ".runtime-")
	if err != nil {
		return runtimeFiles{}, func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	rcloneConfig := current.RcloneConfig
	if includeSource {
		rcloneConfig += sourceRemote(s.cfg)
	}
	rclonePath := filepath.Join(dir, "rclone.conf")
	passwordPath := filepath.Join(dir, "restic-password")
	if err := os.WriteFile(rclonePath, []byte(rcloneConfig), 0o600); err != nil {
		cleanup()
		return runtimeFiles{}, func() {}, err
	}
	if err := os.WriteFile(passwordPath, []byte(current.ResticPassword), 0o600); err != nil {
		cleanup()
		return runtimeFiles{}, func() {}, err
	}
	env := append(os.Environ(), "RCLONE_CONFIG="+rclonePath, "RESTIC_PASSWORD_FILE="+passwordPath, "RESTIC_REPOSITORY=rclone:"+current.Repository)
	return runtimeFiles{dir: dir, rclonePath: rclonePath, env: env}, cleanup, nil
}

func (s *Service) run(ctx context.Context, current settings) (string, error) {
	if current.Policy.Schedule == "" {
		current.Policy = defaultPolicy()
	}
	s.mu.Lock()
	quiesce := s.quiesce
	s.mu.Unlock()
	if quiesce != nil {
		release := quiesce()
		defer release()
	}
	runtime, cleanupRuntime, err := s.runtimeFiles(current, true)
	if err != nil {
		return "", fmt.Errorf("runtime: %w", err)
	}
	defer cleanupRuntime()
	stage, err := os.MkdirTemp(s.workRoot, "backup-")
	if err != nil {
		return "", fmt.Errorf("staging: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := os.MkdirAll(filepath.Join(stage, "database"), 0o700); err != nil {
		return "", fmt.Errorf("staging: %w", err)
	}
	if err := s.backupDatabase(ctx, runtime.env, stage); err != nil {
		return "", fmt.Errorf("database: %w", err)
	}
	if err := s.backupObjects(ctx, runtime.env, stage); err != nil {
		return "", fmt.Errorf("objects: %w", err)
	}
	if err := s.backupConfiguration(stage); err != nil {
		return "", fmt.Errorf("configuration: %w", err)
	}
	manifest := map[string]any{"version": 2, "createdAt": time.Now().UTC().Format(time.RFC3339), "databaseBackend": "sqlite", "objectBackend": "s3", "policy": current.Policy}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(stage, "manifest.json"), data, 0o600); err != nil {
		return "", fmt.Errorf("manifest: %w", err)
	}
	output, err := commandOutput(ctx, runtime.env, "restic", "backup", stage, "--tag", backupTag, "--json")
	if err != nil {
		return "", fmt.Errorf("restic: %w", err)
	}
	snapshot := snapshotID(output)
	if snapshot == "" {
		return "", fmt.Errorf("restic: missing snapshot identifier")
	}
	if err := s.validateSnapshot(ctx, runtime.env, snapshot); err != nil {
		return "", fmt.Errorf("validation: %w", err)
	}
	if err := s.applyRetention(ctx, runtime.env, current.Policy); err != nil {
		return "", fmt.Errorf("retention: %w", err)
	}
	s.mu.Lock()
	now := time.Now().UTC().Format(time.RFC3339)
	s.status.LastValidatedAt = now
	s.status.LastDrillAt = now
	s.mu.Unlock()
	return snapshot, nil
}

func RunPendingRecovery(ctx context.Context, cfg config.Runtime, logger *slog.Logger) (bool, error) {
	service, err := New(ctx, cfg, logger)
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(service.recoveryRequestPath())
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	var request recoveryRequest
	if json.Unmarshal(data, &request) != nil || !snapshotIDExpr.MatchString(request.SnapshotID) {
		return true, fmt.Errorf("invalid recovery request")
	}
	defer os.Remove(service.recoveryRequestPath())
	if err := service.writeRecoveryStatus(recoveryStatus{State: "running", SnapshotID: request.SnapshotID}); err != nil {
		return true, err
	}
	if err := service.RecoverNow(ctx, request.SnapshotID, true); err != nil {
		_ = service.writeRecoveryStatus(recoveryStatus{State: "failed", SnapshotID: request.SnapshotID, FinishedAt: time.Now().UTC().Format(time.RFC3339), Error: "recovery_failed"})
		return true, err
	}
	if err := service.writeRecoveryStatus(recoveryStatus{State: "succeeded", SnapshotID: request.SnapshotID, FinishedAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		return true, err
	}
	return true, nil
}

func (s *Service) RecoverNow(ctx context.Context, snapshotID string, safetyBackup bool) error {
	if !snapshotIDExpr.MatchString(snapshotID) {
		return ErrInvalid
	}
	current := s.settings
	if err := current.valid(); err != nil {
		return err
	}
	if safetyBackup {
		if _, err := s.run(ctx, current); err != nil {
			return fmt.Errorf("safety backup: %w", err)
		}
	}
	runtime, cleanupRuntime, err := s.runtimeFiles(current, true)
	if err != nil {
		return fmt.Errorf("runtime: %w", err)
	}
	defer cleanupRuntime()
	restoreRoot, err := os.MkdirTemp(s.workRoot, "recovery-")
	if err != nil {
		return fmt.Errorf("staging: %w", err)
	}
	defer os.RemoveAll(restoreRoot)
	if _, err := commandOutput(ctx, runtime.env, "restic", "restore", snapshotID, "--target", restoreRoot); err != nil {
		return fmt.Errorf("restic: %w", err)
	}
	payload, manifest, err := recoveryPayload(restoreRoot)
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	if manifest.DatabaseBackend != "sqlite" || manifest.ObjectBackend != "s3" {
		return fmt.Errorf("manifest: storage backend mismatch")
	}
	if err := s.restoreObjects(ctx, runtime.env, payload); err != nil {
		return fmt.Errorf("objects: %w", err)
	}
	// Replace the database last: a failed object restore never makes document
	// metadata point to a partially restored object set.
	if err := s.restoreDatabase(ctx, runtime.env, payload); err != nil {
		return fmt.Errorf("database: %w", err)
	}
	if err := s.restoreConfiguration(payload); err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	return nil
}

type backupManifest struct {
	Version         int    `json:"version"`
	DatabaseBackend string `json:"databaseBackend"`
	ObjectBackend   string `json:"objectBackend"`
}

func recoveryPayload(root string) (string, backupManifest, error) {
	var payload string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "manifest.json" {
			if payload != "" {
				return fmt.Errorf("multiple manifests found")
			}
			payload = filepath.Dir(path)
		}
		return nil
	})
	if err != nil {
		return "", backupManifest{}, err
	}
	if payload == "" {
		return "", backupManifest{}, fmt.Errorf("manifest not found")
	}
	data, err := os.ReadFile(filepath.Join(payload, "manifest.json"))
	if err != nil {
		return "", backupManifest{}, err
	}
	var manifest backupManifest
	if json.Unmarshal(data, &manifest) != nil || manifest.Version != 2 || manifest.DatabaseBackend == "" || manifest.ObjectBackend == "" {
		return "", backupManifest{}, fmt.Errorf("invalid manifest")
	}
	return payload, manifest, nil
}

func (s *Service) restoreDatabase(ctx context.Context, env []string, payload string) error {
	source := filepath.Join(payload, "database")
	return replaceFile(filepath.Join(source, "hohoho.db"), s.cfg.DatabasePath)
}

func (s *Service) restoreObjects(ctx context.Context, env []string, payload string) error {
	source := filepath.Join(payload, "objects")
	return commandRun(ctx, env, nil, "rclone", "sync", "--delete-during", source, sourceRemoteName+":"+s.cfg.S3Bucket)
}

func (s *Service) restoreConfiguration(payload string) error {
	source := filepath.Join(payload, "configuration", filepath.Base(s.cfg.ConfigPath))
	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return replaceFile(source, s.cfg.ConfigPath)
}

func (s *Service) backupDatabase(ctx context.Context, env []string, stage string) error {
	destination := filepath.Join(stage, "database")
	db, err := sql.Open("sqlite", s.cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer db.Close()
	path := filepath.Join(destination, "hohoho.db")
	escaped := strings.ReplaceAll(path, "'", "''")
	_, err = db.ExecContext(ctx, "vacuum into '"+escaped+"'")
	return err
}

func (s *Service) backupObjects(ctx context.Context, env []string, stage string) error {
	destination := filepath.Join(stage, "objects")
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	return commandRun(ctx, env, nil, "rclone", "copy", sourceRemoteName+":"+s.cfg.S3Bucket, destination)
}

func (s *Service) backupConfiguration(stage string) error {
	destination := filepath.Join(stage, "configuration")
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	for _, source := range []string{s.cfg.ConfigPath, s.path} {
		if err := copyFile(source, filepath.Join(destination, filepath.Base(source))); err != nil {
			if errors.Is(err, os.ErrNotExist) && source == s.cfg.ConfigPath {
				continue
			}
			return err
		}
	}
	return nil
}

func (s *Service) applyRetention(ctx context.Context, env []string, policy Policy) error {
	return commandRun(ctx, env, nil, "restic", "forget", "--tag", backupTag, "--keep-daily", fmt.Sprint(policy.KeepDaily), "--keep-weekly", fmt.Sprint(policy.KeepWeekly), "--keep-monthly", fmt.Sprint(policy.KeepMonthly), "--prune")
}

// validateSnapshot checks repository data and performs a non-destructive restore
// drill. It proves the snapshot contains a readable SQLite database before it is
// reported as successful.
func (s *Service) validateSnapshot(ctx context.Context, env []string, snapshot string) error {
	if err := commandRun(ctx, env, nil, "restic", "check", "--read-data-subset=5%"); err != nil {
		return err
	}
	drill, err := os.MkdirTemp(s.workRoot, "restore-drill-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(drill)
	if err := commandRun(ctx, env, nil, "restic", "restore", snapshot, "--target", drill); err != nil {
		return err
	}
	payload, manifest, err := recoveryPayload(drill)
	if err != nil || manifest.DatabaseBackend != "sqlite" || manifest.ObjectBackend != "s3" {
		if err != nil {
			return err
		}
		return fmt.Errorf("unexpected backup manifest")
	}
	db, err := sql.Open("sqlite", filepath.Join(payload, "database", "hohoho.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	var integrity string
	if err := db.QueryRowContext(ctx, "pragma integrity_check").Scan(&integrity); err != nil {
		return err
	}
	if integrity != "ok" {
		return fmt.Errorf("sqlite integrity check: %s", integrity)
	}
	return nil
}

func sourceRemote(cfg config.Runtime) string {
	scheme := "http"
	if cfg.S3UseTLS {
		scheme = "https"
	}
	return fmt.Sprintf("\n[%s]\ntype = s3\nprovider = Other\naccess_key_id = %s\nsecret_access_key = %s\nendpoint = %s://%s\nregion = %s\nforce_path_style = %t\n", sourceRemoteName, cfg.S3AccessKey, cfg.S3SecretKey, scheme, cfg.S3Endpoint, cfg.S3Region, cfg.S3PathStyle)
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	return output.Close()
}

func replaceFile(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".recovery-file-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		return err
	}
	defer os.Remove(temporaryPath)
	if err := copyFile(source, temporaryPath); err != nil {
		return err
	}
	return os.Rename(temporaryPath, destination)
}

func commandOutput(ctx context.Context, env []string, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Env = env
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, commandError{code: classifyCommandError(name, stderr.String()), detail: strings.TrimSpace(stderr.String())}
	}
	return stdout.Bytes(), nil
}

func commandRun(ctx context.Context, env []string, stdout io.Writer, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Env = env
	command.Stdout = stdout
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return commandError{code: classifyCommandError(name, stderr.String()), detail: strings.TrimSpace(stderr.String())}
	}
	return nil
}

func classifyCommandError(command, output string) string {
	message := strings.ToLower(output)
	switch {
	case strings.Contains(message, "code=8002") || strings.Contains(message, "correct domain"):
		return "proton_username_invalid"
	case strings.Contains(message, "two-factor") || strings.Contains(message, "2fa") || strings.Contains(message, "totp"):
		return "proton_2fa_required"
	case strings.Contains(message, "failed to reveal") || strings.Contains(message, "base64 decode") || strings.Contains(message, "obscur"):
		return "rclone_password_format"
	case strings.Contains(message, "wrong password") || strings.Contains(message, "no key found"):
		return "restic_password_invalid"
	case strings.Contains(message, "429") || strings.Contains(message, "too many requests"):
		return "destination_rate_limited"
	case strings.Contains(message, "401") || strings.Contains(message, "403") || strings.Contains(message, "unauthorized") || strings.Contains(message, "login") || strings.Contains(message, "credential"):
		return "destination_authentication_failed"
	case command == "rclone":
		return "rclone_destination_failed"
	case command == "restic":
		return "restic_repository_failed"
	default:
		return "backup_command_failed"
	}
}

func snapshotID(output []byte) string {
	for _, line := range strings.Split(string(output), "\n") {
		var event struct {
			MessageType string `json:"message_type"`
			SnapshotID  string `json:"snapshot_id"`
		}
		if json.Unmarshal([]byte(line), &event) == nil && event.MessageType == "summary" {
			return event.SnapshotID
		}
	}
	return ""
}

func safeStage(err error) string {
	message := err.Error()
	if stage, _, ok := strings.Cut(message, ":"); ok {
		return stage
	}
	return "unknown"
}
