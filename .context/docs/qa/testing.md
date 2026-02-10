---
status: filled
slug: testing
category: testing
---

# How do I run and write tests?

## Testing Stack

| Layer | Framework | Location |
|-------|-----------|----------|
| Frontend Unit | Vitest | `client/src/**/*.test.ts` |
| Frontend E2E | Playwright | `client/e2e/` |
| Backend Unit | Go testing | `automatiza-go/**/*_test.go` |
| Backend E2E | Go testing + testcontainers | `automatiza-go/tests/e2e/` |
| Load Testing | Go + k6 | `automatiza-go/tests/load/` |

## Frontend Testing

### Unit Tests (Vitest)

```bash
cd client

# Run in watch mode
npm run test

# Single run
npm run test:run

# With coverage
npm run test:coverage
```

**Configuration:** `client/vitest.config.ts`

```typescript
export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/test/setup.ts',
  },
});
```

### E2E Tests (Playwright)

```bash
cd client

# Run headless
npm run test:e2e

# Run with browser visible
npm run test:e2e:headed

# Interactive UI mode
npm run test:e2e:ui

# View last report
npm run test:e2e:report
```

**Configuration:** `client/playwright.config.ts`

```typescript
export default defineConfig({
  testDir: './e2e',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
  },
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:5173',
  },
});
```

## Backend Testing

### Unit & Integration Tests

```bash
cd automatiza-go

# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package
go test ./internal/services/...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### E2E Tests (with Database)

```bash
# Requires Docker for PostgreSQL container
go test -v ./tests/e2e/...

# Skip long-running tests
go test -short ./...
```

## Test Organization

### Frontend E2E Structure
```
client/e2e/
├── pages/               # Page Object Models
│   ├── auth.page.ts
│   ├── dashboard.page.ts
│   ├── groups.page.ts
│   └── ...
├── fixtures/            # Test Data & Mocks
│   ├── auth.fixture.ts
│   ├── dashboard.fixture.ts
│   └── ...
└── specs/               # Test Files
    ├── auth.spec.ts
    ├── dashboard.spec.ts
    └── ...
```

### Backend E2E Structure
```
automatiza-go/tests/e2e/
├── setup_test.go        # Test environment setup
├── auth_test.go         # Auth flow tests
├── automations_test.go  # Automation tests
└── ...
```

## Writing Tests

### Frontend E2E Example
```typescript
import { test, expect } from '@playwright/test';
import { AuthPage } from './pages/auth.page';
import { loginUser, TEST_USER } from './fixtures/auth.fixture';

test.describe('Authentication', () => {
  test('should login successfully', async ({ page }) => {
    const authPage = new AuthPage(page);
    await authPage.goto();

    // Mock API
    await page.route('**/auth.v1.AuthService/Login', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify({ user: TEST_USER })),
      });
    });

    // Perform login
    await authPage.login(TEST_USER.email, TEST_USER.password);

    // Assert redirect
    await expect(page).toHaveURL('/dashboard');
  });
});
```

### Backend Test Example
```go
func TestAutomations_Create(t *testing.T) {
  env := newTestEnv(t)
  defer env.cleanup(t)

  client := newHTTPClient(t, env.router)
  client.setAuthToken("user-123")

  payload := map[string]interface{}{
    "name": "Test Automation",
    "mode": 1,
  }

  w := client.post("/connect/automations.v1.AutomationsService/Create", payload)

  assert.Equal(t, http.StatusOK, w.Code)
}
```

## Mocking Patterns

### Frontend (Playwright Route Interception)
```typescript
await page.route('**/groups.v1.GroupsService/List', async (route) => {
  await route.fulfill({
    status: 200,
    contentType: 'application/grpc-web+proto',
    headers: { 'grpc-status': '0' },
    body: Buffer.from(JSON.stringify({ groups: MOCK_GROUPS })),
  });
});
```

### Backend (Repository Mocks)
```go
type mockGroupRepo struct {
  groups map[string]*models.Group
}

func (m *mockGroupRepo) ListGroups(ctx context.Context, instanceID string) ([]*models.Group, error) {
  var result []*models.Group
  for _, g := range m.groups {
    if g.InstanceID == instanceID {
      result = append(result, g)
    }
  }
  return result, nil
}
```

## Test Fixtures

### Reusable Test Data
```typescript
// fixtures/groups.fixture.ts
export const MOCK_GROUPS = [
  { id: 'grp-1', name: 'Marketing', memberCount: 150 },
  { id: 'grp-2', name: 'Sales', memberCount: 75 },
];

export async function setupGroupsMocks(page: Page) {
  await mockGroupsListApi(page, MOCK_GROUPS);
  await mockWhatsAppStatusApi(page, { connected: true });
}
```

## CI/CD Integration

Tests run automatically on:
- Pull request creation
- Push to main branch

```yaml
# Example CI config
test:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - name: Frontend Tests
      run: |
        cd client
        npm ci
        npm run test:run
        npm run test:e2e
    - name: Backend Tests
      run: |
        cd automatiza-go
        go test ./...
```
