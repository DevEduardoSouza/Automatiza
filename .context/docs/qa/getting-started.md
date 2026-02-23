---
slug: getting-started
category: getting-started
generatedAt: 2026-02-18T22:38:09.787Z
---

# How do I set up and run this project?

## Prerequisites

- **Go** 1.24+
- **Node.js** LTS (v20+) and npm
- **Docker** and Docker Compose (for PostgreSQL, Redis, WAHA)
- **Make** (build automation)
- **Buf CLI** (for Protocol Buffer generation)

## Installation

```bash
# Clone the repository
git clone <repository-url>
cd novo-projeto

# Install all dependencies (Go modules + npm packages)
make install
```

## Starting the Development Environment

### Full Stack (recommended)

```bash
# Start database containers + backend + frontend
make dev
```

This runs PostgreSQL, Redis, the Go backend, and the Vite dev server.

### With WAHA (WhatsApp integration)

```bash
# Start everything including WAHA Plus
make dev-all
```

### Individual Services

```bash
# Database only (PostgreSQL + Redis)
make db-up

# Run migrations
make db-migrate

# Frontend only (Vite dev server)
make dev-client

# Backend only (Go server)
make dev-go

# WAHA only
make waha-up
```

## Default Ports

| Service | Port | URL |
|---------|------|-----|
| Frontend (Vite) | 3003 | `http://localhost:3003` |
| Backend (Go/Gin) | 3000 | `http://localhost:3000` |
| PostgreSQL | 5432 | `localhost:5432` |
| Redis | 6379 | `localhost:6379` |

## Database Management

```bash
# Start PostgreSQL + Redis
make db-up

# Run pending migrations
make db-migrate

# View PostgreSQL logs
make db-logs

# Access PostgreSQL shell
make db-shell

# Full reset (DESTRUCTIVE - drops all data)
make db-reset
```

## Worker Mode

The backend can run in worker mode for background job processing:

```bash
cd automatiza-go && go run ./cmd/server/main.go -worker
```

## Useful Commands

```bash
# Check status of all services
make status

# View logs from all services
make logs

# Stop everything
make stop

# Generate Protocol Buffer stubs (TypeScript + Go)
make proto-generate

# Run all linters
make lint

# Run all tests
make test
```

## Troubleshooting

- If database migrations fail, try `make db-reset && make db-migrate` for a clean start.
- Never use PM2 in the local environment. Use `make dev` commands instead.
- If ports are occupied, stop existing services with `make stop` before starting.
