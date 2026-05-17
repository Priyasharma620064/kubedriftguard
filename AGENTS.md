# AGENTS.md — KubeDriftGuard Context Engineering Configuration
# This file defines how the agentic self-healing engine behaves.
# It follows the AGENTS.md standard for AI-native project configuration.

## Project Overview
KubeDriftGuard is a Kubernetes controller that detects configuration drift
between live cluster state and Git-declared manifests, then applies
configurable remediation strategies.

## Remediation Strategies
Define how each drift severity level should be handled:

### cosmetic: ignore
Label and annotation changes are safe to skip.

### config: auto-revert
General configuration drift should be reverted to the Git source of truth.

### security: alert-and-hold
Security-sensitive changes require human review before remediation.
Alert via webhook and wait for manual approval.

### critical: auto-revert
Critical drift (replicas, images) must be immediately reverted to prevent
availability issues.

## Boundaries
- Never modify resources in the `kube-system` namespace
- Never modify resources in the `kube-public` namespace
- Never delete PersistentVolumeClaims
- Never modify Custom Resource Definitions
- Always create a backup annotation before reverting
- Always log the full diff before any remediation action
- Respect `driftguard.io/skip=true` annotation on resources

## Notification Channels
- webhook: https://hooks.example.com/drift-alerts
- severity_filter: security,critical

## Scan Configuration
- default_interval: 30s
- max_concurrent_scans: 5
- timeout_per_resource: 10s
