.PHONY: build test clean install fmt vet run-demo help

# Binary name
BINARY=cka-lab-runner
BUILD_DIR=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

help: ## Show this help
	@echo "CKA Lab Runner - Makefile commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""

build: ## Build the binary
	@echo "Building $(BINARY)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY) -v ./cmd/cka-lab-runner
	@echo "Binary built at $(BUILD_DIR)/$(BINARY)"

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

install: build ## Install binary to /usr/local/bin
	@echo "Installing $(BINARY) to /usr/local/bin..."
	@sudo cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/
	@echo "Installation complete!"

fmt: ## Format code
	@echo "Formatting code..."
	$(GOFMT) ./...

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

lint: fmt vet ## Run formatters and linters

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

run-demo: build ## Run the demo script
	@echo "Running demo..."
	@cd /tmp && rm -rf cka-demo && mkdir cka-demo && cd cka-demo && ../$(BUILD_DIR)/$(BINARY) init

dev: build ## Build and show version
	@$(BUILD_DIR)/$(BINARY) --help

ci: lint test build ## Run CI checks (lint, test, build)
	@echo "All CI checks passed!"

.DEFAULT_GOAL := help
