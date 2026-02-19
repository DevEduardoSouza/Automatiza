---
slug: api-endpoints
category: features
generatedAt: 2026-02-18T22:38:13.920Z
relevantFiles:
  - client/src/services/api.service.ts
  - client/src/components/admin/types/team.types.ts
  - client/src/components/admin/types/email-templates.types.ts
---

# What API endpoints are available?

## Primary API: ConnectRPC (gRPC-Web)

All primary API endpoints are defined as ConnectRPC services in Protocol Buffer files under `proto/`. The backend exposes these via Gin, and the frontend consumes them using `@connectrpc/connect-query` (useQuery/useMutation) for automatic caching and deduplication.

### Service Catalog

| Service | Proto Package | Purpose |
|---------|--------------|---------|
| **AuthService** | `auth/v1` | Login, logout, token refresh, session management |
| **AutomationsService** | `automations/v1` | CRUD for automation rules, execution control |
| **GroupsService** | `groups/v1` | WhatsApp group listing, sync, metadata |
| **WhatsAppService** | `whatsapp/v1` | Message sending, instance management |
| **InstancesService** | `instances/v1` | WhatsApp instance lifecycle (create, connect, disconnect) |
| **WebhooksService** | `webhooks/v1` | Webhook configuration and event listing |
| **RedirectService** | `redirect/v1` | URL redirect/tracking link management |
| **MlLinksService** | `ml_links/v1` | MercadoLivre product link management |
| **AffiliatesService** | `affiliates/v1` | Affiliate platform credential management |
| **JobsService** | `jobs/v1` | Background job listing, retry, cancellation |
| **DashboardService** | `dashboard/v1` | Analytics, stats, and overview data |
| **AdminService** | `admin/v1` | User management, system config (admin-only) |
| **AIService** | `ai/v1` | AI-powered features (text generation, etc.) |
| **ProfilesService** | `profiles/v1` | User profile management, password change |
| **PlansService** | `plans/v1` | Subscription plan details and limits |
| **SettingsService** | `settings/v1` | User preferences and app settings |
| **HistoryService** | `history/v1` | Action history and audit log |
| **DispatchTextsService** | `dispatch_texts/v1` | Message template CRUD |
| **ImagesService** | `images/v1` | Image upload and management |
| **CampaignsService** | `campaigns/v1` | Campaign management |
| **CouponsService** | `coupons/v1` | Coupon/discount management |

### ConnectRPC Client Setup

The frontend configures the ConnectRPC transport in `client/src/lib/connect/` with mandatory interceptors:

- **Auth interceptor**: Injects JWT token from cookies.
- **Tracing interceptor**: Propagates `traceparent` header for distributed tracing.
- **Error interceptor**: Centralizes error handling and maps gRPC codes to UI actions.

## Non-RPC Endpoints

These endpoints are served by Gin handlers outside the ConnectRPC layer:

### WAHA Webhook Handler

- **Route**: `POST /webhooks/waha`
- **Purpose**: Receives webhook events from WAHA Plus (WhatsApp session events, messages, etc.).
- **Behavior**: Validates HMAC signature (when enabled), ACKs immediately, and enqueues the payload into the Redis job queue for async processing. No business logic in the handler.
- **Source**: `internal/handlers/waha_webhook_handler.go`

### Public Redirect Handler

- **Route**: `GET /r/:code`
- **Purpose**: Public-facing URL redirect for tracking links. No authentication required.
- **Source**: `internal/handlers/redirect_public_handler.go`

### Session SSE (Server-Sent Events)

- **Route**: SSE endpoint for real-time session status.
- **Purpose**: Pushes session state changes (login, logout, expiration) to the frontend in real time.

## Legacy REST API

A small number of legacy endpoints are accessed via Axios through `client/src/services/api.service.ts`. These are being phased out in favor of ConnectRPC.

## API Contract Rules

- **Protobuf is the single source of truth**: All request/response models are defined in `.proto` files. TypeScript and Go code are always generated (`make proto-generate`).
- **Validation**: Input validation rules live in the `.proto` files (protovalidate/CEL annotations) and are enforced via backend interceptors.
- **Versioning**: Breaking changes require a new version (v2) while maintaining backward compatibility with v1.
- **Payload limits**: Request size limits are enforced at the server level.
