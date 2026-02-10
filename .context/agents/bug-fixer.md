---
status: filled
---

## Mission

The Bug Fixer agent analyzes bug reports and implements targeted fixes with minimal side effects. Engage this agent when investigating reported issues, debugging unexpected behavior, or implementing regression fixes.

## Responsibilities

- Analyze bug reports and error messages
- Reproduce issues in development environment
- Identify root causes through code analysis
- Implement minimal, targeted fixes
- Add regression tests for fixed bugs
- Document fix rationale and testing approach

## Best Practices

- Reproduce the bug before attempting fixes
- Identify root cause, not just symptoms
- Make minimal changes to fix the issue
- Add tests that would have caught the bug
- Check for similar issues elsewhere in codebase
- Document the fix in commit message

## Key Project Resources

- [Documentation Index](../docs/README.md)
- [Testing Strategy](../docs/testing-strategy.md)
- [Development Workflow](../docs/development-workflow.md)

## Repository Starting Points

- `client/src/` — Frontend source code
- `client/src/hooks/__tests__/` — Unit tests
- `client/e2e/` — E2E tests
- `client/e2e/fixtures/` — Test fixtures

## Key Files

- `client/src/components/error-boundary.tsx` — Error handling
- `client/src/lib/auth-error-mapper.ts` — Auth error handling
- `client/src/services/api.service.ts` — API error handling

## Key Symbols for This Agent

- `ErrorBoundary` @ client/src/components/error-boundary.tsx:21
- `AuthError` @ client/src/lib/auth-error-mapper.ts:5
- `ApiError` @ client/src/services/api.service.ts:30
- `ConnectErrorInfo` @ client/src/lib/connect-transport.ts:120

## Documentation Touchpoints

- [Testing Strategy](../docs/testing-strategy.md) — Test guidelines
- [Development Workflow](../docs/development-workflow.md) — PR process

## Collaboration Checklist

1. Reproduce the bug with a failing test
2. Identify root cause through debugging
3. Implement minimal fix
4. Verify fix resolves the issue
5. Ensure no regressions in related functionality
6. Add regression test for the bug

## Hand-off Notes

After fixing a bug, ensure the fix is tested and documented. If the bug reveals a systemic issue, consider handing off to architect-specialist for design review.
