# Gochin Framework Makefile

# Variables
BINARY_NAME=gochin
BUILD_DIR=.
CMD_DIR=./cmd/gochin
CONFIG_DIR=./config

# Default target
.DEFAULT_GOAL := help

## Build the CLI binary
build:
	@echo "Building Gochin CLI..."
	CGO_ENABLED=0 go build -o $(BINARY_NAME) $(CMD_DIR)
	@echo "✅ Build complete: ./$(BINARY_NAME)"

## Build the smart installer
build-installer:
	@echo "Building smart installer..."
	CGO_ENABLED=0 go build -o installer/gochin-installer installer/main.go
	@echo "✅ Installer build complete: installer/gochin-installer"

## Smart install - auto-detects OS and installs appropriately
install:
	@echo "Running smart installer (auto-detects OS)..."
	go run installer/main.go

## Manual install for current platform (Linux/macOS only)
install-manual: build
	@echo "Installing Gochin CLI globally..."
	sudo cp $(BINARY_NAME) /usr/local/bin/
	@echo "✅ Gochin installed globally"

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
	go test -v ./...

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

## Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	@echo "✅ Clean complete"

## Test CLI commands (uses global installation)
test-cli:
	@echo "Testing CLI commands..."
	@echo "\n=== Testing help ==="
	gochin --help
	@echo "\n=== Testing run start ==="
	gochin run start --help
	@echo "\n=== Testing make controller ==="
	gochin make controller --help
	@echo "\n=== Testing db migrate ==="
	gochin db migrate --help

## Test configuration system
test-config:
	@echo "Testing configuration system..."
	go run $(CONFIG_DIR)/main.go

## Demo OS detection capabilities
demo-os:
	@echo "Running OS detection demo..."
	go run demo/os-detection.go

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

## Build for multiple platforms
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

.PHONY: build install test bench bench-baseline bench-compare fmt lint tidy clean test-cli test-config dev init help


## Framework Commands

## Start the Gochin server
start:
	@echo "Starting Gochin application..."
	gochin run start

## Generate a controller
controller:
	@if [ -z "$(name)" ]; then \
		echo "Usage: make controller name=ControllerName"; \
		echo "Example: make controller name=User"; \
	else \
		gochin make controller $(name); \
	fi

## Run database migrations
migrate:
	@echo "Running database migrations..."
	gochin db migrate