---
type: skill
name: Refactoring
description: Safe code refactoring with step-by-step approach for Automatiza
skillSlug: refactoring
phases: [E]
status: filled
---

# Refactoring

## When to Use

- Improving code structure without changing behavior
- Reducing code duplication
- Improving readability and maintainability
- Applying project patterns consistently

## Refactoring Process

### 1. Verify Test Coverage
```bash
# Frontend
cd client && npm run test:coverage

# Backend
cd automatiza-go && go test -coverprofile=coverage.out ./...
```

### 2. Make Small, Incremental Changes
- One logical change per commit
- Run tests after each change
- Keep changes reversible

### 3. Verify Behavior Unchanged
- All existing tests pass
- Manual testing for critical paths
- No regressions

## Common Refactorings

### Extract Hook (React)

**Before:**
```typescript
function GroupsPage() {
  const [groups, setGroups] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchGroups().then(setGroups).finally(() => setLoading(false));
  }, []);

  // More component logic...
}
```

**After:**
```typescript
// hooks/use-groups.ts
export function useGroups() {
  const [groups, setGroups] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchGroups().then(setGroups).finally(() => setLoading(false));
  }, []);

  return { groups, loading };
}

// pages/groups.tsx
function GroupsPage() {
  const { groups, loading } = useGroups();
  // Component logic...
}
```

### Extract Component

**Before:**
```tsx
function GroupsPage() {
  return (
    <div>
      {groups.map(group => (
        <div className="card">
          <h3>{group.name}</h3>
          <p>{group.memberCount} members</p>
          <button onClick={() => syncGroup(group.id)}>Sync</button>
        </div>
      ))}
    </div>
  );
}
```

**After:**
```tsx
function GroupCard({ group, onSync }: GroupCardProps) {
  return (
    <div className="card">
      <h3>{group.name}</h3>
      <p>{group.memberCount} members</p>
      <button onClick={() => onSync(group.id)}>Sync</button>
    </div>
  );
}

function GroupsPage() {
  return (
    <div>
      {groups.map(group => (
        <GroupCard key={group.id} group={group} onSync={syncGroup} />
      ))}
    </div>
  );
}
```

### Extract Service (Go)

**Before:**
```go
func (h *Handler) CreateAutomation(ctx context.Context, req *pb.CreateRequest) (*pb.Response, error) {
  // Validation logic here
  // Business logic here
  // Database operations here
  return response, nil
}
```

**After:**
```go
// services/automation.go
type AutomationService struct {
  repo   AutomationRepository
  logger *zap.Logger
}

func (s *AutomationService) Create(ctx context.Context, input CreateInput) (*Automation, error) {
  // Validation and business logic
  return s.repo.Create(ctx, automation)
}

// handlers/automation.go
func (h *Handler) CreateAutomation(ctx context.Context, req *pb.CreateRequest) (*pb.Response, error) {
  result, err := h.svc.Create(ctx, toInput(req))
  if err != nil {
    return nil, mapError(err)
  }
  return toResponse(result), nil
}
```

### Simplify Conditionals

**Before:**
```typescript
if (user.role === 'admin') {
  return true;
} else if (user.role === 'moderator' && resource.ownerId === user.id) {
  return true;
} else if (user.role === 'user' && resource.ownerId === user.id && resource.isPublic) {
  return true;
} else {
  return false;
}
```

**After:**
```typescript
const isAdmin = user.role === 'admin';
const isOwner = resource.ownerId === user.id;
const canModerate = user.role === 'moderator' && isOwner;
const canAccess = user.role === 'user' && isOwner && resource.isPublic;

return isAdmin || canModerate || canAccess;
```

## Refactoring Checklist

- [ ] Tests exist for affected code
- [ ] All tests pass before starting
- [ ] Changes are incremental
- [ ] Tests pass after each change
- [ ] No behavior changes
- [ ] Code follows project patterns
- [ ] Documentation updated if needed

## Red Flags to Avoid

1. **Changing behavior**: Refactoring should NOT change functionality
2. **Large changes**: Break into smaller, testable steps
3. **No tests**: Add tests before refactoring
4. **Multiple concerns**: One refactoring at a time
5. **Premature optimization**: Focus on clarity first

## Project-Specific Patterns

### Hook Pattern
```typescript
// Always export from hooks/index.ts
export { useGroups } from './use-groups-connect';
```

### Repository Pattern
```go
// Always use interface for dependency
type GroupRepository interface {
  GetByID(ctx context.Context, id string) (*Group, error)
}
```

### Component Pattern
```tsx
// Always export named components
export function GroupCard({ group }: GroupCardProps) { ... }
```
