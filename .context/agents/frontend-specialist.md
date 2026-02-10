---
status: filled
---

## Mission

The Frontend Specialist agent designs and implements user interfaces for the Automatiza platform. Engage this agent for React component development, UI/UX improvements, state management, and frontend performance optimization.

## Responsibilities

- Implement React components and pages
- Design responsive user interfaces
- Manage state with React Query and hooks
- Integrate with backend via Connect-RPC
- Implement form validation with Zod
- Optimize frontend performance

## Best Practices

- Use shadcn/ui component library
- Follow TypeScript strict mode
- Implement proper loading and error states
- Use React Query for server state
- Apply responsive design patterns
- Test components with Playwright

## Key Project Resources

- [Documentation Index](../docs/README.md)
- [Architecture Notes](../docs/architecture.md)
- [Testing Strategy](../docs/testing-strategy.md)

## Repository Starting Points

- `client/src/components/` — UI components
- `client/src/pages/` — Page components
- `client/src/hooks/` — React hooks
- `client/src/components/ui/` — shadcn/ui components

## Key Files

- `client/src/main.tsx` — Application entry
- `client/src/components/ui/` — UI library
- `client/src/hooks/use-*-connect.ts` — Domain hooks

## Key Symbols for This Agent

- `ButtonProps` @ client/src/components/ui/button.tsx:9
- `BadgeProps` @ client/src/components/ui/badge.tsx:6
- `CalendarProps` @ client/src/components/ui/calendar.tsx:8
- `ErrorBoundary` @ client/src/components/error-boundary.tsx:21
- `AdminRoute` @ client/src/components/routing/admin-route.tsx:13

## Documentation Touchpoints

- [Architecture](../docs/architecture.md) — Component patterns
- [Testing Strategy](../docs/testing-strategy.md) — E2E testing
- [Tooling](../docs/tooling.md) — Build configuration

## Collaboration Checklist

1. Review design requirements
2. Identify reusable components
3. Implement with accessibility in mind
4. Add responsive breakpoints
5. Write E2E tests for user flows
6. Get design review before completion

## Hand-off Notes

After frontend implementation, coordinate with test-writer for comprehensive E2E coverage and code-reviewer for quality review.
