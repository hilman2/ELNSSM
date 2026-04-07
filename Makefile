VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

PKG     := github.com/hilman2/ELNSSM
LDFLAGS := -s -w \
           -X $(PKG)/internal/buildinfo.Version=$(VERSION) \
           -X $(PKG)/internal/buildinfo.Commit=$(COMMIT) \
           -X $(PKG)/internal/buildinfo.BuildDate=$(BUILD_DATE)

.PHONY: help build build-all clean test test-race vet lint tidy snapshot release-check

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build elnssm.exe for the current platform
	go build -trimpath -ldflags "$(LDFLAGS)" -o elnssm.exe .

build-all: ## Cross-build for windows/amd64 and windows/arm64
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/elnssm-windows-amd64.exe .
	GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/elnssm-windows-arm64.exe .

clean: ## Remove build artifacts
	rm -rf elnssm.exe dist/

test: ## Run unit tests
	go test ./...

test-cover: ## Run unit tests with coverage (CGO-free, race detector not supported)
	CGO_ENABLED=0 go test -coverprofile=coverage.out -covermode=atomic ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (must be installed)
	golangci-lint run

tidy: ## Tidy go.mod and go.sum
	go mod tidy

snapshot: ## Build a local GoReleaser snapshot (requires goreleaser)
	goreleaser release --snapshot --clean --skip=publish

release-check: ## Validate the GoReleaser config
	goreleaser check
