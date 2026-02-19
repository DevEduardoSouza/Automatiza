---
type: doc
name: architecture
description: System architecture, layers, patterns, and design decisions
category: architecture
generated: 2026-02-18
status: filled
scaffoldVersion: "2.0.0"
---

# Architecture Notes

Automatiza is a WhatsApp Marketing Automation Platform built as a modular monolith. The system enables users to create, schedule, and dispatch automated WhatsApp messages to groups, track engagement through redirect links, manage affiliate integrations (Shopee, Mercado Livre), and process payment webhooks (Kiwify). This document describes the architectural layers, detected patterns, entry points, and public API surface.

## System Architecture Overview

The platform is structured as a modular monolith comprising three main runtime components:

1. **Go Backend** (`automatiza-go/`) -- A single Go binary serving both HTTP/ConnectRPC handlers and an async worker mode. Gin handles HTTP routing, while ConnectRPC (gRPC-Web) provides typed RPC endpoints for the frontend. The backend follows Clean Architecture: handlers parse requests and delegate to services, services contain business logic, and repositories handle persistence via PostgreSQL (pgx) and Redis.

2. **React Frontend** (`client/`) -- A React 19 + Vite + TypeScript SPA organized following Feature-Sliced Design (FSD). The frontend communicates exclusively via ConnectRPC (gRPC-Web) for typed API calls and uses SSE for real-time session status updates. TailwindCSS 4.1 and shadcn/ui provide the design system.

3. **PostgreSQL 16 + Redis** -- PostgreSQL stores all persistent data across 33 tables. Redis serves as both a cache layer and the backing store for the async message queue (Asynq). No soft-delete strategy is used; hard delete with separate archival when auditing is needed.

**External dependencies:**
- **WAHA Plus** (WhatsApp HTTP API) -- Provides WhatsApp connectivity. The backend receives events via WAHA webhooks (HMAC-validated) and sends messages through WAHA's REST API.
- **Kiwify** -- Payment platform that sends purchase/subscription webhooks for plan management.
- **Shopee / Mercado Livre** -- Affiliate program integrations for link generation and product scraping.

```
+-----------+     ConnectRPC      +----------------+      pgx       +------------+
|  React    | ◄─────────────────► |   Go Backend   | ◄────────────► | PostgreSQL |
|  Frontend |     (gRPC-Web)      |   (Gin + RPC)  |                |   16       |
+-----------+                     +-------+--------+                +------------+
                                          |
                                  ┌───────┼────────┐
                                  │       │        │
                               Redis   WAHA     Kiwify
                              (Queue)  (WS/WH)  (Webhooks)
```

## Architectural Layers

### Backend (`automatiza-go/internal/`)

| Layer | Directory | Responsibility |
|-------|-----------|----------------|
| **Handlers (ConnectRPC)** | `internal/connectrpc/` | Typed RPC handlers implementing proto service interfaces. Includes interceptors for auth, tracing, and Sentry error capture. |
| **Handlers (HTTP)** | `internal/handlers/` | Traditional HTTP handlers for webhooks (WAHA, Kiwify), SSE endpoints, public redirect routes, and the Gin router setup. |
| **Middleware** | `internal/middleware/` | Cross-cutting concerns: auth (JWT extraction/validation), CORS, structured error handling, request logging, security headers, input validation, and webhook rate limiting. |
| **Services** | `internal/services/` | Business logic layer. Each domain has its own service package: `automation/`, `whatsapp/`, `redirect/`, `shopee/`, `kiwify/`, `jobs/`, `profile/`, `email/`, `dashboard/`, `ai/`, `coupons/`, `mercadolivre/`, `mllinklists/`, `monitoring/`, `reconciliation/`, `waha/`, `watchdog/`, `cron/`, `loadbalancer/`, `user/`. |
| **Repository** | `internal/repository/postgres/` | Data access layer. One repository per aggregate/table. Uses pgx directly (no ORM). Implements interfaces defined in `internal/repository/interfaces.go`. |
| **Models** | `internal/models/` | Domain structs shared across layers. Represents database entities and internal data structures. |
| **Adapters** | `internal/adapters/` | External system adapters: `whatsapp/` (WAHA HTTP client with HMAC validation), `email/` (email sending). |
| **Workers** | `internal/workers/` | Async job processors backed by Redis/Asynq: message dispatch (`message_queue_worker.go`), dispatch log processing, group sync, webhook event consumption, monitoring message processing, reconciliation. |
| **Auth** | `internal/auth/` | JWT token generation, validation, and cookie management. |
| **Config** | `internal/config/` | Application configuration loading from environment variables. |
| **Observability** | `internal/observability/` | Sentry initialization, ConnectRPC interceptor for Sentry, and Asynq error handler for Sentry. |
| **Startup** | `internal/startup/` | Application bootstrap routines including backfill and reconciliation on startup. |
| **Generated Code** | `internal/gen/` | Auto-generated Go code from Protocol Buffer definitions (ConnectRPC stubs). |

