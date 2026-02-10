---
type: skill
name: Documentation
description: Generate and update technical documentation for Automatiza
skillSlug: documentation
phases: [P, C]
status: filled
---

# Documentation

## When to Use

- Documenting new features
- Updating README files
- Creating API documentation
- Writing technical guides

## Documentation Locations

| Type | Location |
|------|----------|
| Main README | `/README.md` |
| AI Context | `/.context/` |
| API (Proto) | `/proto/*/v1/*.proto` |
| Frontend | `/client/README.md` |
| Backend | `/automatiza-go/README.md` |

## README Template

```markdown
# Feature Name

Brief description of what this feature does.

## Overview

More detailed explanation of the feature, its purpose, and when to use it.

## Usage

### Basic Example
```typescript
// Code example
```

### Advanced Usage
```typescript
// More complex example
```

## API Reference

### Methods

#### `methodName(params): ReturnType`

Description of what the method does.

**Parameters:**
- `param1` (string): Description
- `param2` (number, optional): Description

**Returns:** Description of return value

**Example:**
```typescript
const result = methodName('value', 42);
```

## Configuration

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `option1` | string | `''` | Description |
| `option2` | number | `10` | Description |

## Troubleshooting

### Common Issue 1
**Problem:** Description
**Solution:** Steps to fix

## See Also

- [Related Feature](./related.md)
- [API Documentation](../api/feature.md)
```

## Proto Documentation

```protobuf
// ExampleService handles example-related operations.
//
// Use this service to manage examples in the system.
// All methods require authentication.
service ExampleService {
  // ListExamples returns a paginated list of examples.
  //
  // Supports filtering by status and searching by name.
  // Results are ordered by created_at descending.
  rpc ListExamples(ListExamplesRequest) returns (ListExamplesResponse);

  // GetExample retrieves a single example by ID.
  //
  // Returns NOT_FOUND if the example doesn't exist.
  rpc GetExample(GetExampleRequest) returns (GetExampleResponse);
}

// Example represents a single example entity.
message Example {
  // Unique identifier (UUID format)
  string id = 1;

  // Display name (1-100 characters)
  string name = 2;

  // Optional description (max 1000 characters)
  optional string description = 3;

  // Current status of the example
  ExampleStatus status = 4;

  // When the example was created
  google.protobuf.Timestamp created_at = 5;
}
```

## Code Comments

### TypeScript/React
```typescript
/**
 * useGroups hook for managing WhatsApp groups.
 *
 * Provides CRUD operations and messaging functionality
 * for WhatsApp groups via ConnectRPC.
 *
 * @example
 * ```tsx
 * const { groups, isLoading, createGroup } = useGroups();
 *
 * if (isLoading) return <Spinner />;
 *
 * return <GroupList groups={groups} />;
 * ```
 */
export function useGroups() {
  // Implementation
}
```

### Go
```go
// GroupService handles business logic for WhatsApp groups.
//
// It coordinates between the repository layer and external
// services (WAHA) to manage group operations.
type GroupService struct {
  repo     GroupRepository
  whatsapp WhatsAppProvider
  logger   *zap.Logger
}

// SyncGroups fetches the latest groups from WhatsApp
// and updates the local database.
//
// This operation may take several seconds depending on
// the number of groups. Consider running in background.
func (s *GroupService) SyncGroups(ctx context.Context, instanceID string) error {
  // Implementation
}
```

## Documentation Checklist

- [ ] Clear title and description
- [ ] Usage examples included
- [ ] API reference complete
- [ ] Configuration documented
- [ ] Troubleshooting section
- [ ] Links to related docs
- [ ] Code examples tested

## Writing Guidelines

1. **Be Concise**: Get to the point quickly
2. **Use Examples**: Show, don't just tell
3. **Keep Updated**: Update docs with code changes
4. **Use Tables**: For configuration and options
5. **Add Links**: Connect related documentation
