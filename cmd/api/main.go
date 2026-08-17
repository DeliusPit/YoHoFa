package main

import (
	"context"
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
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.FromEnv()
	if cfg.AppEnv == "production" && (cfg.AuthUsername == "" || cfg.AuthPassword == "") {
		logger.Warn("authentication is disabled; set AUTH_USERNAME and AUTH_PASSWORD")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.StoreBackend, cfg.DatabaseURL, cfg.DatabasePath)
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
	var objects objectstore.Store
	if documents != nil {
		objects, err = objectstore.Open(ctx, cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOUseSSL)
		if err != nil {
			logger.Error("open object store", "error", err)
		}
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpserver.New(httpserver.Dependencies{Config: cfg, Store: db, Documents: documents, Assets: assets, Liabilities: liabilities, Cases: cases, Relations: relations, Events: events, Objects: objects, Logger: logger}),
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
