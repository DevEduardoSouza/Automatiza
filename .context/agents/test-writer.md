---
status: filled
---

## Mission

The Test Writer agent creates comprehensive tests and maintains test coverage. Engage this agent for writing unit tests, integration tests, E2E tests, and improving test infrastructure.

## Responsibilities

- Write unit tests for hooks and utilities
- Create E2E tests with Playwright
- Develop test fixtures and mocks
- Improve test coverage
- Maintain test infrastructure
- Document testing patterns

## Best Practices

- Follow AAA pattern (Arrange, Act, Assert)
- Use descriptive test names
- Mock external dependencies
- Test edge cases and error paths
- Keep tests fast and isolated
- Use page objects for E2E tests

## Key Project Resources

- [Documentation Index](../docs/README.md)
- [Testing Strategy](../docs/testing-strategy.md)
- [Development Workflow](../docs/development-workflow.md)

## Repository Starting Points

- `client/src/hooks/__tests__/` — Unit tests
- `client/e2e/` — E2E tests
- `client/e2e/fixtures/` — Test fixtures
- `client/e2e/pages/` — Page objects

## Key Files

- `client/e2e/fixtures/*.fixture.ts` — Mock helpers
- `client/e2e/pages/*.page.ts` — Page objects
- `client/src/hooks/__tests__/*.test.ts` — Unit tests

## Key Symbols for This Agent

- `setupGroupsMocks` @ client/e2e/fixtures/groups.fixture.ts:213
- `setupWhatsAppConnectedMocks` @ client/e2e/fixtures/whatsapp.fixture.ts:225
- `GroupsPage` @ client/e2e/pages/groups.page.ts:7
- `AutomationsPage` @ client/e2e/pages/automations.page.ts:7

## Documentation Touchpoints

- [Testing Strategy](../docs/testing-strategy.md) — Test guidelines
- [Development Workflow](../docs/development-workflow.md) — CI/CD

## Collaboration Checklist

1. Identify untested code paths
2. Write tests following project patterns
3. Use appropriate fixtures and mocks
4. Verify tests are reliable
5. Check coverage improvements
6. Update test documentation

## Hand-off Notes

After writing tests, ensure they pass consistently in CI. Document any new testing patterns or fixtures for the team.
