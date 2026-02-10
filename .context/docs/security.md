#---
status: filled
---

# Security & Compliance Notes

This document outlines the security policies, authentication mechanisms, and data handling practices for the Automatiza platform.

## Authentication & Authorization

### Authentication Flow

The platform uses token-based authentication managed through the `AuthProvider` context:

- **Login Methods**: Email/password, magic link
- **Token Storage**: Secure browser storage
- **Session Management**: Token refresh and validation via backend
- **Password Reset**: Email-based reset flow

Key components:
- [`AuthProvider`](../client/src/contexts/auth-context.tsx:215) - Authentication context provider
- [`AuthUser`](../client/src/contexts/auth-context.tsx:46) - Authenticated user type
- [`AuthError`](../client/src/lib/auth-error-mapper.ts:5) - Error handling interface

### Authorization Model

- **Role-based Access**: Admin vs. regular user routes
- **Route Protection**: [`AdminRoute`](../client/src/components/routing/admin-route.tsx:13) for admin-only pages
- **API Authorization**: Tokens passed via Connect-RPC transport

### Session Security

- Sessions validated on each API request
- SSE connection for real-time session status monitoring
- Automatic logout on session invalidation

## Secrets & Sensitive Data

### Credential Storage

- **Affiliate Credentials**: OAuth tokens for Shopee/ML stored encrypted in backend database
- **WhatsApp Instance Keys**: API keys for Evolution/WAHA instances
- **User Passwords**: Hashed and salted (never stored in plain text)

### Environment Configuration

- API endpoints configured via environment variables
- No secrets in client-side code
- Backend manages all external API authentication

### Data Classification

| Data Type | Classification | Handling |
|-----------|---------------|----------|
| User passwords | Sensitive | Hashed, never transmitted after auth |
| OAuth tokens | Sensitive | Encrypted at rest, backend-only |
| WhatsApp messages | User data | Processed, not stored long-term |
| Group metadata | User data | Cached for performance |
| Analytics data | Internal | Aggregated, anonymized |

## Compliance & Policies

### Platform Policies

- **Rate Limiting**: Respects WhatsApp's terms of service regarding message rates
- **User Consent**: Users must own/admin groups they target
- **Data Retention**: Action history retained per user plan limits

### Security Best Practices

1. **Input Validation**: Zod schemas validate all form inputs
2. **XSS Prevention**: React's built-in escaping, no dangerouslySetInnerHTML
3. **CSRF Protection**: Token-based authentication prevents CSRF
4. **Error Handling**: Generic error messages to users, detailed logs to backend

### Code Quality

- SonarQube analysis for security vulnerabilities (see `docker-compose.sonarqube.yml`)
- TypeScript strict mode for type safety
- ESLint security rules enabled

## Incident Response

### Error Monitoring

- `ErrorBoundary` component catches React errors
- `ConnectErrorInfo` standardizes API error handling
- Backend logging for security events

### Security Issues

For security vulnerabilities, contact the development team directly. Do not create public issues for security concerns.

See [Architecture](./architecture.md) for system design details.
