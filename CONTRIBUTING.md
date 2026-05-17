# Contributing to KubeDriftGuard

Thank you for your interest in contributing to KubeDriftGuard! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for everyone.

## How to Contribute

### Reporting Issues

- Use GitHub Issues to report bugs or request features
- Include steps to reproduce for bug reports
- Provide your environment details (Go version, K8s version, OS)

### Pull Requests

1. Fork the repository
2. Create a feature branch from `main`:
   ```bash
   git checkout -b feature/your-feature-name
   ```
3. Make your changes
4. Run tests and lint:
   ```bash
   make test
   make lint
   make vet
   ```
5. Commit with a descriptive message:
   ```bash
   git commit -m "Add support for custom resource monitoring"
   ```
6. Push and create a Pull Request

### Development Setup

```bash
# Clone your fork
git clone https://github.com/<your-username>/kubedriftguard.git
cd kubedriftguard

# Install dependencies
go mod tidy

# Build
make build

# Run tests
make test

# Run linter
make lint
```

## Project Structure

```
kubedriftguard/
├── api/v1alpha1/           # CRD type definitions
├── cmd/controller/         # CLI entrypoint
├── internal/
│   ├── agents/             # AGENTS.md parser
│   ├── classifier/         # Drift severity classification
│   ├── config/             # Configuration management
│   ├── controller/         # K8s reconciliation loop
│   ├── differ/             # Drift detection engine
│   ├── gitsync/            # Git repository sync
│   ├── metrics/            # Prometheus instrumentation
│   ├── models/             # Domain types
│   ├── remediation/        # Self-healing strategies
│   └── version/            # Build version info
├── charts/                 # Helm chart
├── config/samples/         # Example CRD manifests
├── docs/                   # Documentation
└── .github/workflows/      # CI/CD
```

## Coding Guidelines

- Follow standard Go conventions and `gofmt` formatting
- Write unit tests for new functionality
- Add godoc comments to all exported types and functions
- Keep packages focused on a single responsibility
- Use meaningful variable and function names

## Testing

```bash
# Run all tests
make test

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test -v ./internal/differ/...

# Run with coverage
make coverage
```

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
