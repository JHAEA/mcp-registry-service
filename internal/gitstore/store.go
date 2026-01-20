package gitstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/mcpregistry/server/internal/github"
)

// Store provides disk-based git repository access
type Store struct {
	config       Config
	repo         *git.Repository
	worktree     *git.Worktree
	currentCommit string
	mu           sync.RWMutex
	logger       *slog.Logger
}

// Config holds git store configuration
type Config struct {
	RepoURL   string
	Branch    string
	LocalPath string
	Auth      *github.AppAuth
	Logger    *slog.Logger
}

// New creates a new git store instance
func New(cfg Config) (*Store, error) {
	if cfg.RepoURL == "" {
		return nil, errors.New("repo URL is required")
	}
	if cfg.LocalPath == "" {
		return nil, errors.New("local path is required")
	}
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Store{
		config: cfg,
		logger: cfg.Logger,
	}, nil
}

// Clone performs initial repository clone with context timeout
func (s *Store) Clone(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(s.config.LocalPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Remove existing directory if present (clean clone)
	if err := os.RemoveAll(s.config.LocalPath); err != nil {
		return fmt.Errorf("failed to clean existing directory: %w", err)
	}

	auth, err := s.getAuth(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth: %w", err)
	}

	s.logger.Info("cloning repository",
		"url", s.config.RepoURL,
		"branch", s.config.Branch,
		"path", s.config.LocalPath,
	)

	cloneOpts := &git.CloneOptions{
		URL:           s.config.RepoURL,
		Auth:          auth,
		Depth:         1, // Shallow clone for efficiency
		SingleBranch:  true,
		ReferenceName: plumbing.NewBranchReferenceName(s.config.Branch),
		Progress:      nil,
	}

	repo, err := git.PlainCloneContext(ctx, s.config.LocalPath, false, cloneOpts)
	if err != nil {
		return fmt.Errorf("clone failed: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	s.repo = repo
	s.worktree = worktree

	// Get current commit
	if err := s.updateCurrentCommit(); err != nil {
		return fmt.Errorf("failed to get current commit: %w", err)
	}

	s.logger.Info("clone completed", "commit", s.currentCommit)
	return nil
}

// Pull fetches and merges changes from remote
func (s *Store) Pull(ctx context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.repo == nil {
		return false, errors.New("repository not initialized")
	}

	oldCommit := s.currentCommit

	auth, err := s.getAuth(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get auth: %w", err)
	}

	err = s.worktree.PullContext(ctx, &git.PullOptions{
		RemoteName: "origin",
		Auth:       auth,
		Force:      true,
	})

	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("pull failed: %w", err)
	}

	if err := s.updateCurrentCommit(); err != nil {
		return false, fmt.Errorf("failed to update commit: %w", err)
	}

	changed := oldCommit != s.currentCommit
	if changed {
		s.logger.Info("repository updated",
			"old_commit", oldCommit,
			"new_commit", s.currentCommit,
		)
	}

	return changed, nil
}

// PullWithRetry attempts to pull with exponential backoff
func (s *Store) PullWithRetry(ctx context.Context, maxRetries int) (bool, error) {
	var lastErr error
	backoff := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		changed, err := s.Pull(ctx)
		if err == nil {
			return changed, nil
		}

		lastErr = err
		s.logger.Warn("pull attempt failed",
			"attempt", attempt+1,
			"max_retries", maxRetries,
			"error", err,
			"next_backoff", backoff,
		)

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}

	return false, fmt.Errorf("pull failed after %d retries: %w", maxRetries, lastErr)
}

// ReadFile reads a file from the current HEAD
func (s *Store) ReadFile(path string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}

	fullPath := filepath.Join(s.config.LocalPath, path)
	return os.ReadFile(fullPath)
}

// ListFiles returns all files in a directory
func (s *Store) ListFiles(dir string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}

	fullPath := filepath.Join(s.config.LocalPath, dir)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// FileExists checks if a file exists in the repository
func (s *Store) FileExists(path string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fullPath := filepath.Join(s.config.LocalPath, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

// CurrentCommit returns the current HEAD commit SHA
func (s *Store) CurrentCommit() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentCommit
}

// RepoURL returns the configured repository URL
func (s *Store) RepoURL() string {
	return s.config.RepoURL
}

// Branch returns the configured branch
func (s *Store) Branch() string {
	return s.config.Branch
}

// WalkFiles walks all files in the repository matching a pattern
func (s *Store) WalkFiles(dir string, fn func(path string, content []byte) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.repo == nil {
		return errors.New("repository not initialized")
	}

	ref, err := s.repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	commit, err := s.repo.CommitObject(ref.Hash())
	if err != nil {
		return fmt.Errorf("failed to get commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("failed to get tree: %w", err)
	}

	return tree.Files().ForEach(func(f *object.File) error {
		if dir != "" && !strings.HasPrefix(f.Name, dir) {
			return nil
		}

		reader, err := f.Reader()
		if err != nil {
			return err
		}
		defer reader.Close()

		content, err := io.ReadAll(reader)
		if err != nil {
			return err
		}

		return fn(f.Name, content)
	})
}

func (s *Store) getAuth(ctx context.Context) (*http.BasicAuth, error) {
	token, err := s.config.Auth.Token(ctx)
	if err != nil {
		return nil, err
	}

	return &http.BasicAuth{
		Username: "x-access-token",
		Password: token,
	}, nil
}

func (s *Store) updateCurrentCommit() error {
	ref, err := s.repo.Head()
	if err != nil {
		return err
	}
	s.currentCommit = ref.Hash().String()
	return nil
}
