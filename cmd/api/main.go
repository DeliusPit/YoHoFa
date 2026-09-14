package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"geldzaken/internal/config"
	"geldzaken/internal/httpserver"
	"geldzaken/internal/objectstore"
	"geldzaken/internal/store"
	"geldzaken/internal/systembackup"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	configuration, err := config.NewManager(config.FromEnv())
	if err != nil {
		logger.Error("load system configuration", "error", err)
		os.Exit(1)
	}
	cfg := configuration.Effective()
	if recovered, err := systembackup.RunPendingRecovery(ctx, cfg, logger); err != nil {
		logger.Error("restore system backup", "error", "recovery_failed")
		os.Exit(1)
	} else if recovered {
		configuration, err = config.NewManager(config.FromEnv())
		if err != nil {
			logger.Error("reload recovered system configuration", "error", err)
			os.Exit(1)
		}
		cfg = configuration.Effective()
	}
	if cfg.AppEnv == "production" && (cfg.AuthUsername == "" || cfg.AuthPassword == "") {
		logger.Error("authentication is required in production; set AUTH_USERNAME and AUTH_PASSWORD")
		os.Exit(1)
	}

	db, err := openDatabase(ctx, cfg)
	if err != nil {
		logger.Error("open store", "backend", cfg.StoreBackend, "error", err)
		os.Exit(1)
	}
	defer db.Close()
	documents, _ := db.(store.DocumentRepository)
	assets, _ := db.(store.AssetRepository)
	liabilities, _ := db.(store.LiabilityRepository)
	cases, _ := db.(store.CaseRepository)
	relations, _ := db.(store.EntityRelationRepository)
	events, _ := db.(store.DomainEventRepository)
	objects, err := openObjectStore(ctx, cfg, documents != nil)
	if err != nil {
		logger.Error("open object store; document functionality is unavailable", "error", err)
	}
	backup, err := systembackup.New(ctx, cfg, logger)
	if err != nil {
		logger.Error("load backup configuration", "error", "configuration_unavailable")
	} else {
		backup.SetShutdown(stop)
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpserver.New(httpserver.Dependencies{Config: cfg, Store: db, Documents: documents, Assets: assets, Liabilities: liabilities, Cases: cases, Relations: relations, Events: events, Objects: objects, ObjectStoreRequired: documents != nil, Configuration: configuration, SystemBackup: backup, Logger: logger}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("api listening", "addr", cfg.HTTPAddr, "store_backend", cfg.StoreBackend)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("listen", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "error", err)
	}
}

func openDatabase(ctx context.Context, cfg config.Config) (store.Backend, error) {
	connectionString := cfg.DatabaseURL
	if cfg.StoreBackend == "mysql" {
		connectionString = cfg.MySQLDSN
	}
	retryCtx, cancelRetry := context.WithTimeout(ctx, 60*time.Second)
	defer cancelRetry()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		attemptCtx, cancelAttempt := context.WithTimeout(retryCtx, 15*time.Second)
		db, err := store.Open(attemptCtx, cfg.StoreBackend, connectionString, cfg.DatabasePath)
		cancelAttempt()
		if err == nil {
			return db, nil
		}
		select {
		case <-retryCtx.Done():
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, err
		case <-ticker.C:
		}
	}
}

func openObjectStore(ctx context.Context, cfg config.Config, enabled bool) (objectstore.Store, error) {
	if !enabled {
		return nil, nil
	}
	if cfg.ObjectBackend == "filesystem" {
		objects, err := objectstore.NewFilesystem(cfg.ObjectFilesystemRoot)
		if err != nil {
			return nil, err
		}
		return objects, objects.Ping(ctx)
	}
	if cfg.ObjectBackend != "s3" {
		return nil, fmt.Errorf("unknown object store backend %q", cfg.ObjectBackend)
	}

	retryCtx, cancelRetry := context.WithTimeout(ctx, 60*time.Second)
	defer cancelRetry()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		attemptCtx, cancelAttempt := context.WithTimeout(retryCtx, 15*time.Second)
		objects, err := objectstore.Open(attemptCtx, objectstore.Config{Endpoint: cfg.S3Endpoint, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, Bucket: cfg.S3Bucket, UseTLS: cfg.S3UseTLS, Region: cfg.S3Region, PathStyle: cfg.S3PathStyle, CreateBucket: cfg.S3CreateBucket})
		cancelAttempt()
		if err == nil {
			return objects, nil
		}
		select {
		case <-retryCtx.Done():
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, err
		case <-ticker.C:
		}
	}
}
