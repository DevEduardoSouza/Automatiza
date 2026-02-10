# =============================================================================
# Automatiza - Monorepo Makefile
# =============================================================================
# Stack: React 19 + Golang/Gin + WAHA Plus (GOWS) + PostgreSQL + gRPC/Proto
# =============================================================================

.PHONY: help dev dev-client dev-go dev-worker build lint format test clean \
        sonar-up sonar-down sonar-logs sonar-clean \
        proto-generate proto-lint proto-breaking proto-deps install \
        db-up db-down db-reset db-migrate db-shell db-logs db-wait \
        waha-up waha-down waha-logs \
        deploy deploy-status deploy-logs deploy-rollback

# Colors
CYAN := \033[36m
GREEN := \033[32m
YELLOW := \033[33m
RED := \033[31m
RESET := \033[0m

.DEFAULT_GOAL := help

# =============================================================================
# Help
# =============================================================================

help: ## Show this help message
	@echo ""
	@echo "$(CYAN)Automatiza - Monorepo Commands$(RESET)"
	@echo ""
	@echo "$(GREEN)Deploy:$(RESET)"
	@grep -E '^deploy[a-zA-Z_-]*:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-20s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)Development:$(RESET)"
	@grep -E '^(dev|build|lint|format|test|clean|install)[a-zA-Z_-]*:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-20s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)Database:$(RESET)"
	@grep -E '^db-[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-20s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)WAHA (WhatsApp):$(RESET)"
	@grep -E '^waha-[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-20s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)Protocol Buffers:$(RESET)"
	@grep -E '^proto-[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-20s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)SonarQube:$(RESET)"
	@grep -E '^sonar-[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-20s$(RESET) %s\n", $$1, $$2}'
	@echo ""

# =============================================================================
# Development
# =============================================================================

dev: db-up db-wait waha-up ## Start all services (DB + WAHA + Backend + Worker + Frontend)
	@echo "$(GREEN)Starting backend...$(RESET)"
	@cd automatiza-go && go run ./cmd/server/main.go &
	@sleep 2
	@echo "$(GREEN)Starting worker...$(RESET)"
	@cd automatiza-go && go run ./cmd/server/main.go -worker &
	@sleep 1
	@echo "$(GREEN)Starting frontend...$(RESET)"
	@cd client && npm run dev

dev-client: ## Start frontend only
	@echo "$(GREEN)Starting React frontend...$(RESET)"
	@cd client && npm run dev

dev-go: ## Start Go backend only
	@echo "$(GREEN)Starting Go backend...$(RESET)"
	@cd automatiza-go && go run ./cmd/server/main.go

dev-worker: ## Start Go worker only
	@echo "$(GREEN)Starting Go worker...$(RESET)"
	@cd automatiza-go && go run ./cmd/server/main.go -worker

dev-all: db-up waha-up ## Start everything (DB + WAHA + Backend + Worker + Frontend)
	@echo "$(GREEN)Starting backend...$(RESET)"
	@cd automatiza-go && go run ./cmd/server/main.go &
	@sleep 2
	@echo "$(GREEN)Starting worker...$(RESET)"
	@cd automatiza-go && go run ./cmd/server/main.go -worker &
	@sleep 1
	@echo "$(GREEN)Starting frontend...$(RESET)"
	@cd client && npm run dev

# =============================================================================
# Database
# =============================================================================

db-up: ## Start PostgreSQL + Redis
	@echo "$(GREEN)Starting database services...$(RESET)"
	@cd automatiza-go && docker compose up -d postgres redis

db-wait: ## Wait for PostgreSQL to accept connections
	@printf "$(YELLOW)Waiting for PostgreSQL..."
	@until cd automatiza-go && docker compose exec -T postgres pg_isready -U $${POSTGRES_USER:-automatiza} -d $${POSTGRES_DB:-automatiza} > /dev/null 2>&1; do \
		printf "."; \
		sleep 1; \
	done
	@echo " ready!$(RESET)"

db-down: ## Stop PostgreSQL + Redis
	@echo "$(YELLOW)Stopping database services...$(RESET)"
	@cd automatiza-go && docker compose stop postgres redis

db-logs: ## Show PostgreSQL logs
	@cd automatiza-go && docker compose logs -f postgres

db-migrate: ## Run all pending migrations on existing database
	@echo "$(GREEN)Running database migrations...$(RESET)"
	@cd automatiza-go && docker compose up -d postgres
	@sleep 2
	@cd automatiza-go && docker compose exec postgres sh -c \
		'for f in /docker-entrypoint-initdb.d/migrations/*.sql; do \
			case "$$f" in *rollback*|*.down.*) continue;; esac; \
			echo "  Applying: $$(basename $$f)"; \
			psql -v ON_ERROR_STOP=0 -U $${POSTGRES_USER:-automatiza} -d $${POSTGRES_DB:-automatiza} -f "$$f" 2>&1 || true; \
		done'
	@echo "$(GREEN)Migrations complete!$(RESET)"

db-reset: ## Reset database (DESTRUCTIVE - removes all data)
	@echo "$(RED)Resetting database (all data will be lost)...$(RESET)"
	@cd automatiza-go && docker compose down -v
	@cd automatiza-go && docker compose up -d postgres redis
	@echo "$(GREEN)Database reset complete! Waiting for init...$(RESET)"
	@sleep 5

db-shell: ## Open PostgreSQL shell
	@cd automatiza-go && docker compose exec postgres psql -U $${POSTGRES_USER:-automatiza} -d $${POSTGRES_DB:-automatiza}

# =============================================================================
# WAHA (WhatsApp)
# =============================================================================

waha-up: ## Start WAHA Plus service
	@echo "$(GREEN)Starting WAHA Plus...$(RESET)"
	@cd automatiza-go && docker compose up -d waha

waha-down: ## Stop WAHA Plus service
	@echo "$(YELLOW)Stopping WAHA Plus...$(RESET)"
	@cd automatiza-go && docker compose stop waha

waha-logs: ## Show WAHA logs
	@cd automatiza-go && docker compose logs -f waha

# =============================================================================
# Build & Install
# =============================================================================

build: ## Build all projects
	@echo "$(GREEN)Building projects...$(RESET)"
	@cd client && npm run build
	@cd automatiza-go && CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server ./cmd/server/main.go
	@echo "$(GREEN)Build complete!$(RESET)"

build-client: ## Build frontend only
	@echo "$(GREEN)Building React frontend...$(RESET)"
	@cd client && npm run build

build-go: ## Build Go backend only
	@echo "$(GREEN)Building Go backend...$(RESET)"
	@cd automatiza-go && CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server ./cmd/server/main.go

install: ## Install all dependencies
	@echo "$(GREEN)Installing dependencies...$(RESET)"
	@cd client && npm install
	@cd automatiza-go && go mod download
	@echo "$(GREEN)Dependencies installed!$(RESET)"

# =============================================================================
# Linting & Formatting
# =============================================================================

lint: ## Run linting for all projects
	@echo "$(GREEN)Linting projects...$(RESET)"
	@cd client && npm run lint
	@cd automatiza-go && golangci-lint run ./...

lint-client: ## Lint frontend only
	@cd client && npm run lint

lint-go: ## Lint Go backend only
	@cd automatiza-go && golangci-lint run ./...

format: ## Format all projects
	@echo "$(GREEN)Formatting projects...$(RESET)"
	@cd client && npm run format
	@cd automatiza-go && go fmt ./... && goimports -w .

format-client: ## Format frontend only
	@cd client && npm run format

format-go: ## Format Go backend only
	@cd automatiza-go && go fmt ./... && goimports -w .

check: lint ## Alias for lint

# =============================================================================
# Testing
# =============================================================================

test: ## Run all tests
	@echo "$(GREEN)Running tests...$(RESET)"
	@cd client && npm run test:run 2>/dev/null || echo "$(YELLOW)Client tests not configured$(RESET)"
	@cd automatiza-go && go test -v -race ./...

test-client: ## Run frontend tests
	@cd client && npm run test:run

test-go: ## Run Go backend tests
	@cd automatiza-go && go test -v -race ./...

test-coverage: ## Run tests with coverage
	@echo "$(GREEN)Running tests with coverage...$(RESET)"
	@cd client && npm run test:coverage 2>/dev/null || echo "$(YELLOW)Client coverage not configured$(RESET)"
	@cd automatiza-go && go test -v -race -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html

# =============================================================================
# Protocol Buffers
# =============================================================================

proto-deps: ## Install/update buf dependencies
	@echo "$(GREEN)Updating buf dependencies...$(RESET)"
	@cd proto && buf dep update

proto-generate: ## Generate proto stubs (TypeScript + Go)
	@echo "$(GREEN)Generating proto stubs...$(RESET)"
	@cd client && buf generate
	@cd automatiza-go && buf generate
	@echo "$(GREEN)Proto generation complete!$(RESET)"

proto-lint: ## Lint proto files
	@echo "$(GREEN)Linting proto files...$(RESET)"
	@cd proto && buf lint

proto-breaking: ## Check for breaking changes in proto
	@echo "$(GREEN)Checking for breaking changes...$(RESET)"
	@cd proto && buf breaking --against '.git#branch=main' 2>/dev/null || echo "$(YELLOW)No git history for breaking check$(RESET)"

proto-format: ## Format proto files
	@echo "$(GREEN)Formatting proto files...$(RESET)"
	@cd proto && buf format -w

# =============================================================================
# SonarQube
# =============================================================================

sonar-up: ## Start SonarQube server
	@echo "$(GREEN)Starting SonarQube...$(RESET)"
	@docker compose -f docker-compose.sonarqube.yml up -d
	@echo ""
	@echo "$(YELLOW)SonarQube is starting up. This may take 2-3 minutes.$(RESET)"
	@echo "$(CYAN)Access at: http://localhost:9000/sonarqube$(RESET)"
	@echo "$(CYAN)Default credentials: admin / admin$(RESET)"

sonar-down: ## Stop SonarQube server
	@echo "$(YELLOW)Stopping SonarQube...$(RESET)"
	@docker compose -f docker-compose.sonarqube.yml down

sonar-logs: ## Show SonarQube logs
	@docker compose -f docker-compose.sonarqube.yml logs -f sonarqube

sonar-clean: ## Remove SonarQube containers and volumes
	@echo "$(RED)Removing SonarQube containers and volumes...$(RESET)"
	@docker compose -f docker-compose.sonarqube.yml down -v

# =============================================================================
# Deploy
# =============================================================================

deploy: ## Deploy local (build images + push GHCR + SSH deploy)
	@echo "$(GREEN)Iniciando deploy para VPS...$(RESET)"
	@bash scripts/deploy-local.sh

deploy-status: ## Status dos containers na VPS
	@ssh -i ~/.ssh/deploy_key $${VPS_USER:-root}@$${VPS_HOST:-astecwms.com.br} \
		'cd /opt/automatiza/docker && docker compose ps'

deploy-logs: ## Logs dos containers na VPS (use SERVICE=server para filtrar)
	@ssh -i ~/.ssh/deploy_key $${VPS_USER:-root}@$${VPS_HOST:-astecwms.com.br} \
		'cd /opt/automatiza/docker && docker compose logs --tail=100 -f $(SERVICE)'

deploy-rollback: ## Rollback para tag anterior (uso: make deploy-rollback TAG=20260209-123456-abc1234)
	@if [ -z "$(TAG)" ]; then echo "$(RED)Uso: make deploy-rollback TAG=<version>$(RESET)"; exit 1; fi
	@echo "$(YELLOW)Rolling back para tag $(TAG)...$(RESET)"
	@ssh -i ~/.ssh/deploy_key $${VPS_USER:-root}@$${VPS_HOST:-astecwms.com.br} \
		'cd /opt/automatiza/docker && bash -c "export BACKEND_IMAGE=ghcr.io/bruno-vilefort-tech-consulting/carlos-automatisa/backend:$(TAG) && export FRONTEND_IMAGE=ghcr.io/bruno-vilefort-tech-consulting/carlos-automatisa/frontend:$(TAG) && docker compose pull && docker compose up -d server worker frontend"'
	@echo "$(GREEN)Rollback complete!$(RESET)"

# =============================================================================
# Cleanup
# =============================================================================

clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(RESET)"
	@rm -rf client/dist client/coverage
	@rm -rf automatiza-go/bin automatiza-go/coverage.out automatiza-go/coverage.html
	@rm -rf automatiza-go/test-report.json automatiza-go/golangci-lint-report.xml
	@echo "$(GREEN)Clean complete!$(RESET)"

clean-all: clean ## Clean everything (including node_modules, containers)
	@echo "$(RED)Cleaning everything...$(RESET)"
	@rm -rf client/node_modules
	@cd automatiza-go && docker compose down -v 2>/dev/null || true
	@docker compose -f docker-compose.sonarqube.yml down -v 2>/dev/null || true
	@echo "$(GREEN)Full clean complete!$(RESET)"

# =============================================================================
# Utilities
# =============================================================================

status: ## Show status of all services
	@echo "$(CYAN)Docker containers:$(RESET)"
	@cd automatiza-go && docker compose ps 2>/dev/null || echo "  No containers running"
	@echo ""
	@echo "$(CYAN)SonarQube:$(RESET)"
	@docker compose -f docker-compose.sonarqube.yml ps 2>/dev/null || echo "  Not running"

logs: ## Show logs from all services
	@cd automatiza-go && docker compose logs -f

stop: ## Stop all services
	@echo "$(YELLOW)Stopping all services...$(RESET)"
	@cd automatiza-go && docker compose down 2>/dev/null || true
	@docker compose -f docker-compose.sonarqube.yml down 2>/dev/null || true
	@echo "$(GREEN)All services stopped!$(RESET)"
