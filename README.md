# KubeDriftGuard

[![CI](https://github.com/Priyasharma620064/kubedriftguard/actions/workflows/ci.yml/badge.svg)](https://github.com/Priyasharma620064/kubedriftguard/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Priyasharma620064/kubedriftguard)](https://goreportcard.com/report/github.com/Priyasharma620064/kubedriftguard)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)

**Real-time Kubernetes configuration drift detection and agentic self-healing engine with AGENTS.md-driven remediation.**

KubeDriftGuard is a Kubernetes controller that continuously watches live cluster state against Git-declared manifests, detects configuration drift in real-time, classifies drift by severity, and triggers self-healing workflows using context engineering principles (AGENTS.md).

## ✨ Features

- 🔍 **Drift Detection** — Field-by-field comparison between Git source of truth and live cluster state
- 🏷️ **Severity Classification** — Automatic categorization: cosmetic, config, security, critical
- 🤖 **AGENTS.md-Driven Remediation** — Context engineering config controls self-healing behavior
- 🛡️ **Safety Boundaries** — Protected namespaces, skip annotations, and dry-run mode
- 📊 **Prometheus Metrics** — Full observability: drift events, remediation actions, scan duration
- 🔄 **Git Sync** — Incremental repository sync with GitHub token auth support
- ⎈ **CRD-Based** — Declarative `DriftPolicy` custom resource for configuration
- 🎮 **OpenKruise-Aware** — Native support for CloneSet, SidecarSet, BroadcastJob
- 🐳 **Helm Chart** — Production-ready deployment with RBAC and health probes

## 🏗️ Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                    KubeDriftGuard Controller                   │
│                                                                │
│   DriftPolicy CR                                               │
│        │                                                       │
│        ▼                                                       │
│   ┌──────────┐    ┌──────────┐    ┌────────────┐              │
│   │ Git Sync │───▶│  Differ  │───▶│ Classifier │              │
│   │ (go-git) │    │ (field   │    │ (severity  │              │
│   │          │    │  compare)│    │  assign)   │              │
│   └──────────┘    └──────────┘    └─────┬──────┘              │
│                                         │                      │
│                                         ▼                      │
│                                  ┌────────────┐               │
│                                  │Remediation │               │
│                                  │  Engine    │               │
│                                  │(AGENTS.md) │               │
│                                  └──────┬─────┘               │
│                                    ┌────┴────┐                │
│                                    ▼         ▼                │
│                              ┌─────────┐ ┌────────┐          │
│                              │ Metrics │ │Webhook │          │
│                              │(Prom)   │ │ Alert  │          │
│                              └─────────┘ └────────┘          │
└──────────────────────────────────────────────────────────────┘
```

## 🚀 Quick Start

### Prerequisites

- Go 1.22+
- Kubernetes cluster (kind, minikube, or remote)
- kubectl configured with cluster access

### Install

```bash
git clone https://github.com/Priyasharma620064/kubedriftguard.git
cd kubedriftguard
make build
./bin/kubedriftguard version
```

### Validate Configuration

```bash
./bin/kubedriftguard validate
```

### Start Controller

```bash
# Dry-run mode (recommended for first run)
./bin/kubedriftguard controller --dry-run

# With remediation enabled
./bin/kubedriftguard controller
```

### Deploy with Helm

```bash
helm install kubedriftguard ./charts/kubedriftguard \
  --namespace kubedriftguard-system \
  --create-namespace
```

## 📋 DriftPolicy CRD

```yaml
apiVersion: driftguard.io/v1alpha1
kind: DriftPolicy
metadata:
  name: production-policy
spec:
  targetNamespaces: [default, production]
  watchResources:
    - group: apps
      kind: Deployment
    - group: apps.kruise.io
      kind: CloneSet
  gitSource:
    repoURL: https://github.com/example/k8s-manifests
    branch: main
    path: /production
  scanInterval: "30s"
  classification:
    defaultSeverity: config
    rules:
      - field: spec.replicas
        severity: critical
      - field: metadata.labels
        severity: cosmetic
      - field: spec.template.spec.containers[*].image
        severity: critical
  remediation:
    strategy:
      cosmetic: ignore
      config: auto-revert
      security: alert-and-hold
      critical: auto-revert
```

## 🧠 Context Engineering (AGENTS.md)

KubeDriftGuard uses [AGENTS.md](AGENTS.md) to define how the self-healing engine behaves — following the context engineering standard for AI-native projects:

```markdown
## Remediation Strategies
### cosmetic: ignore
### config: auto-revert
### security: alert-and-hold
### critical: auto-revert

## Boundaries
- Never modify resources in the `kube-system` namespace
- Never delete PersistentVolumeClaims
- Always create a backup annotation before reverting
- Respect `driftguard.io/skip=true` annotation on resources
```

## 📊 Prometheus Metrics

| Metric | Type | Description |
|:---|:---|:---|
| `kubedriftguard_drift_events_total` | Counter | Total drift events by severity |
| `kubedriftguard_remediation_actions_total` | Counter | Remediation actions by result |
| `kubedriftguard_scan_duration_seconds` | Histogram | Scan cycle duration |
| `kubedriftguard_active_drifts` | Gauge | Current unresolved drifts |
| `kubedriftguard_git_sync_status` | Gauge | Git sync health (1=ok) |

## 📁 Project Structure

```
kubedriftguard/
├── AGENTS.md                        # Context engineering config
├── api/v1alpha1/                    # CRD type definitions
├── cmd/controller/                  # CLI entrypoint
├── internal/
│   ├── agents/                      # AGENTS.md parser
│   ├── classifier/                  # Drift severity classification
│   ├── config/                      # Viper-based configuration
│   ├── controller/                  # K8s reconciliation loop
│   ├── differ/                      # Strategic merge diff engine
│   ├── gitsync/                     # Git repository sync
│   ├── metrics/                     # Prometheus instrumentation
│   ├── models/                      # Core domain types
│   ├── remediation/                 # Self-healing strategies
│   └── version/                     # Build version info
├── charts/kubedriftguard/           # Helm chart
├── config/samples/                  # Example DriftPolicy CRs
├── docs/                            # Architecture & guides
├── .github/workflows/               # CI/CD pipelines
├── Dockerfile                       # Multi-stage container build
├── Makefile                         # Build automation
└── config.yaml                      # Default configuration
```

## 🔧 Configuration

Configuration follows layered precedence: `defaults → config.yaml → env vars → CLI flags`

```bash
# Environment variables use KUBEDRIFTGUARD_ prefix
export KUBEDRIFTGUARD_LOG_LEVEL=debug
export KUBEDRIFTGUARD_SCAN_INTERVAL=60s
export KUBEDRIFTGUARD_REMEDIATION_DRY_RUN=true
```

See [config.yaml](config.yaml) for all available options.

## 🤝 Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

```bash
make test    # Run tests
make lint    # Run linter
make build   # Build binary
```

## 📄 License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
