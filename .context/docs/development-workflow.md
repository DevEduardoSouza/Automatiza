#---
status: filled
---

# Development Workflow

This guide covers the day-to-day engineering processes for contributing to the Automatiza codebase.

## Branching & Releases

- **Main Branch**: `main` - production-ready code
- **Feature Branches**: `feature/<description>` for new features
- **Bug Fixes**: `fix/<description>` for bug fixes
- **Hotfixes**: `hotfix/<description>` for urgent production fixes

**Commit Convention**: Use conventional commits format:
- `feat:` for new features
- `fix:` for bug fixes
- `docs:` for documentation
- `refactor:` for code refactoring
- `test:` for test additions/changes

## Local Development

### Frontend (client/)

```bash
# Install dependencies
cd client
npm install

# Start development server
npm run dev

# Build for production
npm run build

# Run linting
npm run lint

# Run type checking
npm run type-check
```

### Proto Generation

```bash
# Generate TypeScript clients from proto files
# Check Makefile for specific commands
make proto-gen
```

### Backend (automatiza-go/)

```bash
# Navigate to backend
cd automatiza-go

# Run backend services
go run ./...
```

## Code Review Expectations

**Before Submitting**:
1. Run `npm run lint` and fix all issues
2. Run `npm run type-check` with no errors
3. Run tests with `npm run test`
4. Ensure E2E tests pass for affected features

**Review Checklist**:
- Code follows existing patterns in the codebase
- New hooks follow the `use-{domain}-connect.ts` naming convention
- Proto changes are backward compatible
- Tests cover new functionality
- No console.log or debug statements

**Required Approvals**: 1 approval minimum before merge

## Onboarding Tasks

New developers should:

1. **Setup Environment**
   - Clone repository
   - Install Node.js (check `.nvmrc` for version)
   - Install Go for backend work
   - Run `npm install` in `client/`

2. **Explore the Codebase**
   - Read [Project Overview](./project-overview.md)
   - Review [Architecture](./architecture.md)
   - Check existing hooks in `client/src/hooks/`

3. **First Tasks**
   - Fix a small bug or add a test
   - Add documentation for an undocumented component
   - Improve error handling in a hook

4. **Resources**
   - Check [Testing Strategy](./testing-strategy.md) for test guidelines
   - See [Tooling Guide](./tooling.md) for available scripts
