#---
status: filled
---

# Architecture Notes

Automatiza follows a modern client-server architecture with a React TypeScript frontend communicating with Go backend services via Connect-RPC (gRPC-compatible HTTP protocol). The system is designed for WhatsApp marketing automation with modular feature domains.

## System Architecture Overview

The platform uses a **modular monolith** frontend with a **microservices-ready** backend:

- **Frontend**: Single-page React application with feature-based module organization
- **Backend**: Go services with Protocol Buffer contracts
- **Communication**: Connect-RPC over HTTP (gRPC-web compatible)
- **WhatsApp Integration**: Evolution API / WAHA for WhatsApp Web automation

Request flow: `Browser → React App → Connect-RPC Client → Go Backend → WhatsApp API / Database`

## Architectural Layers

- **Config**: Application configuration and constants (`client/src/config/`, `client/src/constants/`)
- **Services**: API clients and business logic (`client/src/services/`, `client/src/gen/*/`)
- **Models**: Data schemas and entities (`client/src/schemas/`, `client/src/entities/`)
- **Components**: React UI components (`client/src/components/`)
- **Utils**: Shared utilities (`client/src/lib/`)
- **Controllers**: Routing and navigation (`client/src/components/routing/`)

> See [`codebase-map.json`](./codebase-map.json) for complete symbol counts and dependency graphs.

## Detected Design Patterns

| Pattern | Confidence | Locations | Description |
|---------|------------|-----------|-------------|
| Repository | 90% | `client/src/hooks/groups/` | Data access abstraction via hooks |
| Factory | 85% | `client/src/lib/connect/` | Transport/client factory for API |
| Observer | 80% | `client/src/hooks/use-session-status-sse.ts` | SSE event subscription |
| Facade | 85% | `client/src/hooks/use-*.ts` | Simplified API access hooks |
| Adapter | 90% | `client/src/gen/*/` | Protocol Buffer to TypeScript adapters |

## Entry Points

- [`client/src/main.tsx`](../client/src/main.tsx:1) - React application bootstrap
- [`client/src/App.tsx`](../client/src/App.tsx:1) - Root component with routing
- `automatiza-go/` - Go backend main entry

## Public API

Key exported services and types:

| Symbol | Type | Location |
|--------|------|----------|
| `AuthProvider` | Context | `client/src/contexts/auth-context.tsx:215` |
| `useWhatsAppConnect` | Hook | `client/src/hooks/use-whatsapp-connect.ts` |
| `useAutomationsConnect` | Hook | `client/src/hooks/use-automations-connect.ts` |
| `useGroupsConnect` | Hook | `client/src/hooks/groups/use-groups.ts` |
| `ApiService` | Class | `client/src/services/api.service.ts:62` |
| `ErrorBoundary` | Component | `client/src/components/error-boundary.tsx:21` |

## Internal System Boundaries

**Feature Domains** (bounded contexts):
- **WhatsApp**: Instance management, group sync, messaging (`client/src/gen/whatsapp/v1/`)
- **Automations**: Scheduled tasks and workflows (`client/src/gen/automations/v1/`)
- **Affiliates**: Shopee/ML product integration (`client/src/gen/affiliates/v1/`)
- **Admin**: User management, settings, plans (`client/src/components/admin/`)

Each domain has its own:
- Generated proto client (`client/src/gen/{domain}/v1/`)
- React hook for data access (`client/src/hooks/use-{domain}-connect.ts`)
- UI components (`client/src/components/{domain}/`)

## External Service Dependencies

- **WhatsApp API** (Evolution/WAHA): Message sending, group management
- **Shopee Affiliate API**: Product search, link generation
- **Mercado Livre API**: Product listings, affiliate links
- **Kiwify Webhooks**: Payment event processing

## Key Decisions & Trade-offs

1. **Connect-RPC over REST**: Type-safe API contracts with proto generation, HTTP/2 support
2. **Feature-based hook organization**: Each domain exposes a single hook (e.g., `useWhatsAppConnect`) that encapsulates all operations
3. **Generated clients**: Proto-first development with `buf` tooling
4. **SSE for real-time**: Server-Sent Events for session status updates

## Top Directories Snapshot

- `client/src/components/` — UI components (~50+ files)
- `client/src/gen/` — Generated proto clients (~25 service domains)
- `client/src/hooks/` — React hooks (~30+ files)
- `client/e2e/` — Playwright tests (~20+ spec files)
- `proto/` — Protocol Buffer definitions

## Related Resources

- [Project Overview](./project-overview.md)
- [Data Flow](./data-flow.md)
- [`codebase-map.json`](./codebase-map.json)
