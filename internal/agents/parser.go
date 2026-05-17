// Package agents provides parsing for AGENTS.md context engineering files.
// AGENTS.md is the configuration standard for AI-native projects, defining
// how automated agents should behave within the project context.
package agents

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Priyasharma620064/kubedriftguard/internal/models"
)

// AgentsConfig represents the parsed configuration from AGENTS.md.
type AgentsConfig struct {
	// RemediationStrategies maps severity levels to remediation actions.
	RemediationStrategies map[models.DriftSeverity]models.RemediationAction

	// Boundaries defines safety constraints for the remediation engine.
	Boundaries []string

	// ProtectedNamespaces lists namespaces where remediation is forbidden.
	ProtectedNamespaces []string

	// SkipAnnotation is the annotation key that disables drift scanning on a resource.
	SkipAnnotation string

	// WebhookURL is the notification endpoint for alerts.
	WebhookURL string

	// SeverityFilter controls which severities trigger notifications.
	SeverityFilter []models.DriftSeverity

	// DefaultInterval is the default scan interval from AGENTS.md.
	DefaultInterval string

	// MaxConcurrentScans is the maximum number of parallel scans.
	MaxConcurrentScans int
}

// DefaultConfig returns a sensible default AgentsConfig.
func DefaultConfig() *AgentsConfig {
	return &AgentsConfig{
		RemediationStrategies: map[models.DriftSeverity]models.RemediationAction{
			models.SeverityCosmetic: models.ActionIgnore,
			models.SeverityConfig:   models.ActionAutoRevert,
			models.SeveritySecurity: models.ActionAlertAndHold,
			models.SeverityCritical: models.ActionAutoRevert,
		},
		Boundaries: []string{
			"Never modify resources in the kube-system namespace",
			"Never delete PersistentVolumeClaims",
			"Always create a backup annotation before reverting",
		},
		ProtectedNamespaces: []string{"kube-system", "kube-public"},
		SkipAnnotation:      "driftguard.io/skip",
		DefaultInterval:     "30s",
		MaxConcurrentScans:  5,
	}
}

// ParseFile reads and parses an AGENTS.md file.
func ParseFile(path string) (*AgentsConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No AGENTS.md — return defaults
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("opening AGENTS.md: %w", err)
	}
	defer file.Close()

	config := DefaultConfig()
	scanner := bufio.NewScanner(file)

	var currentSection string
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Track section headers
		if strings.HasPrefix(trimmed, "## ") {
			currentSection = strings.ToLower(strings.TrimPrefix(trimmed, "## "))
			continue
		}

		// Parse remediation strategy lines: "### severity: action"
		if strings.HasPrefix(trimmed, "### ") && currentSection == "remediation strategies" {
			parts := strings.SplitN(strings.TrimPrefix(trimmed, "### "), ":", 2)
			if len(parts) == 2 {
				severity := models.DriftSeverity(strings.TrimSpace(parts[0]))
				action := parseAction(strings.TrimSpace(parts[1]))
				if severity.IsValid() {
					config.RemediationStrategies[severity] = action
				}
			}
			continue
		}

		// Parse boundary lines (bullet points under ## Boundaries)
		if currentSection == "boundaries" && strings.HasPrefix(trimmed, "- ") {
			boundary := strings.TrimPrefix(trimmed, "- ")
			config.Boundaries = append(config.Boundaries, boundary)

			// Extract protected namespaces from boundary rules
			lower := strings.ToLower(boundary)
			if strings.Contains(lower, "never modify resources in the") {
				ns := extractNamespace(boundary)
				if ns != "" {
					config.ProtectedNamespaces = appendUnique(config.ProtectedNamespaces, ns)
				}
			}

			// Extract skip annotation
			if strings.Contains(lower, "respect") && strings.Contains(lower, "annotation") {
				ann := extractAnnotation(boundary)
				if ann != "" {
					config.SkipAnnotation = ann
				}
			}
			continue
		}

		// Parse notification channel lines
		if currentSection == "notification channels" && strings.HasPrefix(trimmed, "- ") {
			kv := strings.TrimPrefix(trimmed, "- ")
			parts := strings.SplitN(kv, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				switch key {
				case "webhook":
					config.WebhookURL = val
				case "severity_filter":
					for _, s := range strings.Split(val, ",") {
						sev := models.DriftSeverity(strings.TrimSpace(s))
						if sev.IsValid() {
							config.SeverityFilter = append(config.SeverityFilter, sev)
						}
					}
				}
			}
			continue
		}

		// Parse scan configuration
		if currentSection == "scan configuration" && strings.HasPrefix(trimmed, "- ") {
			kv := strings.TrimPrefix(trimmed, "- ")
			parts := strings.SplitN(kv, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				switch key {
				case "default_interval":
					config.DefaultInterval = val
				case "max_concurrent_scans":
					config.MaxConcurrentScans = parseInt(val, 5)
				}
			}
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading AGENTS.md: %w", err)
	}

	return config, nil
}

// GetAction returns the remediation action for a given severity.
func (c *AgentsConfig) GetAction(severity models.DriftSeverity) models.RemediationAction {
	if action, ok := c.RemediationStrategies[severity]; ok {
		return action
	}
	return models.ActionNotify
}

// IsNamespaceProtected checks if a namespace is in the protected list.
func (c *AgentsConfig) IsNamespaceProtected(namespace string) bool {
	for _, ns := range c.ProtectedNamespaces {
		if ns == namespace {
			return true
		}
	}
	return false
}

// ShouldSkipResource checks if a resource has the skip annotation.
func (c *AgentsConfig) ShouldSkipResource(annotations map[string]string) bool {
	if c.SkipAnnotation == "" {
		return false
	}
	val, exists := annotations[c.SkipAnnotation]
	return exists && strings.ToLower(val) == "true"
}

// parseAction converts a string to a RemediationAction.
func parseAction(s string) models.RemediationAction {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ignore":
		return models.ActionIgnore
	case "auto-revert":
		return models.ActionAutoRevert
	case "alert-and-hold":
		return models.ActionAlertAndHold
	case "notify":
		return models.ActionNotify
	default:
		return models.ActionNotify
	}
}

// extractNamespace pulls namespace name from boundary text like:
// "Never modify resources in the `kube-system` namespace"
func extractNamespace(boundary string) string {
	// Look for backtick-quoted namespace
	start := strings.Index(boundary, "`")
	if start != -1 {
		end := strings.Index(boundary[start+1:], "`")
		if end != -1 {
			return boundary[start+1 : start+1+end]
		}
	}
	return ""
}

// extractAnnotation pulls annotation key from boundary text like:
// "Respect `driftguard.io/skip=true` annotation on resources"
func extractAnnotation(boundary string) string {
	start := strings.Index(boundary, "`")
	if start != -1 {
		end := strings.Index(boundary[start+1:], "`")
		if end != -1 {
			ann := boundary[start+1 : start+1+end]
			// Strip =value if present
			if eqIdx := strings.Index(ann, "="); eqIdx != -1 {
				ann = ann[:eqIdx]
			}
			return ann
		}
	}
	return ""
}

// parseInt parses a string to int with a fallback default.
func parseInt(s string, def int) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	if n == 0 {
		return def
	}
	return n
}

// appendUnique appends a string to a slice only if not already present.
func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
