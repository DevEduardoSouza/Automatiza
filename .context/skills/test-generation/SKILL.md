---
type: skill
name: Test Generation
description: Generate comprehensive test cases for the Automatiza platform
skillSlug: test-generation
phases: [E, V]
status: filled
---

# Test Generation

## When to Use

- Writing unit tests for React hooks or Go services
- Creating E2E tests with Playwright
- Adding integration tests with database
- Improving test coverage for existing code

## Project Testing Stack

| Layer | Framework | Location |
|-------|-----------|----------|
| Frontend Unit | Vitest | `client/src/**/*.test.ts` |
| Frontend E2E | Playwright | `client/e2e/` |
| Backend Unit | Go testing | `automatiza-go/**/*_test.go` |
| Backend E2E | Go + testcontainers | `automatiza-go/tests/e2e/` |

## Instructions

### 1. Frontend Unit Tests (Vitest)

```typescript
import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useGroups } from '@/hooks/use-groups';

describe('useGroups', () => {
  it('should fetch groups successfully', async () => {
    const { result } = renderHook(() => useGroups());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.groups).toHaveLength(2);
  });
});
```

### 2. Frontend E2E Tests (Playwright)

```typescript
import { test, expect } from '@playwright/test';
import { GroupsPage } from './pages/groups.page';
import { setupGroupsMocks } from './fixtures/groups.fixture';

test.describe('Groups Page', () => {
  let groupsPage: GroupsPage;

  test.beforeEach(async ({ page }) => {
    groupsPage = new GroupsPage(page);
    await setupGroupsMocks(page);
  });

  test('should display groups list', async ({ page }) => {
    await groupsPage.goto();
    await groupsPage.waitForLoad();

    await expect(groupsPage.groupsTable).toBeVisible();
    expect(await groupsPage.getGroupCount()).toBeGreaterThan(0);
  });
});
```

### 3. Page Object Pattern

```typescript
// client/e2e/pages/example.page.ts
export class ExamplePage {
  constructor(private page: Page) {}

  // Locators
  table = this.page.locator('[data-testid="example-table"]');
  createButton = this.page.locator('button:has-text("Create")');

  // Actions
  async goto() {
    await this.page.goto('/example');
  }

  async waitForLoad() {
    await this.table.waitFor({ state: 'visible' });
  }

  async create(data: ExampleData) {
    await this.createButton.click();
    // Fill form...
  }
}
```

### 4. Fixture Pattern

```typescript
// client/e2e/fixtures/example.fixture.ts
export const MOCK_EXAMPLES = [
  { id: '1', name: 'Example 1' },
  { id: '2', name: 'Example 2' },
];

export async function mockExampleListApi(page: Page) {
  await page.route('**/example.v1.ExampleService/List', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify({ examples: MOCK_EXAMPLES })),
    });
  });
}

export async function setupExampleMocks(page: Page) {
  await mockExampleListApi(page);
}
```

### 5. Backend Unit Tests (Go)

```go
func TestService_Create(t *testing.T) {
  tests := []struct {
    name    string
    input   CreateInput
    wantErr bool
  }{
    {
      name:    "valid input",
      input:   CreateInput{Name: "Test"},
      wantErr: false,
    },
    {
      name:    "empty name",
      input:   CreateInput{Name: ""},
      wantErr: true,
    },
  }

  for _, tc := range tests {
    t.Run(tc.name, func(t *testing.T) {
      svc := NewService(mockRepo)
      _, err := svc.Create(context.Background(), tc.input)

      if tc.wantErr {
        assert.Error(t, err)
      } else {
        assert.NoError(t, err)
      }
    })
  }
}
```

### 6. Backend E2E Tests (with DB)

```go
func TestAPI_CreateExample(t *testing.T) {
  env := newTestEnvWithDB(t)
  defer env.cleanup(t)

  client := newHTTPClient(t, env.router)
  client.setAuthToken("user-123")

  payload := map[string]interface{}{
    "name": "Test Example",
  }

  w := client.post("/connect/example.v1.ExampleService/Create", payload)

  assert.Equal(t, http.StatusOK, w.Code)

  // Verify in database
  var result Example
  env.pool.QueryRow(ctx, "SELECT * FROM examples WHERE name = $1", "Test Example").Scan(&result)
  assert.Equal(t, "Test Example", result.Name)
}
```

## Best Practices

1. **AAA Pattern**: Arrange, Act, Assert
2. **Descriptive Names**: `should_return_error_when_input_invalid`
3. **Mock External Dependencies**: Use fixtures for API calls
4. **Isolated Tests**: Each test should be independent
5. **Fast Execution**: Mock slow operations
6. **Edge Cases**: Test error paths and boundary conditions

## Running Tests

```bash
# Frontend
cd client
npm run test           # Watch mode
npm run test:e2e       # E2E tests

# Backend
cd automatiza-go
go test ./...          # All tests
go test -v ./...       # Verbose
```
