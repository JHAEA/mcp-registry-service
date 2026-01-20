package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mcpregistry/server/internal/domain"
	"github.com/mcpregistry/server/internal/registry"
)

// Build information (set at compile time)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// Handlers provides HTTP handlers for the API
type Handlers struct {
	registry *registry.Registry
	logger   *slog.Logger
}

// NewHandlers creates a new handlers instance
func NewHandlers(reg *registry.Registry, logger *slog.Logger) *Handlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handlers{
		registry: reg,
		logger:   logger,
	}
}

// Health returns health check information
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	store := h.registry.Store()

	status := "ok"
	indexStatus := h.registry.IndexStatus()
	if indexStatus != "valid" {
		status = "degraded"
	}

	resp := domain.HealthResponse{
		Status:      status,
		RepoURL:     store.RepoURL(),
		Branch:      store.Branch(),
		CommitSHA:   store.CurrentCommit(),
		LastSyncAt:  h.registry.LastSyncAt().Format(time.RFC3339),
		IndexStatus: indexStatus,
		ServerCount: h.registry.ServerCount(),
		CacheStats:  h.registry.CacheStats(),
	}

	writeJSON(w, http.StatusOK, resp)
}

// Ping returns a simple pong response
func (h *Handlers) Ping(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, domain.PingResponse{Pong: true})
}

// Version returns build version information
func (h *Handlers) Version(w http.ResponseWriter, r *http.Request) {
	version := Version
	commit := GitCommit
	buildTime := BuildTime

	// Try to get from build info if not set
	if info, ok := debug.ReadBuildInfo(); ok && version == "dev" {
		version = info.Main.Version
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				commit = setting.Value
			case "vcs.time":
				buildTime = setting.Value
			}
		}
	}

	writeJSON(w, http.StatusOK, domain.VersionResponse{
		Version:   version,
		GitCommit: commit,
		BuildTime: buildTime,
	})
}

// ListServers returns a paginated list of servers
func (h *Handlers) ListServers(w http.ResponseWriter, r *http.Request) {
	cursor := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")

	limit := 30
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	resp, err := h.registry.ListServers(cursor, limit)
	if err != nil {
		h.logger.Error("failed to list servers", "error", err)
		writeError(w, http.StatusServiceUnavailable, "Service Unavailable",
			"Index not available. Ensure index.yaml exists and is valid.")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetServer returns a server by name (latest version)
func (h *Handlers) GetServer(w http.ResponseWriter, r *http.Request) {
	serverName := chi.URLParam(r, "serverName")
	if serverName == "" {
		writeError(w, http.StatusBadRequest, "Bad Request", "Server name is required")
		return
	}

	// URL decode the server name
	decodedName, err := url.PathUnescape(serverName)
	if err != nil {
		decodedName = serverName
	}

	server, err := h.registry.GetServer(decodedName)
	if err != nil {
		h.logger.Debug("server not found", "name", decodedName, "error", err)
		writeError(w, http.StatusNotFound, "Not Found",
			"Server not found: "+decodedName)
		return
	}

	resp := domain.ServerResponse{
		Server: *server,
		Meta: &domain.ServerMeta{
			Official: &domain.OfficialMeta{
				Status:      "active",
				PublishedAt: h.registry.LastSyncAt(),
				IsLatest:    true,
			},
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetServerVersions returns available versions for a server
// Since we only support latest, this returns just the current version
func (h *Handlers) GetServerVersions(w http.ResponseWriter, r *http.Request) {
	serverName := chi.URLParam(r, "serverName")
	if serverName == "" {
		writeError(w, http.StatusBadRequest, "Bad Request", "Server name is required")
		return
	}

	decodedName, err := url.PathUnescape(serverName)
	if err != nil {
		decodedName = serverName
	}

	server, err := h.registry.GetServer(decodedName)
	if err != nil {
		writeError(w, http.StatusNotFound, "Not Found",
			"Server not found: "+decodedName)
		return
	}

	// Return single version since we only support latest
	versions := []map[string]interface{}{
		{
			"version":   server.Version,
			"is_latest": true,
		},
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"server_name": server.Name,
		"versions":    versions,
	})
}

// GetServerVersion returns a specific version of a server
// Since we only support latest, any version request returns the current version
func (h *Handlers) GetServerVersion(w http.ResponseWriter, r *http.Request) {
	serverName := chi.URLParam(r, "serverName")
	version := chi.URLParam(r, "version")

	if serverName == "" {
		writeError(w, http.StatusBadRequest, "Bad Request", "Server name is required")
		return
	}

	decodedName, err := url.PathUnescape(serverName)
	if err != nil {
		decodedName = serverName
	}

	server, err := h.registry.GetServer(decodedName)
	if err != nil {
		writeError(w, http.StatusNotFound, "Not Found",
			"Server not found: "+decodedName)
		return
	}

	// If specific version requested and doesn't match, return 404
	// (unless "latest" is requested)
	if version != "latest" && version != server.Version {
		writeError(w, http.StatusNotFound, "Not Found",
			"Version not found. This registry only serves the latest version.")
		return
	}

	resp := domain.ServerResponse{
		Server: *server,
		Meta: &domain.ServerMeta{
			Official: &domain.OfficialMeta{
				Status:      "active",
				PublishedAt: h.registry.LastSyncAt(),
				IsLatest:    true,
			},
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

// NotImplemented returns 501 for write endpoints
func (h *Handlers) NotImplemented(w http.ResponseWriter, r *http.Request) {
	resp := domain.NotImplementedResponse{
		Status:  http.StatusNotImplemented,
		Title:   "Not Implemented",
		Detail:  "This registry is read-only. Server definitions are managed via GitOps workflow.",
		SeeAlso: "Submit a pull request to the registry repository to add or update servers.",
	}

	writeJSON(w, http.StatusNotImplemented, resp)
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, title, detail string) {
	resp := domain.ErrorResponse{
		Status: status,
		Title:  title,
		Detail: detail,
	}
	writeJSON(w, status, resp)
}
