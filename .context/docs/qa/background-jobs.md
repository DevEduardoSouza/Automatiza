---
slug: background-jobs
category: operations
generatedAt: 2026-02-18T22:38:15.698Z
relevantFiles:
  - client/e2e/pages/jobs.page.ts
  - client/src/gen/jobs/v1/jobs_pb.ts
---

# How do background jobs work?

## Architecture

Background jobs use **Asynq**, a Redis-backed distributed task queue for Go. The backend supports two execution modes:

- **Server mode** (default): `go run ./cmd/server/main.go` -- serves HTTP/ConnectRPC API.
- **Worker mode**: `go run ./cmd/server/main.go -worker` -- processes background jobs from Redis queues.

Both modes share the same codebase but activate different components at startup.

## Job Workers

Workers live in `automatiza-go/internal/workers/` and process specific job types:

| Worker | File | Purpose |
|--------|------|---------|
| **Message Queue Worker** | `message_queue_worker.go` | Processes queued WhatsApp messages for dispatch |
| **Dispatch Log Processor** | `dispatch_log_processor.go` | Records dispatch results and updates logs |
| **Groups Sync Worker** | `groups_sync_worker.go` | Synchronizes WhatsApp groups from WAHA |
| **Webhook Consumer** | `webhook_consumer_worker.go` | Processes inbound webhook events (WAHA, Kiwify, etc.) |
| **Message Worker** | `message_worker.go` | Handles individual message sending tasks |
| **Monitoring Message Processor** | `monitoring_message_processor.go` | Processes monitoring/alerting messages |
| **Reconciliation Worker** | `reconciliation_worker.go` | Runs data reconciliation and backfill tasks |

## Job Services

Business logic for job management lives in `internal/services/`:

| Service | Purpose |
|---------|---------|
| `services/jobs/` | Job CRUD, scheduling, retry management |
| `services/automation/` | Automation execution engine (triggers workers) |
| `services/monitoring/` | Process monitoring and health checks |
| `services/reconciliation/` | Data consistency checks and gap filling |
| `services/watchdog/` | Silence detection -- alerts when no events arrive for X time |
| `services/cron/` | Scheduled/recurring job definitions |

## Reliability Patterns

### Idempotency

All consumers are idempotent by `payload.id` (or derived `idempotencyKey` when `payload.id` is absent). Duplicate events are safely accepted without side effects. The `webhook_events` table enforces uniqueness via `event_id`.

### Retry with Backoff

- Failed jobs are retried with **exponential backoff + jitter**.
- Retries are only applied because consumers are idempotent.
- Each job type has configurable `max_retries` (stored in the `jobs` table).

### Dead Letter Queue (DLQ)

- Poison messages (jobs that fail all retry attempts) are moved to a DLQ.
- DLQ messages never block the main queue.
- Admin UI allows inspection and manual retry of DLQ entries.

### Watchdog / Silence Detection

The watchdog service (`services/watchdog/`) monitors event flow:
- If no events arrive for a configurable time window, it triggers a health check.
- On detection of silence, it initiates reconnection/restart with backoff (no infinite loops).
- Works in conjunction with reconciliation to fill gaps.

### Reconciliation / Backfill

The reconciliation service (`services/reconciliation/`) acts as a compensatory mechanism:
- Runs on startup and when gaps/silence are detected.
- Uses WAHA message endpoints with timestamp/checkpoint filters to fetch missed events.
- Never relies 100% on real-time events alone.

## Webhook Ingestion Flow

```
WAHA Plus
  |
  v
POST /webhooks/waha  (Gin handler)
  |-- Validate HMAC signature
  |-- ACK immediately (fast response)
  |-- Enqueue payload to Redis (Asynq)
  |
  v
Webhook Consumer Worker
  |-- Dedup by event_id
  |-- Process event (business logic)
  |-- Update dispatch logs, groups, etc.
  |-- On failure: retry with backoff -> DLQ
```

## Job Management UI

The `JobsService` ConnectRPC service exposes job listing, status checking, retry, and cancellation to the frontend admin panel. Job states (`JobStatus`) and action types (`JobActionType`) are defined in `proto/jobs/v1/`.

## Monitoring

- Job processing metrics are exported to Datadog.
- Failed jobs trigger Sentry alerts with full context (job type, payload summary, error trace).
- Structured JSON logs include `traceId` for correlation with request traces.
