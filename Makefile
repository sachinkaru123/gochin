# Gochin Framework Makefile
#
# This is the framework's own repository — it builds the `gochin` CLI.
# To build an application, run `gochin new <name>` and work in the generated
# project instead; these targets are for developing Gochin itself.

BINARY_NAME=gochin
CMD_DIR=./cmd/gochin

# Default target
.DEFAULT_GOAL := help

## Build the CLI binary
build:
	@echo "Building Gochin CLI..."
	CGO_ENABLED=0 go build -o $(BINARY_NAME) $(CMD_DIR)
	@echo "✅ Build complete: ./$(BINARY_NAME)"

## Install the CLI to your GOPATH/bin
install:
	@echo "Installing the gochin CLI (go install)..."
	go install $(CMD_DIR)
	@echo "✅ Installed. Make sure \$$(go env GOPATH)/bin is on your PATH."

## Run benchmarks
bench:
	@echo "Running benchmarks..."
	go test -run XXX -bench . -benchmem -benchtime=2s ./pkg/...

## Save the current benchmarks as the comparison baseline
bench-baseline:
	go test -run XXX -bench . -benchmem -benchtime=2s ./pkg/... > bench-baseline.txt
	@echo "✅ Baseline written to bench-baseline.txt"

## Compare against the saved baseline (requires benchstat)
bench-compare:
	go test -run XXX -bench . -benchmem -benchtime=2s ./pkg/... > bench-new.txt
	benchstat bench-baseline.txt bench-new.txt

## Run tests
test:
	@echo "Running tests..."
	go test -race -v ./pkg/...

## Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

## Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

## Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

## Verify pkg/ has no dependency on app/ (the layering CI also checks this)
check-layering:
	@if go list -deps ./pkg/... | grep -q 'sachinkaru123/gochin/app'; then \
		echo "❌ pkg/ imports app/ — this must never happen in the framework repo"; \
		exit 1; \
	fi
	@echo "✅ pkg/ has no dependency on app/"

## Verify `gochin new` produces a project that builds and migrates
verify-new:
	@echo "Building gochin and scaffolding a throwaway project..."
	go build -o /tmp/gochin-verify $(CMD_DIR)
	rm -rf /tmp/gochin-verify-project
	mkdir -p /tmp/gochin-verify-project
	cd /tmp/gochin-verify-project && GOCHIN_FRAMEWORK_REPLACE=$(CURDIR) /tmp/gochin-verify new demo
	cd /tmp/gochin-verify-project/demo && go build ./...
	@echo "✅ gochin new produces a project that builds"

## Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	@echo "✅ Clean complete"

## Development build with hot reload (requires air)
dev:
	@if command -v air > /dev/null; then \
		echo "Starting development server with hot reload..."; \
		air; \
	else \
		echo "Air not installed. Install with: go install github.com/air-verse/air@latest"; \
		echo "Falling back to regular build..."; \
		make build; \
	fi

## Initialize development environment
init:
	@echo "Initializing development environment..."
	go mod download
	@if ! command -v golangci-lint > /dev/null; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@if ! command -v air > /dev/null; then \
		echo "Installing air for hot reload..."; \
		go install github.com/air-verse/air@latest; \
	fi
	@echo "✅ Development environment ready"

## Build the CLI for multiple platforms
build-all:
	@echo "Building Gochin for multiple platforms..."
	@mkdir -p dist
	@echo "Building for Linux (amd64)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/gochin-linux-amd64 $(CMD_DIR)
	@echo "Building for macOS (amd64)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o dist/gochin-darwin-amd64 $(CMD_DIR)
	@echo "Building for macOS (arm64)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o dist/gochin-darwin-arm64 $(CMD_DIR)
	@echo "Building for Windows (amd64)..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/gochin-windows-amd64.exe $(CMD_DIR)
	@echo "✅ All builds complete in dist/ directory"

## Show available commands
help:
	@echo "Gochin Framework - Available Commands:"
	@echo
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
	@echo

.PHONY: build install bench bench-baseline bench-compare test fmt lint tidy \
	check-layering verify-new clean dev init build-all help
