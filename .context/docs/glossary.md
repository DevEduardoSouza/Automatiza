#---
status: filled
---

# Glossary & Domain Concepts

This document defines key terminology, domain entities, and business rules used throughout the Automatiza platform.

## Type Definitions

Key exported types and interfaces:

- [`Automation`](../client/src/types/automation.ts:32) - Core automation workflow configuration
- [`AutomationGroup`](../client/src/types/automation.ts:65) - Group associated with an automation
- [`AutomationCoupon`](../client/src/types/automation.ts:82) - Coupon configuration for automations
- [`Group`](../client/src/types/group.ts:4) - WhatsApp group entity
- [`AvailableGroup`](../client/src/types/group.ts:19) - Group available for selection
- [`ActionHistory`](../client/src/types/action-history.ts:14) - Historical action record
- [`DispatchLog`](../client/src/types/dispatch-log.ts:10) - Message dispatch log entry
- [`AuthUser`](../client/src/contexts/auth-context.tsx:46) - Authenticated user context
- [`AffiliateCredential`](../client/src/gen/affiliates/v1/credentials_pb.ts:22) - Affiliate program credentials

## Enumerations

- [`AutomationMode`](../client/src/types/automation.ts:9) - `monitoring` | `dispatch` | `manual`
- [`AutomationStatus`](../client/src/types/automation.ts:12) - `active` | `paused` | `error`
- [`InstanceStatus`](../client/src/gen/instances/v1/instances_pb.ts:378) - WhatsApp instance connection state
- [`JobStatus`](../client/src/gen/jobs/v1/jobs_pb.ts:586) - Scheduled job execution status
- [`JobActionType`](../client/src/gen/jobs/v1/jobs_pb.ts:530) - Type of job action
- [`DiscountType`](../client/src/gen/coupons/v1/coupons_pb.ts:497) - Coupon discount type
- [`Store`](../client/src/gen/coupons/v1/coupons_pb.ts:526) - Affiliate store identifier
- [`AffiliateStore`](../client/src/gen/affiliates/v1/credentials_pb.ts:304) - Supported affiliate platforms
- [`DistributionStrategy`](../client/src/gen/redirect/v1/redirect_pb.ts:390) - Link distribution strategy

## Core Terms

### Automation
A scheduled or triggered workflow that performs actions on WhatsApp groups. Automations can run in different modes:
- **Monitoring**: Watches groups for specific text patterns
- **Dispatch**: Sends messages to groups on a schedule
- **Manual**: Triggered by user action

### Group
A WhatsApp group synchronized from a connected WhatsApp instance. Groups are the target for message dispatch and automation actions.

### Instance
A WhatsApp Web session managed by the Evolution API/WAHA. Each instance represents a connected WhatsApp account.

### Job
A scheduled task that runs at specified intervals. Jobs can dispatch messages, sync groups, or run automations.

### Dispatch Text
Pre-configured message templates used for bulk messaging across groups.

### ML Link / ML Link List
Mercado Livre affiliate product links organized in lists for rotation and distribution.

### Coupon
Discount codes associated with affiliate programs (Shopee, Mercado Livre) used in promotional messages.

### Campaign
A coordinated marketing effort with multiple messages and target groups.

## Acronyms & Abbreviations

- **ML**: Mercado Livre (Brazilian e-commerce platform)
- **SSE**: Server-Sent Events (real-time updates)
- **gRPC**: Google Remote Procedure Call
- **Proto/Protobuf**: Protocol Buffers (serialization format)
- **E2E**: End-to-End (testing)
- **WAHA**: WhatsApp HTTP API (alternative to Evolution API)

## Personas / Actors

### Platform User
A business owner or marketer who uses Automatiza to promote products through WhatsApp groups. Key workflows:
- Connect WhatsApp account
- Sync and manage groups
- Create automations for product promotion
- Configure affiliate program credentials
- Monitor dispatch history and engagement

### Admin User
Platform administrator with access to user management, system settings, and billing. Can:
- Manage user accounts and plans
- View system-wide analytics
- Configure platform settings
- Manage email templates and announcements

## Domain Rules & Invariants

1. **One Active Instance**: Each user can have one active WhatsApp instance at a time
2. **Group Sync Required**: Groups must be synced before they can be used in automations
3. **Automation Limits**: Users have limits on active automations based on their plan
4. **Message Rate Limits**: Dispatch respects rate limits to avoid WhatsApp blocks
5. **Coupon Uniqueness**: Coupon codes must be unique per store

See [Project Overview](./project-overview.md) for additional context.
