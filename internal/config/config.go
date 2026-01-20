package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
type Config struct {
	// Registry repository settings
	RegistryRepoURL string
	RegistryBranch  string

	// GitHub App authentication
	GitHubAppID          int64
	GitHubAppPrivateKey  []byte
	GitHubInstallationID int64

	// Webhook settings
	WebhookSecret string

	// Sync settings
	PollInterval time.Duration
	CloneTimeout time.Duration

	// Storage settings
	DataPath  string
	CacheSize int

	// Server settings
	Port int

	// Observability
	OTLPEndpoint string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		// Defaults
		RegistryBranch: "main",
		PollInterval:   5 * time.Minute,
		CloneTimeout:   2 * time.Minute,
		DataPath:       "/data",
		CacheSize:      1000,
		Port:           8080,
	}

	// Required: Registry repo URL
	cfg.RegistryRepoURL = os.Getenv("REGISTRY_REPO_URL")
	if cfg.RegistryRepoURL == "" {
		return nil, fmt.Errorf("REGISTRY_REPO_URL is required")
	}

	// Optional: Branch
	if v := os.Getenv("REGISTRY_BRANCH"); v != "" {
		cfg.RegistryBranch = v
	}

	// Required: GitHub App credentials
	appIDStr := os.Getenv("GITHUB_APP_ID")
	if appIDStr == "" {
		return nil, fmt.Errorf("GITHUB_APP_ID is required")
	}
	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_APP_ID: %w", err)
	}
	cfg.GitHubAppID = appID

	// Private key can be provided as file path or direct value
	privateKeyPath := os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH")
	privateKeyValue := os.Getenv("GITHUB_APP_PRIVATE_KEY")
	if privateKeyPath != "" {
		key, err := os.ReadFile(privateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file: %w", err)
		}
		cfg.GitHubAppPrivateKey = key
	} else if privateKeyValue != "" {
		cfg.GitHubAppPrivateKey = []byte(privateKeyValue)
	} else {
		return nil, fmt.Errorf("GITHUB_APP_PRIVATE_KEY or GITHUB_APP_PRIVATE_KEY_PATH is required")
	}

	installIDStr := os.Getenv("GITHUB_INSTALLATION_ID")
	if installIDStr == "" {
		return nil, fmt.Errorf("GITHUB_INSTALLATION_ID is required")
	}
	installID, err := strconv.ParseInt(installIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_INSTALLATION_ID: %w", err)
	}
	cfg.GitHubInstallationID = installID

	// Required: Webhook secret
	cfg.WebhookSecret = os.Getenv("WEBHOOK_SECRET")
	if cfg.WebhookSecret == "" {
		return nil, fmt.Errorf("WEBHOOK_SECRET is required")
	}

	// Optional: Poll interval
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid POLL_INTERVAL: %w", err)
		}
		cfg.PollInterval = d
	}

	// Optional: Clone timeout
	if v := os.Getenv("CLONE_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid CLONE_TIMEOUT: %w", err)
		}
		cfg.CloneTimeout = d
	}

	// Optional: Data path
	if v := os.Getenv("DATA_PATH"); v != "" {
		cfg.DataPath = v
	}

	// Optional: Cache size
	if v := os.Getenv("CACHE_SIZE"); v != "" {
		size, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid CACHE_SIZE: %w", err)
		}
		cfg.CacheSize = size
	}

	// Optional: Port
	if v := os.Getenv("PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT: %w", err)
		}
		cfg.Port = port
	}

	// Optional: OTLP endpoint for tracing
	cfg.OTLPEndpoint = os.Getenv("OTLP_ENDPOINT")

	return cfg, nil
}
