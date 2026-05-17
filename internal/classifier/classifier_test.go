package classifier

import (
	"testing"

	"github.com/Priyasharma620064/kubedriftguard/internal/differ"
	"github.com/Priyasharma620064/kubedriftguard/internal/models"
)

func TestNewClassifier(t *testing.T) {
	c := NewClassifier(nil, models.SeverityConfig)
	if c == nil {
		t.Fatal("expected non-nil classifier")
	}
	if c.defaultSeverity != models.SeverityConfig {
		t.Errorf("expected default severity %q, got %q", models.SeverityConfig, c.defaultSeverity)
	}
}

func TestNewClassifierInvalidDefault(t *testing.T) {
	c := NewClassifier(nil, "invalid")
	if c.defaultSeverity != models.SeverityConfig {
		t.Errorf("expected fallback to 'config', got %q", c.defaultSeverity)
	}
}

func TestClassifyBuiltinRules(t *testing.T) {
	c := NewClassifier(nil, models.SeverityConfig)
	resource := models.ResourceRef{Kind: "Deployment", Namespace: "default", Name: "nginx"}

	tests := []struct {
		field    string
		expected models.DriftSeverity
	}{
		{"spec.replicas", models.SeverityCritical},
		{"spec.template.spec.containers[0].image", models.SeverityCritical},
		{"spec.template.spec.serviceAccountName", models.SeveritySecurity},
		{"metadata.labels", models.SeverityCosmetic},
		{"metadata.labels.app", models.SeverityCosmetic},
		{"metadata.annotations", models.SeverityCosmetic},
		{"spec.template.spec.nodeName", models.SeverityConfig}, // falls to default
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			diffs := []differ.DiffField{
				{Path: tt.field, Expected: "a", Actual: "b"},
			}
			events := c.Classify(resource, diffs)
			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}
			if events[0].Severity != tt.expected {
				t.Errorf("field %q: expected severity %q, got %q", tt.field, tt.expected, events[0].Severity)
			}
		})
	}
}

func TestClassifyUserRulesOverride(t *testing.T) {
	// User wants labels to be treated as critical (override builtin cosmetic)
	userRules := []Rule{
		{FieldPattern: "metadata.labels", Severity: models.SeverityCritical},
	}
	c := NewClassifier(userRules, models.SeverityConfig)
	resource := models.ResourceRef{Kind: "Deployment", Namespace: "default", Name: "test"}

	diffs := []differ.DiffField{
		{Path: "metadata.labels.app", Expected: "v1", Actual: "v2"},
	}
	events := c.Classify(resource, diffs)

	if events[0].Severity != models.SeverityCritical {
		t.Errorf("expected user rule override to critical, got %q", events[0].Severity)
	}
}

func TestClassifyMultipleDiffs(t *testing.T) {
	c := NewClassifier(nil, models.SeverityConfig)
	resource := models.ResourceRef{Kind: "CloneSet", Namespace: "prod", Name: "web"}

	diffs := []differ.DiffField{
		{Path: "spec.replicas", Expected: int64(3), Actual: int64(1)},
		{Path: "metadata.labels.version", Expected: "v1", Actual: "v2"},
		{Path: "spec.template.spec.containers[0].resources", Expected: "limits", Actual: "none"},
	}

	events := c.Classify(resource, diffs)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	severities := map[models.DriftSeverity]int{}
	for _, e := range events {
		severities[e.Severity]++
	}

	if severities[models.SeverityCritical] != 1 {
		t.Errorf("expected 1 critical, got %d", severities[models.SeverityCritical])
	}
	if severities[models.SeverityCosmetic] != 1 {
		t.Errorf("expected 1 cosmetic, got %d", severities[models.SeverityCosmetic])
	}
	if severities[models.SeveritySecurity] != 1 {
		t.Errorf("expected 1 security, got %d", severities[models.SeveritySecurity])
	}
}

func TestNormalizeArrayIndices(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"spec.containers[0].image", "spec.containers[*].image"},
		{"spec.containers[12].ports[3].containerPort", "spec.containers[*].ports[*].containerPort"},
		{"spec.replicas", "spec.replicas"},
		{"metadata.labels", "metadata.labels"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeArrayIndices(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeArrayIndices(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{nil, "<nil>"},
		{"hello", "hello"},
		{true, "true"},
		{int64(42), "42"},
		{float64(3), "3"},
	}

	for _, tt := range tests {
		got := formatValue(tt.input)
		if got != tt.expected {
			t.Errorf("formatValue(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
