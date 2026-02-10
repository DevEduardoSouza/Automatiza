---
status: filled
slug: error-handling
category: operations
---

# How are errors handled?

## Error Handling Architecture

The platform uses a multi-layer error handling approach:

1. **Frontend:** React Error Boundary + toast notifications
2. **Transport:** ConnectRPC interceptors with error mapping
3. **Backend:** Structured Go errors with gRPC codes

## Frontend Error Handling

### Error Boundary
```typescript
// src/components/error-boundary.tsx
<ErrorBoundary
  FallbackComponent={ErrorFallback}
  onError={(error) => logErrorToService(error)}
>
  <App />
</ErrorBoundary>
```

### Connect Error Mapping
```typescript
// src/lib/connect-transport.ts
interface ConnectErrorInfo {
  code: Code;           // gRPC code
  message: string;      // Raw message
  userMessage: string;  // User-friendly (pt-BR)
  action: 'inline' | 'toast' | 'redirect' | 'retry' | 'none';
  redirectTo?: string;
  retryable: boolean;
}
```

### Error Actions by Code

| gRPC Code | Action | Description |
|-----------|--------|-------------|
| `InvalidArgument` | inline | Show in form field |
| `Unauthenticated` | redirect | Go to /auth |
| `PermissionDenied` | toast | Show permission error |
| `NotFound` | toast | Resource not found |
| `ResourceExhausted` | retry + toast | Rate limited |
| `Unavailable` | retry | Service temporarily down |
| `Internal` | toast | Server error |

### Hook Error Pattern
```typescript
// Typical mutation error handling
const mutation = useMutation({
  mutationFn: async (data) => { ... },
  onError: (error: unknown) => {
    const errorInfo = getErrorInfo(error);
    toast({
      variant: 'destructive',
      title: t('common.error'),
      description: errorInfo.userMessage,
    });
  },
});
```

## Auth Error Mapping

```typescript
// src/lib/auth-error-mapper.ts
const errorMap = {
  invalid_credentials: 'common.errors.auth.invalidCredentials',
  email_not_confirmed: 'common.errors.auth.emailNotConfirmed',
  email_exists: 'common.errors.auth.emailExists',
  session_expired: 'common.errors.auth.sessionExpired',
  rate_limit_exceeded: 'common.errors.auth.rateLimitExceeded',
  weak_password: 'common.errors.auth.weakPassword',
};
```

## Backend Error Handling

### Error Types (Go)
```go
// internal/errors/errors.go
var (
  ErrNotFound          = errors.New("resource not found")
  ErrInvalidInput      = errors.New("invalid input")
  ErrUnauthorized      = errors.New("unauthorized")
  ErrForbidden         = errors.New("forbidden")
  ErrRateLimited       = errors.New("rate limit exceeded")
  ErrInternal          = errors.New("internal error")
)
```

### Auth-Specific Errors
```go
// internal/auth/errors.go
ErrInvalidCredentials  = errors.New("invalid email or password")
ErrAccountLocked       = errors.New("account locked")
ErrEmailNotVerified    = errors.New("email not verified")
ErrSessionExpired      = errors.New("session expired")
ErrInvalidCSRF         = errors.New("invalid csrf token")
```

### Error to gRPC Code Mapping
```go
func mapErrorToCode(err error) connect.Code {
  switch {
  case errors.Is(err, ErrNotFound):
    return connect.CodeNotFound
  case errors.Is(err, ErrInvalidInput):
    return connect.CodeInvalidArgument
  case errors.Is(err, ErrUnauthorized):
    return connect.CodeUnauthenticated
  case errors.Is(err, ErrForbidden):
    return connect.CodePermissionDenied
  case errors.Is(err, ErrRateLimited):
    return connect.CodeResourceExhausted
  default:
    return connect.CodeInternal
  }
}
```

## Retry Strategy

### Retryable Errors
- `Unavailable` - Service temporarily down
- `ResourceExhausted` - Rate limited
- `Aborted` - Transaction conflict
- `DeadlineExceeded` - Timeout

### Retry Configuration
```typescript
const retryConfig = {
  maxRetries: 3,
  baseDelayMs: 100,
  maxDelayMs: 5000,
  // Exponential backoff with jitter
  getDelay: (attempt) =>
    Math.min(baseDelay * 2 ** attempt + randomJitter(), maxDelay)
};
```

### Idempotent Methods Only
Retry is only applied to safe operations:
- `List*`, `Get*`, `Check*`, `Fetch*`, `Search*`, `Find*`, `Count*`

## Distributed Tracing

Errors include trace context for debugging:

```typescript
// Request headers
{
  'traceparent': '00-trace-id-span-id-01',
  'x-trace-id': 'unique-request-id'
}
```

Server logs include the trace ID for correlation.

## User-Facing Messages (pt-BR)

| Error | Message |
|-------|---------|
| Invalid credentials | "Email ou senha inválidos" |
| Session expired | "Sua sessão expirou. Faça login novamente." |
| Rate limited | "Muitas tentativas. Aguarde alguns minutos." |
| Permission denied | "Você não tem permissão para esta ação." |
| Service unavailable | "Serviço temporariamente indisponível." |
