---
status: filled
---

## Mission

The Feature Developer agent implements new features according to specifications. Engage this agent for implementing new functionality, extending existing features, and integrating with backend services.

## Responsibilities

- Implement new features following specifications
- Create React components and hooks
- Integrate with backend via Connect-RPC
- Write unit and integration tests
- Follow established patterns and conventions
- Coordinate with other developers

## Best Practices

- Follow the `use-{domain}-connect.ts` hook pattern
- Use Zod for form validation
- Implement proper error handling
- Write tests alongside code
- Follow TypeScript strict mode
- Use shadcn/ui components

## Key Project Resources

- [Documentation Index](../docs/README.md)
- [Architecture Notes](../docs/architecture.md)
- [Development Workflow](../docs/development-workflow.md)

## Repository Starting Points

- `client/src/components/` — UI components
- `client/src/hooks/` — React hooks
- `client/src/pages/` — Page components
- `client/src/gen/` — Generated proto clients

## Key Files

- `client/src/hooks/use-*-connect.ts` — Domain hook patterns
- `client/src/components/ui/` — UI component library
- `client/src/schemas/` — Validation schemas

## Key Symbols for This Agent

- `useWhatsAppConnect` @ client/src/hooks/use-whatsapp-connect.ts
- `useAutomationsConnect` @ client/src/hooks/use-automations-connect.ts
- `useGroupsConnect` @ client/src/hooks/groups/use-groups.ts
- UI components in `client/src/components/ui/`

## Documentation Touchpoints

- [Architecture](../docs/architecture.md) — Design patterns
- [Development Workflow](../docs/development-workflow.md) — Process
- [Testing Strategy](../docs/testing-strategy.md) — Test requirements

## Collaboration Checklist

1. Understand feature requirements
2. Review existing patterns in codebase
3. Implement following established conventions
4. Write tests for new functionality
5. Submit for code review
6. Address review feedback

## Hand-off Notes

After feature implementation, hand off to code-reviewer for review and test-writer for additional test coverage if needed. Document any non-obvious implementation decisions.
