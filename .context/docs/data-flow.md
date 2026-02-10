#---
status: filled
---

# Data Flow & Integrations

This document describes how data flows through the Automatiza platform, from user interactions to external service integrations.

## Module Dependencies

- **Components** → `hooks`, `lib`, `gen/*` (proto clients)
- **Hooks** → `gen/*` (generated clients), `lib/connect`
- **Pages** → `components`, `hooks`
- **Services** → `lib/connect-transport`
- **Gen** → `@connectrpc/connect` (runtime)

## Service Layer

Key service integrations:

- [`ApiService`](../client/src/services/api.service.ts:62) - HTTP client for public endpoints
- [`connect-transport`](../client/src/lib/connect-transport.ts) - Connect-RPC transport configuration
- Generated proto clients in `client/src/gen/*/` for each domain

## High-level Flow

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐     ┌──────────────┐
│   Browser   │────▶│  React App   │────▶│  Go Backend │────▶│  WhatsApp    │
│   (User)    │◀────│  (Frontend)  │◀────│  (gRPC)     │◀────│  API         │
└─────────────┘     └──────────────┘     └─────────────┘     └──────────────┘
                           │                    │
                           │                    ▼
                           │              ┌──────────────┐
                           │              │  Database    │
                           │              └──────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │  Affiliate   │
                    │  APIs        │
                    │  (Shopee/ML) │
                    └──────────────┘
```

**Primary Flow**:
1. User interacts with React components
2. Components call React hooks (`useWhatsAppConnect`, `useAutomationsConnect`, etc.)
3. Hooks use generated Connect-RPC clients to call backend
4. Backend processes request and interacts with WhatsApp API/Database
5. Response flows back through the same chain

## Internal Movement

**Frontend State Management**:
- React Query for server state caching and synchronization
- Local state via React hooks for UI state
- Context providers for global state (Auth, Theme)

**Real-time Updates**:
- SSE (Server-Sent Events) for session status monitoring (`use-session-status-sse.ts`)
- Polling for automation status updates

**Data Transformation**:
- Proto messages converted to local types at hook layer
- Local types use snake_case for UI compatibility
- Proto types use PascalCase (generated)

## External Integrations

### WhatsApp (Evolution API / WAHA)
- **Purpose**: Message sending, group management, instance control
- **Auth**: API key per instance
- **Operations**: `createInstance`, `sendMessage`, `syncGroups`, `fetchParticipants`

### Shopee Affiliate API
- **Purpose**: Product search, affiliate link generation
- **Auth**: OAuth credentials stored in `AffiliateCredential`
- **Operations**: `searchProducts`, `generateLink`

### Mercado Livre API
- **Purpose**: Product listings, ML link management
- **Auth**: OAuth tokens
- **Operations**: `listProducts`, `createMlLink`

### Kiwify Webhooks
- **Purpose**: Payment event processing
- **Auth**: Webhook signature verification
- **Operations**: `processWebhook`, `simulateWebhook`

## Observability & Failure Modes

**Error Handling**:
- `ConnectErrorInfo` type for standardized API errors
- `AuthError` mapping for authentication failures
- `ErrorBoundary` component for React error catching

**Retry Strategy**:
- Connect-RPC handles transport-level retries
- Application-level retry via React Query's built-in retry logic

**Logging**:
- Action history tracked via `ActionHistory` type
- Dispatch logs for message sending status

See [Architecture](./architecture.md) for system design details.
