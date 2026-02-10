---
type: skill
name: Security Audit
description: Security review checklist for Automatiza code and infrastructure
skillSlug: security-audit
phases: [R, V]
status: filled
---

# Security Audit

## When to Use

- Reviewing code for security vulnerabilities
- Auditing authentication/authorization
- Checking for OWASP Top 10 issues
- Validating secure configurations

## Security Checklist

### 1. Authentication

- [ ] Passwords hashed with bcrypt (cost >= 10)
- [ ] JWT tokens have appropriate expiry
- [ ] Session tokens are HTTP-only, Secure, SameSite
- [ ] CSRF protection on all mutations
- [ ] Rate limiting on login attempts
- [ ] Account lockout after failed attempts
- [ ] Secure password reset flow

**Check:**
```go
// automatiza-go/internal/auth/password.go
bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)

// Cookie settings
cookie.HttpOnly = true
cookie.Secure = true
cookie.SameSite = http.SameSiteStrictMode
```

### 2. Authorization

- [ ] Role-based access control implemented
- [ ] Users can only access their own resources
- [ ] Admin endpoints properly protected
- [ ] No privilege escalation possible

**Check:**
```go
// Verify user owns resource
if resource.UserID != currentUser.ID {
  return connect.NewError(connect.CodePermissionDenied, ...)
}
```

### 3. Input Validation

- [ ] All inputs validated server-side
- [ ] Use parameterized queries (no SQL injection)
- [ ] Validate file uploads (type, size)
- [ ] Sanitize output (prevent XSS)

**Check:**
```go
// Parameterized query - GOOD
db.Query("SELECT * FROM users WHERE id = $1", id)

// String interpolation - BAD
db.Query("SELECT * FROM users WHERE id = " + id)
```

### 4. Data Protection

- [ ] Sensitive data encrypted at rest
- [ ] HTTPS enforced in production
- [ ] No secrets in code or logs
- [ ] PII handled according to policy

**Check:**
```go
// Don't log sensitive data
logger.Info("user login", zap.String("email", user.Email))
// NOT: zap.String("password", password)
```

### 5. API Security

- [ ] Rate limiting on all endpoints
- [ ] Request size limits configured
- [ ] CORS properly configured
- [ ] No sensitive data in error messages

**Check:**
```go
// Proper error response - GOOD
connect.NewError(connect.CodeNotFound, errors.New("resource not found"))

// Leaking info - BAD
connect.NewError(connect.CodeNotFound, fmt.Errorf("user %s not found in table users", id))
```

### 6. Dependencies

- [ ] Dependencies regularly updated
- [ ] No known vulnerabilities
- [ ] Minimal dependency footprint

**Check:**
```bash
# Frontend
npm audit

# Backend
go list -m -json all | go-mod-outdated
```

### 7. Frontend Security

- [ ] No sensitive data in localStorage
- [ ] CSRF tokens used correctly
- [ ] Content Security Policy headers
- [ ] No inline scripts/styles (CSP)

**Check:**
```typescript
// GOOD - token in memory
let csrfToken: string | null = null;

// BAD - token in localStorage
localStorage.setItem('csrfToken', token);
```

### 8. Infrastructure

- [ ] Database not publicly accessible
- [ ] Redis password protected
- [ ] Environment variables for secrets
- [ ] Docker images from trusted sources

## OWASP Top 10 Checklist

| # | Vulnerability | Status |
|---|---------------|--------|
| 1 | Broken Access Control | [ ] Checked |
| 2 | Cryptographic Failures | [ ] Checked |
| 3 | Injection | [ ] Checked |
| 4 | Insecure Design | [ ] Checked |
| 5 | Security Misconfiguration | [ ] Checked |
| 6 | Vulnerable Components | [ ] Checked |
| 7 | Auth/Identity Failures | [ ] Checked |
| 8 | Integrity Failures | [ ] Checked |
| 9 | Logging Failures | [ ] Checked |
| 10 | SSRF | [ ] Checked |

## Audit Report Template

```markdown
## Security Audit Report

**Date:** YYYY-MM-DD
**Scope:** [Area audited]
**Auditor:** [Name]

### Summary
[Brief overview of findings]

### Critical Issues
1. **Issue Title**
   - Location: `file:line`
   - Description: [What's wrong]
   - Impact: [Potential damage]
   - Remediation: [How to fix]

### High Issues
[Same format]

### Medium Issues
[Same format]

### Low Issues
[Same format]

### Recommendations
1. [Recommendation]
2. [Recommendation]

### Passed Checks
- [x] Authentication properly implemented
- [x] Authorization checks in place
- [x] Input validation present
```

## Quick Security Commands

```bash
# Check npm vulnerabilities
cd client && npm audit

# Check for secrets in git
git log -p | grep -E "(password|secret|key|token)" | head -20

# List environment variables
printenv | grep -E "(KEY|SECRET|PASSWORD|TOKEN)"

# Check open ports
lsof -i -P -n | grep LISTEN
```
