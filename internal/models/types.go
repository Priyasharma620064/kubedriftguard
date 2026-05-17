// Package models defines the core domain types for KubeDriftGuard.
package models

import "time"

// ResourceRef identifies a specific Kubernetes resource.
type ResourceRef struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind" yaml:"kind"`
	Namespace  string `json:"namespace" yaml:"namespace"`
	Name       string `json:"name" yaml:"name"`
}

// String returns a human-readable representation of the resource reference.
func (r ResourceRef) String() string {
	if r.Namespace != "" {
		return r.Namespace + "/" + r.Kind + "/" + r.Name
	}
	return r.Kind + "/" + r.Name
}

// GitSource represents a Git repository used as the source of truth.
type GitSource struct {
	RepoURL string `json:"repoURL" yaml:"repoURL"`
	Branch  string `json:"branch" yaml:"branch"`
	Path    string `json:"path" yaml:"path"`
}

// WatchResource defines which Kubernetes resources to monitor for drift.
type WatchResource struct {
	Group   string `json:"group" yaml:"group"`
	Kind    string `json:"kind" yaml:"kind"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

// ClassificationRule maps specific fields to drift severity levels.
type ClassificationRule struct {
	Field    string        `json:"field" yaml:"field"`
	Severity DriftSeverity `json:"severity" yaml:"severity"`
}

// DriftSeverity represents the severity level of a detected drift.
type DriftSeverity string

const (
	SeverityCosmetic DriftSeverity = "cosmetic"
	SeverityConfig   DriftSeverity = "config"
	SeveritySecurity DriftSeverity = "security"
	SeverityCritical DriftSeverity = "critical"
	SeverityUnknown  DriftSeverity = "unknown"
)

// IsValid checks whether the severity is a recognized value.
func (s DriftSeverity) IsValid() bool {
	switch s {
	case SeverityCosmetic, SeverityConfig, SeveritySecurity, SeverityCritical:
		return true
	}
	return false
}

// RemediationAction defines what action to take when drift is detected.
type RemediationAction string

const (
	ActionIgnore       RemediationAction = "ignore"
	ActionAutoRevert   RemediationAction = "auto-revert"
	ActionAlertAndHold RemediationAction = "alert-and-hold"
	ActionNotify       RemediationAction = "notify"
)

// ScanResult captures the outcome of a single drift scan cycle.
type ScanResult struct {
	Resource  ResourceRef   `json:"resource"`
	ScannedAt time.Time     `json:"scannedAt"`
	DriftFound bool         `json:"driftFound"`
	Events    []DriftEvent  `json:"events,omitempty"`
	Error     string        `json:"error,omitempty"`
}

// HealthStatus represents the overall health of a monitored resource.
type HealthStatus string

const (
	HealthSynced  HealthStatus = "Synced"
	HealthDrifted HealthStatus = "Drifted"
	HealthError   HealthStatus = "Error"
	HealthUnknown HealthStatus = "Unknown"
)
