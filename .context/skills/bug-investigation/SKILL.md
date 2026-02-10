---
type: skill
name: Bug Investigation
description: Systematic bug investigation and root cause analysis for Automatiza
skillSlug: bug-investigation
phases: [E, V]
status: filled
---

# Bug Investigation

## When to Use

- Investigating reported bugs
- Diagnosing unexpected behavior
- Finding root causes
- Debugging production issues

## Investigation Process

### 1. Reproduce the Issue

```bash
# Check environment
node --version   # Should be 20+
go version       # Should be 1.24+

# Start services
cd automatiza-go && docker-compose up -d
cd automatiza-go && go run ./cmd/server/main.go
cd client && npm run dev
```

### 2. Gather Information

**Frontend:**
- Browser console (errors, warnings)
- Network tab (failed requests)
- React DevTools (component state)
- Redux/Query DevTools (if applicable)

**Backend:**
- Server logs (`automatiza-go` console)
- Database state (relevant tables)
- Redis state (if applicable)

### 3. Isolate the Problem

**Questions to Answer:**
- Which component/service is affected?
- When did it start happening?
- Is it reproducible consistently?
- What are the exact steps to reproduce?

### 4. Find Root Cause

**Common Patterns:**

#### Frontend Issues

```typescript
// Stale closure in useCallback
const handleClick = useCallback(() => {
  // BUG: Uses stale `count` value
  console.log(count);
}, []); // Missing dependency

// FIX:
const handleClick = useCallback(() => {
  console.log(count);
}, [count]); // Add dependency
```

```typescript
// Missing null check
const name = user.profile.name; // BUG: profile might be null

// FIX:
const name = user?.profile?.name ?? 'Unknown';
```

```typescript
// Missing query invalidation
const mutation = useMutation({
  mutationFn: createGroup,
  // BUG: List doesn't update after creation
});

// FIX:
const mutation = useMutation({
  mutationFn: createGroup,
  onSuccess: () => {
    queryClient.invalidateQueries(['groups']);
  },
});
```

#### Backend Issues

```go
// Missing error check
result, _ := repo.GetByID(ctx, id) // BUG: Ignoring error
if result == nil { ... }

// FIX:
result, err := repo.GetByID(ctx, id)
if err != nil {
  return nil, fmt.Errorf("get by id: %w", err)
}
```

```go
// Race condition
var counter int // BUG: Not thread-safe

// FIX:
var counter atomic.Int64
// or use sync.Mutex
```

```go
// N+1 query
for _, group := range groups {
  members, _ := repo.GetMembers(ctx, group.ID) // BUG: N queries
}

// FIX: Batch query
membersByGroup, _ := repo.GetMembersBatch(ctx, groupIDs)
```

### 5. Verify Fix

1. Write a test that fails without the fix
2. Apply the fix
3. Verify the test passes
4. Run full test suite
5. Manual testing

## Debugging Tools

### Frontend

```typescript
// Debug React Query
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';

// Add to app
<ReactQueryDevtools initialIsOpen={false} />

// Debug in component
const { data, error, isLoading } = useQuery(...);
console.log({ data, error, isLoading });
```

### Backend

```go
// Add structured logging
logger.Info("processing request",
  zap.String("id", id),
  zap.Any("input", input),
)

// Debug SQL
db.LogMode(true)

// Profile performance
import _ "net/http/pprof"
go http.ListenAndServe(":6060", nil)
// Then: go tool pprof http://localhost:6060/debug/pprof/heap
```

### Database

```sql
-- Check recent data
SELECT * FROM groups ORDER BY created_at DESC LIMIT 10;

-- Check for missing relations
SELECT g.* FROM groups g
LEFT JOIN instances i ON g.instance_id = i.id
WHERE i.id IS NULL;

-- Analyze query performance
EXPLAIN ANALYZE SELECT * FROM groups WHERE instance_id = 'xxx';
```

## Bug Report Template

```markdown
## Bug Description
[Clear description of the issue]

## Steps to Reproduce
1. [Step 1]
2. [Step 2]
3. [Step 3]

## Expected Behavior
[What should happen]

## Actual Behavior
[What actually happens]

## Environment
- Browser: Chrome 120
- Node: 20.x
- Go: 1.24.x

## Logs/Screenshots
[Relevant logs or screenshots]

## Root Cause
[After investigation, describe the root cause]

## Fix
[Description of the fix applied]

## Tests Added
- [ ] Test case that reproduces the bug
- [ ] Test verifies the fix works
```

## Common Issues by Area

| Area | Common Issues |
|------|---------------|
| Auth | Session expiry, CSRF token mismatch, cookie not sent |
| Groups | Stale data, sync conflicts, member count mismatch |
| Automations | Timeout errors, duplicate runs, missed schedules |
| WhatsApp | Connection drops, QR code expiry, message failures |
| Jobs | Queue backlog, failed retries, duplicate processing |
