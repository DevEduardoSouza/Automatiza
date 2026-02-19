---
slug: database
category: features
generatedAt: 2026-02-18T22:38:13.918Z
relevantFiles:
  - client/src/entities/dispatch-text/model/types.ts
  - client/src/entities/whatsapp-instance/model/types.ts
  - client/src/entities/ml-link-list/model/types.ts
  - client/src/entities/whatsapp-instance/lib/waha-helpers.ts
---

# How is data stored and accessed?

## Database Stack

- **PostgreSQL 16**: Primary relational database (33 production tables).
- **Redis**: Caching layer, job queue (Asynq), and real-time state.
- **pgx**: Go PostgreSQL driver (high-performance, native protocol).

## Migration Strategy

Migrations live in `automatiza-go/scripts/db/migrations/` and follow this execution order:

1. **`init.sql`**: Extensions, database roles (`anon`, `service_role`, `authenticator`), and the `update_updated_at_column()` trigger function. No tables are created here.
2. **`000_postgresql_init.sql`**: Consolidated migration that creates all 33 production tables. Runs first.
3. **`001` through `029`**: Incremental migrations (historical). These are idempotent after `000` runs.
4. **`seed.sql`**: Test/development seed data (uses `ON CONFLICT (user_id, role)` for the composite UNIQUE constraint on `user_roles`).

**Docker flow**: `init.sql` -> `run-migrations.sh` (000 first, then 001-029) -> `seed.sql`

**VPS flow**: `./server migrate up` (Go binary subcommand)

**Local reset**: `make db-reset && make db-migrate`

## Key Tables

| Table | Purpose |
|-------|---------|
| `users` | User accounts (email, password hash, profile) |
| `user_roles` | Role assignments (UNIQUE on `user_id, role`) |
| `plans` | Subscription plans and limits |
| `automations` | Automation configurations per user |
| `groups` | WhatsApp groups synced from WAHA |
| `dispatch_logs` | Message dispatch history (`group_id` is TEXT/JID, not UUID FK) |
| `dispatch_texts` | Message templates (`body` field, `is_active` flag) |
| `webhook_events` | Inbound webhook event log (`event_id` is VARCHAR(128)) |
| `webhook_logs` | Webhook processing audit trail |
| `jobs` | Background job definitions and scheduling |
| `action_history` | User action audit log |
| `sync_jobs` | Group synchronization job tracking |
| `categories` | System-wide categories (slug, icon, display_order) |
| `affiliate_credentials` | Third-party affiliate platform credentials |
| `saved_redirect_links` | Saved redirect/tracking URLs |
| `ml_link_list_items` | MercadoLivre product link items |

## Schema Design Decisions

- **Hard delete only**: No soft-delete pattern. Archival is handled separately when audit is needed.
- **`dispatch_logs.group_id`** is TEXT (WhatsApp JID format), not a UUID foreign key.
- **`dispatch_texts`** uses `body` (not `content`), `is_active` (not `is_default`), and has no `variables` column.
- **`categories`** are system-wide, not per-user.
- **`user_roles`** has a composite UNIQUE on `(user_id, role)`, allowing multiple roles per user.

## Data Access Patterns

### Repository Layer

All database access goes through the repository layer (`internal/repository/`):

- **Interfaces** defined in `internal/repository/interfaces.go`.
- **PostgreSQL implementations** in `internal/repository/postgres/`.
- Services never call the database directly; they use repository interfaces (dependency inversion).

### Query Patterns

- **Cursor pagination** for all list endpoints (no OFFSET-based pagination except admin screens).
- **Indexes** are created based on actual query patterns with EXPLAIN analysis, never speculatively.
- **Transactions** wrap multiple related writes with proper rollback on error.
- **context.Context** is propagated through every database call for cancellation and tracing.

### Redis Usage

- **Asynq job queue**: Background job scheduling and processing.
- **Caching**: Session data, temporary state, rate limiting.
- **Real-time state**: WhatsApp instance status, active process tracking.

## Performance Considerations

- All list endpoints enforce server-side pagination.
- Queries are optimized based on real usage telemetry (EXPLAIN/Datadog traces).
- Connection pooling is managed by pgx.
- No N+1 queries; batch operations are preferred for bulk data access.
