---
type: doc
name: glossary
description: Project terminology, type definitions, domain entities, and business rules
category: glossary
generated: 2026-02-18
status: filled
scaffoldVersion: "2.0.0"
---

# Glossary and Domain Concepts

This document defines the business terminology, type definitions, enumerations, personas, and domain rules used throughout the Automatiza platform. It serves as the canonical reference for AI agents and developers working with the codebase.

## Type Definitions

Key domain types used across the backend models and Protocol Buffer definitions:

| Type | Description | Key Fields |
|------|-------------|------------|
| `Automation` | A scheduled message dispatch configuration targeting WhatsApp groups | `id`, `user_id`, `name`, `instance_id`, `status`, `schedule`, `distribution_strategy`, `include_image` |
| `Group` | A WhatsApp group entity synced from a WAHA instance | `id`, `user_id`, `instance_id`, `jid` (WhatsApp JID), `name`, `participants_count` |
| `DispatchLog` | A record of a message sent (or attempted) to a specific group | `id`, `automation_id`, `group_id` (TEXT, WhatsApp JID), `status`, `error`, `dispatched_at` |
| `DispatchText` | A reusable message template for automations | `id`, `user_id`, `body` (message content), `is_active` (enabled/disabled toggle) |
| `ActionHistory` | An audit log entry recording user or system actions | `id`, `user_id`, `action`, `entity_type`, `entity_id`, `metadata`, `created_at` |
| `WebhookEvent` | An inbound webhook event from external platforms (Kiwify) | `event_id` (VARCHAR(128)), `event_type`, `payload`, `processed`, `created_at` |
| `WebhookLog` | A processing log for webhook events with status tracking | `id`, `event_id`, `status`, `response`, `created_at` |
| `Plan` | A subscription plan defining usage limits and features | `id`, `name`, `slug`, `max_automations`, `max_groups`, `max_instances`, `price` |
| `Instance` | A WhatsApp session managed through WAHA | `id`, `user_id`, `name`, `status`, `phone_number`, `waha_instance_id` |
| `SyncJob` | A background task for synchronizing WhatsApp groups from a WAHA instance | `id`, `type`, `status`, `progress`, `total`, `error` |
| `Job` | A generic background job with retry support | `id`, `type`, `status`, `payload`, `max_retries`, `attempts`, `scheduled_at` |
| `RedirectLink` | A smart redirect URL with distribution strategy for affiliate marketing | `id`, `user_id`, `slug`, `destination_urls`, `distribution_strategy`, `click_count` |
| `SavedRedirectLink` | A bookmarked or saved redirect link configuration | `id`, `user_id`, `redirect_link_id`, `name` |
| `MLLinkList` | A collection of Mercado Livre affiliate links | `id`, `user_id`, `name`, `category_id` |
| `MLLinkListItem` | An individual Mercado Livre affiliate link within a list | `id`, `list_id`, `product_url`, `status` |
| `AffiliateCredential` | Stored credentials for affiliate platforms (Shopee) | `id`, `user_id`, `platform`, `credentials` (encrypted) |
| `Category` | A system-wide category for organizing content | `id`, `name`, `slug`, `icon`, `display_order` |
| `Contact` | A WhatsApp contact associated with a user account | `id`, `user_id`, `phone`, `name`, `jid` |
| `Coupon` | A discount coupon for subscription plans | `id`, `code`, `discount_type`, `discount_value`, `usage_limit` |
| `User` | A platform user account | `id`, `email`, `name`, `password_hash`, `plan_id`, `is_active` |
| `UserRole` | A role assignment for a user (supports multiple roles per user) | `user_id`, `role`, UNIQUE(user_id, role) |

## Enumerations

Key enums defined in Protocol Buffer contracts and used throughout the system:

### DistributionStrategy

Controls how redirect links distribute traffic across destination URLs.

| Value | Description |
|-------|-------------|
| `ROUND_ROBIN` | Cycles through destination URLs sequentially |
| `RANDOM` | Selects a random destination URL for each click |
| `WEIGHTED` | Distributes traffic based on configured weights per URL |

### InstanceStatus

Represents the connection state of a WhatsApp session (WAHA instance).

| Value | Description |
|-------|-------------|
| `CONNECTED` | Active session, authenticated and ready to send/receive |
| `DISCONNECTED` | Session exists but is not currently connected |
| `QR_CODE` | Awaiting QR code scan for authentication |
| `STARTING` | Instance is initializing |
| `STOPPED` | Instance has been manually stopped |
| `FAILED` | Instance encountered an unrecoverable error |

### EventType (Webhook)

Types of webhook events received from external platforms (primarily Kiwify).

