package registry

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"gopkg.in/yaml.v3"

	"github.com/mcpregistry/server/internal/domain"
	"github.com/mcpregistry/server/internal/gitstore"
)

// Registry provides access to MCP server definitions
type Registry struct {
	store     *gitstore.Store
	cache     *lru.Cache[string, *domain.ServerJSON]
	index     *domain.Index
	indexMu   sync.RWMutex
	cacheSize int
	logger    *slog.Logger

	// Stats
	cacheHits   atomic.Int64
	cacheMisses atomic.Int64
	lastSyncAt  atomic.Value // time.Time
}

// Config holds registry configuration
type Config struct {
	Store     *gitstore.Store
	CacheSize int
	Logger    *slog.Logger
}

// New creates a new registry instance
func New(cfg Config) (*Registry, error) {
	if cfg.Store == nil {
		return nil, errors.New("store is required")
	}
	if cfg.CacheSize <= 0 {
		cfg.CacheSize = 1000
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	cache, err := lru.New[string, *domain.ServerJSON](cfg.CacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create LRU cache: %w", err)
	}

	r := &Registry{
		store:     cfg.Store,
		cache:     cache,
		cacheSize: cfg.CacheSize,
		logger:    cfg.Logger,
	}
	r.lastSyncAt.Store(time.Time{})

	return r, nil
}

// LoadIndex loads and validates the index.yaml file
func (r *Registry) LoadIndex() error {
	r.indexMu.Lock()
	defer r.indexMu.Unlock()

	content, err := r.store.ReadFile("index.yaml")
	if err != nil {
		return fmt.Errorf("index.yaml not found: %w", err)
	}

	var index domain.Index
	if err := yaml.Unmarshal(content, &index); err != nil {
		return fmt.Errorf("failed to parse index.yaml: %w", err)
	}

	if len(index.Servers) == 0 {
		r.logger.Warn("index.yaml contains no servers")
	}

	r.index = &index
	r.lastSyncAt.Store(time.Now())

	r.logger.Info("index loaded",
		"version", index.Version,
		"commit", index.Commit,
		"server_count", len(index.Servers),
	)

	return nil
}

// Refresh reloads the index and invalidates cache
func (r *Registry) Refresh() error {
	// Clear cache before reload
	r.cache.Purge()
	r.cacheHits.Store(0)
	r.cacheMisses.Store(0)

	return r.LoadIndex()
}

// GetServer retrieves a server by name
func (r *Registry) GetServer(name string) (*domain.ServerJSON, error) {
	// Normalize name (URL decode)
	decodedName, err := url.PathUnescape(name)
	if err != nil {
		decodedName = name
	}

	// Check cache first
	if server, ok := r.cache.Get(decodedName); ok {
		r.cacheHits.Add(1)
		return server, nil
	}
	r.cacheMisses.Add(1)

	// Find in index
	r.indexMu.RLock()
	if r.index == nil {
		r.indexMu.RUnlock()
		return nil, errors.New("index not loaded")
	}

	var entry *domain.IndexEntry
	for i := range r.index.Servers {
		if r.index.Servers[i].Name == decodedName {
			entry = &r.index.Servers[i]
			break
		}
	}
	r.indexMu.RUnlock()

	if entry == nil {
		return nil, fmt.Errorf("server not found: %s", decodedName)
	}

	// Load from disk
	content, err := r.store.ReadFile(entry.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read server file: %w", err)
	}

	var server domain.ServerJSON
	if err := yaml.Unmarshal(content, &server); err != nil {
		return nil, fmt.Errorf("failed to parse server file: %w", err)
	}

	// Add to cache
	r.cache.Add(decodedName, &server)

	return &server, nil
}

// ListServers returns a paginated list of servers
func (r *Registry) ListServers(cursor string, limit int) (*domain.ServerListResponse, error) {
	r.indexMu.RLock()
	defer r.indexMu.RUnlock()

	if r.index == nil {
		return nil, errors.New("index not loaded")
	}

	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	// Sort servers by name for consistent pagination
	servers := make([]domain.IndexEntry, len(r.index.Servers))
	copy(servers, r.index.Servers)
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})

	// Find start position
	startIdx := 0
	if cursor != "" {
		for i, s := range servers {
			if s.Name == cursor {
				startIdx = i + 1
				break
			}
		}
	}

	// Collect results
	var results []domain.ServerResponse
	endIdx := startIdx + limit
	if endIdx > len(servers) {
		endIdx = len(servers)
	}

	for i := startIdx; i < endIdx; i++ {
		entry := servers[i]

		// Try to get from cache, otherwise use index info
		server, err := r.GetServer(entry.Name)
		if err != nil {
			// Use minimal info from index if file load fails
			server = &domain.ServerJSON{
				Name:        entry.Name,
				Description: entry.Description,
				Version:     entry.Version,
			}
		}

		results = append(results, domain.ServerResponse{
			Server: *server,
		})
	}

	// Determine next cursor
	var nextCursor string
	if endIdx < len(servers) {
		nextCursor = servers[endIdx-1].Name
	}

	return &domain.ServerListResponse{
		Servers: results,
		Metadata: domain.ListMetadata{
			NextCursor: nextCursor,
			Count:      len(results),
		},
	}, nil
}

// SearchServers searches for servers matching a query
func (r *Registry) SearchServers(query string) ([]domain.IndexEntry, error) {
	r.indexMu.RLock()
	defer r.indexMu.RUnlock()

	if r.index == nil {
		return nil, errors.New("index not loaded")
	}

	query = strings.ToLower(query)
	var results []domain.IndexEntry

	for _, entry := range r.index.Servers {
		if strings.Contains(strings.ToLower(entry.Name), query) ||
			strings.Contains(strings.ToLower(entry.Description), query) {
			results = append(results, entry)
		}
	}

	return results, nil
}

// ServerCount returns the number of servers in the index
func (r *Registry) ServerCount() int {
	r.indexMu.RLock()
	defer r.indexMu.RUnlock()

	if r.index == nil {
		return 0
	}
	return len(r.index.Servers)
}

// IndexStatus returns the current index status
func (r *Registry) IndexStatus() string {
	r.indexMu.RLock()
	defer r.indexMu.RUnlock()

	if r.index == nil {
		return "not_loaded"
	}
	return "valid"
}

// CacheStats returns current cache statistics
func (r *Registry) CacheStats() *domain.CacheStats {
	hits := r.cacheHits.Load()
	misses := r.cacheMisses.Load()
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return &domain.CacheStats{
		Size:     r.cache.Len(),
		Capacity: r.cacheSize,
		HitRate:  hitRate,
	}
}

// LastSyncAt returns the last sync timestamp
func (r *Registry) LastSyncAt() time.Time {
	return r.lastSyncAt.Load().(time.Time)
}

// Store returns the underlying git store
func (r *Registry) Store() *gitstore.Store {
	return r.store
}
