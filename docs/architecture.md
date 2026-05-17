# Architecture

KubeDriftGuard follows a modular, pipeline-based architecture where each component is responsible for a single stage of the drift detection and remediation lifecycle.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     KubeDriftGuard Controller                │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────┐  │
│  │  Git Syncer   │  │   Differ     │  │   Classifier      │  │
│  │  (go-git)     │──│  (go-cmp)    │──│  (rule engine)    │  │
│  │  clone/pull   │  │  field diff  │  │  severity assign  │  │
│  └──────────────┘  └──────────────┘  └───────────────────┘  │
│         │                                      │             │
│         ▼                                      ▼             │
│  ┌──────────────┐                    ┌───────────────────┐  │
│  │  Manifest     │                    │   Remediation     │  │
│  │  Loader       │                    │   Engine          │  │
│  │  (YAML parse) │                    │   (AGENTS.md)     │  │
│  └──────────────┘                    └───────────────────┘  │
│                                              │               │
│                    ┌─────────────────────────┤               │
│                    ▼                         ▼               │
│           ┌──────────────┐         ┌───────────────────┐    │
│           │  Prometheus   │         │   Webhook         │    │
│           │  Metrics      │         │   Notifications   │    │
│           └──────────────┘         └───────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

### Git Syncer (`internal/gitsync/`)
- Clones and incrementally pulls Git repositories
- Maintains a local cache of desired state manifests
- Supports GitHub token authentication for private repos
- Thread-safe with `sync.RWMutex` for concurrent access

### Differ (`internal/differ/`)
- Compares desired (Git) vs actual (cluster) Kubernetes objects
- Uses `unstructured.Unstructured` for generic resource handling
- Auto-ignores dynamic K8s metadata (resourceVersion, uid, managedFields)
- Supports wildcard array index matching (e.g., `containers[*].image`)
- Handles JSON numeric type coercion (float64 vs int64)

### Classifier (`internal/classifier/`)
- Maps drifted fields to severity levels using configurable rules
- Built-in heuristics for common Kubernetes fields
- User rules override builtins for customization
- Prefix matching: `metadata.labels.app` matches `metadata.labels` rule

### AGENTS.md Parser (`internal/agents/`)
- Parses the AGENTS.md context engineering configuration
- Extracts remediation strategies, safety boundaries, protected namespaces
- Falls back to sensible defaults when AGENTS.md is missing
- Provides `ShouldSkipResource()` for annotation-based opt-out

### Remediation Engine (`internal/remediation/`)
- Pluggable strategy pattern: Ignore, AutoRevert, AlertAndHold, Notify
- Retry with exponential backoff for failed remediations
- Namespace protection enforcement
- Dry-run mode for safe testing

### Controller (`internal/controller/`)
- Kubernetes reconciliation loop using `controller-runtime`
- Watches `DriftPolicy` CRD resources
- Orchestrates the full pipeline: sync → diff → classify → remediate
- Updates CRD status with scan results

### Metrics (`internal/metrics/`)
- Prometheus counters, gauges, and histograms
- Tracks drift events, remediation actions, scan duration
- Git sync health monitoring

## Data Flow

1. **DriftPolicy CR** is created by the user
2. **Controller** picks it up and starts reconciliation
3. **Git Syncer** clones/pulls the source of truth repository
4. **Manifest Loader** reads YAML files from the synced repo
5. **Controller** lists live resources from the Kubernetes API
6. **Differ** compares each live resource against its desired state
7. **Classifier** assigns severity to each detected drift
8. **Remediation Engine** applies the appropriate action per AGENTS.md
9. **Metrics** are updated throughout the pipeline
10. **Status** is written back to the DriftPolicy CR

## CRD Design

### DriftPolicy
The primary custom resource that defines:
- **What to watch**: resource types and namespaces
- **Source of truth**: Git repository, branch, and path
- **Classification rules**: field-to-severity mappings
- **Remediation strategy**: severity-to-action mappings
- **Scan interval**: how frequently to check for drift

See `config/samples/driftpolicy_sample.yaml` for a complete example.
