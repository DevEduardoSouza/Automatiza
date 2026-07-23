# Anonymized production-scale staging restore runbook

Status: no compliant snapshot or staging target is configured in this repository. This runbook defines the human gate; it does not claim SAFE-014 operational evidence until a completed change record links the artifacts below.

## Ownership and isolation

A human data owner exports the approved snapshot and records source timestamp, recovery point, checksum, retention, and deletion owner without exposing contents. Restore only to an isolated account/network with separate PostgreSQL, Redis, object storage, queues, secrets, and observability.

Production endpoints and credentials must be absent. Outbound WAHA/API/email/webhook traffic is denied by network policy and application kill switches. WAHA validation uses a disposable test session/shard or fake, never a customer's session.

## Anonymization gate

Anonymization is deterministic where referential joins must survive and irreversible for direct identifiers and message/content fields. The data owner reviews the table/column inventory, tenant isolation, free-text fields, object keys, logs, large objects, and backups of the staging environment. Agents do not read or copy the snapshot.

Validate with aggregate-only reports:

- no production credentials, tokens, cookies, encryption keys, phone numbers, emails, message bodies, or customer media remain;
- primary/foreign-key and tenant ownership invariants hold;
- row-count and size distributions remain realistic enough for query/DDL behavior;
- the anonymized artifact has a short retention and a named deletion owner.

Any failed scan destroys the staging copy through the approved human process and restarts from export/anonymization.

## Restore and validation sequence

1. Human operator confirms backup/restore prerequisites, target capacity, checksums, and isolation.
2. Restore the anonymized PostgreSQL snapshot and required non-customer object fixtures.
3. Start empty, staging-only Redis/Asynq state; never restore production queue commands for execution.
4. Deploy application version N-1 with all external effects disabled.
5. Capture baseline volume, query latency, queue behavior, locks, disk, and WAL.
6. Apply expand migrations with the production runner and reviewed lock/statement budgets.
7. Run batched backfills with checkpoints; interrupt and resume one batch.
8. Run version N against the expanded schema, then roll the application back to N-1 without rolling back schema.
9. Exercise reconciliation with synthetic `unknown` outcomes and crash between fake external effect and persistence/ACK.
10. Validate concurrent DDL/repack only under its own runbook, capacity proof, and human window.
11. Record before/after aggregate metrics, failures, traces without PII, and abort decisions.
12. Human owner deletes the staging snapshot/environment at retention expiry and attaches evidence.

## Acceptance evidence

- Snapshot timestamp/checksum and anonymization report (no raw values).
- Environment isolation and outbound-deny proof.
- N-1 → expand → N → N-1 compatibility result.
- Batch pause/resume and crash/restart results.
- Queue/reconciliation result with no real external effect.
- Rows/size distribution, duration, locks, blocks, WAL, disk, and backup/replica metrics.
- Named owners, approval window, cleanup date, and independent audit.

Until all evidence is attached, report staging validation as **not run / blocked**, not passed.

