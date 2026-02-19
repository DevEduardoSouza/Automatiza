---
slug: testing
category: testing
generatedAt: 2026-02-18T22:38:15.697Z
relevantFiles:
  - automatiza-go/tests/E2E_TEST_SPEC.md
  - automatiza-go/tests/e2e
  - automatiza-go/tests/integration
  - automatiza-go/tests/load
  - automatiza-go/scripts/loadtest/README.md
  - automatiza-go/scripts/loadtest/profile_loadtest.go
  - automatiza-go/tests/e2e/admin_test.go
  - automatiza-go/tests/e2e/auth_test.go
  - automatiza-go/tests/e2e/automations_test.go
  - automatiza-go/tests/e2e/redirect_test.go
---

# How do I run and write tests?

## Running Tests

### Quick Commands

| Command | Scope |
|---------|-------|
| `make test` | Run all tests (frontend + backend) |
| `make test-client` | Frontend unit tests (Vitest) |
| `make test-go` | Backend unit + integration tests (Go) |
| `make test-coverage` | All tests with coverage reports |

### Frontend Tests

```bash
# Unit tests with Vitest
make test-client

# Or directly
cd client && npx vitest run

# Watch mode (development)
cd client && npx vitest
```

### Backend Tests

```bash
# All Go tests
make test-go

# Or directly
cd automatiza-go && go test ./...

# Specific package
cd automatiza-go && go test ./internal/services/automation/...

# With verbose output
cd automatiza-go && go test -v ./internal/services/automation/...
```

### E2E Tests

```bash
# Frontend E2E with Playwright
cd client && npx playwright test

# Backend E2E tests
cd automatiza-go && go test ./tests/e2e/...
```

### Load Tests

```bash
# Load testing scripts
cd automatiza-go && go run ./scripts/loadtest/profile_loadtest.go
```

See `automatiza-go/scripts/loadtest/README.md` for configuration details.

## Test Organization

### Frontend (`client/`)

| Type | Location | Framework |
|------|----------|-----------|
| Unit tests | Co-located with source (`*.test.ts`, `*.test.tsx`) | Vitest |
| E2E tests | `client/e2e/` | Playwright |
| Test utilities | `client/src/test/` | Custom helpers |
| Page objects | `client/e2e/pages/` | Playwright page objects |
| Fixtures | `client/e2e/fixtures/` | Playwright fixtures |

### Backend (`automatiza-go/`)

| Type | Location | Framework |
|------|----------|-----------|
| Unit tests | Co-located with source (`*_test.go`) | Go testing |
| Integration tests | `automatiza-go/tests/integration/` | testcontainers-go |
| E2E tests | `automatiza-go/tests/e2e/` | Go testing + HTTP client |
| Load tests | `automatiza-go/scripts/loadtest/` | Custom Go scripts |
| Test spec | `automatiza-go/tests/E2E_TEST_SPEC.md` | Documentation |

## Writing Tests

### Frontend Unit Tests (Vitest)

```typescript
// Example: client/src/entities/automation/model/automation.test.ts
import { describe, expect, it } from "vitest";

describe("AutomationEntity", () => {
  it("should validate automation config", () => {
    // Arrange
    const config = { /* ... */ };

    // Act
    const result = validateConfig(config);

    // Assert
    expect(result.isValid).toBe(true);
  });
});
```

- Co-locate test files with the source they test.
- Use `describe`/`it` blocks with clear descriptions.
- Test success paths, error paths, and edge cases.

### Backend Unit Tests (Go)

```go
// Example: internal/services/automation/automation_service_test.go
func TestAutomationService_Create(t *testing.T) {
    // Arrange
    repo := newMockRepo()
    svc := NewAutomationService(repo)

    // Act
    result, err := svc.Create(ctx, input)

    // Assert
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

- Co-locate `_test.go` files with their source package.
- Use `testify` for assertions (`require` for fatal, `assert` for non-fatal).
- Use table-driven tests for multiple input/output combinations.

### Integration Tests (testcontainers)

Backend integration tests use **testcontainers-go** to spin up real PostgreSQL and Redis containers:

```go
func TestIntegration_AutomationRepo(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    // testcontainers spins up a real Postgres instance
    db := setupTestDatabase(t)
    repo := postgres.NewAutomationRepository(db)
    // ... test with real database
}
```

- Run with `-short` flag to skip integration tests during rapid development.
- Each test gets an isolated database state.

### Frontend E2E Tests (Playwright)

```typescript
// Example: client/e2e/auth.spec.ts
import { test, expect } from "./fixtures/auth.fixture";

test("user can log in", async ({ authPage }) => {
  await authPage.login("user@example.com", "password");
  await expect(authPage.dashboard).toBeVisible();
});
```

- Page objects in `client/e2e/pages/` encapsulate page interactions.
- Fixtures in `client/e2e/fixtures/` provide reusable test setup (auth, navigation).

## Quality Gates

Every change must pass all quality gates before merge:

1. **TypeScript typecheck** (`tsc --noEmit`)
2. **BiomeJS lint** (frontend)
3. **golangci-lint** (backend, includes gosec, errcheck, bodyclose, complexity limits)
4. **Vitest** (frontend unit tests)
5. **Go tests** (backend unit + integration)
6. **Buf lint + breaking check** (protobuf contract validation)

These gates run in CI on every push. Flaky tests block merge until stabilized.

## Test Coverage

```bash
# Generate coverage reports for all tests
make test-coverage
```

- Coverage reports are generated for both frontend (Vitest) and backend (Go).
- The goal is meaningful coverage of business logic, not arbitrary percentage targets.
- Every new or modified business rule requires corresponding test updates covering success, error, and edge cases.
