# Production change and critical rollout runbook

Use this runbook with a copy of `docs/change-records/TEMPLATE.md`. It defines review and operator gates; it does not authorize an agent or CI job to operate production.

## 1. Classify and bound the change

Choose exactly one risk axis: functional, schema, data-backfill, data-cleanup, dependency, infrastructure, security, or governance. Split another axis into another PR and rollout. List affected files, tables, queues, external APIs, tenants, and persistent state.

Trace the applicable path before editing:

`UI → hook → RPC/REST/SSE → interceptor/middleware → handler → service → repository/outbox → task/worker → WAHA/external API → persistence → SSE/query`

For delivery-only work, trace `PR → CI → deploy → migration → drain → rollout → health/metrics → rollback`.

## 2. Establish baseline and ownership

Record a time-bounded baseline and its source. At minimum consider error rate, p95, backlog age/depth, database lock waits, replication/backup lag, external-effect duplicates, and reconciliation drift. Name the change owner, production operator, approver, and observation owner.

Logs must carry a stable low-cardinality `error_code` and trace correlation. Never put email, phone, message content, tokens, session identifiers, user IDs, automation IDs, or unbounded values in metric labels. Put sensitive identifiers only in approved, access-controlled stores when operationally necessary.

## 3. Feature flag and kill-switch convention

New behavior ships dark by default. A feature flag selects behavior; a kill switch stops an effect. Do not overload one value with both meanings.

For any queue, send, billing, auth, data mutation, or other external effect, implement all four stop scopes:

| Scope | Canonical configuration suffix | Required behavior |
|---|---|---|
| Global | `_ENABLED` | `false` blocks every effect |
| User | `_BLOCKED_USER_IDS` | membership blocks that user's effect |
| Automation | `_BLOCKED_AUTOMATION_IDS` | membership blocks that automation |
| Session | `_BLOCKED_SESSION_IDS` | membership blocks that WAHA/session path |

Use an uppercase domain prefix, for example `MESSAGE_SEND_ENABLED`. Any matching block wins. Evaluate the switches again at the last boundary before the external call or durable enqueue that authorizes it; a UI or scheduler check alone is insufficient. Missing or malformed kill-switch configuration must use the safe documented default.

Emit `error_code=kill_switch_blocked` and low-cardinality fields `effect`, `switch`, and `scope_type`; do not emit the blocked identifier as a metric label. A runtime implementation must test all scopes, configuration failure, concurrent evaluation/reload if supported, and restart defaults. WP-000 defines the convention but does not add domain switches.

## 4. Canary and promotion

Critical changes need a canary that can be discarded. For WAHA, first use a dedicated test shard/session and synthetic account; never use a real customer as the first canary. Keep queue names, Redis keys, and database targets isolated from production where applicable.

Promote one step at a time with a fixed observation window. Default abort examples, to be replaced by a measured baseline where stricter:

- error rate increases by 0.5 percentage point;
- p95 increases by 20%;
- backlog grows for 10 minutes or oldest-job age breaches its SLO;
- a change causes a database lock wait over 5 seconds;
- replication or backup falls outside its established limit;
- duplicate external effects are greater than zero;
- reconciliation drift or unknown outcomes grow.

On any threshold: stop promotion, activate the narrowest safe kill switch (global if effect integrity is uncertain), preserve evidence, and reconcile unknown outcomes before retry.

## 5. Human execution gate

Before merge, humans approve the plan, complete diff, tests, canary, abort thresholds, and rollback. Before production execution, the named operator confirms backup/restore readiness, capacity/lock/I/O budgets, observability, the exact reviewed commands, and the approved window.

Agents and CI must not run production DDL, data cleanup, repack, S3 lifecycle/delete, Redis destructive commands, WAHA session deletion/logout, snapshot export, or restore.

## 6. Rollback and reconciliation

Prefer flag reversal and application image rollback. The previous image must accept the expanded schema and payload. Do not drop expanded fields during an incident. For an external effect with a timeout, record `unknown`; never blindly retry. Reconcile against the persistent ledger and provider state before deciding whether a retry is safe.

Capture before/after metrics and three independent post-change angles when production was mutated: disappearance of the failure, successful path, and persistence through the observation window.

## 7. Handoff

Give the independent auditor only the commit/diff, change record, migrations, executed commands/results, before/after metrics, rollout/rollback, and blockers. The audit starts from the checklist and code in a fresh read-only session.

