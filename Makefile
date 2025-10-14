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

## Build and install globally
install: build
	@echo "Installing Gochin CLI globally..."
	sudo mv $(BINARY_NAME) /usr/local/bin/
	@echo "✅ Gochin installed globally"

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

## Test CLI commands
test-cli: build
	@echo "Testing CLI commands..."
	@echo "\n=== Testing help ==="
	./$(BINARY_NAME) --help
	@echo "\n=== Testing run start ==="
	./$(BINARY_NAME) run start --help
	@echo "\n=== Testing make controller ==="
	./$(BINARY_NAME) make controller --help
	@echo "\n=== Testing db migrate ==="
	./$(BINARY_NAME) db migrate --help

## Test configuration system
test-config:
	@echo "Testing configuration system..."
	go run $(CONFIG_DIR)/main.go

## Development build with hot reload (requires air)
dev:
	@if command -v air > /dev/null; then \
		echo "Starting development server with hot reload..."; \
		air; \
	else \
		echo "Air not installed. Install with: go install github.com/cosmtrek/air@latest"; \
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
		go install github.com/cosmtrek/air@latest; \
	fi
	@echo "✅ Development environment ready"

## Show available commands
help:
	@echo "Gochin Framework - Available Commands:"
	@echo
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
	@echo

.PHONY: build install test fmt lint tidy clean test-cli test-config dev init help


##Freamwork Commands

start:
	@echo "Starting Gochin application..."
	./$(BINARY_NAME) run start