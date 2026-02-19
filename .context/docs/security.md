---
type: doc
name: security
description: Security policies, authentication, secrets management, and compliance requirements
category: security
generated: 2026-02-18
status: filled
scaffoldVersion: "2.0.0"
---

# Security & Compliance Notes

This document covers the authentication and authorization model, secrets management practices, and security measures implemented across the Automatiza platform. These policies apply to both the Go backend and the React frontend.

## Authentication & Authorization

### Authentication Mechanisms

**JWT via HttpOnly Cookies**

The primary authentication mechanism uses JSON Web Tokens stored in HttpOnly cookies:

- **Cookie attributes**: `HttpOnly`, `Secure`, `SameSite=Strict` -- prevents JavaScript access, enforces HTTPS, and mitigates CSRF via same-site policy.
- **Token lifecycle**: Access tokens are short-lived. Refresh tokens enable session renewal without re-authentication.
- **Implementation**: `internal/auth/` handles JWT generation (signing with HMAC-SHA256) and validation. The `internal/middleware/auth.go` middleware extracts and validates the JWT from the cookie on every authenticated request.
- **CSRF protection**: A separate CSRF token is generated per session and must be sent as a request header on all state-changing operations. The backend validates it alongside the JWT cookie.

**Magic Link Authentication**

An alternative passwordless flow is available:

- User requests a magic link via email.
- `internal/services/email/` sends a one-time link with a high-entropy token.
- Token is validated and exchanged for a JWT session on click.
- Tokens have a short TTL and are single-use (deleted after consumption).

**Password Hashing**

- Algorithm: **argon2id** (memory-hard, resistant to GPU/ASIC attacks).
- Implementation: Used for all password storage and verification in `internal/services/profile/password_service.go`.
- Password reset tokens stored in `password_reset_tokens` table with TTL and single-use enforcement.

### Authorization Model

**Role-Based Access Control (RBAC)**

- Two primary roles: `admin` and `user`.
- Roles stored in the `user_roles` table with a composite unique constraint `UNIQUE(user_id, role)`, allowing a user to hold multiple roles.
- The auth middleware extracts roles from the JWT claims and makes them available to handlers.
- ConnectRPC interceptors enforce role requirements per RPC method.
- Admin endpoints (user management, plan assignment, system configuration) are gated behind the `admin` role.

**Plan-Based Feature Gating**

- Users are assigned plans (`plans` table) that determine feature access limits (number of automations, groups, messages per day).
- Plan enforcement happens at the service layer -- services check the user's active plan before allowing resource creation or dispatch.
- Kiwify payment webhooks automatically upgrade/downgrade plans based on subscription events.

**Suspension**

- `internal/connectrpc/suspension_interceptor.go` checks whether a user account is suspended before processing any RPC call, returning `PermissionDenied` for suspended accounts.

## Secrets & Sensitive Data Management

### Production Secrets

- **Storage**: GitHub Secrets (`gh secret list --env production`) for all production credentials.
- **Deployment**: Secrets injected as environment variables during CI/CD deployment. Never committed to the repository.
- **Audit**: `gh secret list` shows when secrets were last updated. Regular rotation is expected for critical credentials.

**Critical secrets managed:**
- `DATABASE_URL` -- PostgreSQL connection string
- `REDIS_URL` -- Redis connection string
- `JWT_SECRET` -- HMAC signing key for JWT tokens
- `WAHA_API_KEY` -- Authentication key for WAHA API
- `KIWIFY_WEBHOOK_SECRET` -- HMAC secret for validating Kiwify webhooks
- `SENTRY_DSN` -- Sentry error reporting endpoint
- `DD_API_KEY` -- Datadog agent API key
- `SHOPEE_APP_ID` / `SHOPEE_SECRET` -- Shopee affiliate API credentials
- `SMTP_PASSWORD` -- Email service credentials

### Local Development

- `.env` files for local configuration (gitignored via `.gitignore`).
- `.env.debug` for production debugging credentials (Sentry token, VPS SSH, etc.).
- `client/.env.production` for frontend production build variables.
- None of these files are committed to the repository.

### Logging & PII

- **No secrets in logs**: Structured JSON logs use redaction to strip sensitive fields (tokens, passwords, API keys) before output.
- **PII redaction**: Phone numbers, email addresses, and user-identifiable data are redacted in logs and traces before export to Sentry/Datadog.
- **Error messages**: User-facing error messages are sanitized -- internal details (stack traces, SQL errors, file paths) are never exposed to the client. Technical details are logged server-side with `traceId` for correlation.

