---
type: doc
name: testing-strategy
description: Test frameworks, patterns, coverage requirements, and quality gates
category: testing
generated: 2026-02-18
status: filled
scaffoldVersion: "2.0.0"
---

# Testing Strategy

This document describes how the Automatiza project maintains quality through automated testing. It covers test types, frameworks, execution commands, and the quality gates enforced in CI.

## Test Types

### Unit Tests (Frontend)

- **Framework:** Vitest
- **Location:** Test files live alongside source as `*.test.ts` / `*.test.tsx`, or inside `__tests__/` directories.
- **Patterns:**
  - Component tests render with `@testing-library/react` and assert on DOM output and user interactions.
  - Custom hooks are tested with `renderHook` from `@testing-library/react`.
  - Pure utility functions are tested with direct input/output assertions.
- **Mocking:** Vitest's built-in `vi.mock()` for module mocking. ConnectRPC transport is mocked at the transport layer to avoid network calls.

### Unit Tests (Backend)

- **Framework:** Go standard `testing` package
- **Location:** Test files use the `*_test.go` convention, colocated with the source file they test.
- **Patterns:**
  - Table-driven tests for functions with multiple input/output combinations.
  - Subtests via `t.Run()` for organized, named test cases.
  - `testify/assert` and `testify/require` for assertions.
  - Mocks and fakes for repository interfaces to isolate service logic.
- **Conventions:** Every exported function in the service layer should have corresponding test coverage for success, error, and edge cases.

### Integration Tests (Backend)

- **Framework:** Go `testing` + testcontainers-go
- **Infrastructure:** Tests spin up real PostgreSQL and Redis containers via testcontainers, ensuring queries run against actual database engines.
- **Patterns:**
  - Repository tests execute real SQL against a containerized PostgreSQL instance.
  - Tests use transaction rollback or per-test database setup for isolation.
  - Connection pooling and container reuse are configured for performance.
- **Location:** Integration tests live alongside unit tests in `*_test.go` files, typically distinguished by build tags or test helper setup.

### End-to-End Tests (E2E)

- **Framework:** Playwright
- **Location:** `client/e2e/*.spec.ts`
- **Patterns:**
  - Page Object Model -- page interactions are abstracted into classes under `client/e2e/pages/`.
  - Tests exercise full user flows (login, navigation, form submission, data verification).
  - Each test is independent and does not rely on state from other tests.
- **Environment:** E2E tests run against the full stack (frontend + backend + database).

## Running Tests

| Task | Command | Notes |
|------|---------|-------|
| All tests | `make test` | Runs frontend + backend tests |
| Frontend only | `make test-client` | Vitest |
| Backend only | `make test-go` | Go test with race detector |
| Coverage report | `make test-coverage` | Generates coverage output |
| Watch mode (frontend) | `cd client && npx vitest --watch` | Re-runs on file change |
| Single test file (frontend) | `cd client && npx vitest run src/path/to/file.test.ts` | Run a specific test |
| Single test (backend) | `cd automatiza-go && go test -run TestFunctionName ./internal/...` | Run a specific Go test |

## Quality Gates

The following checks are enforced in CI. All must pass before a PR can be merged:

### Mandatory Checks

1. **TypeScript typecheck** -- zero errors from `tsc --noEmit`.
2. **BiomeJS lint/format** -- no warnings or errors from the frontend linter.
3. **golangci-lint** -- strict configuration including `gosec`, `errcheck`, `bodyclose`, and complexity limits.
4. **Buf lint** -- all `.proto` files conform to Buf lint rules.
5. **Buf breaking** -- no breaking changes to protobuf contracts without explicit versioning (v2 package).
6. **All tests pass** -- `make test` exits successfully with zero failures.

### Zero-Tolerance Policies

- **No flaky tests:** A test that fails intermittently blocks merge until it is stabilized or removed. Flaky tests are never skipped or ignored.
- **No silent errors in Go:** Empty `if err != nil {}` blocks, `_ = err` assignments, or error handling with only `fmt.Println` / `log.Println` are treated as critical failures. Every error must include structured logging, Sentry reporting (when applicable), and trace context.
- **No `any` types in TypeScript:** All code must be fully typed. Use generics when needed.
- **No linter suppressions without justification:** `// NOSONAR`, `//nolint`, or `// biome-ignore` require a detailed technical comment explaining why the suppression is necessary.

### Pre-Merge Checklist

Before considering a task complete (Definition of Done):

- [ ] `make lint` passes (BiomeJS + golangci-lint)
- [ ] `make test` passes (all test suites)
- [ ] No TypeScript `any` types introduced
- [ ] No dead code or unused imports
- [ ] All user-facing strings use i18n
- [ ] New business logic has corresponding tests (success, error, edge cases)
- [ ] Go error paths include observability (structured log + Sentry + trace)

## Related Documents

- [Development Workflow](./development-workflow.md) -- branching strategy, local setup, and code review process
- [Tooling & Productivity Guide](./tooling.md) -- required tools and IDE configuration