| Value | Description |
|-------|-------------|
| `PURCHASE_APPROVED` | Payment confirmed, grant subscription access |
| `PURCHASE_REFUNDED` | Refund processed, revoke or downgrade access |
| `SUBSCRIPTION_CANCELED` | Subscription cancelled by user or platform |
| `SUBSCRIPTION_RENEWED` | Recurring payment successful, extend access |
| `PURCHASE_CHARGEBACK` | Payment disputed, handle access accordingly |

### DispatchLogStatus

Status of an individual message dispatch attempt.

| Value | Description |
|-------|-------------|
| `PENDING` | Queued for dispatch, not yet attempted |
| `SENT` | Message successfully delivered to WAHA for sending |
| `FAILED` | Dispatch attempt failed (error recorded) |
| `SKIPPED` | Dispatch intentionally skipped (group inactive, duplicate, etc.) |

### AutomationStatus

State of an automation configuration.

| Value | Description |
|-------|-------------|
| `ACTIVE` | Automation is running on its configured schedule |
| `PAUSED` | Automation exists but is not currently executing |
| `DRAFT` | Automation is being configured, not yet activated |

### SyncJobStatus

State of a group synchronization background job.

| Value | Description |
|-------|-------------|
| `PENDING` | Job created, waiting to be picked up by a worker |
| `RUNNING` | Job is actively synchronizing groups |
| `COMPLETED` | Synchronization finished successfully |
| `FAILED` | Synchronization encountered an error |

### UserRole

Roles available in the system (composite UNIQUE constraint on user_id + role allows multiple roles per user).

| Value | Description |
|-------|-------------|
| `admin` | Platform administrator with full access to all features and users |
| `user` | Standard automation operator with access to their own resources |

## Core Terms

### Automation
A scheduled message dispatch configuration. An automation defines which groups receive messages, on what schedule, using which dispatch texts, and through which WhatsApp instance. Automations are the primary business entity of the platform.

### Dispatch
The act of sending a message to one or more WhatsApp groups through a WAHA instance. Each dispatch creates one `DispatchLog` entry per target group, tracking success or failure. Dispatches are executed by background workers based on automation schedules.

### Group
A WhatsApp group entity synced from a WAHA instance. Groups are identified by their JID (Jabber ID), which is a text-format WhatsApp identifier (not a UUID). Groups are periodically synchronized from WAHA to keep metadata (name, participant count) current.

### Instance
A WhatsApp session managed through WAHA (WhatsApp HTTP API). Each instance represents one authenticated WhatsApp connection, identified by a phone number. Instances handle QR code authentication, connection monitoring, and message sending.

### ML Link
A Mercado Livre affiliate link managed within the platform. ML Links are organized into lists (`MLLinkList`) containing individual items (`MLLinkListItem`). Used for affiliate marketing campaigns where users share product links through WhatsApp groups.

### Redirect Link
A smart redirect URL that distributes incoming clicks across multiple destination URLs using a configurable distribution strategy (round-robin, random, or weighted). Used for A/B testing affiliate links and optimizing click-through distribution.

### Webhook
An inbound HTTP notification from an external platform, primarily Kiwify (payment/subscription platform). Webhooks trigger subscription lifecycle events (purchase, refund, cancellation, renewal, chargeback). The webhook receiver validates HMAC signatures, acknowledges quickly, and enqueues the payload for asynchronous processing.

### Action History
A comprehensive audit log that records user and system actions across the platform. Each entry captures the action type, the entity affected, the actor, and metadata. Used for compliance, debugging, and administrative investigation.

### Sync Job
A background task that synchronizes WhatsApp groups from a WAHA instance into the local database. Sync jobs track progress (processed/total), handle failures gracefully, and run during startup (reconciliation/backfill) and on-demand via the UI.

### Dispatch Text
A reusable message template that can be assigned to automations. Dispatch texts have a `body` field (not "content") and an `is_active` flag (not "is_default") controlling whether the template is available for selection. Users manage a library of dispatch texts and assign them to their automations.

### Category
A system-wide organizational entity used to classify content (automation templates, ML link lists, etc.). Categories have a slug, icon, and display order. They are global (not per-user).

### Affiliate Credential
Stored credentials for external affiliate platforms (primarily Shopee). Credentials are encrypted at rest and used by the backend to authenticate API calls to affiliate program endpoints.

### Plan
A subscription tier that defines usage limits (max automations, groups, instances) and pricing. Plans are assigned to users and enforced by the backend. Plan changes are triggered by Kiwify webhook events.

### Coupon
A discount code that can be applied to subscription plans. Coupons have configurable discount types (percentage, fixed), values, usage limits, and expiration dates.

