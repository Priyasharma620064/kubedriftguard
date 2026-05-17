// Package controller implements the Kubernetes reconciliation loop for DriftPolicy resources.
package controller

import (
	"context"
	"fmt"
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	driftguardv1alpha1 "github.com/Priyasharma620064/kubedriftguard/api/v1alpha1"
	"github.com/Priyasharma620064/kubedriftguard/internal/agents"
	"github.com/Priyasharma620064/kubedriftguard/internal/classifier"
	"github.com/Priyasharma620064/kubedriftguard/internal/differ"
	"github.com/Priyasharma620064/kubedriftguard/internal/gitsync"
	"github.com/Priyasharma620064/kubedriftguard/internal/metrics"
	"github.com/Priyasharma620064/kubedriftguard/internal/models"
	"github.com/Priyasharma620064/kubedriftguard/internal/remediation"
)

// DriftPolicyReconciler reconciles a DriftPolicy object.
type DriftPolicyReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	GitSyncer    *gitsync.Syncer
	AgentsConfig *agents.AgentsConfig
	DryRun       bool
	MaxRetries   int
}

// Reconcile is the main reconciliation loop for DriftPolicy resources.
// It performs: Git sync → manifest loading → live state fetch → diff → classify → remediate.
func (r *DriftPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	startTime := time.Now()
	log.Printf("[reconcile] starting reconciliation for %s/%s", req.Namespace, req.Name)

	// 1. Fetch the DriftPolicy
	var policy driftguardv1alpha1.DriftPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		metrics.ReconcileErrorsTotal.WithLabelValues(req.Name, req.Namespace).Inc()
		return ctrl.Result{}, fmt.Errorf("fetching DriftPolicy: %w", err)
	}

	// Check if suspended
	if policy.Spec.Suspend {
		log.Printf("[reconcile] policy %s/%s is suspended — skipping", req.Namespace, req.Name)
		return ctrl.Result{}, nil
	}

	// 2. Sync Git repository
	gitSpec := policy.Spec.GitSource
	branch := gitSpec.Branch
	if branch == "" {
		branch = "main"
	}

	localPath, err := r.GitSyncer.Sync(ctx, gitSpec.RepoURL, branch)
	if err != nil {
		metrics.RecordGitSync(gitSpec.RepoURL, branch, false)
		r.updateStatus(ctx, &policy, "Error", fmt.Sprintf("git sync failed: %v", err))
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}
	metrics.RecordGitSync(gitSpec.RepoURL, branch, true)

	// 3. Load desired manifests from Git
	manifestPaths, err := r.GitSyncer.GetManifestPaths(gitSpec.RepoURL, branch, gitSpec.Path)
	if err != nil {
		r.updateStatus(ctx, &policy, "Error", fmt.Sprintf("listing manifests: %v", err))
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}
	_ = localPath // used implicitly through GetManifestPaths

	// 4. Determine target namespaces
	targetNamespaces := policy.Spec.TargetNamespaces
	if len(targetNamespaces) == 0 {
		targetNamespaces = []string{policy.Namespace}
	}

	// 5. Build classifier from policy rules
	var classifierRules []classifier.Rule
	for _, rule := range policy.Spec.Classification.Rules {
		classifierRules = append(classifierRules, classifier.Rule{
			FieldPattern: rule.Field,
			Severity:     models.DriftSeverity(rule.Severity),
		})
	}
	defaultSeverity := models.DriftSeverity(policy.Spec.Classification.DefaultSeverity)
	if defaultSeverity == "" {
		defaultSeverity = models.SeverityConfig
	}
	cls := classifier.NewClassifier(classifierRules, defaultSeverity)

	// 6. Create differ with standard ignored paths
	d := differ.NewDiffer(nil)

	// 7. Scan each watched resource type across target namespaces
	var allEvents []models.DriftEvent
	totalResources := 0

	for _, watchRes := range policy.Spec.WatchResources {
		gvr := schema.GroupVersionResource{
			Group:    watchRes.Group,
			Version:  watchRes.Version,
			Resource: pluralize(watchRes.Kind),
		}
		if gvr.Version == "" {
			gvr.Version = "v1"
		}

		for _, ns := range targetNamespaces {
			// Skip protected namespaces
			if r.AgentsConfig.IsNamespaceProtected(ns) {
				continue
			}

			// List live resources
			liveList := &unstructured.UnstructuredList{}
			liveList.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   watchRes.Group,
				Version: gvr.Version,
				Kind:    watchRes.Kind + "List",
			})

			if err := r.List(ctx, liveList, client.InNamespace(ns)); err != nil {
				log.Printf("[reconcile] error listing %s in %s: %v", watchRes.Kind, ns, err)
				continue
			}

			for _, liveObj := range liveList.Items {
				totalResources++
				metrics.ScannedResourcesTotal.WithLabelValues(ns, watchRes.Kind).Inc()

				// Check skip annotation
				if r.AgentsConfig.ShouldSkipResource(liveObj.GetAnnotations()) {
					continue
				}

				// Find matching desired manifest from Git
				desiredObj := findMatchingManifest(manifestPaths, &liveObj)
				if desiredObj == nil {
					continue // No desired state found — skip
				}

				// Compute diffs
				diffs, err := d.Compare(desiredObj, &liveObj)
				if err != nil {
					log.Printf("[reconcile] diff error for %s/%s: %v", ns, liveObj.GetName(), err)
					continue
				}

				if len(diffs) == 0 {
					continue // No drift
				}

				// Classify diffs
				ref := models.ResourceRef{
					APIVersion: liveObj.GetAPIVersion(),
					Kind:       liveObj.GetKind(),
					Namespace:  liveObj.GetNamespace(),
					Name:       liveObj.GetName(),
				}
				events := cls.Classify(ref, diffs)

				// Record metrics
				for _, evt := range events {
					metrics.RecordDriftEvent(ns, watchRes.Kind, string(evt.Severity))
					evt.DetectedAt = time.Now()
				}

				allEvents = append(allEvents, events...)
			}
		}
	}

	// 8. Apply remediation
	if len(allEvents) > 0 {
		engine := remediation.NewEngine(r.AgentsConfig, r.DryRun, r.MaxRetries)
		results := engine.Remediate(ctx, allEvents)

		for _, result := range results {
			metrics.RecordRemediation(
				result.Event.Resource.Namespace,
				result.Event.Resource.Kind,
				string(result.Action),
				result.Success,
			)
		}
	}

	// 9. Update status
	scanDuration := time.Since(startTime).Seconds()
	metrics.RecordScan(req.Name, req.Namespace, scanDuration)

	now := metav1.Now()
	policy.Status.LastScanTime = &now
	policy.Status.DriftCount = len(allEvents)
	if len(allEvents) > 0 {
		policy.Status.LastDriftTime = &now
		policy.Status.Phase = "Drifted"
	} else {
		policy.Status.Phase = "Synced"
	}
	policy.Status.ObservedGeneration = policy.Generation

	if err := r.Status().Update(ctx, &policy); err != nil {
		log.Printf("[reconcile] failed to update status: %v", err)
	}

	log.Printf("[reconcile] completed %s/%s: scanned=%d drifted=%d duration=%.2fs",
		req.Namespace, req.Name, totalResources, len(allEvents), scanDuration)

	// Requeue after the configured scan interval
	requeueAfter := 30 * time.Second
	if policy.Spec.ScanInterval != "" {
		if d, err := time.ParseDuration(policy.Spec.ScanInterval); err == nil {
			requeueAfter = d
		}
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// SetupWithManager registers the controller with the manager.
func (r *DriftPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&driftguardv1alpha1.DriftPolicy{}).
		Complete(r)
}

