---
status: filled
slug: database
category: features
---

# How is data stored and accessed?

## Database Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| Primary DB | PostgreSQL 16 | Persistent data storage |
| Cache | Redis | Session cache, rate limiting, task queues |
| ORM | Raw SQL (pgx) | Direct PostgreSQL driver |

## Repository Pattern

All data access goes through repository interfaces in `automatiza-go/internal/repository/`.

### Repository Structure
```
internal/repository/
├── postgres/           # PostgreSQL implementations
│   ├── profile.go
│   ├── instance.go
│   ├── group.go
│   ├── automation.go
│   └── ...
└── interfaces.go       # Repository interfaces
```

### Example Interface
```go
type ProfileRepository interface {
    GetProfile(ctx context.Context, userID string) (*models.Profile, error)
    CreateProfile(ctx context.Context, profile *models.Profile) (*models.Profile, error)
    UpdateProfile(ctx context.Context, profile *models.Profile) error
    GetProfileByEmail(ctx context.Context, email string) (*models.Profile, error)
}
```

## Key Domain Models

Located in `automatiza-go/internal/models/`:

| Model | Description |
|-------|-------------|
| `Profile` | User account with plan, settings |
| `Instance` | WhatsApp instance (WAHA connection) |
| `Group` | WhatsApp group with metadata |
| `Automation` | Automation workflow definition |
| `DispatchLog` | Message dispatch history |
| `ActionHistory` | User action audit trail |
| `ScheduledJob` | Background job definition |
| `Coupon` | Discount coupon |
| `MLLink` | Mercado Livre product link |
| `RedirectLink` | Shortened redirect link |

## Database Schema (Key Tables)

```sql
-- Users and Auth
profiles (id, email, display_name, plan, settings, created_at)
user_roles (id, user_id, role)
password_reset_tokens (id, user_id, token_hash, expires_at)
rate_limit_attempts (key, count, window_start)

-- WhatsApp
instances (id, user_id, name, status, waha_id, qr_code)
groups (id, instance_id, jid, name, member_count, settings)

-- Automations
automations (id, user_id, name, mode, status, config, schedule)
dispatch_logs (id, automation_id, group_id, message, status, sent_at)
scheduled_jobs (id, automation_id, next_run, status)

-- Commerce
coupons (id, user_id, store, code, description, active)
ml_links (id, user_id, url, title, category)
redirect_links (id, user_id, short_code, target_url, clicks)

-- Audit
action_history (id, user_id, action_type, details, created_at)
system_events (id, event_type, payload, created_at)
```

## Connection Configuration

```go
// automatiza-go/internal/config/database.go
pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))

// Pool settings
config.MaxConns = 25
config.MinConns = 5
config.MaxConnLifetime = time.Hour
config.MaxConnIdleTime = 30 * time.Minute
```

## Redis Usage

```go
// Session caching
redis.Set(ctx, "session:"+sessionID, sessionData, 24*time.Hour)

// Rate limiting
redis.Incr(ctx, "ratelimit:"+key)
redis.Expire(ctx, "ratelimit:"+key, windowDuration)

// Task queues (Asynq)
asynq.NewTask("send_message", payload)
```

## Frontend Data Access

Frontend accesses data through ConnectRPC hooks:

```typescript
// Example: useGroupsConnect hook
const { data: groups, isLoading } = useQuery({
  queryKey: ['groups', 'list', userId],
  queryFn: async () => {
    const client = createClient(GroupsService, transport);
    const response = await client.listGroups({});
    return response.groups.map(fromProtoGroup);
  },
});
```

## Migrations

Located in `automatiza-go/scripts/db/`:

```bash
# Run migrations
cd automatiza-go
go run ./scripts/db/migrate up

# Create new migration
go run ./scripts/db/migrate create <name>
```
