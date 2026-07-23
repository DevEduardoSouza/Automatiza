# Online migrations, backfills, cleanup, and index runbook

This is a review checklist, not authorization to run production SQL. A named human operator executes reviewed commands in an approved window.

## Schema lifecycle

### Expand

- Add nullable columns, new tables, or compatible enum/constraint states.
- Keep both N-1 and N application versions functional.
- Make writes/readers tolerant during mixed-version operation.
- For online DDL that can wait on a relation lock, set a short `lock_timeout` and an operation-specific `statement_timeout`. A timeout aborts the attempt; it is not a reason to retry blindly.
- Create indexes on hot tables with `CREATE INDEX CONCURRENTLY`, outside an explicit transaction. Handle an interrupted invalid index explicitly.

### Migrate

- Backfill in small bounded batches ordered by a stable key.
- Persist a checkpoint outside the batch transaction.
- Support dry-run, pause, resume, rate reduction, and safe restart.
- Report rows scanned/changed, duration, blocks read/hit/dirtied, WAL generated, errors, and checkpoint.
- Re-read ownership for multi-tenant rows and never move data across tenants.

### Observe

- Run both code versions against the expanded schema in isolated staging.
- Compare counts/invariants, error rate, p95, locks, WAL, replica/backup lag, and reconciliation drift for the declared window.
- Prove the old field/path is no longer read or written before contraction.

### Contract

- Use a separate change record and rollout.
- Only after the observation window, backup/restore proof, and explicit human approval.
- Never make contraction part of application rollback.

## Prohibited migration patterns

Do not add `DROP COLUMN`, destructive type changes, or immediate `SET NOT NULL` on a large/hot table. Do not run blocking `VACUUM FULL`, `CLUSTER`, or `REINDEX` on a hot table. Prefer a new column plus dual read/write, batched backfill, validated constraints, concurrent index operations, or `pg_repack` only with its own prerequisites and window.

A safe `NOT NULL` transition is: add a `CHECK (... IS NOT NULL) NOT VALID`, backfill, validate the constraint, observe, then perform contraction in a later rollout after lock impact is proven.

## DDL preflight checklist

- [ ] Table size, write rate, long transactions, lock graph, disk headroom, WAL budget, and replica/backup lag measured.
- [ ] N-1 application compatibility tested.
- [ ] `lock_timeout` is short and the operation aborts cleanly when it cannot acquire a lock.
- [ ] Concurrent DDL is not wrapped in a transaction.
- [ ] Invalid/interrupted index recovery is defined.
- [ ] Canary target and abort thresholds are in the change record.
- [ ] Rollback is application/flag based and accepts the expanded schema.

## Backfill contract

The implementation must accept explicit batch size and maximum runtime, checkpoint after each committed batch, stop on a pause signal, and resume from the checkpoint without reapplying effects. A single transaction over the full dataset is prohibited. Concurrent workers need a documented partition/lock strategy and a race/restart test.

Success evidence includes zero invariant violations and stable resource budgets. A transient failure test must show that the last committed checkpoint is resumed. A crash test must show that a partially processed batch is safe to replay.

## Cleanup and I/O budget

Every cleanup has dry-run, per-run row limit, maximum duration, and explicit I/O/WAL budgets. Record rows scanned/deleted, blocks, WAL, duration, and remaining candidates. Stop when any budget is reached. Deletion or lifecycle changes require logical recovery or a verified restore path.

## Index removal

Never remove data or an index based only on size or `idx_scan=0`. Record statistics reset time and observe through a representative seasonal window. Check query plans, constraints, application SQL, scheduled jobs, and replicas. Index removal is a separate schema-risk rollout with a rollback/recreate plan and disk/WAL budget.

