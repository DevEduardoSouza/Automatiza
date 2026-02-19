---
slug: authentication
category: features
generatedAt: 2026-02-18T22:38:13.917Z
relevantFiles:
  - client/e2e/pages/auth.page.ts
  - client/src/contexts/auth-context.tsx
  - client/e2e/fixtures/auth.fixture.ts
---

# How does authentication work?

## Overview

Authentication uses JWT tokens stored in HttpOnly cookies with CSRF protection. The system supports role-based access control (RBAC) with admin and user roles managed through a dedicated `user_roles` table.

## Authentication Flow

1. **Login**: User submits credentials via the `auth/v1` ConnectRPC service.
2. **Password Verification**: Backend validates password using **argon2id** hashing (high-security, resistant to GPU/ASIC attacks).
3. **Token Issuance**: On success, the server issues a JWT and sets it as an **HttpOnly + Secure + SameSite=Strict** cookie.
4. **Session Tracking**: Active sessions are tracked server-side. Real-time session status is available via SSE (Server-Sent Events).
5. **Subsequent Requests**: The ConnectRPC auth interceptor automatically injects the token from cookies into every request.

## Token Storage

- Tokens are stored in **HttpOnly cookies** (not localStorage) to prevent XSS-based token theft.
- Cookies are configured with `Secure` (HTTPS only) and `SameSite=Strict` flags.
- Session identifiers use high-entropy values generated via CSPRNG.

## Role-Based Access Control

| Role | Description |
|------|-------------|
| `admin` | Full access to all features, user management, system settings |
| `user` | Standard access to automations, groups, messaging |

- Roles are stored in the `user_roles` table with a composite UNIQUE constraint on `(user_id, role)`, allowing multiple roles per user.
- Backend middleware validates the user role on every protected endpoint.
- Frontend uses `AuthContext` to conditionally render UI based on the current user's role.

## Key Components

### Backend

- **Auth middleware** (`internal/middleware/`): Validates JWT on every request, extracts user identity and roles.
- **Auth service** (`internal/auth/`): Handles login, token generation/refresh, session management.
- **Password hashing**: Uses argon2id with configurable parameters (memory, iterations, parallelism).

### Frontend

- **AuthContext** (`client/src/contexts/auth-context.tsx`): Provides `AuthUser`, login/logout functions, and loading state to the component tree.
- **Auth interceptor** (`client/src/lib/connect/`): ConnectRPC interceptor that attaches credentials to every RPC call.
- **Auth pages** (`client/src/pages/auth.tsx`, `forgot-password.tsx`, `reset-password.tsx`): Login, password recovery, and reset flows.

## Additional Auth Features

- **Magic link authentication**: Passwordless login via email link.
- **Password reset flow**: Forgot password -> email with reset link -> new password form.
- **Session SSE**: Real-time session status updates via Server-Sent Events endpoint, allowing the frontend to react immediately to session invalidation.

## Security Considerations

- Passwords are never stored in plain text; only argon2id hashes are persisted.
- Failed login attempts are rate-limited.
- Sensitive comparisons (token validation) use constant-time comparison (`crypto/subtle.ConstantTimeCompare`).
- No tokens or secrets are exposed in client-side JavaScript or logs.