// updateStatus is a helper to set phase and message on the policy status.
func (r *DriftPolicyReconciler) updateStatus(ctx context.Context, policy *driftguardv1alpha1.DriftPolicy, phase, message string) {
	policy.Status.Phase = phase
	now := metav1.Now()
	policy.Status.LastScanTime = &now
	policy.Status.ObservedGeneration = policy.Generation

	if err := r.Status().Update(ctx, policy); err != nil {
		log.Printf("[reconcile] failed to update status to %s: %v", phase, err)
	}
}

// findMatchingManifest searches for a desired manifest matching the live object.
// In a full implementation, this would parse YAML files and match by GVK + namespace + name.
func findMatchingManifest(manifestPaths []string, live *unstructured.Unstructured) *unstructured.Unstructured {
	// TODO: Implement full YAML parsing and matching.
	// For now, this is a placeholder that returns nil (no match found).
	// The actual implementation will:
	// 1. Parse each YAML file into unstructured objects
	// 2. Match by apiVersion + kind + namespace + name
	// 3. Return the first match
	_ = manifestPaths
	_ = live
	return nil
}

// pluralize converts a Kind name to its plural resource form.
func pluralize(kind string) string {
	lower := ""
	for _, c := range kind {
		if c >= 'A' && c <= 'Z' {
			lower += string(c + 32)
		} else {
			lower += string(c)
		}
	}

	// Common Kubernetes pluralization rules
	switch {
	case endsWith(lower, "s"):
		return lower + "es"
	case endsWith(lower, "y"):
		return lower[:len(lower)-1] + "ies"
	default:
		return lower + "s"
	}
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
