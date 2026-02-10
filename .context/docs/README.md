---
status: filled
---

# Documentation Index

Welcome to the Automatiza repository knowledge base. This documentation provides context for AI agents and developers working with the codebase.

## Core Guides

- [Project Overview](./project-overview.md) - High-level overview of the project
- [Architecture Notes](./architecture.md) - System architecture and design decisions
- [Development Workflow](./development-workflow.md) - Day-to-day development practices
- [Testing Strategy](./testing-strategy.md) - Test approach and coverage
- [Glossary & Domain Concepts](./glossary.md) - Business terminology
- [Data Flow & Integrations](./data-flow.md) - System diagrams and integration specs
- [Security & Compliance Notes](./security.md) - Auth model and security considerations
- [Tooling & Productivity Guide](./tooling.md) - CLI scripts and automation workflows

## Repository Structure

| Directory | Description |
|-----------|-------------|
| `automatiza-go/` | Go backend services |
| `client/` | React TypeScript frontend |
| `proto/` | Protocol Buffer definitions |

## Quick Links

- **Detailed Analysis**: For complete symbol counts, architecture layers, and dependency graphs, see [`codebase-map.json`](./codebase-map.json)
- **API Definitions**: See `proto/` directory for gRPC service definitions
- **Frontend Entry**: `client/src/main.tsx`

## Document Map

| Guide | File | Primary Inputs |
|-------|------|----------------|
| Project Overview | `project-overview.md` | Roadmap, README, stakeholder notes |
| Architecture Notes | `architecture.md` | ADRs, service boundaries, dependency graphs |
| Development Workflow | `development-workflow.md` | Branching rules, CI config, contributing guide |
| Testing Strategy | `testing-strategy.md` | Test configs, CI gates, known flaky suites |
| Glossary & Domain Concepts | `glossary.md` | Business terminology, user personas, domain rules |
| Data Flow & Integrations | `data-flow.md` | System diagrams, integration specs, queue topics |
| Security & Compliance Notes | `security.md` | Auth model, secrets management, compliance requirements |
| Tooling & Productivity Guide | `tooling.md` | CLI scripts, IDE configs, automation workflows |
