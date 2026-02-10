#---
status: filled
---

# Testing Strategy

Quality is maintained through a layered testing approach combining unit tests, integration tests, and end-to-end tests.

## Test Types

### Unit Tests
- **Framework**: Vitest (Jest-compatible)
- **Location**: `client/src/**/__tests__/*.test.ts`
- **Focus**: React hooks, utility functions, data transformations
- **Key files tested**:
  - `client/src/hooks/groups/__tests__/use-groups-*.test.ts`
  - Hook logic and state management

### Integration Tests
- **Focus**: Component integration with hooks
- **Mocking**: API calls mocked at Connect-RPC level

### E2E Tests
- **Framework**: Playwright
- **Location**: `client/e2e/*.spec.ts`
- **Page Objects**: `client/e2e/pages/*.page.ts`
- **Fixtures**: `client/e2e/fixtures/*.fixture.ts`

**E2E Test Coverage**:
- `automations.spec.ts` - Automation workflows
- `groups.spec.ts` - Group management
- `whatsapp.spec.ts` - WhatsApp instance management
- `jobs.spec.ts` - Scheduled jobs
- `history.spec.ts` - Action history
- `ml-link-lists.spec.ts` - ML link management
- `affiliate-programs.spec.ts` - Affiliate integrations

## Running Tests

### Unit Tests

```bash
# Run all unit tests
cd client
npm run test

# Watch mode for development
npm run test -- --watch

# Run with coverage
npm run test -- --coverage

# Run specific test file
npm run test -- use-groups.test.ts
```

### E2E Tests

```bash
# Run all E2E tests
cd client
npm run e2e

# Run specific E2E test
npx playwright test automations.spec.ts

# Run with UI mode
npx playwright test --ui

# Run headed (visible browser)
npx playwright test --headed
```

## Quality Gates

### Coverage Requirements
- Minimum 70% coverage for new code
- Critical hooks (groups, automations) should have 80%+ coverage

### Pre-merge Checks
- All unit tests passing
- All E2E tests passing
- No TypeScript errors (`npm run type-check`)
- No linting errors (`npm run lint`)
- Code review approved

### CI Pipeline
- Tests run on every pull request
- E2E tests run against staging environment
- SonarQube analysis for code quality (see `docker-compose.sonarqube.yml`)

## Troubleshooting

### Flaky E2E Tests
- Tests involving WhatsApp status may be timing-sensitive
- Use `setupWhatsAppConnectedMocks()` / `setupWhatsAppDisconnectedMocks()` fixtures
- Add appropriate waits for async operations

### Mock Setup
Each feature has dedicated fixtures:
- `setupGroupsMocks()` - Groups page mocks
- `setupJobsMocks()` - Jobs page mocks
- `setupHistoryMocks()` - History page mocks
- `setupMlLinkListsMocks()` - ML Links mocks

### Common Issues
- **Timeout errors**: Increase timeout or check for missing mocks
- **Selector not found**: Page structure may have changed, update page objects
- **API mock not matched**: Verify fixture URL patterns match actual requests

See [Development Workflow](./development-workflow.md) for related processes.