### Frontend (`client/src/`)

| Layer | Directory | Responsibility |
|-------|-----------|----------------|
| **Pages** | `src/pages/` | Route-level components. Each page is UI/JSX only; all logic lives in a corresponding `usePageName()` hook. Pages: auth, dashboard, groups, whatsapp, offer-automations, dispatch-texts, redirect-link, ml-link-lists, affiliate-programs, coupons, jobs, history, automation-logs, profile, admin, public-redirect, forgot/reset-password. |
| **Components** | `src/components/` | Reusable UI components built on shadcn/ui primitives. |
| **Hooks** | `src/hooks/` | Custom hooks encapsulating ConnectRPC queries/mutations via `@connectrpc/connect-query` (useQuery/useMutation), SSE subscriptions, and page-level logic. |
| **Entities** | `src/entities/` | FSD entities layer -- domain-specific data types and shared entity logic. |
| **Services** | `src/services/` | API service layer (legacy Axios-based calls for non-ConnectRPC endpoints). |
| **Contexts** | `src/contexts/` | React contexts for cross-cutting state (auth, theme, etc.). |
| **Integrations** | `src/integrations/` | Third-party integration wrappers. |
| **i18n** | `src/i18n/` | Internationalization files (pt-BR locale). |
| **Generated Code** | `src/gen/` | Auto-generated TypeScript code from Protocol Buffer definitions (ConnectRPC client stubs). |
| **Schemas** | `src/schemas/` | Form validation schemas (react-hook-form resolvers). |
| **Config** | `src/config/` | Frontend configuration constants. |
| **Types** | `src/types/` | Shared TypeScript type definitions. |

### Shared

| Layer | Directory | Responsibility |
|-------|-----------|----------------|
| **Proto Definitions** | `proto/` | Protocol Buffers (.proto files) -- the single source of truth for all API contracts. Organized by domain: `auth/`, `automations/`, `groups/`, `whatsapp/`, `webhooks/`, `redirect/`, `ml_links/`, `affiliates/`, `jobs/`, `dashboard/`, `admin/`, `ai/`, `plans/`, `profiles/`, `settings/`, `campaigns/`, `coupons/`, `dispatch_texts/`, `history/`, `images/`, `instances/`, `common/`, `scripts/`. |

## Detected Design Patterns

| Pattern | Where Applied | Purpose |
|---------|--------------|---------|
| **Clean Architecture** | Backend layers (handler -> service -> repository) | Separation of concerns; business logic independent of framework/DB |
| **Repository Pattern** | `internal/repository/postgres/` with interfaces in `internal/repository/interfaces.go` | Abstracts data access; enables testability via interface mocking |
| **Adapter Pattern** | `internal/adapters/whatsapp/`, `internal/adapters/email/` | Isolates external API communication behind clean interfaces |
| **Observer / Webhook** | `internal/handlers/waha_webhook_handler.go`, Kiwify webhook processing | Event-driven ingestion from external systems (WAHA, Kiwify) |
| **Worker / Queue** | `internal/workers/` with Redis/Asynq | Async processing for message dispatch, group sync, reconciliation |
| **Interceptor** | `internal/connectrpc/interceptors.go`, `internal/observability/sentry_interceptor.go` | Cross-cutting concerns (auth, tracing, error capture) on RPC calls |
| **Feature-Sliced Design** | `client/src/` (pages, entities, hooks, components) | Frontend architectural pattern enforcing layer boundaries |
| **Schema-Driven Contracts** | `proto/` definitions generating both Go and TypeScript | Single source of truth for API types; prevents drift between frontend/backend |
| **Watchdog** | `internal/services/watchdog/` | Monitors WAHA silence, triggers reconnection with backoff |
| **Reconciliation** | `internal/services/reconciliation/`, `internal/workers/reconciliation_worker.go` | Compensatory mechanism to backfill missed events on startup or gap detection |

## Entry Points

