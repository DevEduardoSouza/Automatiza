# Admin Hooks Migration Status

This document tracks the migration status of admin hooks from Supabase to ConnectRPC.

## Migration Date: 2026-01-31

## Summary

| Hook | Status | Notes |
|------|--------|-------|
| `use-users-tab.ts` | Partial | Core profile operations migrated, admin role and bulk operations pending |
| `use-plans-tab.ts` | Blocked | Requires PlansService proto and backend |
| `use-email-templates-tab.ts` | Blocked | Requires EmailTemplatesService proto and backend |
| `use-team-tab.ts` | Complete | Connected to AdminService (ListAdminUsers, GrantAdminRole, RevokeAdminRole) |

---

## use-users-tab.ts

### Migrated Operations

| Operation | Old Implementation | New Implementation |
|-----------|-------------------|-------------------|
| List profiles | `supabase.from('profiles').select(...)` | `useQuery(listProfiles, ...)` |
| Search profiles | `supabase.from('profiles').select(...).or(...)` | `useQuery(searchProfiles, ...)` |
| Block user | `supabase.from('profiles').update({billing_status: 'suspended', ...})` | `blockUser` mutation via `ProfilesService.BlockUser` |
| Unblock user | `supabase.from('profiles').update({billing_status: null, ...})` | `unblockUser` mutation via `ProfilesService.UnblockUser` |
| Update max_automations | `supabase.from('profiles').update({max_automations: ...})` | `updateUserPlan` mutation via `ProfilesService.UpdateUserPlan` |
| Fetch system events | Supabase | Already uses `useAdminConnect().fetchSystemEvents` |
| Reset password | Supabase auth | Already uses `useAdminConnect().resetUserPassword` |
| Load balancer operations | N/A | Already uses `useLoadBalancerConnect()` |

### Pending Operations (Require Backend Implementation)

| Operation | Required Proto/RPC | Description |
|-----------|-------------------|-------------|
| Create user | `AdminService.CreateUser` | Admin user creation with email/password/plan |
| List admin roles | `AdminService.ListAdminUsers` or `ProfilesService.ListAdminRoles` | Fetch all admin user IDs |
| Grant admin role | `AdminService.GrantAdminRole` | Make a user an admin |
| Revoke admin role | `AdminService.RevokeAdminRole` | Remove admin privileges |
| List user instances | `InstancesService.ListInstancesByUserId` | Admin endpoint to list instances for any user |
| List user automations | `AutomationsService.ListAutomationsByUserId` | Admin endpoint to list automations for any user |
| Bulk pause automations | `AutomationsService.BulkUpdateStatus` | Pause all active automations for a user |
| Bulk activate automations | `AutomationsService.BulkUpdateStatus` | Activate all paused automations for a user |
| Bulk reschedule next_run | `AutomationsService.BulkReschedule` | Spread next_run_at across automations |

---

## use-plans-tab.ts

### Status: BLOCKED

No proto definitions exist for plans management.

### Required Proto Definition

File: `proto/plans/v1/plans.proto`

```protobuf
syntax = "proto3";

package plans.v1;

option go_package = "github.com/automatiza/proto/plans/v1;plansv1";

message Plan {
  string id = 1;
  string name = 2;
  string slug = 3;
  optional string description = 4;
  optional double price_monthly = 5;
  optional double price_semester = 6;
  optional double price_annual = 7;
  int32 group_limit = 8;
  int64 storage_limit = 9;
  repeated string features = 10;
  optional string kiwify_product_id_monthly = 11;
  optional string kiwify_product_id_semester = 12;
  optional string kiwify_product_id_annual = 13;
  bool is_active = 14;
  string created_at = 15;
  string updated_at = 16;
}

message ListPlansRequest {}
message ListPlansResponse {
  repeated Plan plans = 1;
}

message CreatePlanRequest {
  string name = 1;
  string slug = 2;
  optional string description = 3;
  optional double price_monthly = 4;
  optional double price_semester = 5;
  optional double price_annual = 6;
  int32 group_limit = 7;
  int64 storage_limit = 8;
  repeated string features = 9;
  optional string kiwify_product_id_monthly = 10;
  optional string kiwify_product_id_semester = 11;
  optional string kiwify_product_id_annual = 12;
}
message CreatePlanResponse {
  Plan plan = 1;
}

message UpdatePlanRequest {
  string id = 1;
  // ... same fields as CreatePlanRequest
}
message UpdatePlanResponse {
  Plan plan = 1;
}

message DeletePlanRequest {
  string id = 1;
}
message DeletePlanResponse {}

service PlansService {
  rpc ListPlans(ListPlansRequest) returns (ListPlansResponse);
  rpc CreatePlan(CreatePlanRequest) returns (CreatePlanResponse);
  rpc UpdatePlan(UpdatePlanRequest) returns (UpdatePlanResponse);
  rpc DeletePlan(DeletePlanRequest) returns (DeletePlanResponse);
}
```

---

## use-email-templates-tab.ts

### Status: BLOCKED

No proto definitions exist for email templates management.

### Required Proto Definition

File: `proto/email_templates/v1/email_templates.proto`

```protobuf
syntax = "proto3";

package email_templates.v1;

option go_package = "github.com/automatiza/proto/email_templates/v1;emailtemplatesv1";

message EmailTemplate {
  string id = 1;
  string name = 2;
  string subject = 3;
  string content = 4;
  optional string description = 5;
  bool is_active = 6;
  string created_at = 7;
  string updated_at = 8;
}

message ListEmailTemplatesRequest {}
message ListEmailTemplatesResponse {
  repeated EmailTemplate templates = 1;
}

message CreateEmailTemplateRequest {
  string name = 1;
  string subject = 2;
  string content = 3;
  optional string description = 4;
  bool is_active = 5;
}
message CreateEmailTemplateResponse {
  EmailTemplate template = 1;
}

message UpdateEmailTemplateRequest {
  string id = 1;
  string name = 2;
  string subject = 3;
  string content = 4;
  optional string description = 5;
  bool is_active = 6;
}
message UpdateEmailTemplateResponse {
  EmailTemplate template = 1;
}

message ToggleEmailTemplateActiveRequest {
  string id = 1;
}
message ToggleEmailTemplateActiveResponse {
  EmailTemplate template = 1;
}

service EmailTemplatesService {
  rpc ListEmailTemplates(ListEmailTemplatesRequest) returns (ListEmailTemplatesResponse);
  rpc CreateEmailTemplate(CreateEmailTemplateRequest) returns (CreateEmailTemplateResponse);
  rpc UpdateEmailTemplate(UpdateEmailTemplateRequest) returns (UpdateEmailTemplateResponse);
  rpc ToggleEmailTemplateActive(ToggleEmailTemplateActiveRequest) returns (ToggleEmailTemplateActiveResponse);
}
```

---

## use-team-tab.ts

### Status: COMPLETE

Connected to `AdminService` via ConnectRPC. Uses `ListAdminUsers`, `GrantAdminRole`, and `RevokeAdminRole` RPCs. User lookup by email uses `ProfilesService.SearchProfiles`.

---

## Next Steps

1. **Backend Team**: Implement the missing proto definitions and services
2. **Frontend Team**: Once backend is ready, update the hooks to use ConnectRPC
3. **Testing**: Verify all admin operations work correctly with the new backend

## Notes

- The `searchProfiles` RPC from ProfilesService can be used to find users by email when adding an admin
- All mutations should invalidate relevant query caches using `queryClient.invalidateQueries()`
- Error handling uses `getErrorInfo()` from `@/lib/connect-transport` for consistent error messages
