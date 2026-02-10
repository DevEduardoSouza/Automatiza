---
type: skill
name: Commit Message
description: Generate commit messages following conventional commits with scope detection
skillSlug: commit-message
phases: [E, C]
status: filled
---

# Commit Message

## When to Use

- Creating commits for code changes
- Following conventional commit format
- Generating meaningful commit messages

## Format

```
<type>(<scope>): <description>

[optional body]

[optional footer]

Co-Authored-By: Claude <noreply@anthropic.com>
```

## Types

| Type | Description | Example |
|------|-------------|---------|
| `feat` | New feature | `feat(groups): add bulk delete action` |
| `fix` | Bug fix | `fix(auth): handle session expiry correctly` |
| `refactor` | Code refactoring | `refactor(hooks): simplify useGroups` |
| `docs` | Documentation | `docs(readme): update installation steps` |
| `test` | Tests | `test(groups): add E2E tests for sync` |
| `chore` | Maintenance | `chore(deps): update React to v19` |
| `perf` | Performance | `perf(queries): add index for groups` |
| `style` | Formatting | `style: run Biome formatter` |

## Scopes (Project-Specific)

### Frontend
- `auth` - Authentication
- `groups` - WhatsApp groups
- `automations` - Automation workflows
- `dashboard` - Dashboard page
- `whatsapp` - Instance management
- `jobs` - Background jobs
- `hooks` - React hooks
- `ui` - UI components
- `e2e` - E2E tests

### Backend
- `api` - API handlers
- `services` - Business logic
- `repo` - Repository layer
- `workers` - Background workers
- `auth` - Authentication
- `config` - Configuration

### Infrastructure
- `proto` - Protocol Buffers
- `deps` - Dependencies
- `ci` - CI/CD
- `docker` - Docker config

## Examples

### Simple Feature
```
feat(groups): add member count badge

Display member count in group cards for quick reference.
```

### Bug Fix
```
fix(auth): clear session on logout error

Previously, failed logout attempts left stale session data.
Now properly clears local state regardless of server response.
```

### Multi-scope Change
```
feat(automations,hooks): add automation diagnosis

- Add DiagnoseAutomation RPC with 120s timeout
- Create useAutomationDiagnosis hook
- Display diagnosis results in modal
```

### Breaking Change
```
feat(api)!: rename ListGroups to GetGroups

BREAKING CHANGE: ListGroups renamed to GetGroups for consistency.
Update all clients to use new method name.
```

## Instructions

1. **Analyze Changes**: Review staged files
2. **Determine Type**: Choose appropriate commit type
3. **Identify Scope**: Use project-specific scope
4. **Write Description**: Clear, imperative, <50 chars
5. **Add Body**: Explain "why" if not obvious
6. **Include Footer**: Co-authored-by attribution

## Git Commands

```bash
# View staged changes
git diff --staged

# Commit with message
git commit -m "$(cat <<'EOF'
feat(groups): add bulk sync action

Allow users to sync multiple groups at once.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```
