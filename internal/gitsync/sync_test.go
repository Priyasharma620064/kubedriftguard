package gitsync

import (
	"testing"
)

func TestRepoKey(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		branch   string
		expected string
	}{
		{
			name:     "https URL with main branch",
			repoURL:  "https://github.com/openkruise/kruise",
			branch:   "main",
			expected: "github.com_openkruise_kruise_main",
		},
		{
			name:     "https URL with .git suffix",
			repoURL:  "https://github.com/openkruise/kruise.git",
			branch:   "master",
			expected: "github.com_openkruise_kruise_master",
		},
		{
			name:     "URL with different branch",
			repoURL:  "https://github.com/openkruise/rollouts",
			branch:   "release-0.5",
			expected: "github.com_openkruise_rollouts_release-0.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repoKey(tt.repoURL, tt.branch)
			if got != tt.expected {
				t.Errorf("repoKey(%q, %q) = %q, want %q", tt.repoURL, tt.branch, got, tt.expected)
			}
		})
	}
}

func TestNewSyncer(t *testing.T) {
	tmpDir := t.TempDir()

	s, err := NewSyncer(tmpDir, 30)
	if err != nil {
		t.Fatalf("NewSyncer() error = %v", err)
	}
	if s == nil {
		t.Fatal("NewSyncer() returned nil")
	}
	if s.cacheDir != tmpDir {
		t.Errorf("cacheDir = %q, want %q", s.cacheDir, tmpDir)
	}
}

func TestLastSyncInfoNotSynced(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := NewSyncer(tmpDir, 30)

	_, _, err := s.LastSyncInfo("https://github.com/example/repo", "main")
	if err == nil {
		t.Error("expected error for unsynced repo, got nil")
	}
}

func TestGetManifestPathsNotSynced(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := NewSyncer(tmpDir, 30)

	_, err := s.GetManifestPaths("https://github.com/example/repo", "main", "")
	if err == nil {
		t.Error("expected error for unsynced repo, got nil")
	}
}
