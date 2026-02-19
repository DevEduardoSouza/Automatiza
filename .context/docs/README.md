---
type: doc
name: README
description: Documentation index and navigation hub for the Automatiza knowledge base
category: overview
generated: 2026-02-18
status: filled
scaffoldVersion: "2.0.0"
---

# Documentation Index

Welcome to the Automatiza repository knowledge base. This documentation provides context for AI agents and developers working with the codebase. Automatiza is a WhatsApp Marketing Automation Platform targeting the Brazilian market, built with a Go backend, React frontend, and PostgreSQL database.

Start with the [Project Overview](./project-overview.md) for a high-level understanding, then dive into specific guides as needed.

## Core Guides

- [Project Overview](./project-overview.md) - High-level overview of the project, its purpose, stack, and key domains
- [Architecture Notes](./architecture.md) - System architecture, layers, Clean Architecture patterns, and design decisions
- [Development Workflow](./development-workflow.md) - Branching rules, CI configuration, and contributing guide
- [Testing Strategy](./testing-strategy.md) - Test approach, coverage, and tooling (Vitest, Playwright, Go testing, testcontainers)
- [Glossary & Domain Concepts](./glossary.md) - Business terminology, type definitions, enumerations, personas, and domain rules
- [Data Flow & Integrations](./data-flow.md) - System diagrams, integration specs (Kiwify, WAHA, Shopee), and queue topics
- [Security & Compliance Notes](./security.md) - Auth model, secrets management, CSP, and compliance considerations
- [Tooling & Productivity Guide](./tooling.md) - CLI scripts, Makefile commands, IDE configs, and automation workflows

## Repository Structure

| Directory | Description |
|-----------|-------------|
| `automatiza-go/` | Go backend services: Gin HTTP server, ConnectRPC handlers, services, repositories, workers, migrations, and observability (Sentry + Datadog) |
| `client/` | React 19 + TypeScript frontend: Vite build, TailwindCSS 4.1 + shadcn/ui styling, Feature-Sliced Design architecture, ConnectRPC hooks |
| `proto/` | Protocol Buffer definitions: service contracts, request/response messages, enums. Source of truth for all API types (generates Go + TypeScript stubs via `buf`) |
| `docker/` | Docker Compose files for production deployment |
| `nginx/` | Nginx configuration for reverse proxy and static file serving |
| `scripts/` | Utility scripts for deployment, database operations, and automation |
| `systemd/` | Systemd service unit files for VPS production deployment |
| `design/` | Design assets and mockups |

## Quick Links

- **Codebase Map**: [`codebase-map.json`](./codebase-map.json) - Complete symbol counts, architecture layers, and dependency graphs
- **API Definitions**: `proto/` directory for all gRPC/ConnectRPC service definitions (source of truth for contracts)
- **Frontend Entry**: `client/src/main.tsx` - React application bootstrap
- **Backend Entry**: `automatiza-go/cmd/server/main.go` - Go server entry point (HTTP + gRPC + worker mode)
- **Database Schema**: `automatiza-go/scripts/db/migrations/000_postgresql_init.sql` - Consolidated init with all 33 production tables

## Document Map

| Guide | File | Primary Inputs |
|-------|------|----------------|
| Project Overview | `project-overview.md` | Roadmap, README, stakeholder notes, stack summary |
| Architecture Notes | `architecture.md` | ADRs, service boundaries, dependency graphs, Clean Architecture layers |
| Development Workflow | `development-workflow.md` | Branching rules, CI config, Makefile commands, contributing guide |
| Testing Strategy | `testing-strategy.md` | Test configs (Vitest, Playwright, Go testing), CI gates, known flaky suites |
| Glossary & Domain Concepts | `glossary.md` | Business terminology, type definitions, user personas, domain rules |
| Data Flow & Integrations | `data-flow.md` | System diagrams, integration specs (Kiwify, WAHA, Shopee, Mercado Livre), queue topics |
| Security & Compliance Notes | `security.md` | Auth model (JWT + cookies), secrets management, CSP with nonce, compliance requirements |
| Tooling & Productivity Guide | `tooling.md` | CLI scripts, Makefile reference, IDE configs, MCP servers, automation workflows |
