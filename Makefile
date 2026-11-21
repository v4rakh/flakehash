BINARY_NAME=flakehash
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags "-s -w"

CMD_GO_FILES=./cmd/flakehash

.PHONY: all build build-all clean test lint lint-fix fmt run install dependencies audit scan help

all: clean lint test build

## build: Build the flakehash binary for the host platform
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) -trimpath $(LDFLAGS) -o bin/$(BINARY_NAME) $(CMD_GO_FILES)

## build-all: Cross-compile for linux/darwin, amd64/arm64 (the platforms Nix runs on)
build-all: build-all-linux-amd64 build-all-linux-arm64 build-all-darwin-amd64 build-all-darwin-arm64

build-all-linux-amd64:
	GOOS=linux GOARCH=amd64 $(GO) build -trimpath $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 $(CMD_GO_FILES)
build-all-linux-arm64:
	GOOS=linux GOARCH=arm64 $(GO) build -trimpath $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 $(CMD_GO_FILES)
build-all-darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(GO) build -trimpath $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 $(CMD_GO_FILES)
build-all-darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(GO) build -trimpath $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 $(CMD_GO_FILES)

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

## test: Run all tests with race detection and coverage
test:
	@echo "Running tests..."
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

## lint: Run golangci-lint
lint:
	@echo "Running linter..."
	golangci-lint run ./...

## lint-fix: Run golangci-lint with autofix
lint-fix:
	@echo "Running linter with auto-fix..."
	golangci-lint run --fix ./...

## fmt: Format and vet code
fmt:
	$(GO) fmt ./...
	$(GO) vet ./...

## audit: Audit dependencies for known vulnerabilities
audit:
	@echo "Auditing dependencies..."
	govulncheck ./...

## scan: Scan built binaries for known vulnerabilities with grype
scan:
	@NO_COLOR=1 grype -v -o table --file bin/grype.txt --fail-on critical bin/ || true
	@cat bin/grype.txt

## run: Build and run flakehash
run: build
	./bin/$(BINARY_NAME)

## dependencies: Download module dependencies without mutating go.mod (CI)
dependencies:
	$(GO) mod download

## install: Tidy and verify module dependencies (local dev)
install:
	$(GO) mod download
	$(GO) mod tidy

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
