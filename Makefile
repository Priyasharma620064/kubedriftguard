BINARY_NAME=kubedriftguard
VERSION?=0.1.0
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags "-X github.com/Priyasharma620064/kubedriftguard/internal/version.Version=$(VERSION)"

.PHONY: all build clean test lint fmt vet help

all: build

## build: Build the binary
build:
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/controller/

## clean: Remove build artifacts
clean:
	rm -rf bin/
	rm -f cover.out coverage.html

## test: Run unit tests
test:
	$(GO) test $(GOFLAGS) ./... -coverprofile=cover.out

## coverage: Show test coverage
coverage: test
	$(GO) tool cover -html=cover.out -o coverage.html

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## fmt: Format Go source files
fmt:
	$(GO) fmt ./...

## vet: Run go vet
vet:
	$(GO) vet ./...

## tidy: Tidy go modules
tidy:
	$(GO) mod tidy

## help: Show this help
help:
	@echo "Usage:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
