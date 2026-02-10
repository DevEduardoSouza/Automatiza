# =============================================================================
# Automatiza-Go Makefile
# =============================================================================
# Backend-specific commands (can be run independently or via root Makefile)
# =============================================================================

.PHONY: help dev build test lint lint-fix format clean tidy

# Colors
CYAN := \033[36m
GREEN := \033[32m
YELLOW := \033[33m
RESET := \033[0m

.DEFAULT_GOAL := help

# =============================================================================
# Help
# =============================================================================

help: ## Show this help message
	@echo ""
	@echo "$(CYAN)Automatiza-Go - Backend Commands$(RESET)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-15s$(RESET) %s\n", $$1, $$2}'
	@echo ""

# =============================================================================
# Development
# =============================================================================

dev: ## Run the server in development mode
	@echo "$(GREEN)Starting Go server...$(RESET)"
	go run ./cmd/server/main.go

build: ## Build the server binary
	@echo "$(GREEN)Building server...$(RESET)"
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server ./cmd/server/main.go

# =============================================================================
# Testing
# =============================================================================

test: ## Run all tests
	@echo "$(GREEN)Running tests...$(RESET)"
	go test -v -race ./...

test-coverage: ## Run tests with coverage
	@echo "$(GREEN)Running tests with coverage...$(RESET)"
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report: coverage.html$(RESET)"

# =============================================================================
# Linting & Formatting
# =============================================================================

lint: ## Run golangci-lint
	@echo "$(GREEN)Running golangci-lint...$(RESET)"
	golangci-lint run ./...

lint-fix: ## Run golangci-lint with auto-fix
	@echo "$(GREEN)Running golangci-lint with auto-fix...$(RESET)"
	golangci-lint run --fix ./...

format: ## Format Go code
	@echo "$(GREEN)Formatting Go code...$(RESET)"
	go fmt ./...
	goimports -w .

# =============================================================================
# Dependencies
# =============================================================================

tidy: ## Tidy go.mod and go.sum
	@echo "$(GREEN)Tidying dependencies...$(RESET)"
	go mod tidy

download: ## Download dependencies
	@echo "$(GREEN)Downloading dependencies...$(RESET)"
	go mod download

# =============================================================================
# Security
# =============================================================================

vuln: ## Run govulncheck for vulnerabilities
	@echo "$(GREEN)Checking for vulnerabilities...$(RESET)"
	go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

# =============================================================================
# Cleanup
# =============================================================================

clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(RESET)"
	rm -rf bin/
	rm -f coverage.out coverage.html
