package agents

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Priyasharma620064/kubedriftguard/internal/models"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.RemediationStrategies[models.SeverityCosmetic] != models.ActionIgnore {
		t.Errorf("cosmetic should be ignore, got %v", cfg.RemediationStrategies[models.SeverityCosmetic])
	}
	if cfg.RemediationStrategies[models.SeverityCritical] != models.ActionAutoRevert {
		t.Errorf("critical should be auto-revert, got %v", cfg.RemediationStrategies[models.SeverityCritical])
	}
	if cfg.SkipAnnotation != "driftguard.io/skip" {
		t.Errorf("expected skip annotation driftguard.io/skip, got %v", cfg.SkipAnnotation)
	}
}

func TestParseFileMissing(t *testing.T) {
	cfg, err := ParseFile("/nonexistent/AGENTS.md")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	// Should return defaults
	if cfg.RemediationStrategies[models.SeverityCosmetic] != models.ActionIgnore {
		t.Error("expected default config when file is missing")
	}
}

func TestParseFile(t *testing.T) {
	content := `# AGENTS.md

## Remediation Strategies

### cosmetic: ignore
### config: auto-revert
### security: alert-and-hold
### critical: auto-revert

## Boundaries
- Never modify resources in the ` + "`kube-system`" + ` namespace
- Never modify resources in the ` + "`monitoring`" + ` namespace
- Never delete PersistentVolumeClaims
- Respect ` + "`driftguard.io/skip=true`" + ` annotation on resources

## Notification Channels
- webhook: https://hooks.example.com/alerts
- severity_filter: security,critical

## Scan Configuration
- default_interval: 60s
- max_concurrent_scans: 10
`

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "AGENTS.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	// Check remediation strategies
	if cfg.GetAction(models.SeverityCosmetic) != models.ActionIgnore {
		t.Errorf("cosmetic action: got %v, want ignore", cfg.GetAction(models.SeverityCosmetic))
	}
	if cfg.GetAction(models.SeveritySecurity) != models.ActionAlertAndHold {
		t.Errorf("security action: got %v, want alert-and-hold", cfg.GetAction(models.SeveritySecurity))
	}
	if cfg.GetAction(models.SeverityCritical) != models.ActionAutoRevert {
		t.Errorf("critical action: got %v, want auto-revert", cfg.GetAction(models.SeverityCritical))
	}

	// Check protected namespaces
	if !cfg.IsNamespaceProtected("kube-system") {
		t.Error("expected kube-system to be protected")
	}
	if !cfg.IsNamespaceProtected("monitoring") {
		t.Error("expected monitoring to be protected")
	}
	if cfg.IsNamespaceProtected("default") {
		t.Error("expected default to NOT be protected")
	}

	// Check notification
	if cfg.WebhookURL != "https://hooks.example.com/alerts" {
		t.Errorf("webhook URL: got %q", cfg.WebhookURL)
	}
	if len(cfg.SeverityFilter) != 2 {
		t.Fatalf("expected 2 severity filters, got %d", len(cfg.SeverityFilter))
	}

	// Check scan config
	if cfg.DefaultInterval != "60s" {
		t.Errorf("default interval: got %q, want 60s", cfg.DefaultInterval)
	}
	if cfg.MaxConcurrentScans != 10 {
		t.Errorf("max concurrent scans: got %d, want 10", cfg.MaxConcurrentScans)
	}
}

func TestShouldSkipResource(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name        string
		annotations map[string]string
		expected    bool
	}{
		{"no annotations", map[string]string{}, false},
		{"skip=true", map[string]string{"driftguard.io/skip": "true"}, true},
		{"skip=false", map[string]string{"driftguard.io/skip": "false"}, false},
		{"skip=True", map[string]string{"driftguard.io/skip": "True"}, true},
		{"other annotation", map[string]string{"app": "nginx"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.ShouldSkipResource(tt.annotations)
			if got != tt.expected {
				t.Errorf("ShouldSkipResource(%v) = %v, want %v", tt.annotations, got, tt.expected)
			}
		})
	}
}

func TestGetActionUnknownSeverity(t *testing.T) {
	cfg := DefaultConfig()
	action := cfg.GetAction(models.SeverityUnknown)
	if action != models.ActionNotify {
		t.Errorf("expected notify for unknown severity, got %v", action)
	}
}
