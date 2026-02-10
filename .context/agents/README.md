---
status: filled
---

# Agent Handbook

This directory contains ready-to-use playbooks for AI agents collaborating on the Automatiza platform - a WhatsApp Marketing Automation system.

## Project Context

**Stack:** React 19 + TypeScript + Go 1.24 + ConnectRPC + PostgreSQL + Redis

**Key Features:**
- WhatsApp instance management via WAHA
- Automation workflows for offer sending
- Affiliate integrations (Shopee, Mercado Livre)
- Background job scheduling

## Available Agents

### Development Agents

| Agent | Use Case |
|-------|----------|
| [Feature Developer](./feature-developer.md) | Implement new features end-to-end |
| [Bug Fixer](./bug-fixer.md) | Analyze and fix bugs |
| [Refactoring Specialist](./refactoring-specialist.md) | Improve code structure |
| [Test Writer](./test-writer.md) | Write unit and E2E tests |

### Review Agents

| Agent | Use Case |
|-------|----------|
| [Code Reviewer](./code-reviewer.md) | Review code quality |
| [Security Auditor](./security-auditor.md) | Identify security issues |
| [Performance Optimizer](./performance-optimizer.md) | Find performance bottlenecks |

### Specialist Agents

| Agent | Use Case |
|-------|----------|
| [Frontend Specialist](./frontend-specialist.md) | React/TypeScript expertise |
| [Backend Specialist](./backend-specialist.md) | Go/gRPC expertise |
| [Database Specialist](./database-specialist.md) | PostgreSQL optimization |
| [DevOps Specialist](./devops-specialist.md) | CI/CD and infrastructure |
| [Architect Specialist](./architect-specialist.md) | System design decisions |
| [Mobile Specialist](./mobile-specialist.md) | Mobile/responsive development |

### Documentation Agent

| Agent | Use Case |
|-------|----------|
| [Documentation Writer](./documentation-writer.md) | Create technical documentation |

## How To Use These Playbooks

1. **Pick the agent** that matches your task
2. **Read the playbook** for context and guidelines
3. **Follow the collaboration checklist** for structured work
4. **Use the hand-off notes** when transitioning work

## Quick Start by Task Type

| Task | Agent to Use |
|------|--------------|
| "Add a new API endpoint" | Feature Developer + Backend Specialist |
| "Fix a bug in the dashboard" | Bug Fixer |
| "Improve code organization" | Refactoring Specialist |
| "Add tests for groups" | Test Writer |
| "Review this PR" | Code Reviewer |
| "Check for security issues" | Security Auditor |
| "Optimize slow queries" | Database Specialist + Performance Optimizer |
| "Set up CI/CD" | DevOps Specialist |
| "Plan new feature architecture" | Architect Specialist |

## Key Repository Locations

| Area | Location |
|------|----------|
| Frontend source | `client/src/` |
| Backend source | `automatiza-go/internal/` |
| Proto definitions | `proto/*/v1/*.proto` |
| E2E tests | `client/e2e/` |
| Backend tests | `automatiza-go/tests/` |
| Documentation | `.context/docs/` |

## Related Resources

- [Documentation Index](../docs/README.md)
- [Skills Library](../skills/README.md)
- [Project Overview](../docs/project-overview.md)
- [Architecture Notes](../docs/architecture.md)
