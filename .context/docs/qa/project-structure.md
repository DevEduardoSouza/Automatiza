---
slug: project-structure
category: architecture
generatedAt: 2026-02-18T22:38:13.911Z
relevantFiles:
  - client/src/app.css
  - client/src/app.tsx
  - client/src/assets
  - client/src/components
  - client/src/config
  - client/src/constants
  - client/src/contexts
  - client/src/entities
  - client/src/gen
  - client/src/hooks
---

# How is the codebase organized?

## Monorepo Layout

```
novo-projeto/
  automatiza-go/       # Go backend (server + worker)
  client/              # React TypeScript frontend
  proto/               # Protocol Buffer definitions (source of truth)
  docker/              # Production Docker Compose files
  nginx/               # Nginx reverse proxy config
  scripts/             # Utility scripts
  systemd/             # systemd service definitions (VPS)
  design/              # UI/UX design assets
  Makefile             # Build automation (all commands)
```

## Backend (`automatiza-go/`)

Follows **Clean Architecture** with clear layer separation:

```
automatiza-go/
  cmd/server/main.go       # Entry point (server + worker modes)
  internal/
    adapters/              # External service adapters (WAHA, etc.)
    auth/                  # JWT, session, and role management
    config/                # App configuration loading
    connectrpc/            # ConnectRPC handlers (gRPC-Web API layer)
    errors/                # Custom error types and mapping
    gen/                   # Generated code from Protocol Buffers
    handlers/              # HTTP handlers (webhooks, redirects, SSE)
    lib/                   # Shared utilities
    metrics/               # Prometheus/Datadog metrics
    middleware/            # Gin middleware (auth, CORS, logging, tracing)
    models/                # Domain models and entities
    observability/         # Sentry + Datadog initialization
    repository/            # Data access layer
      interfaces.go        # Repository interface definitions
      postgres/            # PostgreSQL implementations (pgx)
    services/              # Business logic layer
      automation/          # Automation engine
      whatsapp/            # WhatsApp messaging
      jobs/                # Job management
      monitoring/          # Process monitoring
      reconciliation/      # Data reconciliation/backfill
      watchdog/            # Silence detection
      ...                  # (20+ service packages)
    shared/                # Cross-cutting utilities
    startup/               # Application bootstrap and backfill
    workers/               # Asynq background job processors
  scripts/db/migrations/   # SQL migration files
  tests/                   # E2E, integration, and load tests
```

**Layer rules:**
- `connectrpc/` handlers call `services/` (never repositories directly)
- `services/` contain business logic, call `repository/` for data access
- `repository/` is the only layer that touches the database
- `models/` are shared across layers (no business logic in models)
- `adapters/` wrap external APIs (WAHA, Shopee, MercadoLivre, etc.)

## Frontend (`client/`)

Follows **Feature-Sliced Design (FSD)** principles:

```
client/src/
  app.tsx                  # Root component, router setup
  main.tsx                 # Entry point
  assets/                  # Static assets (images, fonts)
  components/              # Shared UI components (widgets layer)
    admin/                 # Admin-specific components
    ui/                    # shadcn/ui base components
  config/                  # App configuration
  constants/               # Application constants
  contexts/                # React contexts (auth, theme)
  entities/                # Domain entities (FSD entities layer)
    automation/
    dispatch-text/
    group/
    ml-link-list/
    whatsapp-instance/
    ...
  gen/                     # Generated ConnectRPC client code
  hooks/                   # Shared custom hooks
  i18n/                    # Internationalization (pt-br)
  integrations/            # Third-party integrations
  lib/                     # Utilities and helpers
    connect/               # ConnectRPC client setup, interceptors, error handler
  pages/                   # Page components (route-level, minimal logic)
  schemas/                 # Validation schemas
  services/                # Legacy API service (Axios-based)
  test/                    # Test utilities and fixtures
  types/                   # Shared TypeScript types
```

**FSD layer rules:**
- Pages are orchestration only (no business logic) -- logic lives in `usePageName()` hooks
- Entities do not import from each other (composed in higher layers)
- Shared (`lib/`, `components/ui/`) is business-agnostic
- `gen/` is auto-generated (never edit manually)

## Protocol Buffers (`proto/`)

Single source of truth for all API contracts:

```
proto/
  buf.yaml                 # Buf configuration
  admin/v1/                # Admin service definitions
  affiliates/v1/           # Affiliate management
  ai/v1/                   # AI service
  auth/v1/                 # Authentication
  automations/v1/          # Automation engine
  campaigns/v1/            # Campaign management
  common/v1/               # Shared types (pagination, etc.)
  coupons/v1/              # Coupon management
  dashboard/v1/            # Dashboard analytics
  dispatch_texts/v1/       # Message templates
  groups/v1/               # WhatsApp group management
  history/v1/              # Action history
  images/v1/               # Image management
  instances/v1/            # WhatsApp instance management
  jobs/v1/                 # Background job management
  ml_links/v1/             # MercadoLivre link management
  plans/v1/                # Subscription plans
  profiles/v1/             # User profiles
  redirect/v1/             # URL redirect service
  settings/v1/             # User settings
  webhooks/v1/             # Webhook management
  whatsapp/v1/             # WhatsApp messaging
```

Generated code goes to `client/src/gen/` (TypeScript) and `automatiza-go/internal/gen/` (Go). Regenerate with `make proto-generate`.
