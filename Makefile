.PHONY: all build test lint clean coverage help install

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
BINARY_NAME=covenant
MAIN_PATH=./cmd/contract

# Build info
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

all: lint test build ## Run lint, test, and build

build: ## Build the binary
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PATH)

build-all: ## Build for all platforms
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)

install: ## Install the binary
	$(GOCMD) install $(LDFLAGS) $(MAIN_PATH)

test: ## Run tests
	$(GOTEST) -race -v ./...

test-short: ## Run tests (short mode)
	$(GOTEST) -short -v ./...

coverage: ## Run tests with coverage
	$(GOTEST) -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint: ## Run linter
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

fmt: ## Format code
	$(GOFMT) ./...

vet: ## Run go vet
	$(GOCMD) vet ./...

tidy: ## Tidy and verify module dependencies
	$(GOMOD) tidy
	$(GOMOD) verify

clean: ## Clean build artifacts
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*
	rm -f coverage.out coverage.html

deps: ## Download dependencies
	$(GOMOD) download

update-deps: ## Update dependencies
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Development helpers
dev-broker: build ## Run broker in development mode
	./$(BINARY_NAME) broker --storage filesystem --storage-path ./data --port 8080

dev-test: ## Run tests in watch mode (requires entr)
	@if command -v entr >/dev/null 2>&1; then \
		find . -name '*.go' | entr -c $(GOTEST) -v ./...; \
	else \
		echo "entr not installed. Install with: brew install entr"; \
		exit 1; \
	fi

help: ## Display this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

# Default target
.DEFAULT_GOAL := help
