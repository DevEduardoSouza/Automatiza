# Profile Load Test

Load testing tool for Profile and Password Reset ConnectRPC endpoints.

## Prerequisites

- Go 1.24+
- Running backend server (`make dev-backend`)
- PostgreSQL and Redis running (`make docker-deps-up`)

## Quick Start

```bash
# Run all scenarios (requires auth token for authenticated endpoints)
go run ./scripts/loadtest/profile_loadtest.go -url http://localhost:3000 -token <jwt_token>

# Run only password reset test (no auth required)
go run ./scripts/loadtest/profile_loadtest.go -url http://localhost:3000 -scenario resetpassword

# Run specific scenario with verbose output
go run ./scripts/loadtest/profile_loadtest.go -url http://localhost:3000 -token <jwt_token> -scenario getprofile -v
```

## Scenarios

| Scenario       | RPS | Duration | Target p99 | Auth Required |
|----------------|-----|----------|------------|---------------|
| getprofile     | 100 | 30s      | < 100ms    | Yes           |
| getlimits      | 100 | 30s      | < 150ms    | Yes           |
| resetpassword  | 10  | 60s      | < 500ms    | No            |
| updateprofile  | 20  | 30s      | < 200ms    | Yes           |

## Options

| Flag       | Default               | Description                           |
|------------|-----------------------|---------------------------------------|
| `-url`     | http://localhost:3000 | Base URL of the server                |
| `-token`   | (empty)               | JWT auth token                        |
| `-csrf`    | (empty)               | CSRF token (optional)                 |
| `-scenario`| all                   | Scenario to run                       |
| `-v`       | false                 | Verbose output (show errors)          |

## Getting a Test Token

Generate a test JWT token for load testing:

```bash
# Option 1: Use the gentokens utility
cd automatiza-go && go run ./cmd/gentokens/main.go

# Option 2: Login via the frontend and extract the token from cookies/headers
```

## Expected Output

```
========================================
  Profile & Password Reset Load Test
========================================
Base URL: http://localhost:3000
Auth Token: eyJhbGciOi...xxxxx

----------------------------------------
Scenario: getprofile
Endpoint: /profiles.v1.ProfilesService/GetProfile
Target: 100 req/s for 30s (p99 < 100ms)
----------------------------------------
Running...
  Progress: 500 requests, 100.2 req/s, p99=45ms
  Progress: 1000 requests, 100.1 req/s, p99=48ms
  ...

Results:
  Total Requests:   3000
  Success:          3000 (100.0%)
  Errors:           0 (0.0%)
  Duration:         30.001s
  Actual RPS:       100.0

Latency:
  Min:              5ms
  Avg:              25ms
  P50:              22ms
  P95:              42ms
  P99:              48ms
  Max:              125ms

PASS: p99 (48ms) <= target (100ms)
```

## Rate Limiting Test

The `resetpassword` scenario tests the rate limiting implementation:

- Sends requests to 10 different email addresses
- Expects rate limiting to kick in after ~3 requests per email
- Should see ~70% of requests rate limited after initial burst

Expected output for rate limiting:

```
Results:
  Total Requests:   600
  Success:          30 (5.0%)
  Errors:           0 (0.0%)
  Rate Limited:     570 (95.0%)
  ...

PASS: Rate limiting detected (570 requests limited)
```

## Troubleshooting

### "SKIP: Requires auth token"

Run with a valid JWT token:
```bash
go run ./scripts/loadtest/profile_loadtest.go -token <your_jwt_token>
```

### Connection refused

Ensure the backend is running:
```bash
make dev-backend
```

### High error rate

1. Check server logs for errors
2. Ensure PostgreSQL and Redis are running
3. Run with `-v` flag to see error details

### p99 exceeds target

1. Check if the database has appropriate indexes
2. Monitor server CPU/memory during test
3. Consider reducing RPS or increasing target
