---
slug: error-handling
category: operations
generatedAt: 2026-02-18T22:38:13.920Z
relevantFiles:
  - client/src/components/error-boundary.tsx
  - client/src/components/admin/types/team.types.ts
  - client/src/components/admin/types/email-templates.types.ts
  - client/src/contexts/auth-context.tsx
  - client/src/lib/connect/error-handler.ts
---

# How are errors handled?

## Philosophy

Errors are never silently ignored. Every error path must include observability (structured logging, Sentry reporting, and trace tagging). The system maps gRPC error codes to user-friendly UI actions through a centralized handler.

## Backend Error Handling (Go)

### Error Wrapping

All errors are wrapped with context using `fmt.Errorf("context: %w", err)` or custom error types defined in `internal/errors/`. This preserves the error chain for debugging while adding meaningful context at each layer.

### ConnectRPC Error Codes

Services return typed ConnectRPC errors that map to standard gRPC status codes:

| Code | Usage | Example |
|------|-------|---------|
| `InvalidArgument` | Validation failures | Missing required field, invalid format |
| `NotFound` | Resource does not exist | User, automation, or group not found |
| `AlreadyExists` | Duplicate creation attempt | Email already registered |
| `Unauthenticated` | Missing or invalid token | Expired JWT, no cookie |
| `PermissionDenied` | Insufficient role/permissions | User accessing admin endpoint |
| `Internal` | Unexpected server error | Database connection failure |
| `Unavailable` | Temporary service issue | Redis down, WAHA unreachable |
| `ResourceExhausted` | Rate limit exceeded | Too many requests |

### Observability on Error

Every `if err != nil` block that results in an HTTP response must:

1. **Tag the active span** (if tracing is active) with error details.
2. **Log structured JSON** with `traceId`, `module`, `flow`, and error message.
3. **Report to Sentry** when the error is unexpected or critical (with tags for `module` and `flow`).

### Prohibited Patterns

- `_ = err` (ignoring errors)
- Empty `if err != nil {}` blocks
- `fmt.Println(err)` or `log.Println(err)` without structured logging/observability
- Leaking internal error details to the client

## Frontend Error Handling (React/TypeScript)

### Centralized Error Interceptor

The ConnectRPC error interceptor (`client/src/lib/connect/error-handler.ts`) maps gRPC codes to UI actions centrally. Individual components do not use scattered try/catch blocks.

### gRPC Code to UI Action Mapping

| gRPC Code | UI Action |
|-----------|-----------|
| `InvalidArgument` | Inline form errors (per-field via react-hook-form) |
| `NotFound` | Toast notification or redirect to 404 |
| `Unauthenticated` | Token refresh attempt, then redirect to login |
| `PermissionDenied` | Access denied message |
| `Unavailable` | Reconnect with backoff, "service unavailable" toast |
| `ResourceExhausted` | Rate limit toast with retry hint |
| `Internal` | Generic "something went wrong" toast |

### Error Boundary

The `ErrorBoundary` component (`client/src/components/error-boundary.tsx`) catches unhandled React rendering errors and displays a fallback UI instead of a blank screen.

### Sentry Integration (Frontend)

- Unhandled exceptions are automatically captured by the Sentry SDK.
- ConnectRPC errors include tags for the service name and method.
- User context (ID, email, role) is attached to Sentry events for debugging.

## Validation Errors

- Validation rules are defined in `.proto` files using protovalidate (CEL annotations).
- Backend enforces validation via a ConnectRPC interceptor.
- Frontend uses react-hook-form with a resolver based on `@bufbuild/protovalidate`, keeping validation rules in sync with the backend.
- `InvalidArgument` errors are structured with per-field details, which the frontend maps directly to form field errors.

## Error Logging Standards

- All logs are structured JSON with automatic PII redaction.
- Every log entry includes `traceId`/`correlationId` for cross-service traceability.
- Sensitive data (passwords, tokens, API keys) is never included in error messages or logs.
- Internal error details are logged server-side only; client-facing messages are user-friendly.
