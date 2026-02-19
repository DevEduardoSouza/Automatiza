---
type: doc
name: project-overview
description: High-level overview of the project, its purpose, and key components
category: overview
generated: 2026-02-18
status: filled
scaffoldVersion: "2.0.0"
---

# Project Overview

Automatiza is a WhatsApp Marketing Automation Platform designed for the Brazilian market. The platform enables users to manage WhatsApp groups, automate message dispatch on configurable schedules, integrate with payment platforms (Kiwify) for subscription billing, and leverage affiliate programs (Shopee, Mercado Livre) for monetization.

The system is built as a monorepo with a Go backend serving a React frontend through ConnectRPC (gRPC-Web). WhatsApp connectivity is provided by WAHA Plus (WhatsApp HTTP API) using the GoWS engine, with webhook-based event ingestion for real-time message processing.

## Key Domains

- **Auth**: User authentication, role-based access control (admin/user), password management
- **Automations**: Scheduled WhatsApp message dispatch with configurable timing, group targeting, and distribution strategies
- **Groups**: WhatsApp group management, synchronization with WAHA instances, metadata tracking
- **Webhooks**: Kiwify payment notification processing for subscription lifecycle management
- **Redirect Links**: Smart redirect URLs with distribution strategies (round-robin, weighted) for affiliate marketing
- **ML Links**: Mercado Livre affiliate link management and tracking
- **Affiliate Programs**: Shopee affiliate integration and credential management
- **Jobs**: Background task processing (group sync, message dispatch, reconciliation)
- **Dashboard**: Analytics, metrics, and operational overview
- **Admin Panel**: Platform administration, user management, investigation tools
- **Billing/Plans**: Subscription plans, coupons, usage limits
- **Dispatch Texts**: Message templates with body content and active/inactive state management
- **Instances**: WhatsApp session management via WAHA (connection, QR code, health monitoring)
- **Action History**: Comprehensive audit log for all user and system actions

## Codebase Reference

For complete symbol counts, architecture layers, dependency graphs, and file structure details, see [`codebase-map.json`](./codebase-map.json).

## Quick Facts

| Property | Value |
|----------|-------|
| Root Path | `/Users/brunovilefort/Desktop/automatiza/novo-projeto` |
| Languages | TypeScript (frontend), Go 1.24 (backend), Protocol Buffers (contracts) |
| Frontend Framework | React 19 + Vite + TailwindCSS 4.1 + shadcn/ui |
| Backend Framework | Go 1.24 + Gin + ConnectRPC |
| Database | PostgreSQL 16 (33 tables) + Redis (cache/queues) |
| WhatsApp Engine | WAHA Plus (GoWS Engine) |
| Architecture (Frontend) | Feature-Sliced Design (FSD) |
| Architecture (Backend) | Clean Architecture: Handler -> Service -> Repository |
| API Protocol | Protocol Buffers + ConnectRPC (gRPC-Web) |
| Observability | Sentry (error tracking) + Datadog (APM, metrics, traces) |
| Linting | BiomeJS (frontend) + golangci-lint (backend) |
| Testing | Vitest + Playwright (frontend), Go testing + testcontainers (backend) |
| Routing | React Router DOM 7.x |
| Total DB Tables | 33 (consolidated in `000_postgresql_init.sql`) |

## Entry Points

| Component | Path | Description |
|-----------|------|-------------|
| Frontend | `client/src/main.tsx` | React application bootstrap, router setup, provider tree |
| Backend (Server) | `automatiza-go/cmd/server/main.go` | Gin HTTP server, ConnectRPC handler registration, middleware chain, Sentry/Datadog init |
| Backend (Worker) | `automatiza-go/cmd/server/main.go -worker` | Background worker mode for message dispatch, group sync, and scheduled jobs |
| Migrations | `automatiza-go/scripts/db/migrations/` | Database migration files (000 = consolidated init, 001-029 = incremental) |
| Proto Definitions | `proto/` | Service contracts, message types, enums (generates Go + TypeScript via `buf`) |

## File Structure

