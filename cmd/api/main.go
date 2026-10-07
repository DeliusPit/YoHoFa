package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"hohoho/internal/config"
	"hohoho/internal/httpserver"
	"hohoho/internal/store"
	"hohoho/internal/systembackup"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("Applicatie kon niet worden gestart", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg := config.RuntimeFromEnv()
	if err := cfg.Validate(); err != nil {
		return err
	}
	backupCfg := backupConfigFromRuntime(cfg)
	if recovered, err := systembackup.RunPendingRecovery(ctx, backupCfg, logger); err != nil {
		return err
	} else if recovered {
		logger.Info("Recovery uitgevoerd voor start")
	}
	db, err := store.OpenCore(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer db.Close()

	// De S3-client wordt uit de opgeslagen runtimeconfiguratie geopend en kan
	// via het configuratiescherm warm worden verwisseld; er is geen filesystem-backval.
	objectStorage := httpserver.NewCoreObjectStorage()
	if cfg.S3Endpoint != "" {
		objectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		client, err := httpserver.OpenCoreObjectStore(objectCtx, cfg)
		cancel()
		if err != nil {
			logger.Warn("S3-documentopslag is nog niet bereikbaar", "error", err)
		}
		objectStorage.Swap(client)
	}
	backupService, err := systembackup.New(ctx, backupCfg, logger)
	if err != nil {
		return err
	}
	coreDeps := httpserver.CoreDependencies{Config: cfg, Store: db, Objects: objectStorage, SystemBackup: backupService, Logger: logger}
	// De PDF-AI-analyse blijft bewust los van de opslag: zonder URL is er
	// geen analyzer en werkt uploaden gewoon door.
	if analyzer := pdfAIAnalyzer(cfg, objectStorage); analyzer != nil {
		coreDeps.DocumentAnalyzer = analyzer
		coreDeps.DocumentAnalysisQueue = make(chan store.CoreDocument, 16)
	}
	server := &http.Server{
		Addr: cfg.HTTPAddr, ReadHeaderTimeout: 5 * time.Second,
		Handler: httpserver.NewCore(coreDeps),
	}
	backupService.SetShutdown(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	})
	if coreDeps.DocumentAnalyzer != nil {
		go httpserver.RunCoreDocumentAnalysis(ctx, coreDeps)
	}
	result := make(chan error, 1)
	go func() {
		logger.Info("HoHoHo v3 gestart", "addr", cfg.HTTPAddr, "database", "sqlite", "schema_version", store.CoreSchemaVersion)
		result <- server.ListenAndServe()
	}()
	select {
	case err := <-result:
		if err != http.ErrServerClosed {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

func backupConfigFromRuntime(cfg config.Runtime) config.Config {
	configPath := cfg.ConfigPath
	if !filepath.IsAbs(configPath) {
		configPath, _ = filepath.Abs(configPath)
	}
	backupWorkRoot := cfg.BackupWorkRoot
	if !filepath.IsAbs(backupWorkRoot) {
		backupWorkRoot, _ = filepath.Abs(backupWorkRoot)
	}
	return config.Config{
		AppEnv: cfg.AppEnv, HTTPAddr: cfg.HTTPAddr, CORSOrigin: cfg.CORSOrigin, WebDir: cfg.WebDir,
		AuthUsername: cfg.AuthUsername, AuthPassword: cfg.AuthPassword,
		StoreBackend: "sqlite", DatabasePath: cfg.DatabasePath,
		ObjectBackend: "s3", S3Endpoint: cfg.S3Endpoint, S3AccessKey: cfg.S3AccessKey, S3SecretKey: cfg.S3SecretKey,
		S3Bucket: cfg.S3Bucket, S3UseTLS: cfg.S3UseTLS, S3Region: cfg.S3Region, S3PathStyle: cfg.S3PathStyle, S3CreateBucket: cfg.S3CreateBucket,
		PDFAIURL: cfg.PDFAIURL, ConfigPath: configPath, BackupWorkRoot: backupWorkRoot,
	}
}
