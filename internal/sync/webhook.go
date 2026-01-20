package sync

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// WebhookHandler handles GitHub webhook events
type WebhookHandler struct {
	secret  []byte
	manager *Manager
	branch  string
	logger  *slog.Logger
}

// PushEvent represents a GitHub push event payload
type PushEvent struct {
	Ref        string `json:"ref"`
	Before     string `json:"before"`
	After      string `json:"after"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	Pusher struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"pusher"`
	Commits []struct {
		ID       string   `json:"id"`
		Message  string   `json:"message"`
		Added    []string `json:"added"`
		Removed  []string `json:"removed"`
		Modified []string `json:"modified"`
	} `json:"commits"`
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(secret string, manager *Manager, branch string, logger *slog.Logger) *WebhookHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &WebhookHandler{
		secret:  []byte(secret),
		manager: manager,
		branch:  branch,
		logger:  logger,
	}
}

// ServeHTTP handles incoming webhook requests
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body
	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		h.logger.Error("failed to read webhook body", "error", err)
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Validate signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if !h.validateSignature(signature, body) {
		h.logger.Warn("invalid webhook signature",
			"remote_addr", r.RemoteAddr,
		)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// Check event type
	eventType := r.Header.Get("X-GitHub-Event")
	deliveryID := r.Header.Get("X-GitHub-Delivery")

	h.logger.Info("webhook received",
		"event", eventType,
		"delivery_id", deliveryID,
	)

	// Only process push events
	if eventType != "push" {
		h.logger.Debug("ignoring non-push event", "event", eventType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ignored", "reason": "not a push event"}`))
		return
	}

	// Parse push event
	var event PushEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.logger.Error("failed to parse push event", "error", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Check if push is to our branch
	expectedRef := "refs/heads/" + h.branch
	if event.Ref != expectedRef {
		h.logger.Debug("ignoring push to different branch",
			"ref", event.Ref,
			"expected", expectedRef,
		)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ignored", "reason": "different branch"}`))
		return
	}

	// Log commit info
	h.logger.Info("push event for tracked branch",
		"ref", event.Ref,
		"before", event.Before[:8],
		"after", event.After[:8],
		"commit_count", len(event.Commits),
		"pusher", event.Pusher.Name,
	)

	// Trigger sync
	h.manager.Trigger()

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status": "accepted"}`))
}

func (h *WebhookHandler) validateSignature(signature string, body []byte) bool {
	if signature == "" {
		return false
	}

	// Signature format: sha256=<hex>
	parts := strings.SplitN(signature, "=", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		return false
	}

	mac := hmac.New(sha256.New, h.secret)
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(parts[1]), []byte(expectedMAC))
}
