# Q&A Index

Project type: **WhatsApp Marketing Automation Platform**

Generated: 2026-02-18T22:38:19.229Z

## Getting Started

- [How do I set up and run this project?](./getting-started.md)

## Architecture

- [How is the codebase organized?](./project-structure.md)

## Features

- [How does authentication work?](./authentication.md)
- [How is data stored and accessed?](./database.md)
- [What API endpoints are available?](./api-endpoints.md)

## Operations

- [How are errors handled?](./error-handling.md)
- [How do background jobs work?](./background-jobs.md)
- [How do I deploy this project?](./deployment.md)

## Testing

- [How do I run and write tests?](./testing.md)

## Quick Reference

| Area | Frontend | Backend |
|------|----------|---------|
| Language | TypeScript | Go 1.24 |
| Framework | React 19 + Vite | Gin + ConnectRPC |
| Database | - | PostgreSQL 16 |
| Cache/Queue | - | Redis + Asynq |
| WhatsApp | - | WAHA Plus (GoWS Engine) |
| API Protocol | ConnectRPC (gRPC-Web) | ConnectRPC + Protocol Buffers |
| Styling | TailwindCSS 4.1 + shadcn/ui | - |
| Linter/Formatter | BiomeJS | golangci-lint + gofmt |
| Unit Tests | Vitest | Go testing + testcontainers |
| E2E Tests | Playwright | Go E2E tests |
| Observability | Sentry (frontend) | Sentry + Datadog (backend) |
| Routing | React Router DOM 7.x | Gin router |
