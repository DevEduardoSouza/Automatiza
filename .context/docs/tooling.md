#---
status: filled
---

# Tooling & Productivity Guide

This guide covers the tools, scripts, and configurations that streamline development on the Automatiza platform.

## Required Tooling

### Node.js
- **Version**: Check `.nvmrc` for required version
- **Installation**: Use nvm for version management
- **Purpose**: Frontend development, build tools

```bash
# Install nvm and use project version
nvm install
nvm use
```

### Go
- **Version**: Latest stable
- **Purpose**: Backend services
- **Installation**: https://go.dev/dl/

### Protocol Buffers (buf)
- **Purpose**: Generate TypeScript clients from proto files
- **Installation**: https://buf.build/docs/installation

```bash
# Install buf CLI
brew install bufbuild/buf/buf
```

### Playwright
- **Purpose**: E2E testing
- **Installation**: Installed via npm

```bash
cd client
npx playwright install
```

### Docker (Optional)
- **Purpose**: Local services, SonarQube
- **Installation**: Docker Desktop

## Recommended Automation

### Pre-commit Checks

Before committing, run these checks:

```bash
cd client

# Type checking
npm run type-check

# Linting
npm run lint

# Format check (if configured)
npm run format:check
```

### Proto Generation

When proto files change:

```bash
# From project root
make proto-gen

# Or manually with buf
cd proto
buf generate
```

### Development Server

```bash
cd client
npm run dev
```

The dev server supports:
- Hot module replacement (HMR)
- Automatic TypeScript compilation
- Proxy to backend API

### Test Automation

```bash
# Unit tests with watch
npm run test -- --watch

# E2E tests with UI
npx playwright test --ui

# Coverage report
npm run test -- --coverage
```

## IDE / Editor Setup

### VS Code Extensions

Recommended extensions for this project:

- **ESLint** - JavaScript/TypeScript linting
- **Prettier** - Code formatting
- **Tailwind CSS IntelliSense** - Tailwind autocomplete
- **TypeScript Vue Plugin (Volar)** - If using Vue
- **Proto3** - Protocol Buffer syntax highlighting
- **Go** - Go language support
- **Playwright Test** - E2E test runner

### Workspace Settings

Add to `.vscode/settings.json`:

```json
{
  "editor.formatOnSave": true,
  "editor.defaultFormatter": "esbenp.prettier-vscode",
  "typescript.preferences.importModuleSpecifier": "relative",
  "editor.codeActionsOnSave": {
    "source.fixAll.eslint": true
  }
}
```

### Snippets

Useful snippets for common patterns:

**React Hook** (`rh`):
```typescript
export function use${1:Name}() {
  return {
    $0
  };
}
```

**Connect Hook** (`ch`):
```typescript
export function use${1:Domain}Connect() {
  const client = useClient(${1:Domain}Service);
  return {
    $0
  };
}
```

## Productivity Tips

### Terminal Aliases

Add to your shell profile:

```bash
# Quick navigation
alias proj="cd ~/Desktop/automatiza/novo-projeto"
alias client="cd ~/Desktop/automatiza/novo-projeto/client"

# Common commands
alias dev="npm run dev"
alias test="npm run test"
alias lint="npm run lint"
alias e2e="npx playwright test"
```

### Makefile Commands

Check the `Makefile` for available commands:

```bash
make help        # List all commands
make proto-gen   # Generate proto clients
make test        # Run all tests
```

### Docker Compose

For local services:

```bash
# Start SonarQube for code analysis
docker-compose -f docker-compose.sonarqube.yml up -d
```

### Debugging

- Use React DevTools for component inspection
- Use Network tab to debug API calls
- Playwright UI mode for E2E debugging

See [Development Workflow](./development-workflow.md) for related processes.
