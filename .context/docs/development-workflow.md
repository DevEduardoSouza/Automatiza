---
type: doc
name: development-workflow
description: Day-to-day engineering processes, branching, and contribution guidelines
category: workflow
generated: 2026-02-18
status: filled
scaffoldVersion: "2.0.0"
---

# Development Workflow

This document describes the day-to-day development process for the Automatiza project. It covers branching strategy, local environment setup, code review expectations, and the standards every contributor must follow before merging code.

## Branching & Releases

The project follows a trunk-based development model with short-lived feature branches.

- **Main branch:** `main` is the single source of truth and must always be deployable.
- **Feature branches:** Created from `main` using the pattern `feature/<description>` or `fix/<description>`.
- **Commit messages:** All commits must follow [Conventional Commits](https://www.conventionalcommits.org/):
  - Format: `type(scope): description`
  - Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `perf`, `ci`
  - Examples: `feat(whatsapp): add group message dispatch`, `fix(auth): handle expired refresh token`
  - Breaking changes must include `BREAKING CHANGE:` in the commit footer.
- **Pull Requests:** Every change goes through a PR targeting `main`. PRs must pass all CI checks before merge.
- **Releases:** Deployed via CI/CD pipeline triggered on merge to `main`. The VPS receives updates through the GitOps flow (push to remote, CI builds and deploys).

## Local Development

### Prerequisites

Ensure you have installed: Node.js 20+, Go 1.24+, Docker + Docker Compose, and the Buf CLI. See [tooling.md](./tooling.md) for the full list.

### First-Time Setup

```bash
# 1. Install all dependencies (Go modules + npm packages)
make install

# 2. Start PostgreSQL and Redis containers
make db-up

# 3. Run database migrations
make db-migrate

# 4. Start the full development environment (DB + Backend + Frontend)
make dev
```

### Day-to-Day Commands

| Task | Command | Notes |
|------|---------|-------|
| Start everything | `make dev` | DB + Backend + Frontend |
| Start everything + WAHA | `make dev-all` | Includes WhatsApp API |
| Frontend only | `make dev-client` | Vite dev server on port 3003 |
| Backend only | `make dev-go` | Gin server on port 3000 |
| Run worker | `cd automatiza-go && go run ./cmd/server/main.go -worker` | Background job processor |
| Generate protobuf stubs | `make proto-generate` | Run after any `.proto` change |
| Check service status | `make status` | Shows all running services |
| View logs | `make logs` | Aggregated logs from all services |
| Stop everything | `make stop` | Stops all services |
| Fresh database | `make db-reset && make db-migrate` | Destroys and recreates DB |

### Port Assignments

| Service | Port |
|---------|------|
| Frontend (Vite) | 3003 |
| Backend (Go/Gin) | 3000 |
| PostgreSQL | 5432 |
| Redis | 6379 |

### Protocol Buffers Workflow

Protobuf is the single source of truth for all API contracts. After editing any `.proto` file:

```bash
# Generate TypeScript + Go stubs
make proto-generate

# Lint proto files
make proto-lint

# Check for breaking changes
make proto-breaking
```

Generated code lives in `client/src/gen/` (TypeScript) and `automatiza-go/internal/gen/` (Go). These directories are gitignored and must never be edited manually.

## Code Review Expectations

Every PR must satisfy the following before merge:

1. **Typecheck passes** -- zero TypeScript errors, zero Go compilation errors.
2. **Linter clean** -- BiomeJS (frontend) and golangci-lint (backend) report no issues.
3. **All tests pass** -- `make test` exits with code 0. Flaky tests block merge until stabilized.
4. **No `any` types** -- use generics or proper typing. No exceptions without explicit justification.
5. **No dead code** -- remove unused imports, variables, functions, and legacy logic.
6. **i18n for all user-facing text** -- no hardcoded strings in the UI; everything goes through the i18n system (`client/src/i18n/locales/pt-br.json`).
7. **Observability in Go** -- every `if err != nil` block that produces an HTTP response must include structured logging, Sentry reporting (when applicable), and trace marking.
8. **Buf checks pass** -- `buf lint` and `buf breaking` must succeed for any proto changes.
9. **Small, focused diffs** -- prefer multiple small PRs over one large changeset.

## Related Documents

- [Testing Strategy](./testing-strategy.md) -- test types, running tests, and quality gates
- [Tooling & Productivity Guide](./tooling.md) -- required tools, IDE setup, and automation tips
- [Architecture Notes](./architecture.md) -- system design and layer boundaries
