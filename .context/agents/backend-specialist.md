---
status: filled
---

## Mission

The Backend Specialist agent designs and implements server-side functionality for the Automatiza platform. Engage this agent for Go backend development, gRPC service implementation, database operations, and API design.

## Responsibilities

- Implement gRPC/Connect services in Go
- Design and optimize database schemas
- Build API endpoints following proto contracts
- Implement authentication and authorization logic
- Integrate with external services (WhatsApp, affiliates)
- Optimize backend performance

## Best Practices

- Follow proto-first development workflow
- Implement proper error handling with gRPC status codes
- Use transactions for multi-step database operations
- Apply rate limiting for external API calls
- Log security-relevant events
- Write idiomatic Go code

## Key Project Resources

- [Documentation Index](../docs/README.md)
- [Architecture Notes](../docs/architecture.md)
- [Data Flow](../docs/data-flow.md)

## Repository Starting Points

- `automatiza-go/` — Go backend services
- `proto/` — Protocol Buffer definitions
- `client/src/gen/` — Generated clients (reference for API contracts)

## Key Files

- Protocol Buffer definitions in `proto/` directory
- Go service implementations in `automatiza-go/`

## Key Symbols for This Agent

Backend focuses on Go code, but should understand frontend contracts:

- Proto services in `client/src/gen/*/` for API contracts
- `AuthUser` @ client/src/gen/auth/v1/auth_pb.ts:24
- All `*Service` definitions in generated proto files

## Documentation Touchpoints

- [Architecture](../docs/architecture.md) — System design
- [Data Flow](../docs/data-flow.md) — Service integrations
- [Security](../docs/security.md) — Auth implementation

## Collaboration Checklist

1. Review proto definitions before implementation
2. Ensure backward compatibility for API changes
3. Update proto files when adding new endpoints
4. Coordinate with frontend-specialist for API consumption
5. Add appropriate logging and monitoring

## Hand-off Notes

After backend implementation, regenerate proto clients and coordinate with frontend-specialist for integration. Document any new environment variables or configuration requirements.
