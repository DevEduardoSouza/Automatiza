---
type: skill
name: Code Review
description: Review code quality, patterns, and best practices for Automatiza
skillSlug: code-review
phases: [R, V]
status: filled
---

# Code Review

## When to Use

- Reviewing pull requests
- Checking code quality before merge
- Validating adherence to project patterns
- Identifying potential issues

## Review Checklist

### 1. Architecture & Patterns

**Frontend (React + TypeScript)**
- [ ] Follows hook pattern: `use-{domain}-connect.ts`
- [ ] Uses TanStack Query for server state
- [ ] Uses Zod schemas for validation
- [ ] Follows shadcn/ui component patterns
- [ ] Proper error handling with `getErrorInfo()`

**Backend (Go)**
- [ ] Follows repository pattern
- [ ] Service layer contains business logic
- [ ] Handlers are thin (delegate to services)
- [ ] Uses structured logging (zap)
- [ ] Proper error wrapping

### 2. Type Safety

**TypeScript**
```typescript
// Good - explicit types
function processGroup(group: Group): ProcessedGroup { ... }

// Bad - any types
function processGroup(group: any): any { ... }
```

**Go**
```go
// Good - interface for dependency
type GroupRepository interface {
  GetByID(ctx context.Context, id string) (*Group, error)
}

// Bad - concrete dependency
func NewService(repo *PostgresGroupRepo) *Service { ... }
```

### 3. Error Handling

**Frontend**
```typescript
// Good - proper error handling
const mutation = useMutation({
  mutationFn: createGroup,
  onError: (error) => {
    const info = getErrorInfo(error);
    toast({ variant: 'destructive', description: info.userMessage });
  },
});
```

**Backend**
```go
// Good - wrapped errors with context
if err != nil {
  return nil, fmt.Errorf("get group by id %s: %w", id, err)
}
```

### 4. Security

- [ ] No sensitive data in logs
- [ ] Input validation on all endpoints
- [ ] CSRF token used for mutations
- [ ] Rate limiting on sensitive operations
- [ ] SQL injection prevention (parameterized queries)

### 5. Performance

**Frontend**
- [ ] Appropriate use of `useMemo`/`useCallback`
- [ ] No unnecessary re-renders
- [ ] Proper query staleTime/refetchInterval

**Backend**
- [ ] Efficient database queries (indexes used)
- [ ] No N+1 queries
- [ ] Appropriate caching

### 6. Testing

- [ ] New code has tests
- [ ] Tests follow AAA pattern
- [ ] Edge cases covered
- [ ] Mocks used appropriately

### 7. Code Style

**General**
- [ ] Meaningful variable/function names
- [ ] No commented-out code
- [ ] No console.log/print statements
- [ ] Consistent formatting (Biome/gofmt)

**React Components**
```typescript
// Good - clear structure
export function GroupCard({ group, onEdit }: GroupCardProps) {
  const handleClick = useCallback(() => {
    onEdit(group.id);
  }, [group.id, onEdit]);

  return (
    <Card onClick={handleClick}>
      <CardHeader>{group.name}</CardHeader>
    </Card>
  );
}
```

### 8. Proto/API Changes

- [ ] Backward compatible changes
- [ ] Proper field numbering (no reuse)
- [ ] Clear field documentation
- [ ] Generated code committed

## Review Template

```markdown
## Summary
Brief description of changes reviewed.

## Checklist
- [x] Architecture patterns followed
- [x] Type safety maintained
- [x] Error handling appropriate
- [x] Security considerations addressed
- [ ] Performance implications reviewed
- [x] Tests included
- [x] Code style consistent

## Findings

### Must Fix
- Issue description and location

### Suggestions
- Optional improvements

### Questions
- Clarifications needed

## Verdict
- [ ] Approved
- [ ] Request changes
- [ ] Needs discussion
```

## Common Issues to Watch

1. **Missing error handling** in async operations
2. **Hardcoded values** that should be constants
3. **Missing null checks** for optional data
4. **Over-fetching** data not used by component
5. **Missing loading states** for async operations
6. **Stale closures** in useCallback/useEffect
7. **Missing query invalidation** after mutations
