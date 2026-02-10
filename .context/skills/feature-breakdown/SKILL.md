---
type: skill
name: Feature Breakdown
description: Break down features into implementable tasks for Automatiza
skillSlug: feature-breakdown
phases: [P]
status: filled
---

# Feature Breakdown

## When to Use

- Planning new feature implementation
- Breaking large tasks into smaller pieces
- Creating implementation roadmap
- Estimating work scope

## Breakdown Process

### 1. Understand the Feature
- What problem does it solve?
- Who are the users?
- What are the acceptance criteria?

### 2. Identify Components

For a typical Automatiza feature:

| Layer | Components |
|-------|------------|
| Proto | Service definition, messages, enums |
| Backend | Handler, service, repository, models |
| Frontend | Hook, page/components, tests |
| Tests | Unit tests, E2E tests |

### 3. Create Task List

```markdown
## Feature: [Feature Name]

### Phase 1: API Design
- [ ] Define proto service and messages
- [ ] Run code generation
- [ ] Review with team

### Phase 2: Backend Implementation
- [ ] Create domain model
- [ ] Implement repository
- [ ] Implement service
- [ ] Create ConnectRPC handler
- [ ] Add unit tests

### Phase 3: Frontend Implementation
- [ ] Create React hook
- [ ] Build UI components
- [ ] Add to page/routing
- [ ] Add loading/error states
- [ ] Add i18n translations

### Phase 4: Testing
- [ ] Add backend unit tests
- [ ] Add frontend unit tests
- [ ] Add E2E tests
- [ ] Manual testing

### Phase 5: Documentation
- [ ] Update README if needed
- [ ] Add proto documentation
- [ ] Update .context docs
```

## Example: Add Coupon Feature

### Feature Description
Allow users to manage discount coupons for their affiliate products.

### Task Breakdown

#### 1. Proto (1 task)
```
- [ ] Define CouponsService in proto/coupons/v1/coupons.proto
      - ListCoupons, GetCoupon, CreateCoupon, UpdateCoupon, DeleteCoupon
      - Coupon message with id, store, code, description, active, dates
```

#### 2. Backend (5 tasks)
```
- [ ] Create Coupon model in internal/models/coupon.go
- [ ] Implement CouponRepository in internal/repository/postgres/coupon.go
- [ ] Create CouponService in internal/services/coupon/service.go
- [ ] Create CouponsHandler in internal/connectrpc/coupons_handler.go
- [ ] Add unit tests for service layer
```

#### 3. Frontend (6 tasks)
```
- [ ] Create useCouponsConnect hook in hooks/use-coupons-connect.ts
- [ ] Create CouponsPage in pages/coupons.tsx
- [ ] Build CouponCard component
- [ ] Build CouponForm component (create/edit)
- [ ] Add route to router
- [ ] Add i18n translations
```

#### 4. Testing (3 tasks)
```
- [ ] Create E2E page object: e2e/pages/coupons.page.ts
- [ ] Create E2E fixtures: e2e/fixtures/coupons.fixture.ts
- [ ] Write E2E tests: e2e/specs/coupons.spec.ts
```

#### 5. Polish (2 tasks)
```
- [ ] Add to navigation menu
- [ ] Add empty state and loading states
```

## Task Template

```markdown
### Task: [Task Title]

**Description:**
[What needs to be done]

**Files to Create/Modify:**
- `path/to/file1.ts` - [What to do]
- `path/to/file2.go` - [What to do]

**Acceptance Criteria:**
- [ ] Criterion 1
- [ ] Criterion 2

**Dependencies:**
- Depends on: [Other task]
- Blocks: [Other task]

**Notes:**
[Any additional context]
```

## Estimation Guidelines

| Task Type | Typical Size |
|-----------|--------------|
| Proto definition | Small |
| Model + Repository | Small-Medium |
| Service implementation | Medium |
| RPC Handler | Small |
| React Hook | Medium |
| UI Components | Medium-Large |
| E2E Tests | Medium |
| Unit Tests | Small |

## Checklist

- [ ] Feature clearly understood
- [ ] All layers identified
- [ ] Tasks are atomic (single responsibility)
- [ ] Dependencies mapped
- [ ] Acceptance criteria defined
- [ ] Testing tasks included
- [ ] Documentation tasks included
