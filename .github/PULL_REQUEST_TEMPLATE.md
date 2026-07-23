## Change

- Change record: `docs/change-records/<date>-<slug>.md`
- Risk axis (choose exactly one): <!-- functional | schema | data-backfill | data-cleanup | dependency | infrastructure | security | governance -->
- Owner:
- Related ADR/runbook:

## Evidence

- Hypothesis and baseline:
- Focused tests:
- Broader checks:
- Tests not run and why:

## Safe rollout

- [ ] The change record contains affected files/tables, before/after metrics, canary, abort thresholds, rollout, rollback, owner, and evidence.
- [ ] This PR has one risk axis; functional changes, cleanup, and dependency upgrades are separate rollouts.
- [ ] The previous application version accepts any expanded schema/payload.
- [ ] New migrations use expand → migrate → observe → contract and avoid destructive/blocking DDL.
      <!-- GOV-SQL-003: toda migração nova com ALTER TABLE/CREATE INDEX deve abrir com
           SET lock_timeout = '5s';
           e criar índices com CREATE INDEX CONCURRENTLY (fora de BEGIN/COMMIT).
           Migrações antigas não são revalidadas; só arquivos alterados no push. -->
- [ ] Backfills/cleanup have dry-run, small batches, checkpoint, pause/resume, limits, and I/O/WAL budgets.
- [ ] External effects are blocked before the boundary by global, user, automation, and session kill switches.
- [ ] Critical changes start with a non-customer canary and have measurable abort thresholds.
- [ ] Logs/metrics use `error_code` and trace correlation without PII or high-cardinality labels.
- [ ] Production commands and destructive operations are assigned to a human operator in an approved window.

## Human gates

- [ ] Plan, diff, tests, rollout, and rollback reviewed.
- [ ] Production owner approved.
- [ ] Independent read-only audit attached for a critical rollout.

