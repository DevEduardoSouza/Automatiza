---
status: filled
slug: authentication
category: features
---

# How does authentication work?

## Overview

The Automatiza platform uses **cookie-based session authentication** with JWT tokens, CSRF protection, and optional magic link (passwordless) login.

## Authentication Flow

### Password Login
```
1. User submits email + password
2. Backend validates credentials
3. Backend creates session + generates JWT
4. JWT stored in HTTP-only secure cookie
5. CSRF token returned and stored in memory
6. Frontend updates AuthContext state
```

### Magic Link Login
```
1. User requests magic link for email
2. Backend sends email with unique token
3. User clicks link → verifyMagicLink(token)
4. Backend validates token, creates session
5. Same cookie/CSRF flow as password login
```

## Key Components

### Frontend

| File | Purpose |
|------|---------|
| `src/contexts/auth-context.tsx` | AuthProvider, AuthContext, useAuth hook |
| `src/pages/auth.tsx` | Login UI (password + magic link) |
| `src/pages/auth-magic.tsx` | Magic link verification |
| `src/lib/auth-error-mapper.ts` | Error code → user message mapping |
| `src/lib/connect-transport.ts` | CSRF interceptor |

### Backend

| File | Purpose |
|------|---------|
| `internal/auth/service.go` | Core auth logic |
| `internal/auth/session.go` | Session management |
| `internal/auth/csrf.go` | CSRF token generation/validation |
| `internal/auth/rate_limiter.go` | Rate limiting |
| `internal/connectrpc/auth_handler.go` | RPC handler |

## AuthUser Type

```typescript
interface AuthUser {
  id: string;           // User UUID
  email: string;        // Email address
  displayName?: string; // Optional display name
  role: string;         // "user" | "admin"
  emailVerified: boolean;
  createdAt: Date;
}
```

## Session Management

```typescript
interface SessionInfo {
  sessionId: string;
  expiresAt: Date;
  deviceFingerprint?: string;
  ipAddress?: string;
  userAgent?: string;
  createdAt: Date;
  lastActivityAt: Date;
  isCurrent: boolean;
}
```

**Operations:**
- `listSessions()` - View all active sessions
- `revokeSession(sessionId)` - Logout specific device
- `signOut(logoutAllDevices)` - Logout current or all sessions

## Security Features

### CSRF Protection
- Double-submit cookie pattern
- Token stored in memory (never localStorage)
- X-CSRF-Token header on mutations
- Exempt paths: Login, Register, PasswordReset, VerifyEmail

### Rate Limiting
- Per-IP and per-email login rate limiting
- Redis-backed rate limiter
- Exponential backoff for locked accounts

### Cookie Configuration
```
Secure: true (production)
HttpOnly: true
SameSite: Strict
```

## Error Handling

| Error Code | User Message |
|------------|--------------|
| `invalid_credentials` | Email ou senha inválidos |
| `email_not_confirmed` | Email não confirmado |
| `email_exists` | Email já cadastrado |
| `session_expired` | Sessão expirada |
| `rate_limit_exceeded` | Muitas tentativas |

## Usage Example

```typescript
// In a component
const { user, signIn, signOut, isLoading } = useAuth();

// Login
const { error } = await signIn(email, password);
if (error) {
  toast({ variant: 'destructive', description: error.message });
}

// Check auth state
if (isLoading) return <Spinner />;
if (!user) return <Navigate to="/auth" />;
```
