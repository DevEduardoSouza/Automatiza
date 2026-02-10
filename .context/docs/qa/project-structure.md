---
status: filled
slug: project-structure
category: architecture
---

# How is the codebase organized?

## High-Level Structure

```
novo-projeto/
├── client/              # React TypeScript Frontend
├── automatiza-go/       # Go Backend Services
├── proto/               # Protocol Buffer Definitions
├── .context/            # AI Agent Context
├── Makefile             # Development Automation
└── CLAUDE.md            # AI Documentation Index
```

## Frontend (`client/`)

```
client/
├── src/
│   ├── main.tsx              # Entry point
│   ├── app.tsx               # Root component with routing
│   │
│   ├── pages/                # 23+ page components
│   │   ├── auth.tsx          # Login page
│   │   ├── dashboard.tsx     # Main dashboard
│   │   ├── groups.tsx        # WhatsApp groups
│   │   ├── whatsapp.tsx      # Instance management
│   │   └── ...
│   │
│   ├── components/           # Reusable UI components
│   │   ├── ui/               # shadcn/ui primitives (60+)
│   │   ├── groups/           # Group-related components
│   │   ├── automations/      # Automation components
│   │   ├── layout/           # Layout components
│   │   └── ...
│   │
│   ├── hooks/                # 50+ custom React hooks
│   │   ├── use-groups-connect.ts
│   │   ├── use-automations-connect.ts
│   │   ├── use-whatsapp-connect.ts
│   │   └── ...
│   │
│   ├── gen/                  # Generated proto code (TS)
│   │   ├── auth/v1/
│   │   ├── groups/v1/
│   │   └── ...
│   │
│   ├── contexts/             # React contexts (auth, theme)
│   ├── schemas/              # Zod validation schemas
│   ├── lib/                  # Utilities (connect-transport, etc.)
│   ├── entities/             # Domain models
│   ├── constants/            # App constants
│   └── i18n/                 # Internationalization
│
├── e2e/                      # Playwright E2E tests
│   ├── pages/                # Page object models
│   └── fixtures/             # Test fixtures & mocks
│
├── vite.config.ts            # Vite bundler config
├── playwright.config.ts      # Playwright config
└── package.json              # Dependencies
```

## Backend (`automatiza-go/`)

```
automatiza-go/
├── cmd/
│   └── server/               # Main server entry point
│
├── internal/
│   ├── connectrpc/           # 25+ ConnectRPC service handlers
│   │   ├── auth_handler.go
│   │   ├── groups_handler.go
│   │   ├── automations_handler.go
│   │   └── ...
│   │
│   ├── services/             # 22+ business logic services
│   │   ├── automation/       # Workflow execution
│   │   ├── whatsapp/         # WAHA adapter
│   │   ├── email/            # SMTP service
│   │   └── ...
│   │
│   ├── repository/           # Data access layer
│   │   └── postgres/         # PostgreSQL implementations
│   │
│   ├── models/               # 26+ domain models
│   ├── handlers/             # HTTP handlers (Gin, SSE)
│   ├── middleware/           # CORS, logging, auth
│   ├── auth/                 # JWT, CSRF, rate limiting
│   ├── workers/              # Background job workers
│   ├── adapters/             # External integrations
│   ├── gen/                  # Generated proto code (Go)
│   └── config/               # Configuration management
│
├── tests/
│   ├── e2e/                  # End-to-end tests
│   ├── integration/          # Integration tests
│   └── load/                 # Load tests
│
├── Dockerfile                # Multi-stage Docker build
├── docker-compose.yml        # Local dev environment
└── go.mod                    # Go dependencies
```

## Protocol Buffers (`proto/`)

```
proto/
├── auth/v1/auth.proto        # Authentication service
├── groups/v1/groups.proto    # WhatsApp groups
├── automations/v1/           # Automation workflows
├── whatsapp/v1/              # Instance management
├── jobs/v1/                  # Background jobs
├── affiliates/v1/            # Shopee, ML integrations
├── common/v1/pagination.proto # Shared types
└── buf.yaml                  # Buf configuration
```

## Key Architectural Patterns

| Pattern | Location | Purpose |
|---------|----------|---------|
| Page Objects | `client/e2e/pages/` | E2E test abstraction |
| Repository | `automatiza-go/internal/repository/` | Data access |
| Service Layer | `automatiza-go/internal/services/` | Business logic |
| ConnectRPC Handlers | `automatiza-go/internal/connectrpc/` | API endpoints |
| React Hooks | `client/src/hooks/` | API consumption |
| Interceptors | `client/src/lib/connect-transport.ts` | CSRF, retry, tracing |