```
novo-projeto/
├── automatiza-go/           # Go backend
│   ├── cmd/server/          # Application entry point (server + worker modes)
│   ├── internal/
│   │   ├── adapters/        # External service adapters (WAHA, Shopee, etc.)
│   │   ├── connectrpc/      # ConnectRPC handler implementations
│   │   ├── handlers/        # HTTP handlers (webhooks, redirects, static files)
│   │   ├── middleware/       # Gin middleware (auth, CORS, rate limiting, CSP)
│   │   ├── models/          # Domain models and database structs
│   │   ├── observability/   # Sentry and Datadog initialization
│   │   ├── repository/      # Data access layer (PostgreSQL repositories)
│   │   ├── services/        # Business logic layer (use cases)
│   │   ├── startup/         # Application bootstrapping (backfill, reconciliation)
│   │   └── workers/         # Background job workers (message queue, scheduler)
│   ├── scripts/db/          # Database migrations and seed data
│   └── docker-compose.yml   # Local development services (PostgreSQL, Redis, WAHA)
├── client/                  # React frontend
│   ├── src/
│   │   ├── app.tsx          # Root application component, router configuration
│   │   ├── pages/           # Page components (UI-only, logic in hooks)
│   │   ├── hooks/           # Custom hooks (use-*-connect.ts pattern for ConnectRPC)
│   │   ├── services/        # API service layer
│   │   ├── components/      # Shared UI components (shadcn/ui based)
│   │   ├── i18n/            # Internationalization (pt-BR locale)
│   │   ├── gen/             # Generated ConnectRPC client code (do not edit)
│   │   └── index.css        # TailwindCSS entry point
│   └── vite.config.ts       # Vite build configuration
├── proto/                   # Protocol Buffer definitions
│   └── automatiza/v1/       # Service definitions, messages, enums
├── docker/                  # Production Docker Compose files
├── nginx/                   # Nginx reverse proxy configuration
├── scripts/                 # Utility and deployment scripts
├── systemd/                 # Systemd service unit files
├── Makefile                 # Project command runner (dev, test, lint, build, proto)
└── CLAUDE.md                # AI agent instructions and project rules
```

## Technology Stack Summary

### Frontend

| Category | Technology | Purpose |
|----------|-----------|---------|
| Runtime | React 19 | UI rendering with concurrent features |
| Build | Vite | Fast dev server and optimized production builds |
| Language | TypeScript | Type-safe frontend development |
| Styling | TailwindCSS 4.1 + shadcn/ui | Utility-first CSS with accessible component library |
| Routing | React Router DOM 7.x | Client-side navigation and route management |
| API Client | ConnectRPC (@connectrpc/connect-query) | Type-safe gRPC-Web communication with React Query integration |
| State | React Query (via connect-query) | Server state management with cache/dedup |
| i18n | i18next | Internationalization (pt-BR primary locale) |
| Linting | BiomeJS | Fast linting and formatting |
| Testing | Vitest + Playwright | Unit/integration tests + end-to-end tests |
| Architecture | Feature-Sliced Design (FSD) | Layered frontend architecture with strict import boundaries |

### Backend

| Category | Technology | Purpose |
|----------|-----------|---------|
| Language | Go 1.24 | High-performance backend services |
| HTTP | Gin | HTTP router and middleware framework |
| API | ConnectRPC | gRPC-Web compatible RPC framework |
| Database | PostgreSQL 16 | Primary data store (33 tables) |
| Cache | Redis | Caching, session storage, message queues |
| WhatsApp | WAHA Plus (GoWS) | WhatsApp Web API bridge |
| Observability | Sentry + Datadog (dd-trace-go) | Error tracking + APM/traces/metrics |
| Linting | golangci-lint | Comprehensive Go linting (gosec, errcheck, bodyclose) |
| Testing | Go testing + testcontainers | Unit tests with containerized dependencies |
| Migrations | Custom Go migrator | SQL-based migrations with consolidation strategy |

### Infrastructure

| Category | Technology | Purpose |
|----------|-----------|---------|
| Contracts | Protocol Buffers + buf | API schema definition and code generation |
| Containerization | Docker + Docker Compose | Local development and production deployment |
| Reverse Proxy | Nginx | TLS termination, static file serving, request routing |
| Process Manager | systemd (production) | Service management on VPS |
| CI/CD | GitHub Actions | Automated testing, linting, and deployment |

## Getting Started Checklist

1. **Install dependencies**:
   ```bash
   make install
   ```

2. **Start database services** (PostgreSQL + Redis):
   ```bash
   make db-up
   ```

3. **Run database migrations**:
   ```bash
   make db-migrate
   ```

4. **Start development environment** (backend + frontend):
   ```bash
   make dev
   ```

5. **Optional: Start WAHA** (for WhatsApp connectivity):
   ```bash
   make waha-up
   ```

6. **Verify everything is running**:
   ```bash
   make status
   ```

The frontend will be available at `http://localhost:5173` and the backend API at `http://localhost:3000`.
