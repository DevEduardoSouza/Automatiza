---
type: doc
name: tooling
description: Scripts, IDE settings, automation, and developer productivity tips
category: tooling
generated: 2026-02-18
status: filled
scaffoldVersion: "2.0.0"
---

# Tooling & Productivity Guide

This document lists the tools required to work on the Automatiza project, recommended automation patterns, IDE configuration, and the MCP server integrations available for AI-assisted development.

## Required Tooling

| Tool | Version | Purpose |
|------|---------|---------|
| **Node.js** | 20+ | Frontend runtime (React 19 + Vite + TypeScript) |
| **Go** | 1.24+ | Backend runtime (Gin + ConnectRPC) |
| **Docker + Docker Compose** | Latest | Runs PostgreSQL 16, Redis, and WAHA Plus containers |
| **Buf CLI** | Latest | Protobuf linting, formatting, breaking change detection, and code generation |
| **BiomeJS** | Latest | Frontend linting and formatting (replaces ESLint + Prettier) |
| **golangci-lint** | Latest | Backend linting with strict rules (gosec, errcheck, bodyclose, complexity limits) |
| **Playwright** | Latest | End-to-end browser testing |

### Installation

```bash
# Install all project dependencies (Go modules + npm packages)
make install

# Install Playwright browsers (for E2E tests)
cd client && npx playwright install
```

## Recommended Automation

### After Editing `.proto` Files

```bash
# Generate TypeScript + Go stubs from protobuf definitions
make proto-generate

# Lint proto files for style compliance
make proto-lint

# Check for breaking changes against the main branch
make proto-breaking
```

### Before Committing

```bash
# Run linters (BiomeJS + golangci-lint)
make lint

# Format code (BiomeJS + gofmt)
make format
```

### Before Pushing

```bash
# Run all tests (frontend + backend)
make test

# Run with coverage report
make test-coverage
```

### Database Operations

```bash
# Start PostgreSQL + Redis containers
make db-up

# Run pending migrations
make db-migrate

# Fresh database (destroys all data and recreates)
make db-reset && make db-migrate

# Access PostgreSQL shell directly
make db-shell

# View database logs
make db-logs
```

### WAHA (WhatsApp API)

```bash
# Start WAHA Plus container
make waha-up

# View WAHA logs
make waha-logs

# Stop WAHA
make waha-down
```

### Build & Deployment

```bash
# Production build (frontend + backend)
make build

# Frontend only (outputs to client/dist/)
make build-client

# Backend only (outputs Go binary)
make build-go
```

## MCP Server Integrations

The project has Model Context Protocol (MCP) servers configured in `.claude/mcp.json` for AI-assisted development. These provide direct access to project infrastructure without manual CLI commands.

| MCP Server | Purpose | Replaces |
|------------|---------|----------|
| **postgres** | SQL queries, schema inspection, EXPLAIN analysis | `psql` via Bash |
| **redis** | Cache inspection, key/TTL debugging, queue state | `redis-cli` via Bash |
| **docker** | Container status, logs, service inspection | `docker ps/logs/inspect` |
| **chrome-devtools** | Browser console logs, network request debugging | Manual browser DevTools |
| **ai-context** | Codebase exploration, scaffolding, PREVC workflow | Manual file reading |

**Priority rule:** Always prefer MCP servers over equivalent Bash commands when the MCP server is available and responsive. Fall back to Bash only for operations the MCP does not support (e.g., `docker compose up/down`, `make db-reset`).

## IDE Setup

### VS Code (Recommended)

Install the following extensions for the best development experience:

| Extension | Purpose |
|-----------|---------|
| **Biome** (`biomejs.biome`) | Frontend lint/format integration with BiomeJS |
| **Go** (`golang.go`) | Go language support, debugging, and test runner |
| **vscode-proto3** (`zxh404.vscode-proto3`) | Protocol Buffer syntax highlighting and linting |
| **Tailwind CSS IntelliSense** (`bradlc.vscode-tailwindcss`) | Autocomplete for TailwindCSS 4.1 classes |
| **Playwright Test for VSCode** (`ms-playwright.playwright`) | Run and debug E2E tests from the editor |

### Recommended Settings

```jsonc
// .vscode/settings.json
{
  // Use BiomeJS as the default formatter for TypeScript/JavaScript
  "[typescript]": { "editor.defaultFormatter": "biomejs.biome" },
  "[typescriptreact]": { "editor.defaultFormatter": "biomejs.biome" },
  "[json]": { "editor.defaultFormatter": "biomejs.biome" },

  // Format and lint on save
  "editor.formatOnSave": true,
  "editor.codeActionsOnSave": {
    "quickfix.biome": "explicit",
    "source.organizeImports.biome": "explicit"
  },

  // Go settings
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--fast"],
  "go.testFlags": ["-race"],

  // Tailwind CSS class sorting
  "tailwindCSS.experimental.classRegex": [
    ["cva\\(([^)]*)\\)", "[\"'`]([^\"'`]*).*?[\"'`]"]
  ]
}
```

## Observability Tools

The project uses Sentry and Datadog for production observability:

| Tool | Purpose | Access Method |
|------|---------|---------------|
| **Sentry** | Error tracking, stack traces, breadcrumbs | `sentry-cli` (authenticated via `SENTRY_AUTH_TOKEN`) |
| **Datadog** | Metrics (CPU/RAM), APM traces, distributed tracing | Datadog MCP (never use `dog` CLI) |
| **GitHub CLI** | Secrets audit, CI/CD logs, PR management | `gh` CLI (authenticated via system keychain) |

For production incident debugging, follow the PRIP (Production Incident Remediation Protocol) documented in `CLAUDE.md`.

## Related Documents

- [Development Workflow](./development-workflow.md) -- branching strategy, local setup, and code review expectations
- [Testing Strategy](./testing-strategy.md) -- test frameworks, running tests, and quality gates
