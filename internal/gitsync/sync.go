// Package gitsync provides Git repository synchronization for KubeDriftGuard.
// It clones and incrementally pulls repositories to maintain a local cache
// of the desired state manifests.
package gitsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// Syncer manages local clones of Git repositories used as source of truth.
type Syncer struct {
	cacheDir string
	timeout  time.Duration

	mu    sync.RWMutex
	repos map[string]*repoState
}

// repoState tracks the local state of a cloned repository.
type repoState struct {
	localPath  string
	lastSynced time.Time
	headCommit string
	repo       *git.Repository
}

// NewSyncer creates a new Git syncer with the given cache directory.
func NewSyncer(cacheDir string, timeout time.Duration) (*Syncer, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}
	return &Syncer{
		cacheDir: cacheDir,
		timeout:  timeout,
		repos:    make(map[string]*repoState),
	}, nil
}

// repoKey generates a unique cache key from repo URL and branch.
func repoKey(repoURL, branch string) string {
	// Normalize the URL to create a filesystem-safe directory name
	name := repoURL
	name = strings.TrimPrefix(name, "https://")
	name = strings.TrimPrefix(name, "http://")
	name = strings.TrimSuffix(name, ".git")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, ":", "_")
	return fmt.Sprintf("%s_%s", name, branch)
}

// Sync clones or pulls the specified repository and branch.
// Returns the local path to the synced repository.
func (s *Syncer) Sync(ctx context.Context, repoURL, branch string) (string, error) {
	if branch == "" {
		branch = "main"
	}

	key := repoKey(repoURL, branch)

	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.repos[key]
	if exists {
		// Pull latest changes
		if err := s.pull(ctx, state, branch); err != nil {
			return "", fmt.Errorf("pulling %s@%s: %w", repoURL, branch, err)
		}
		return state.localPath, nil
	}

	// Clone the repository
	localPath := filepath.Join(s.cacheDir, key)
	state, err := s.clone(ctx, repoURL, branch, localPath)
	if err != nil {
		return "", fmt.Errorf("cloning %s@%s: %w", repoURL, branch, err)
	}

	s.repos[key] = state
	return state.localPath, nil
}

// clone performs a fresh clone of the repository.
func (s *Syncer) clone(ctx context.Context, repoURL, branch, localPath string) (*repoState, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Clean up any existing directory
	_ = os.RemoveAll(localPath)

	cloneOpts := &git.CloneOptions{
		URL:           repoURL,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Depth:         1,
	}

	// Use token from environment if available
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		cloneOpts.Auth = &http.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}

	repo, err := git.PlainCloneContext(ctx, localPath, false, cloneOpts)
	if err != nil {
		return nil, fmt.Errorf("git clone: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("getting HEAD: %w", err)
	}

	return &repoState{
		localPath:  localPath,
		lastSynced: time.Now(),
		headCommit: head.Hash().String(),
		repo:       repo,
	}, nil
}

// pull fetches and merges the latest changes from the remote.
func (s *Syncer) pull(ctx context.Context, state *repoState, branch string) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	w, err := state.repo.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	pullOpts := &git.PullOptions{
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Force:         true,
	}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		pullOpts.Auth = &http.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}

	err = w.PullContext(ctx, pullOpts)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("git pull: %w", err)
	}

	head, err := state.repo.Head()
	if err != nil {
		return fmt.Errorf("getting HEAD after pull: %w", err)
	}

	state.lastSynced = time.Now()
	state.headCommit = head.Hash().String()
	return nil
}

// GetManifestPaths returns all YAML/JSON manifest file paths within the
// specified subdirectory of the synced repository.
func (s *Syncer) GetManifestPaths(repoURL, branch, subPath string) ([]string, error) {
	key := repoKey(repoURL, branch)

	s.mu.RLock()
	state, exists := s.repos[key]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("repository %s@%s not synced", repoURL, branch)
	}

	searchDir := state.localPath
	if subPath != "" {
		searchDir = filepath.Join(searchDir, subPath)
	}

	var paths []string
	err := filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" || ext == ".json" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking manifest directory: %w", err)
	}

	return paths, nil
}

// LastSyncInfo returns the last sync time and HEAD commit for a repo.
func (s *Syncer) LastSyncInfo(repoURL, branch string) (time.Time, string, error) {
	key := repoKey(repoURL, branch)

	s.mu.RLock()
	defer s.mu.RUnlock()

	state, exists := s.repos[key]
	if !exists {
		return time.Time{}, "", fmt.Errorf("repository %s@%s not synced", repoURL, branch)
	}

	return state.lastSynced, state.headCommit, nil
}
