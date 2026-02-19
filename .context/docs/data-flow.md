---
type: doc
name: data-flow
description: How data moves through the system and external integrations
category: data-flow
generated: 2026-02-18
status: filled
scaffoldVersion: "2.0.0"
---

# Data Flow & Integrations

This document describes how data moves through the Automatiza platform -- from user interactions in the frontend, through backend services and async workers, to external systems and back. It covers module dependencies, the service layer, internal data movement, external integrations, and the observability pipeline.

## Module Dependencies

The system's modules have the following cross-cutting dependencies:

```
Frontend (React)
  └── ConnectRPC transport ──► Backend ConnectRPC Handlers
                                   │
                          ┌────────┼────────────┐
                          ▼        ▼            ▼
                      Services  Middleware   Interceptors
                          │        │            │
                          ▼        │            │
                     Repositories  │        Observability
                          │        │        (Sentry, Datadog)
                          ▼        ▼
                     PostgreSQL   Redis
                                   │
                                   ▼
                              Asynq Workers
                                   │
                          ┌────────┼────────┐
                          ▼        ▼        ▼
                        WAHA    Shopee   Mercado Livre
```

**Key dependency rules:**
- Handlers depend on services (never on repositories directly).
- Services depend on repository interfaces (not concrete implementations).
- Workers depend on services and the Redis-backed Asynq queue.
- Adapters (`internal/adapters/`) wrap external API calls and are consumed by services.
- Generated code (`internal/gen/`, `client/src/gen/`) is consumed but never manually edited.
- Proto definitions (`proto/`) are the source of truth; both Go and TypeScript code is generated from them.

## Service Layer

The backend organizes business logic into domain-specific service packages under `automatiza-go/internal/services/`:

| Service | Package | Responsibility |
|---------|---------|----------------|
| **AutomationService** | `automation/` | Core automation lifecycle: creation, scheduling, dispatch orchestration, process monitoring, search, and locking. Coordinates with WhatsApp service for message sending and dispatch log repository for tracking. |
| **WhatsAppService** | `whatsapp/` | WhatsApp session management, message sending through WAHA adapter, instance health checks. |
| **WAHAService** | `waha/` | Low-level WAHA API client operations, session lifecycle. |
| **RedirectService** | `redirect/` | Redirect link creation, click tracking, analytics aggregation. Public redirect resolution for shortened URLs. |
| **ShopeeService** | `shopee/` | Shopee affiliate API integration: product link generation, credential management. |
| **MercadoLivreService** | `mercadolivre/` | Mercado Livre link scraping and affiliate link generation. |
| **MlLinkListsService** | `mllinklists/` | Management of Mercado Livre link collections and list items. |
| **KiwifyService** | `kiwify/` | Processes Kiwify payment webhooks: subscription creation, plan upgrades, cancellations. |
| **JobsService** | `jobs/` | Background job management, status tracking, retry logic. |
| **DashboardService** | `dashboard/` | Aggregated statistics for the user dashboard (message counts, group stats, automation metrics). |
| **ProfileService** | `profile/` | User profile management, password changes (argon2id hashing). |
| **EmailService** | `email/` | Transactional email sending (password reset, magic links). |
| **UserService** | `user/` | User CRUD, role management, plan assignment. |
| **CouponService** | `coupons/` | Discount coupon creation and validation. |
| **AIService** | `ai/` | AI-powered tools for text generation and content assistance. |
| **CronService** | `cron/` | Scheduled task management and execution. |
| **MonitoringService** | `monitoring/` | WAHA instance health monitoring and alerting. |
| **WatchdogService** | `watchdog/` | Silence detection for WAHA event streams; triggers reconnection with backoff when no events arrive within threshold. |
| **ReconciliationService** | `reconciliation/` | Compensatory backfill mechanism that runs on startup or gap detection to recover missed WAHA events using timestamp-based queries. |
| **LoadBalancerService** | `loadbalancer/` | Distribution of WhatsApp sessions across WAHA instances. |

## High-Level Flow: Automation Dispatch

The primary data flow -- from automation creation to message delivery -- follows this sequence:

```
1. User creates automation (Frontend)
   │
   ▼
2. ConnectRPC call → AutomationsHandler.CreateAutomation
   │
   ▼
3. AutomationService validates and persists to PostgreSQL
   │  (automations table + related dispatch_texts)
   ▼
4. CronService schedules the automation based on user-defined timing
   │
   ▼
5. At trigger time: AutomationService.ProcessAutomation
   │  - Selects target groups
   │  - Selects dispatch text (rotation/random)
   │  - Builds message payloads
   ▼
6. Messages enqueued to Redis via Asynq
   │  (one task per message/group)
   ▼
7. MessageQueueWorker dequeues and dispatches
   │  - Calls WAHA adapter to send message
   │  - Respects rate limits and delays
   ▼
8. WAHA sends WhatsApp message
   │
   ▼
9. DispatchLogProcessor records result in dispatch_logs
   │
   ▼
10. ActionHistory records the automation execution event
```

## Internal Data Movement

### ConnectRPC (Frontend to Backend)

All typed API communication uses ConnectRPC (gRPC-Web protocol). The frontend uses `@connectrpc/connect-query` with TanStack Query for automatic caching, deduplication, and background refetching.

