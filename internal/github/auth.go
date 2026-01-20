package github

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
)

// AppAuth provides GitHub App installation authentication
type AppAuth struct {
	transport *ghinstallation.Transport
	mu        sync.RWMutex
}

// NewAppAuth creates a new GitHub App authenticator
func NewAppAuth(appID int64, privateKey []byte, installationID int64) (*AppAuth, error) {
	transport, err := ghinstallation.New(
		http.DefaultTransport,
		appID,
		installationID,
		privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub App transport: %w", err)
	}

	return &AppAuth{
		transport: transport,
	}, nil
}

// Token returns a valid installation access token
// Tokens are automatically refreshed by ghinstallation when expired
func (a *AppAuth) Token(ctx context.Context) (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	token, err := a.transport.Token(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get installation token: %w", err)
	}

	return token, nil
}

// Transport returns the HTTP transport for use with git operations
func (a *AppAuth) Transport() http.RoundTripper {
	return a.transport
}

// TokenExpiry returns when the current token expires (approximate)
func (a *AppAuth) TokenExpiry() time.Time {
	// GitHub App tokens expire after 1 hour
	// ghinstallation handles refresh automatically
	return time.Now().Add(1 * time.Hour)
}
