package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mcpregistry/server/internal/api"
	"github.com/mcpregistry/server/internal/config"
	"github.com/mcpregistry/server/internal/github"
	"github.com/mcpregistry/server/internal/gitstore"
	"github.com/mcpregistry/server/internal/middleware"
	"github.com/mcpregistry/server/internal/registry"
	"github.com/mcpregistry/server/internal/sync"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger.Info("starting MCP registry server",
		"repo_url", cfg.RegistryRepoURL,
		"branch", cfg.RegistryBranch,
		"clone_timeout", cfg.CloneTimeout,
		"cache_size", cfg.CacheSize,
	)

	// Initialize GitHub App authentication
	ghAuth, err := github.NewAppAuth(
		cfg.GitHubAppID,
		cfg.GitHubAppPrivateKey,
		cfg.GitHubInstallationID,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize GitHub App auth: %w", err)
	}

	// Create context with clone timeout for initial setup
	cloneCtx, cloneCancel := context.WithTimeout(context.Background(), cfg.CloneTimeout)
	defer cloneCancel()

	// Initialize git store with disk-based storage
	store, err := gitstore.New(gitstore.Config{
		RepoURL:   cfg.RegistryRepoURL,
		Branch:    cfg.RegistryBranch,
		LocalPath: cfg.DataPath,
		Auth:      ghAuth,
		Logger:    logger,
	})
	if err != nil {
		return fmt.Errorf("failed to create git store: %w", err)
	}

	// Perform initial clone
	logger.Info("cloning registry repository", "timeout", cfg.CloneTimeout)
	if err := store.Clone(cloneCtx); err != nil {
		logger.Error("failed to clone repository",
			"error", err,
			"repo_url", cfg.RegistryRepoURL,
			"timeout", cfg.CloneTimeout,
		)
		return fmt.Errorf("failed to clone repository within %s: %w", cfg.CloneTimeout, err)
	}
	logger.Info("repository cloned successfully", "commit", store.CurrentCommit())

	// Initialize server registry with LRU cache
	reg, err := registry.New(registry.Config{
		Store:     store,
		CacheSize: cfg.CacheSize,
		Logger:    logger,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize registry: %w", err)
	}

	// Load and validate index
	if err := reg.LoadIndex(); err != nil {
		logger.Error("failed to load index.yaml",
			"error", err,
			"message", "index.yaml is required - ensure CI generates it on merge",
		)
		return fmt.Errorf("failed to load index: %w", err)
	}
	logger.Info("index loaded", "server_count", reg.ServerCount())

	// Initialize sync manager
	syncMgr := sync.NewManager(sync.Config{
		Store:        store,
		Registry:     reg,
		PollInterval: cfg.PollInterval,
		Debounce:     10 * time.Second,
		Logger:       logger,
	})

	// Initialize observability
	shutdownTracer, err := middleware.InitTracer(cfg.OTLPEndpoint)
	if err != nil {
		logger.Warn("failed to initialize tracer, continuing without tracing", "error", err)
	}

	// Initialize API router
	router := api.NewRouter(api.Config{
		Registry:      reg,
		SyncManager:   syncMgr,
		WebhookSecret: cfg.WebhookSecret,
		Logger:        logger,
	})

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      middleware.Chain(router, logger),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start sync manager
	syncCtx, syncCancel := context.WithCancel(context.Background())
	defer syncCancel()
	go syncMgr.Start(syncCtx)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		logger.Info("HTTP server listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		logger.Info("shutdown signal received")
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	syncCancel() // Stop sync manager

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	if shutdownTracer != nil {
		if err := shutdownTracer(shutdownCtx); err != nil {
			logger.Warn("tracer shutdown error", "error", err)
		}
	}

	logger.Info("server stopped gracefully")
	return nil
}
