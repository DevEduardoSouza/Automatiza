---
type: skill
name: API Design
description: Design ConnectRPC APIs with Protocol Buffers for Automatiza
skillSlug: api-design
phases: [P, R]
status: filled
---

# API Design

## When to Use

- Designing new API endpoints
- Adding fields to existing services
- Creating new proto services
- Planning API contracts

## Proto File Structure

```
proto/
├── {domain}/v1/
│   └── {domain}.proto
├── common/v1/
│   └── pagination.proto
└── buf.yaml
```

## Service Definition Template

```protobuf
syntax = "proto3";

package example.v1;

import "common/v1/pagination.proto";
import "google/protobuf/timestamp.proto";

option go_package = "automatiza-go/internal/gen/example/v1";

service ExampleService {
  // List examples with pagination
  rpc ListExamples(ListExamplesRequest) returns (ListExamplesResponse);

  // Get a single example by ID
  rpc GetExample(GetExampleRequest) returns (GetExampleResponse);

  // Create a new example
  rpc CreateExample(CreateExampleRequest) returns (CreateExampleResponse);

  // Update an existing example
  rpc UpdateExample(UpdateExampleRequest) returns (UpdateExampleResponse);

  // Delete an example
  rpc DeleteExample(DeleteExampleRequest) returns (DeleteExampleResponse);
}

// Domain model
message Example {
  string id = 1;
  string name = 2;
  string description = 3;
  ExampleStatus status = 4;
  google.protobuf.Timestamp created_at = 5;
  google.protobuf.Timestamp updated_at = 6;
}

enum ExampleStatus {
  EXAMPLE_STATUS_UNSPECIFIED = 0;
  EXAMPLE_STATUS_ACTIVE = 1;
  EXAMPLE_STATUS_INACTIVE = 2;
}

// Request/Response messages
message ListExamplesRequest {
  common.v1.Pagination pagination = 1;
  optional string filter = 2;
}

message ListExamplesResponse {
  repeated Example examples = 1;
  int32 total_count = 2;
}

message GetExampleRequest {
  string id = 1;
}

message GetExampleResponse {
  Example example = 1;
}

message CreateExampleRequest {
  string name = 1;
  optional string description = 2;
}

message CreateExampleResponse {
  Example example = 1;
}

message UpdateExampleRequest {
  string id = 1;
  optional string name = 2;
  optional string description = 3;
  optional ExampleStatus status = 4;
}

message UpdateExampleResponse {
  Example example = 1;
}

message DeleteExampleRequest {
  string id = 1;
}

message DeleteExampleResponse {}
```

## Naming Conventions

### Services
- PascalCase: `GroupsService`, `AutomationsService`
- Use plural for CRUD services

### Methods
- `List{Resource}s` - Get paginated list
- `Get{Resource}` - Get single by ID
- `Create{Resource}` - Create new
- `Update{Resource}` - Update existing
- `Delete{Resource}` - Delete by ID
- `{Action}{Resource}` - Custom actions (e.g., `SyncGroups`)

### Fields
- snake_case: `created_at`, `member_count`
- IDs: `{resource}_id` (e.g., `group_id`, `instance_id`)
- Timestamps: `created_at`, `updated_at`, `deleted_at`

### Enums
- SCREAMING_SNAKE_CASE values
- Prefix with type name: `GROUP_STATUS_ACTIVE`
- Always include `_UNSPECIFIED = 0`

## Design Guidelines

### 1. Pagination
```protobuf
message Pagination {
  int32 limit = 1;   // Max 500, default 50
  int32 offset = 2;  // Starting position
}
```

### 2. Timestamps
```protobuf
import "google/protobuf/timestamp.proto";

message Example {
  google.protobuf.Timestamp created_at = 5;
}
```

### 3. Optional Fields
```protobuf
// Use optional for nullable fields
optional string description = 2;

// Don't use optional for required fields
string id = 1;
```

### 4. Error Handling
Use standard gRPC codes:
- `INVALID_ARGUMENT` - Validation errors
- `NOT_FOUND` - Resource doesn't exist
- `ALREADY_EXISTS` - Duplicate creation
- `PERMISSION_DENIED` - Access denied
- `UNAUTHENTICATED` - Not logged in

### 5. Backward Compatibility
- Never reuse field numbers
- Never change field types
- Add new fields at the end
- Deprecate, don't remove

## Code Generation

```bash
# Generate from proto files
cd client
npx buf generate ../proto

# Files generated:
# - client/src/gen/{domain}/v1/{domain}_pb.ts
# - automatiza-go/internal/gen/{domain}/v1/*.go
```

## Implementation Checklist

1. [ ] Create proto file in `proto/{domain}/v1/`
2. [ ] Run code generation
3. [ ] Implement Go handler in `internal/connectrpc/`
4. [ ] Implement Go service in `internal/services/`
5. [ ] Create React hook in `client/src/hooks/`
6. [ ] Add tests for new endpoints
7. [ ] Update documentation