## Acronyms

| Acronym | Full Form | Context |
|---------|-----------|---------|
| WAHA | WhatsApp HTTP API | The WhatsApp bridge service (WAHA Plus with GoWS engine) that provides HTTP/WebSocket access to WhatsApp Web |
| FSD | Feature-Sliced Design | Frontend architecture methodology with strict layer hierarchy: App > Processes > Pages > Widgets > Features > Entities > Shared |
| RPC | Remote Procedure Call | Communication protocol; the project uses ConnectRPC (gRPC-Web compatible) for frontend-backend communication |
| JID | Jabber ID | WhatsApp group/contact identifier format (e.g., `120363123456789@g.us` for groups). Stored as TEXT, not UUID |
| ML | Mercado Livre | Brazilian e-commerce marketplace; the platform manages affiliate links for Mercado Livre products |
| CSP | Content Security Policy | HTTP security header; the project uses strict CSP with per-request nonce generation and `strict-dynamic` |
| APM | Application Performance Monitoring | Datadog APM is used for distributed tracing and performance metrics |
| DLQ | Dead Letter Queue | Queue for messages that failed processing after all retry attempts; prevents poison messages from blocking the main queue |
| PII | Personally Identifiable Information | Sensitive user data that must be redacted from logs and traces before export |
| BFF | Backend for Frontend | Pattern where the backend aggregates data from multiple sources into frontend-optimized responses |
| ADR | Architecture Decision Record | Documentation of significant architectural decisions and their rationale |
| HMAC | Hash-based Message Authentication Code | Used to validate webhook payload authenticity (Kiwify, WAHA) |
| GoWS | Go WebSocket | The WebSocket engine used by WAHA Plus for WhatsApp Web protocol communication |
| CSPRNG | Cryptographically Secure Pseudo-Random Number Generator | Used for generating session identifiers and nonce values |

## Personas

### Admin
The platform owner/operator with full administrative access. Admins can:
- Manage all users and their subscriptions
- View platform-wide analytics and metrics
- Investigate user issues via the admin panel
- Configure system-wide settings (categories, plans, coupons)
- Monitor WAHA instance health and system status
- Access action history for audit and compliance purposes

### User
A standard automation operator who subscribes to the platform. Users can:
- Create and manage automations (within plan limits)
- Connect WhatsApp instances and manage groups
- Configure dispatch texts (message templates)
- Set up redirect links and ML links for affiliate marketing
- View their own dashboard, dispatch history, and analytics
- Manage their profile and subscription

## Domain Rules

These are critical schema and business rules that must be respected across the codebase:

1. **`dispatch_logs.group_id` is TEXT (WhatsApp JID), NOT a UUID foreign key.** WhatsApp groups are identified by their JID string format, and dispatch logs reference groups by this identifier directly.

2. **`user_roles` has a composite UNIQUE constraint on `(user_id, role)`.** This allows a single user to hold multiple roles simultaneously (e.g., both `admin` and `user`). Seed data and upsert operations must use `ON CONFLICT (user_id, role)`.

3. **`dispatch_texts` uses `body` (NOT `content`) and `is_active` (NOT `is_default`).** These field names are authoritative; any migration or code referencing `content` or `is_default` for dispatch texts is incorrect.

4. **`webhook_events.event_id` is VARCHAR(128), NOT VARCHAR(26).** Event IDs from external platforms (Kiwify) can be longer than ULID format.

5. **`categories` are system-wide (global), NOT per-user.** Categories have `slug`, `icon`, and `display_order` fields and are shared across all users.

6. **`ml_link_list_items` uses `product_url`/`status` schema**, not `origin_url`/`order_index`. The canonical schema is defined in migration `000`.

7. **`sync_jobs` uses `(type, status, progress, total, error)` fields**, not `(job_id, state, synced_groups)`. The canonical schema is defined in migration `000`.

8. **`webhook_logs` does NOT have `email TEXT` or `plan VARCHAR(50)` columns.** These were removed from the schema.

9. **Hard delete policy**: The project uses hard deletes (no soft-delete pattern). Archival strategies are implemented separately when audit trails are needed (via `action_history`).

10. **Migration ordering**: `init.sql` handles extensions and roles only. Migration `000_postgresql_init.sql` creates all 33 tables. Migrations `001-029` are incremental and idempotent after `000`.

11. **Consumer idempotency**: All message consumers must be idempotent by `payload.id` (or derived `idempotencyKey`). Duplicate events must be accepted without side effects.

12. **Webhook receivers must ACK fast**: No business logic in the receiver handler. Validate HMAC, enqueue to broker, return 200. Processing happens asynchronously.
