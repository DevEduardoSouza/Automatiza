#---
status: filled
---

# Project Overview

Automatiza is a WhatsApp marketing automation platform that enables businesses to manage groups, dispatch messages, run campaigns, and integrate with affiliate programs (Shopee, Mercado Livre, etc.). The system automates product promotion workflows through scheduled jobs, coupon management, and AI-powered engagement analysis.

## Codebase Reference

> **Detailed Analysis**: For complete symbol counts, architecture layers, and dependency graphs, see [`codebase-map.json`](./codebase-map.json).

## Quick Facts

- **Root**: `/Users/brunovilefort/Desktop/automatiza/novo-projeto`
- **Languages**: TypeScript (frontend), Go (backend)
- **Frontend Entry**: `client/src/main.tsx`
- **Backend**: `automatiza-go/`
- **API Definitions**: `proto/` (Protocol Buffers)
- Full analysis: [`codebase-map.json`](./codebase-map.json)

## Entry Points

- [`client/src/main.tsx`](../client/src/main.tsx) - Frontend React application entry
- `automatiza-go/` - Go backend services
- `proto/` - gRPC/Connect service definitions

## Key Exports

The frontend exposes 700+ exported symbols including:

- **Services**: WhatsApp integration, Automations, Campaigns, Jobs, Groups, Coupons
- **Components**: UI library (shadcn/ui), Admin dashboard, Automation forms
- **Hooks**: React hooks for each service domain (`useWhatsApp`, `useAutomations`, etc.)

> See [`codebase-map.json`](./codebase-map.json) for the complete export list.

## File Structure & Code Organization

- `client/` — React TypeScript frontend with Vite
  - `src/components/` — UI components organized by feature
  - `src/hooks/` — React hooks for data fetching and state
  - `src/gen/` — Generated Protocol Buffer TypeScript clients
  - `src/pages/` — Route-level page components
  - `e2e/` — Playwright end-to-end tests
- `automatiza-go/` — Go backend services
- `proto/` — Protocol Buffer service definitions
- `.context/` — AI context documentation (this folder)

## Technology Stack Summary

**Frontend**: React 18 with TypeScript, Vite build tool, TailwindCSS + shadcn/ui components, Connect-RPC for API communication.

**Backend**: Go services with gRPC/Connect protocol, Protocol Buffers for type-safe API contracts.

**Infrastructure**: Docker Compose for local development, SonarQube for code quality analysis.

## Core Framework Stack

- **Frontend**: React + TypeScript with Vite bundler
- **UI**: TailwindCSS with shadcn/ui component library
- **API Client**: Connect-RPC (generated from proto files)
- **Forms**: React Hook Form with Zod validation
- **State**: React Query for server state management

## Getting Started Checklist

1. Clone the repository and navigate to the project root
2. Install frontend dependencies: `cd client && npm install`
3. Generate proto clients if needed: check `Makefile` for proto generation commands
4. Start the development server: `npm run dev` in the client directory
5. Review [Development Workflow](./development-workflow.md) for day-to-day tasks

## Next Steps

- Review [Architecture Notes](./architecture.md) for system design details
- Check [Testing Strategy](./testing-strategy.md) for test requirements
- See [Tooling Guide](./tooling.md) for available scripts and automation