- **Transport**: HTTP/1.1 with gRPC-Web content type (binary or JSON).
- **Auth**: JWT token injected via interceptor from HttpOnly cookie. CSRF token sent as header.
- **Tracing**: `traceparent` header propagated via interceptor for distributed tracing.
- **Error handling**: Centralized interceptor maps gRPC status codes to UI actions (inline errors, toasts, redirects).
- **Cancellation**: `AbortSignal` from React Query prevents orphaned requests.

### Redis Message Queue (Asynq)

Redis backs the async job processing system via the Asynq library:

- **Message dispatch**: Automation messages are enqueued as Asynq tasks with payloads containing group JID, message content, media references, and dispatch metadata.
- **Dispatch log processing**: After each message send, a separate task records the result (success/failure, latency, error details) in `dispatch_logs`.
- **Group sync**: Periodic synchronization of WhatsApp groups from WAHA to PostgreSQL.
- **Webhook event consumption**: WAHA webhook events are ACKed immediately and enqueued for async processing.
- **Monitoring messages**: Health check messages for instance monitoring.
- **Reconciliation**: Backfill tasks triggered on startup or silence detection.

Workers are configured with retry policies (backoff + jitter), DLQ for poison messages, and concurrency limits.

### SSE (Server-Sent Events)

Real-time session status updates flow from backend to frontend via SSE:

- **Endpoint**: `GET /sse/session-status`
- **Bridge**: `internal/handlers/sse_redis_bridge.go` uses Redis Pub/Sub to broadcast WAHA session state changes to all connected SSE clients.
- **Use case**: Frontend displays real-time WhatsApp connection status (connected, disconnected, QR scan needed).

### Database Access

All PostgreSQL access goes through repository implementations in `internal/repository/postgres/`:

- **Connection**: pgx connection pool with context-aware queries.
- **Pagination**: Cursor-based pagination for all list endpoints (OFFSET only for small admin views).
- **Transactions**: Used for multi-table mutations (e.g., creating automation + dispatch texts).
- **Migrations**: Consolidated in `scripts/db/migrations/` -- `000_postgresql_init.sql` creates all 33 tables, followed by incremental migrations `001-029`.

## External Integrations

### WAHA (WhatsApp HTTP API)

| Direction | Mechanism | Details |
|-----------|-----------|---------|
| **Outbound** | REST API calls | `internal/adapters/whatsapp/waha_adapter.go` sends messages, manages sessions, fetches group data via WAHA's HTTP API. |
| **Inbound** | Webhooks | `internal/handlers/waha_webhook_handler.go` receives WAHA events (message received, session status change, QR code). HMAC validation via `internal/adapters/whatsapp/hmac_validator.go` using `crypto/subtle.ConstantTimeCompare`. |
| **Processing** | Async worker | Webhook payloads are ACKed immediately, enqueued to Redis, and processed by `internal/workers/webhook_consumer_worker.go`. Consumer is idempotent by `event_id`. |
| **Resilience** | Watchdog + Reconciliation | `internal/services/watchdog/` monitors event silence and triggers reconnection. `internal/services/reconciliation/` backfills missed events on startup using timestamp checkpoints stored in `reconciliation_checkpoints` table. |

### Kiwify (Payment Webhooks)

| Direction | Mechanism | Details |
|-----------|-----------|---------|
| **Inbound** | Webhooks | Receives purchase, subscription, and cancellation events. Webhook signature validated via HMAC. |
| **Processing** | KiwifyService | Maps webhook events to plan changes: new subscriptions create user plans, cancellations downgrade, refunds trigger plan removal. Events logged in `webhook_logs` and `webhook_events`. |

### Shopee (Affiliate API)

| Direction | Mechanism | Details |
|-----------|-----------|---------|
| **Outbound** | REST API | `internal/services/shopee/shopee_service.go` generates affiliate links from product URLs using Shopee's affiliate API. Credentials stored in `affiliate_credentials` table per user. |

### Mercado Livre (Affiliate Links)

| Direction | Mechanism | Details |
|-----------|-----------|---------|
| **Outbound** | HTTP scraping + API | `internal/services/mercadolivre/` scrapes product data and generates affiliate links. Links organized into lists (`ml_link_lists`, `ml_link_list_items`). |

## Observability Pipeline

### Sentry (Error Tracking)

- **Backend**: Initialized in `main.go` with `sentry.Init()`. Errors captured via:
  - ConnectRPC interceptor (`internal/observability/sentry_interceptor.go`) -- captures panics and unhandled errors with tags (module, method).
  - Asynq error handler (`internal/observability/sentry_asynq.go`) -- captures worker failures.
  - Explicit `sentry.CaptureException()` calls in critical error paths.
- **Frontend**: Sentry SDK captures unhandled exceptions and performance transactions.
- **Tags**: `module`, `flow`, `user_id`, `method` for filtering and grouping.

### Datadog (APM / Metrics)

- **Backend**: dd-trace-go initialized in `main.go`. Gin instrumented with Datadog middleware. All HTTP requests and ConnectRPC calls generate traces.
- **Spans**: Custom spans in services for external API calls (WAHA, Shopee). Errors mark spans as failed.
- **Metrics**: System metrics (CPU, memory) collected by Datadog agent on VPS.

### Structured Logging

- **Format**: JSON structured logs with consistent fields: `traceId`, `method`, `duration`, `status`, `error`.
- **PII Redaction**: Sensitive fields (tokens, passwords, phone numbers) are redacted before logging.
- **Correlation**: `traceId` propagated from frontend (`traceparent` header) through backend to logs, enabling end-to-end request tracing across Sentry events, Datadog traces, and application logs.
