package sync

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/mcpregistry/server/internal/gitstore"
	"github.com/mcpregistry/server/internal/registry"
)

// Manager handles repository synchronization
type Manager struct {
	store        *gitstore.Store
	registry     *registry.Registry
	pollInterval time.Duration
	debounce     time.Duration
	logger       *slog.Logger

	triggerChan chan struct{}
	mu          sync.Mutex
	lastSync    time.Time
	syncing     bool
}

// Config holds sync manager configuration
type Config struct {
	Store        *gitstore.Store
	Registry     *registry.Registry
	PollInterval time.Duration
	Debounce     time.Duration
	Logger       *slog.Logger
}

// NewManager creates a new sync manager
func NewManager(cfg Config) *Manager {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 5 * time.Minute
	}
	if cfg.Debounce <= 0 {
		cfg.Debounce = 10 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Manager{
		store:        cfg.Store,
		registry:     cfg.Registry,
		pollInterval: cfg.PollInterval,
		debounce:     cfg.Debounce,
		logger:       cfg.Logger,
		triggerChan:  make(chan struct{}, 1),
	}
}

// Start begins the sync manager polling loop
func (m *Manager) Start(ctx context.Context) {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	m.logger.Info("sync manager started",
		"poll_interval", m.pollInterval,
		"debounce", m.debounce,
	)

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("sync manager stopped")
			return

		case <-ticker.C:
			m.doSync(ctx, "poll")

		case <-m.triggerChan:
			// Debounce webhook triggers
			m.debounceSync(ctx)
		}
	}
}

// Trigger initiates a sync (called by webhook handler)
func (m *Manager) Trigger() {
	select {
	case m.triggerChan <- struct{}{}:
		m.logger.Debug("sync triggered")
	default:
		m.logger.Debug("sync already pending")
	}
}

// LastSyncTime returns the last successful sync time
func (m *Manager) LastSyncTime() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastSync
}

// IsSyncing returns whether a sync is in progress
func (m *Manager) IsSyncing() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.syncing
}

func (m *Manager) debounceSync(ctx context.Context) {
	m.mu.Lock()
	if time.Since(m.lastSync) < m.debounce {
		m.mu.Unlock()
		m.logger.Debug("sync debounced", "last_sync", m.lastSync)
		return
	}
	m.mu.Unlock()

	m.doSync(ctx, "webhook")
}

func (m *Manager) doSync(ctx context.Context, source string) {
	m.mu.Lock()
	if m.syncing {
		m.mu.Unlock()
		m.logger.Debug("sync already in progress")
		return
	}
	m.syncing = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.syncing = false
		m.mu.Unlock()
	}()

	start := time.Now()
	m.logger.Info("starting sync", "source", source)

	// Pull with retry
	changed, err := m.store.PullWithRetry(ctx, 3)
	if err != nil {
		m.logger.Error("sync failed",
			"source", source,
			"error", err,
			"duration", time.Since(start),
		)
		return
	}

	if !changed {
		m.logger.Debug("no changes detected", "source", source)
		m.mu.Lock()
		m.lastSync = time.Now()
		m.mu.Unlock()
		return
	}

	// Refresh registry (reloads index and clears cache)
	if err := m.registry.Refresh(); err != nil {
		m.logger.Error("failed to refresh registry",
			"source", source,
			"error", err,
		)
		return
	}

	m.mu.Lock()
	m.lastSync = time.Now()
	m.mu.Unlock()

	m.logger.Info("sync completed",
		"source", source,
		"commit", m.store.CurrentCommit(),
		"server_count", m.registry.ServerCount(),
		"duration", time.Since(start),
	)
}
