# E2E Test Specification

This document outlines the E2E tests required to validate the migration from Node.js to Go.

## Test Framework
- Go: `testing` package with table-driven tests
- HTTP testing: `net/http/httptest`
- Assertions: `stretchr/testify`

## 1. Auth Flow Tests

### Login
- [ ] Login with valid email/password returns session cookie
- [ ] Login with invalid password returns 401
- [ ] Login with non-existent email returns 401
- [ ] Login sets HttpOnly, Secure, SameSite cookies

### Registration
- [ ] Register creates user and sends welcome email
- [ ] Register with existing email returns error
- [ ] Register validates email format
- [ ] Register validates password strength

### Password Reset
- [ ] Request reset sends email (doesn't reveal if email exists)
- [ ] Valid token allows password reset
- [ ] Expired token returns error
- [ ] Used token returns error
- [ ] Rate limiting prevents abuse

### Logout
- [ ] Logout invalidates session
- [ ] Logout clears cookies

## 2. WhatsApp Flow Tests

### Instance Management
- [ ] Create instance returns QR code data
- [ ] Get instance status returns current state
- [ ] Restart instance triggers reconnection
- [ ] Delete instance removes session

### Groups
- [ ] Sync groups fetches from WAHA
- [ ] Get groups returns user's groups
- [ ] Get group participants returns member list
- [ ] Update group subject requires admin
- [ ] Update group description requires admin
- [ ] Get invite code returns valid code

### Messaging
- [ ] Send text message enqueues job
- [ ] Send image message with base64 works
- [ ] Send image message with URL works
- [ ] Send poll message creates poll

## 3. Automations Flow Tests

### CRUD
- [ ] List automations returns user's automations
- [ ] Create automation validates required fields
- [ ] Update automation updates specified fields
- [ ] Delete automation removes automation

### Execution
- [ ] Diagnose checks WhatsApp connection
- [ ] Diagnose checks groups configured
- [ ] Diagnose checks affiliate credentials
- [ ] Run automation searches for offers
- [ ] Run automation applies deduplication
- [ ] Run automation respects delay_seconds
- [ ] Reset history clears dedup cache

## 4. Redirect Links Flow Tests

### Public Redirect
- [ ] Valid slug returns invite URL
- [ ] Invalid slug returns 404
- [ ] Inactive link returns 404
- [ ] member_limit strategy skips full groups
- [ ] click_limit strategy respects limits
- [ ] Click counter increments

### Management
- [ ] Create link validates slug uniqueness
- [ ] Update link updates fields
- [ ] Delete link removes link

## 5. Kiwify Webhook Tests

### Purchase Approved
- [ ] New user creates profile with temp password
- [ ] New user sends welcome email
- [ ] Existing user updates plan
- [ ] Out-of-order events are ignored

### Refund/Chargeback
- [ ] Suspends user account
- [ ] Doesn't override manual blocks

### Subscription Canceled
- [ ] Updates billing status

## 6. Admin Tests

### User Management
- [ ] List users requires admin role
- [ ] Search users filters by query
- [ ] Block user updates billing status
- [ ] Unblock user reactivates account
- [ ] Update plan changes subscription

### System Events
- [ ] Get events returns recent events
- [ ] Events are filtered by limit

## Running Tests

```bash
# Run all tests
cd automatiza-go && go test ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package
go test ./internal/handlers/...
go test ./internal/services/...
```

## Test Data

Tests should use:
- In-memory database or test transactions
- Mock WAHA service
- Test fixtures for common data