## Security Measures

### Content Security Policy (CSP)

- **Nonce-based CSP**: The server generates a cryptographic nonce per request and injects it into the HTML template.
- **Vite integration**: Production builds use a `__CSP_NONCE__` placeholder in the HTML that the server replaces at runtime with the request-specific nonce.
- **Policy**: `script-src 'nonce-...' 'strict-dynamic'; object-src 'none'; base-uri 'none'` -- prevents inline script injection while allowing legitimate scripts.

### Webhook Security

**WAHA Webhook HMAC Validation**

- Implementation: `internal/adapters/whatsapp/hmac_validator.go`
- The webhook receiver validates the HMAC-SHA256 signature of incoming WAHA payloads.
- Comparison uses `crypto/subtle.ConstantTimeCompare` to prevent timing attacks.
- Invalid signatures result in immediate rejection (HTTP 401) -- no payload processing occurs.
- Validated payloads are ACKed quickly and enqueued to Redis for async processing (no business logic in the receiver).

**Kiwify Webhook Validation**

- Kiwify webhooks are validated using their webhook secret via HMAC.
- Events are logged in `webhook_events` (deduplicated by `event_id` VARCHAR(128)) and `webhook_logs`.

**Rate Limiting**

- `internal/middleware/webhook_rate_limiter.go` enforces per-IP rate limits on webhook endpoints to prevent abuse.

### Input Validation

- **Protovalidate**: Input validation rules are defined in `.proto` files using CEL annotations and enforced on the backend via a validation interceptor (`internal/middleware/validator.go`).
- **Frontend validation**: Forms use `react-hook-form` with resolvers based on `@bufbuild/protovalidate`, ensuring frontend and backend validation rules are consistent (no duplicated Zod/Yup schemas).
- **Structured error responses**: `InvalidArgument` gRPC errors are mapped to per-field errors in react-hook-form, providing consistent user-facing validation messages.

### Transport Security

- **TLS minimum**: TLS 1.2 enforced; TLS 1.0 and 1.1 are rejected.
- **HTTPS only**: All cookies are `Secure` (HTTPS-only). Frontend is served over HTTPS in production.
- **CORS**: `internal/middleware/cors.go` restricts allowed origins to the configured frontend domain.

### Constant-Time Comparisons

- All security-sensitive comparisons (HMAC signatures, token validation) use `crypto/subtle.ConstantTimeCompare` to prevent timing side-channel attacks.
- This applies to WAHA webhook HMAC validation, Kiwify webhook validation, and any manual token comparison.

### Session Security

- **High-entropy identifiers**: Session tokens and magic link tokens use cryptographically secure random number generation (CSPRNG).
- **Token storage**: JWT in HttpOnly cookies (not LocalStorage) prevents XSS-based token theft.
- **Session invalidation**: Logout invalidates the session server-side and clears the cookie.

### Error Handling & Information Leakage

gRPC status codes are mapped to specific UI actions to prevent information leakage while maintaining good UX:

| gRPC Code | UI Action | Details Exposed |
|-----------|-----------|-----------------|
| `InvalidArgument` | Inline field errors | Per-field validation messages (safe) |
| `Unauthenticated` | Redirect to login / token refresh | None |
| `PermissionDenied` | Access denied message | None (no role/permission details) |
| `NotFound` | Not found page or message | None (no internal IDs/paths) |
| `Unavailable` | Reconnect with backoff | None |
| `Internal` | Generic error toast | None (details in server logs only) |

Internal error details (stack traces, SQL errors, file paths) are logged server-side with `traceId` for debugging but never included in client responses.

### Supply Chain Security

- **Static binaries**: Backend built with `CGO_ENABLED=0` and `-ldflags "-s -w"` for reproducible, minimal builds.
- **Distroless images**: Production Docker images use distroless base (no shell, no package manager).
- **Image signing**: Container images signed via Cosign (keyless) and verified on deployment.
- **Dependency scanning**: `go mod audit` and `npm audit` run in CI to catch known vulnerabilities.
- **No wildcards**: WAHA webhook subscriptions use specific event types -- `events=*` is prohibited in production.

### XSS Prevention

- React's default JSX escaping prevents most XSS vectors.
- `dangerouslySetInnerHTML` is avoided; where unavoidable, content is sanitized with DOMPurify (frontend) or bluemonday (backend).
- CSP nonce policy provides defense-in-depth against injected scripts.
