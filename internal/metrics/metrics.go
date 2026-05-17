// Package metrics provides Prometheus instrumentation for KubeDriftGuard.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "kubedriftguard"
)

var (
	// DriftEventsTotal counts the total number of drift events detected.
	DriftEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "drift_events_total",
			Help:      "Total number of drift events detected.",
		},
		[]string{"namespace", "resource_kind", "severity"},
	)

	// RemediationActionsTotal counts the total remediation actions performed.
	RemediationActionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "remediation_actions_total",
			Help:      "Total number of remediation actions performed.",
		},
		[]string{"namespace", "resource_kind", "action", "result"},
	)

	// ScanDurationSeconds tracks the duration of each drift scan cycle.
	ScanDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "scan_duration_seconds",
			Help:      "Duration of drift scan cycles in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"policy_name", "namespace"},
	)

	// ActiveDriftGauge tracks the current number of unresolved drift events.
	ActiveDriftGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "active_drifts",
			Help:      "Current number of unresolved drift events.",
		},
		[]string{"namespace", "resource_kind"},
	)

	// ScannedResourcesTotal counts the total resources scanned.
	ScannedResourcesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "scanned_resources_total",
			Help:      "Total number of Kubernetes resources scanned.",
		},
		[]string{"namespace", "resource_kind"},
	)

	// GitSyncStatus tracks the last Git sync result (1=success, 0=failure).
	GitSyncStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "git_sync_status",
			Help:      "Last Git sync status (1=success, 0=failure).",
		},
		[]string{"repo_url", "branch"},
	)

	// ReconcileErrorsTotal counts controller reconciliation errors.
	ReconcileErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "reconcile_errors_total",
			Help:      "Total number of reconciliation errors.",
		},
		[]string{"policy_name", "namespace"},
	)

	// PolicyInfoGauge is an informational gauge for active policies.
	PolicyInfoGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "policy_info",
			Help:      "Information about active DriftPolicy resources.",
		},
		[]string{"policy_name", "namespace", "scan_interval"},
	)
)

// RecordDriftEvent records metrics for a single drift event.
func RecordDriftEvent(namespace, kind, severity string) {
	DriftEventsTotal.WithLabelValues(namespace, kind, severity).Inc()
	ActiveDriftGauge.WithLabelValues(namespace, kind).Inc()
}

// RecordRemediation records metrics for a remediation action.
func RecordRemediation(namespace, kind, action string, success bool) {
	result := "success"
	if !success {
		result = "failure"
	}
	RemediationActionsTotal.WithLabelValues(namespace, kind, action, result).Inc()
	if success {
		ActiveDriftGauge.WithLabelValues(namespace, kind).Dec()
	}
}

// RecordScan records metrics for a completed scan cycle.
func RecordScan(policyName, namespace string, durationSeconds float64) {
	ScanDurationSeconds.WithLabelValues(policyName, namespace).Observe(durationSeconds)
}

// RecordGitSync records the status of a Git sync operation.
func RecordGitSync(repoURL, branch string, success bool) {
	val := float64(1)
	if !success {
		val = 0
	}
	GitSyncStatus.WithLabelValues(repoURL, branch).Set(val)
}