| Entry Point | Path | Description |
|-------------|------|-------------|
| **Backend Server** | `automatiza-go/cmd/server/main.go` | Starts the Go backend in either server mode (HTTP + ConnectRPC) or worker mode (`-worker` flag for Asynq processors). Initializes Sentry, Datadog tracing, database connections, Redis, and all service dependencies. |
| **Frontend App** | `client/src/main.tsx` | React application entry point. Mounts the app with routing, ConnectRPC transport, query client, and i18n providers. |
| **App Component** | `client/src/app.tsx` | Root React component defining route structure and layout. |

## Public API

All public API endpoints are defined as ConnectRPC services in `proto/`. Key services include:

| Proto Package | Service | Description |
|--------------|---------|-------------|
| `auth` | AuthService | Login, logout, magic link auth, token refresh, CSRF |
| `automations` | AutomationsService | CRUD for offer automations, scheduling, dispatch control |
| `groups` | GroupsService | WhatsApp group management, sync, search |
| `whatsapp` | WhatsAppService | Session management, message sending, instance control |
| `webhooks` | WebhooksService | Webhook log listing and management |
| `redirect` | RedirectService | Redirect link creation, analytics, public redirect resolution |
| `ml_links` | MlLinksService | Mercado Livre link management and scraping |
| `affiliates` | AffiliatesService | Affiliate credential management (Shopee) |
| `jobs` | JobsService | Background job status and management |
| `dashboard` | DashboardService | Dashboard statistics and analytics |
| `admin` | AdminService | User management, plan assignment, system administration |
| `ai` | AiService | AI-powered tools (text generation, etc.) |
| `plans` | PlansService | Subscription plan management |
| `profiles` | ProfilesService | User profile and password management |
| `settings` | SettingsService | Application settings |
| `dispatch_texts` | DispatchTextsService | Message template management |
| `history` | HistoryService | Action history and audit trail |
| `instances` | InstancesService | WAHA instance management |
| `images` | ImagesService | Image upload and management |
| `coupons` | CouponsService | Coupon/discount management |
| `campaigns` | CampaignsService | Campaign management |

Additional HTTP endpoints (non-ConnectRPC):
- `POST /webhook/waha` -- WAHA webhook receiver (HMAC-validated)
- `GET /r/:code` -- Public redirect handler
- `GET /sse/session-status` -- Server-Sent Events for real-time session updates

## Top Directories Snapshot

```
novo-projeto/
  automatiza-go/
    cmd/server/          # Entry point (main.go)
    internal/
      adapters/          # External system adapters (WAHA, email)
      auth/              # JWT/cookie auth
      config/            # Configuration
      connectrpc/        # ConnectRPC handlers + interceptors
      errors/            # Custom error types
      gen/               # Generated Go code from proto
      handlers/          # HTTP handlers (webhooks, SSE, router)
      lib/               # Shared utilities
      metrics/           # Metrics collection
      middleware/        # Gin middleware (auth, CORS, security, logging)
      models/            # Domain models
      observability/     # Sentry integration
      repository/        # Data access (postgres/)
      services/          # Business logic (20+ domain packages)
      shared/            # Shared utilities
      startup/           # Bootstrap/backfill routines
      workers/           # Async job processors (Asynq)
    scripts/db/          # Database migrations
    docker-compose.yml   # Local dev services
  client/
    src/
      components/        # Reusable UI components
      config/            # Frontend config
      constants/         # Constants
      contexts/          # React contexts
      entities/          # FSD entities layer
      gen/               # Generated TS from proto
      hooks/             # Custom hooks (ConnectRPC, SSE, page logic)
      i18n/              # Internationalization (pt-BR)
      integrations/      # Third-party integrations
      lib/               # Utilities
      pages/             # Route-level page components
      schemas/           # Form validation schemas
      services/          # API service layer
      types/             # TypeScript types
    vite.config.ts       # Vite configuration
  proto/                 # Protocol Buffer definitions (source of truth)
  docker/                # Production Docker Compose
  Makefile               # Development commands
```

## Related Resources

- **Development Workflow**: See `development-workflow.md` for branching, CI/CD, and day-to-day practices.
- **Data Flow**: See `data-flow.md` for how data moves between components and external integrations.
- **Security**: See `security.md` for authentication model, secrets management, and security policies.
- **Testing**: See `testing-strategy.md` for test approach and coverage.
- **Codebase Map**: See `codebase-map.json` for complete symbol counts, dependency graphs, and detailed structure analysis.
