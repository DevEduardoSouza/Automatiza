---
type: skill
name: PR Review
description: Review pull requests against team standards and best practices
skillSlug: pr-review
phases: [R, V]
status: filled
---

# PR Review

## When to Use

- Reviewing pull requests before merge
- Validating implementation against requirements
- Ensuring code quality and standards

## Review Process

### 1. Understand Context
- Read PR description and linked issues
- Understand the problem being solved
- Check if scope is appropriate

### 2. Review Changes
- Use the code-review skill checklist
- Focus on correctness, not style (Biome handles that)
- Check for security implications

### 3. Test Coverage
- New code has appropriate tests
- Existing tests updated if behavior changed
- E2E tests for user-facing changes

### 4. Documentation
- README updated if needed
- API changes documented in proto files
- Complex logic has comments

## PR Checklist

### General
- [ ] PR title follows conventional commits
- [ ] Description explains what and why
- [ ] Linked to issue/ticket if applicable
- [ ] Appropriate reviewers assigned

### Code Quality
- [ ] Follows project patterns
- [ ] No unnecessary complexity
- [ ] Error handling appropriate
- [ ] No hardcoded secrets/URLs

### Testing
- [ ] Tests pass locally
- [ ] New tests for new functionality
- [ ] E2E tests for UI changes
- [ ] Edge cases covered

### Security
- [ ] Input validation present
- [ ] No sensitive data exposure
- [ ] Auth checks in place

### Performance
- [ ] No obvious performance issues
- [ ] Database queries optimized
- [ ] No N+1 queries

## Review Comments

### Requesting Changes
```markdown
**Issue**: [Description of the problem]

**Location**: `file.ts:42`

**Suggestion**:
```typescript
// Suggested fix
```

**Why**: [Explanation of why this matters]
```

### Asking Questions
```markdown
**Question**: Why was this approach chosen over [alternative]?

This seems like it might cause [potential issue]. Could you clarify the reasoning?
```

### Approving
```markdown
## Summary
[Brief description of what was reviewed]

## Highlights
- Good use of [pattern/approach]
- Clean implementation of [feature]

## Minor Suggestions (non-blocking)
- Consider [optional improvement]

✅ **Approved**
```

## Using GitHub CLI

```bash
# View PR changes
gh pr diff 123

# View PR comments
gh api repos/owner/repo/pulls/123/comments

# Submit review
gh pr review 123 --approve
gh pr review 123 --request-changes --body "..."
gh pr review 123 --comment --body "..."
```

## Project-Specific Checks

### Frontend Changes
- [ ] Uses shadcn/ui components
- [ ] Follows hook pattern
- [ ] Proper loading/error states
- [ ] i18n for user-facing text

### Backend Changes
- [ ] Repository pattern followed
- [ ] Proper error wrapping
- [ ] Structured logging
- [ ] Proto changes backward compatible

### Proto Changes
- [ ] Field numbers not reused
- [ ] Backward compatible
- [ ] Generated code committed
- [ ] Both Go and TS updated
