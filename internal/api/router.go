package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mcpregistry/server/internal/registry"
	"github.com/mcpregistry/server/internal/sync"
)

// Config holds API router configuration
type Config struct {
	Registry      *registry.Registry
	SyncManager   *sync.Manager
	WebhookSecret string
	Logger        *slog.Logger
}

// NewRouter creates a new HTTP router with all API routes
func NewRouter(cfg Config) http.Handler {
	r := chi.NewRouter()

	// Base middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)

	// Create handlers
	handlers := NewHandlers(cfg.Registry, cfg.Logger)
	webhookHandler := sync.NewWebhookHandler(
		cfg.WebhookSecret,
		cfg.SyncManager,
		cfg.Registry.Store().Branch(),
		cfg.Logger,
	)

	// Health and utility endpoints (no version prefix)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	// Webhook endpoint
	r.Post("/webhooks/github", webhookHandler.ServeHTTP)

	// API v0.1 routes
	r.Route("/v0.1", func(r chi.Router) {
		// Health endpoints
		r.Get("/health", handlers.Health)
		r.Get("/ping", handlers.Ping)
		r.Get("/version", handlers.Version)

		// Server listing
		r.Get("/servers", handlers.ListServers)

		// Server details - supports both formats
		r.Get("/servers/{serverName}", handlers.GetServer)
		r.Get("/servers/{serverName}/versions", handlers.GetServerVersions)
		r.Get("/servers/{serverName}/versions/{version}", handlers.GetServerVersion)

		// Write endpoints (return 501 Not Implemented)
		r.Post("/publish", handlers.NotImplemented)
		r.Put("/servers/{serverName}/versions/{version}", handlers.NotImplemented)

		// Auth endpoints (return 501 Not Implemented)
		r.Post("/auth/github-at", handlers.NotImplemented)
		r.Post("/auth/github-oidc", handlers.NotImplemented)
		r.Post("/auth/oidc", handlers.NotImplemented)
		r.Post("/auth/dns", handlers.NotImplemented)
		r.Post("/auth/http", handlers.NotImplemented)
		r.Post("/auth/none", handlers.NotImplemented)
	})

	// API v0 routes (alias to v0.1 for compatibility)
	r.Route("/v0", func(r chi.Router) {
		r.Get("/health", handlers.Health)
		r.Get("/ping", handlers.Ping)
		r.Get("/version", handlers.Version)
		r.Get("/servers", handlers.ListServers)
		r.Get("/servers/{serverName}", handlers.GetServer)
		r.Get("/servers/{serverName}/versions", handlers.GetServerVersions)
		r.Get("/servers/{serverName}/versions/{version}", handlers.GetServerVersion)
		r.Post("/publish", handlers.NotImplemented)
	})

	return r
}
