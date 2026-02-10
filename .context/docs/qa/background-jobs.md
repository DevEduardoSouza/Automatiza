---
status: filled
slug: background-jobs
category: operations
---

# How do background jobs work?

## Job System Architecture

The platform uses **Asynq** (Redis-based task queue) for background processing.

```
┌─────────────┐     ┌─────────┐     ┌─────────────┐
│  HTTP/RPC   │────▶│  Redis  │────▶│   Workers   │
│   Server    │     │  Queue  │     │  (Asynq)    │
└─────────────┘     └─────────┘     └─────────────┘
                                           │
                                           ▼
                                    ┌─────────────┐
                                    │ SSE Bridge  │
                                    │ (Real-time) │
                                    └─────────────┘
```

## Running Workers

```bash
# Start worker process
cd automatiza-go
go run ./cmd/server/main.go -worker

# Or via Makefile
make worker
```

## Job Types

### Message Jobs
```go
// Send WhatsApp message
type SendMessagePayload struct {
  InstanceID string
  GroupJID   string
  Message    string
  MediaURL   string
  Delay      time.Duration
}
```

### Automation Jobs
```go
// Execute automation workflow
type AutomationJobPayload struct {
  AutomationID string
  UserID       string
  Mode         AutomationMode // SEARCH, MONITOR, LIST
}
```

### Sync Jobs
```go
// Sync WhatsApp groups
type SyncGroupsPayload struct {
  InstanceID string
  UserID     string
}
```

### Reconciliation Jobs
```go
// Fill gaps in message history
type ReconciliationPayload struct {
  InstanceID string
  StartTime  time.Time
  EndTime    time.Time
}
```

## Worker Implementation

Located in `automatiza-go/internal/workers/`:

| Worker | File | Purpose |
|--------|------|---------|
| Message Worker | `message_worker.go` | Send WhatsApp messages |
| Groups Sync | `groups_sync_worker.go` | Sync group lists |
| Webhook Consumer | `webhook_consumer.go` | Process WAHA events |
| Reconciliation | `reconciliation_worker.go` | Gap filling |
| SSE Notifier | `sse_notifier.go` | Real-time UI updates |

### Worker Pattern
```go
func (w *MessageWorker) ProcessTask(ctx context.Context, task *asynq.Task) error {
  var payload SendMessagePayload
  if err := json.Unmarshal(task.Payload(), &payload); err != nil {
    return fmt.Errorf("unmarshal payload: %w", err)
  }

  // Execute with retry logic
  return w.sendMessage(ctx, payload)
}
```

## Job Scheduling

### Scheduled Jobs (Cron-like)
```go
// Create scheduled job
job := &models.ScheduledJob{
  AutomationID: automationID,
  CronExpr:     "0 8-22 * * *", // Every hour 8am-10pm
  NextRun:      nextRunTime,
  Status:       JobStatusActive,
}
```

### One-time Jobs
```go
// Queue immediate job
task := asynq.NewTask("send_message", payload)
client.Enqueue(task, asynq.MaxRetry(3))

// Queue delayed job
client.Enqueue(task, asynq.ProcessIn(5*time.Minute))
```

## Job Status & Monitoring

### Job Status Enum
```protobuf
enum JobStatus {
  JOB_STATUS_UNSPECIFIED = 0;
  JOB_STATUS_PENDING = 1;
  JOB_STATUS_RUNNING = 2;
  JOB_STATUS_COMPLETED = 3;
  JOB_STATUS_FAILED = 4;
  JOB_STATUS_CANCELLED = 5;
}
```

### Queue Metrics (Admin)
```typescript
interface QueueMetrics {
  pending: number;
  active: number;
  completed: number;
  failed: number;
  scheduled: number;
}
```

## Frontend Integration

### Jobs Page
```typescript
// client/src/hooks/use-jobs-connect.ts
const { data: jobs } = useJobsConnect();

// Real-time updates via SSE
useJobsRealtime((event) => {
  if (event.type === 'JOB_COMPLETED') {
    queryClient.invalidateQueries(['jobs']);
  }
});
```

### Page Object (E2E)
```typescript
// client/e2e/pages/jobs.page.ts
class JobsPage {
  jobsTable = this.page.locator('[data-testid="jobs-table"]');

  async cancelJob(jobId: string) { ... }
  async retryJob(jobId: string) { ... }
}
```

## Real-time Updates (SSE)

Workers publish events to Redis Pub/Sub, which are streamed to clients:

```go
// Worker publishes
redis.Publish(ctx, "sse:user:"+userID, event)

// SSE handler streams
func (h *SSEHandler) Stream(c *gin.Context) {
  pubsub := h.redis.Subscribe(ctx, "sse:user:"+userID)
  for msg := range pubsub.Channel() {
    fmt.Fprintf(w, "data: %s\n\n", msg.Payload)
    w.Flush()
  }
}
```

## Error Handling & Retries

```go
// Retry configuration
asynq.MaxRetry(3)           // Max 3 retries
asynq.Timeout(5*time.Minute) // 5 min timeout
asynq.Queue("critical")      // Priority queue

// Dead letter queue
// Failed jobs after max retries go to dead letter
// Can be manually retried via admin panel
```
