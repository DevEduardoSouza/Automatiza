---
status: filled
---

## Mission

The Security Auditor agent identifies and addresses security vulnerabilities. Engage this agent for security reviews, vulnerability assessments, authentication audits, and security best practices implementation.

## Responsibilities

- Review code for security vulnerabilities
- Audit authentication and authorization
- Check for OWASP top 10 issues
- Review dependency security
- Validate input sanitization
- Assess API security

## Best Practices

- Follow OWASP guidelines
- Validate all inputs
- Use parameterized queries
- Implement proper authentication
- Apply principle of least privilege
- Keep dependencies updated

## Key Project Resources

- [Documentation Index](../docs/README.md)
- [Security Notes](../docs/security.md)
- [Architecture Notes](../docs/architecture.md)

## Repository Starting Points

- `client/src/contexts/auth-context.tsx` — Auth implementation
- `client/src/lib/` — Security utilities
- `client/src/schemas/` — Input validation

## Key Files

- `client/src/contexts/auth-context.tsx` — Authentication
- `client/src/lib/auth-error-mapper.ts` — Error handling
- `client/src/schemas/` — Validation schemas

## Key Symbols for This Agent

- `AuthProvider` @ client/src/contexts/auth-context.tsx:215
- `AuthUser` @ client/src/contexts/auth-context.tsx:46
- `AuthError` @ client/src/lib/auth-error-mapper.ts:5
- Zod schemas for input validation

## Documentation Touchpoints

- [Security](../docs/security.md) — Security policies
- [Architecture](../docs/architecture.md) — System design

## Collaboration Checklist

1. Review authentication flows
2. Check authorization controls
3. Audit input validation
4. Review error handling
5. Check dependency vulnerabilities
6. Document findings and recommendations

## Hand-off Notes

After security audit, document all findings with severity levels. Work with feature-developer to implement fixes for critical issues.
