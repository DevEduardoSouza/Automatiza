---
slug: deployment
category: operations
generatedAt: 2026-02-18T22:38:19.217Z
relevantFiles:
  - docker-compose.sonarqube.yml
  - automatiza-go/Dockerfile
  - automatiza-go/docker-compose.yml
  - client/Dockerfile
  - docker/docker-compose.prod.yml
  - .github/workflows/ci.yml
  - .github/workflows/deploy.yml
  - .github/workflows/setup-vps.yml
---

# How do I deploy this project?

## Deployment Model

The project follows a **GitOps** approach: Git is the single source of truth. All changes to production go through the repository and CI/CD pipeline. Direct edits on the VPS are strictly prohibited.

## CI/CD Pipeline

GitHub Actions workflows are defined in `.github/workflows/`:

| Workflow | File | Trigger | Purpose |
|----------|------|---------|---------|
| **CI** | `ci.yml` | Push/PR | Lint, typecheck, tests, buf checks |
| **Deploy** | `deploy.yml` | Merge to main | Build, push, deploy to VPS |
| **Setup VPS** | `setup-vps.yml` | Manual | Initial VPS provisioning |

### CI Workflow (`ci.yml`)

Runs on every push and pull request:

1. **Frontend**: TypeScript typecheck, BiomeJS lint, Vitest tests
2. **Backend**: golangci-lint, Go tests (unit + integration via testcontainers)
3. **Proto**: buf lint, buf breaking (contract validation)
4. All gates must pass before merge is allowed.

### Deploy Workflow (`deploy.yml`)

Triggered on merge to `main`:

1. Build Go binary (static: `CGO_ENABLED=0`, `-ldflags "-s -w"`)
2. Build frontend with Vite (`npm run build`)
3. Deploy artifacts to VPS via SSH
4. Restart services via systemd

## Build Configuration

### Backend (Go)

```bash
CGO_ENABLED=0 go build -ldflags "-s -w" -o server ./cmd/server/main.go
```

- Static binary with no CGo dependencies.
- Stripped debug symbols (`-s -w`) for smaller binary size.
- Distroless container image for production.

### Frontend (React/Vite)

```bash
cd client && npm run build
```

- Vite produces optimized static assets in `client/dist/`.
- CSP nonce placeholder (`__CSP_NONCE__`) is embedded in the HTML for server-side substitution at runtime.

## Production Infrastructure

### VPS Architecture

```
Internet
  |
  v
Nginx (reverse proxy, TLS termination, CSP nonce injection)
  |
  +-- /api/* --> Go backend (port 3000, systemd service)
  +-- /r/*   --> Go backend (public redirect handler)
  +-- /*     --> Static frontend files (Vite build output)
  |
Go Backend
  +-- PostgreSQL 16 (Docker container)
  +-- Redis (Docker container)
  +-- WAHA Plus (Docker container)
```

### System Services

Production services are managed via **systemd** (definitions in `systemd/`):

- **Backend server**: Runs the Go binary in server mode.
- **Backend worker**: Runs the Go binary in worker mode (`-worker` flag).
- **Docker services**: PostgreSQL, Redis, and WAHA Plus run as Docker containers managed by Docker Compose (`docker/docker-compose.prod.yml`).

### Nginx Configuration

Nginx (`nginx/`) serves as the reverse proxy with:

- TLS termination (HTTPS)
- CSP nonce injection per request
- Static file serving for the frontend build
- Reverse proxy to the Go backend for API and webhook routes

## Secrets Management

- Secrets are managed via **GitHub Secrets** (`gh secret list --env production`).
- Non-sensitive configuration uses **GitHub Variables** (`gh variable list --env production`).
- Secrets include: database credentials, JWT signing keys, WAHA API keys, Sentry DSN, Datadog API key, etc.
- Secrets are injected into the deployment environment at deploy time; they are never committed to the repository.

## Database Migrations in Production

Migrations run via the Go binary's `migrate` subcommand:

```bash
./server migrate up
```

This executes all pending migrations in order (000 first, then incremental 001-029).

## Monitoring in Production

- **Sentry**: Error tracking and alerting for both frontend and backend.
- **Datadog**: APM traces, infrastructure metrics (CPU, memory, disk), log aggregation.
- **systemd journal**: Local service logs (`journalctl -u automatiza-server`).
- **Docker logs**: Container-level logs (`docker logs --tail 100 <container>`).

## Rollback

- Rollback is done by reverting the problematic commit and re-deploying via CI/CD.
- Risky changes are deployed behind **feature flags** for gradual rollout and quick rollback.
- Database migrations are designed to be forward-compatible; destructive schema changes are avoided.
