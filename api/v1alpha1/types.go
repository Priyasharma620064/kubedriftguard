package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DriftPolicySpec defines the desired state of DriftPolicy.
type DriftPolicySpec struct {
	// TargetNamespaces is the list of namespaces to monitor.
	// If empty, the policy's own namespace is used.
	// +optional
	TargetNamespaces []string `json:"targetNamespaces,omitempty"`

	// WatchResources defines which Kubernetes resource types to monitor.
	// +kubebuilder:validation:MinItems=1
	WatchResources []WatchResourceRef `json:"watchResources"`

	// GitSource defines the Git repository used as the source of truth.
	GitSource GitSourceSpec `json:"gitSource"`

	// ScanInterval defines how often to check for drift.
	// Defaults to 30s.
	// +optional
	ScanInterval string `json:"scanInterval,omitempty"`

	// Classification defines rules for categorizing drift severity.
	// +optional
	Classification ClassificationSpec `json:"classification,omitempty"`

	// Remediation defines the self-healing behavior.
	// +optional
	Remediation RemediationSpec `json:"remediation,omitempty"`

	// Suspend, if true, pauses all drift scanning for this policy.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// WatchResourceRef identifies a Kubernetes resource type to monitor.
type WatchResourceRef struct {
	// Group is the API group (e.g., "apps", "apps.kruise.io").
	Group string `json:"group"`

	// Kind is the resource kind (e.g., "Deployment", "CloneSet").
	Kind string `json:"kind"`

	// Version is the API version. Defaults to the preferred version.
	// +optional
	Version string `json:"version,omitempty"`
}

// GitSourceSpec defines how to access the Git source of truth.
type GitSourceSpec struct {
	// RepoURL is the Git repository URL.
	RepoURL string `json:"repoURL"`

	// Branch is the Git branch to track. Defaults to "main".
	// +optional
	Branch string `json:"branch,omitempty"`

	// Path is the subdirectory within the repo containing manifests.
	// +optional
	Path string `json:"path,omitempty"`
}

// ClassificationSpec defines how to classify drift severity.
type ClassificationSpec struct {
	// Rules maps specific field paths to severity levels.
	// +optional
	Rules []ClassificationRule `json:"rules,omitempty"`

	// DefaultSeverity is the severity for fields without a specific rule.
	// Defaults to "config".
	// +optional
	DefaultSeverity string `json:"defaultSeverity,omitempty"`
}

// ClassificationRule maps a field path to a severity level.
type ClassificationRule struct {
	// Field is a dot-notation path (e.g., "spec.replicas").
	Field string `json:"field"`

	// Severity is the drift severity for this field.
	// +kubebuilder:validation:Enum=cosmetic;config;security;critical
	Severity string `json:"severity"`
}

// RemediationSpec defines self-healing behavior.
type RemediationSpec struct {
	// Strategy maps severity levels to remediation actions.
	// +optional
	Strategy map[string]string `json:"strategy,omitempty"`

	// DryRun, if true, logs actions without applying them.
	// +optional
	DryRun bool `json:"dryRun,omitempty"`
}

// DriftPolicyStatus defines the observed state of DriftPolicy.
type DriftPolicyStatus struct {
	// Phase indicates the current phase of the policy.
	// +optional
	Phase string `json:"phase,omitempty"`

	// LastScanTime is the timestamp of the last completed scan.
	// +optional
	LastScanTime *metav1.Time `json:"lastScanTime,omitempty"`

	// LastDriftTime is the timestamp of the last detected drift.
	// +optional
	LastDriftTime *metav1.Time `json:"lastDriftTime,omitempty"`

	// DriftCount is the total number of active (unresolved) drift events.
	// +optional
	DriftCount int `json:"driftCount,omitempty"`

	// Conditions contains the latest available observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the most recent generation observed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Drifts",type=integer,JSONPath=`.status.driftCount`
// +kubebuilder:printcolumn:name="Last Scan",type=date,JSONPath=`.status.lastScanTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// DriftPolicy is the Schema for the driftpolicies API.
// It defines which resources to monitor, the source of truth,
// and how to classify and remediate detected drift.
type DriftPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DriftPolicySpec   `json:"spec,omitempty"`
	Status DriftPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DriftPolicyList contains a list of DriftPolicy.
type DriftPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DriftPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DriftPolicy{}, &DriftPolicyList{})
}
