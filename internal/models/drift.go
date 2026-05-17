package models

import "time"

// DriftEvent represents a single detected configuration drift.
type DriftEvent struct {
	// ID is a unique identifier for this drift event.
	ID string `json:"id"`

	// Resource identifies the Kubernetes resource that drifted.
	Resource ResourceRef `json:"resource"`

	// Severity indicates how critical this drift is.
	Severity DriftSeverity `json:"severity"`

	// Field is the specific field path that drifted (e.g., "spec.replicas").
	Field string `json:"field"`

	// ExpectedValue is the value declared in the Git source of truth.
	ExpectedValue string `json:"expectedValue"`

	// ActualValue is the current value observed in the live cluster.
	ActualValue string `json:"actualValue"`

	// DetectedAt is when this drift was first detected.
	DetectedAt time.Time `json:"detectedAt"`

	// ResolvedAt is when this drift was remediated (zero if unresolved).
	ResolvedAt time.Time `json:"resolvedAt,omitempty"`

	// Action is the remediation action taken (or pending).
	Action RemediationAction `json:"action"`

	// ActionResult describes the outcome of the remediation attempt.
	ActionResult string `json:"actionResult,omitempty"`

	// Diff contains the full textual diff for this field.
	Diff string `json:"diff,omitempty"`
}

// IsResolved returns true if the drift event has been remediated.
func (e *DriftEvent) IsResolved() bool {
	return !e.ResolvedAt.IsZero()
}

// DriftReport aggregates all drift events from a single scan cycle.
type DriftReport struct {
	// PolicyName is the name of the DriftPolicy that triggered this report.
	PolicyName string `json:"policyName"`

	// Namespace is the namespace of the DriftPolicy.
	Namespace string `json:"namespace"`

	// GeneratedAt is when this report was generated.
	GeneratedAt time.Time `json:"generatedAt"`

	// TotalResources is the number of resources scanned.
	TotalResources int `json:"totalResources"`

	// DriftedResources is the number of resources with drift detected.
	DriftedResources int `json:"driftedResources"`

	// Events contains all individual drift events.
	Events []DriftEvent `json:"events"`

	// SeverityCounts maps severity levels to their occurrence count.
	SeverityCounts map[DriftSeverity]int `json:"severityCounts"`
}

// NewDriftReport creates a new empty DriftReport.
func NewDriftReport(policyName, namespace string) *DriftReport {
	return &DriftReport{
		PolicyName:     policyName,
		Namespace:      namespace,
		GeneratedAt:    time.Now(),
		Events:         make([]DriftEvent, 0),
		SeverityCounts: make(map[DriftSeverity]int),
	}
}

// AddEvent adds a drift event to the report and updates counts.
func (r *DriftReport) AddEvent(event DriftEvent) {
	r.Events = append(r.Events, event)
	r.SeverityCounts[event.Severity]++
}

// HasCritical returns true if any critical drift was detected.
func (r *DriftReport) HasCritical() bool {
	return r.SeverityCounts[SeverityCritical] > 0
}
