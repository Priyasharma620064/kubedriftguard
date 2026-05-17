// Package classifier provides drift severity classification based on
// configurable rules. It maps drifted field paths to severity levels
// (cosmetic, config, security, critical) using both user-defined rules
// and built-in heuristics for common Kubernetes fields.
package classifier

import (
	"strings"

	"github.com/Priyasharma620064/kubedriftguard/internal/differ"
	"github.com/Priyasharma620064/kubedriftguard/internal/models"
)

// Classifier categorizes drift events by severity.
type Classifier struct {
	rules           []Rule
	defaultSeverity models.DriftSeverity
}

// Rule maps a field pattern to a severity level.
type Rule struct {
	// FieldPattern is a dot-notation path, optionally with wildcards.
	// Examples: "spec.replicas", "spec.template.spec.containers[*].image"
	FieldPattern string

	// Severity is the drift severity to assign when this rule matches.
	Severity models.DriftSeverity
}

// builtinRules provides sensible defaults for common Kubernetes fields.
var builtinRules = []Rule{
	// Critical — changes that directly affect availability
	{FieldPattern: "spec.replicas", Severity: models.SeverityCritical},
	{FieldPattern: "spec.template.spec.containers[*].image", Severity: models.SeverityCritical},
	{FieldPattern: "spec.template.spec.initContainers[*].image", Severity: models.SeverityCritical},
	{FieldPattern: "spec.updateStrategy", Severity: models.SeverityCritical},

	// Security — changes that affect security posture
	{FieldPattern: "spec.template.spec.serviceAccountName", Severity: models.SeveritySecurity},
	{FieldPattern: "spec.template.spec.containers[*].securityContext", Severity: models.SeveritySecurity},
	{FieldPattern: "spec.template.spec.securityContext", Severity: models.SeveritySecurity},
	{FieldPattern: "spec.template.spec.containers[*].resources", Severity: models.SeveritySecurity},
	{FieldPattern: "spec.template.spec.volumes[*].secret", Severity: models.SeveritySecurity},
	{FieldPattern: "spec.template.spec.containers[*].env", Severity: models.SeveritySecurity},
	{FieldPattern: "spec.template.spec.containers[*].envFrom", Severity: models.SeveritySecurity},

	// Cosmetic — changes that don't affect runtime behavior
	{FieldPattern: "metadata.labels", Severity: models.SeverityCosmetic},
	{FieldPattern: "metadata.annotations", Severity: models.SeverityCosmetic},
}

// NewClassifier creates a Classifier with user rules merged on top of builtins.
// User rules take precedence over builtins for the same field pattern.
func NewClassifier(userRules []Rule, defaultSeverity models.DriftSeverity) *Classifier {
	if !defaultSeverity.IsValid() {
		defaultSeverity = models.SeverityConfig
	}

	// Start with builtins, then overlay user rules
	ruleMap := make(map[string]Rule)
	for _, r := range builtinRules {
		ruleMap[r.FieldPattern] = r
	}
	for _, r := range userRules {
		ruleMap[r.FieldPattern] = r
	}

	merged := make([]Rule, 0, len(ruleMap))
	for _, r := range ruleMap {
		merged = append(merged, r)
	}

	return &Classifier{
		rules:           merged,
		defaultSeverity: defaultSeverity,
	}
}

// Classify assigns a severity to each diff field and returns DriftEvents.
func (c *Classifier) Classify(resource models.ResourceRef, diffs []differ.DiffField) []models.DriftEvent {
	events := make([]models.DriftEvent, 0, len(diffs))

	for _, d := range diffs {
		severity := c.classifyField(d.Path)

		event := models.DriftEvent{
			Resource:      resource,
			Severity:      severity,
			Field:         d.Path,
			ExpectedValue: formatValue(d.Expected),
			ActualValue:   formatValue(d.Actual),
			Action:        models.ActionNotify, // Default; remediation engine overrides
		}

		events = append(events, event)
	}

	return events
}

// classifyField finds the best matching rule for a field path.
func (c *Classifier) classifyField(fieldPath string) models.DriftSeverity {
	bestMatch := ""
	bestSeverity := c.defaultSeverity

	for _, rule := range c.rules {
		if matchesPattern(fieldPath, rule.FieldPattern) {
			// Prefer more specific (longer) patterns
			if len(rule.FieldPattern) > len(bestMatch) {
				bestMatch = rule.FieldPattern
				bestSeverity = rule.Severity
			}
		}
	}

	return bestSeverity
}

// matchesPattern checks if a field path matches a rule pattern.
// Supports exact match, prefix match, and wildcard array indices.
func matchesPattern(fieldPath, pattern string) bool {
	// Exact match
	if fieldPath == pattern {
		return true
	}

	// Normalize array indices in fieldPath to [*] for comparison
	normalizedPath := normalizeArrayIndices(fieldPath)
	if normalizedPath == pattern {
		return true
	}

	// Prefix match: "metadata.labels.app" matches rule "metadata.labels"
	if strings.HasPrefix(fieldPath, pattern+".") {
		return true
	}
	if strings.HasPrefix(normalizedPath, pattern+".") {
		return true
	}

	return false
}

// normalizeArrayIndices replaces [0], [1], etc. with [*].
func normalizeArrayIndices(path string) string {
	result := path
	for {
		start := strings.Index(result, "[")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "]")
		if end == -1 {
			break
		}
		end = start + end
		result = result[:start] + "[*]" + result[end+1:]
	}
	return result
}

// formatValue converts an interface{} to a string for display.
func formatValue(v interface{}) string {
	if v == nil {
		return "<nil>"
	}
	return strings.TrimSpace(strings.Replace(
		strings.Replace(
			strings.Replace(
				formatInterface(v), "\n", " ", -1),
			"\t", " ", -1),
		"  ", " ", -1))
}

// formatInterface handles formatting of various Go types.
func formatInterface(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []interface{}:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = formatInterface(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]interface{}:
		parts := make([]string, 0, len(val))
		for k, item := range val {
			parts = append(parts, k+":"+formatInterface(item))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return strings.TrimSpace(strings.Replace(
			strings.Replace(
				strings.Replace(
					func() string {
						s := make([]byte, 0, 64)
						s = append(s, []byte(func() string {
							return strings.Replace(
								strings.Replace(
									func() string {
										// Use fmt-free approach to avoid import
										switch v := val.(type) {
										case int:
											return intToString(int64(v))
										case int32:
											return intToString(int64(v))
										case int64:
											return intToString(v)
										case float32:
											return floatToString(float64(v))
										case float64:
											return floatToString(v)
										case bool:
											if v {
												return "true"
											}
											return "false"
										default:
											return "<unknown>"
										}
									}(),
									"\n", " ", -1),
								"\t", " ", -1)
						}())...)
						return string(s)
					}(),
					"\n", " ", -1),
				"\t", " ", -1),
			"  ", " ", -1))
	}
}

// intToString converts int64 to string without fmt.
func intToString(n int64) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	digits := make([]byte, 0, 20)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// floatToString provides a basic float64 to string conversion.
func floatToString(f float64) string {
	// Check if it's a whole number
	if f == float64(int64(f)) {
		return intToString(int64(f))
	}
	// For non-whole numbers, use a simple representation
	whole := int64(f)
	frac := f - float64(whole)
	if frac < 0 {
		frac = -frac
	}
	fracStr := intToString(int64(frac * 1000000))
	// Trim trailing zeros
	fracStr = strings.TrimRight(fracStr, "0")
	if fracStr == "" {
		return intToString(whole)
	}
	return intToString(whole) + "." + fracStr
}
