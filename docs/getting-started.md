# Getting Started

This guide walks you through installing and using KubeDriftGuard.

## Prerequisites

- Go 1.22+
- A Kubernetes cluster (kind, minikube, or remote)
- kubectl configured with cluster access
- Git

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/Priyasharma620064/kubedriftguard.git
cd kubedriftguard

# Build the binary
make build

# Verify
./bin/kubedriftguard version
```

### Using Helm

```bash
helm install kubedriftguard ./charts/kubedriftguard \
  --namespace kubedriftguard-system \
  --create-namespace
```

## Quick Start

### 1. Install the CRD

```bash
kubectl apply -f config/samples/driftpolicy_sample.yaml
```

### 2. Validate Configuration

```bash
./bin/kubedriftguard validate --config config.yaml
```

Expected output:
```
✓ Configuration loaded successfully
  Log level:      info
  Scan interval:  30s
  Metrics addr:   :8080
  Remediation:    enabled=true dry_run=false

✓ AGENTS.md parsed successfully
  Protected namespaces: [kube-system kube-public]
  Skip annotation:      driftguard.io/skip
  Strategies:
    cosmetic → ignore
    config → auto-revert
    security → alert-and-hold
    critical → auto-revert
  Boundaries:           7 rules

✓ Git syncer initialized (cache: /tmp/kubedriftguard/repos)

✅ All validations passed
```

### 3. Start the Controller

```bash
# Start in dry-run mode first (recommended)
./bin/kubedriftguard controller --dry-run

# Start with remediation enabled
./bin/kubedriftguard controller
```

### 4. Monitor Drift

```bash
# Check DriftPolicy status
kubectl get driftpolicies

# View detailed status
kubectl describe driftpolicy production-policy

# Check Prometheus metrics
curl http://localhost:8080/metrics | grep kubedriftguard
```

## Opting Out Resources

Add the skip annotation to any resource you want KubeDriftGuard to ignore:

```yaml
metadata:
  annotations:
    driftguard.io/skip: "true"
```

## Configuration

See [config.yaml](../config.yaml) for all available options.

Environment variables use the `KUBEDRIFTGUARD_` prefix:

```bash
export KUBEDRIFTGUARD_LOG_LEVEL=debug
export KUBEDRIFTGUARD_SCAN_INTERVAL=60s
export KUBEDRIFTGUARD_REMEDIATION_DRY_RUN=true
```

## Next Steps

- Read the [Architecture](architecture.md) guide
- Customize your [AGENTS.md](../AGENTS.md) configuration
- Set up [Prometheus monitoring](https://prometheus.io/) for drift metrics
