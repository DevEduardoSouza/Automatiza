<!-- Template-Version: 1.0 -->
# Change record: <WP or short title>

## Metadata

- Status: Draft
- Owner: <person or team>
- Date: YYYY-MM-DD
- Risk axis: <functional|schema|data-backfill|data-cleanup|dependency|infrastructure|security|governance>
- Related PR:
- Related ADR/runbook:
- Production operator:
- Approved window:

## Hypothesis

State the expected result and the evidence that would falsify it.

## Scope

### Files

- `<path>` — reason

### Tables

- None, or table/column/index and access pattern.

### External effects

- None, or API/queue/WAHA effect and idempotency/unknown-state handling.

## Baseline

Record timestamp, query/dashboard, sample window, and values. Do not include customer data or secrets.

| Metric | Before | Expected after | Source |
|---|---:|---:|---|
| `<metric>` | TBD | TBD | `<dashboard/query>` |

## Expand / migrate / observe / contract

- Expand:
- Migrate:
- Observe:
- Contract: <this PR or a later change record>
- N-1 compatibility proof:

## Canary

- Population:
- Duration:
- Promotion criteria:
- Kill switches:

## Abort thresholds

| Signal | Abort when | Observation window | Action |
|---|---|---|---|
| Error rate | +0.5 percentage point over baseline | 15 min | Disable flag and stop rollout |
| p95 latency | +20% over baseline | 15 min | Disable flag and stop rollout |
| Queue backlog | Sustained growth | 10 min | Pause producers/rollout |
| Database locks | Any lock wait >5 s caused by change | Immediate | Cancel operation |
| Duplicate external effects | >0 | Immediate | Stop effect and reconcile |

## Rollout

Number each operator action. Include prechecks, canary, observation windows, promotion, owner, and commands only after they have been verified against repository scripts.

## Rollback

Application rollback must work with the expanded schema. Describe flag reversal, image rollback, logical data recovery, reconciliation, and verification. Never require a schema rollback during the incident window.

## Evidence

### Commands and results

| Command | Environment | Result |
|---|---|---|
| `<command>` | local/staging | Not run |

### Metrics/logs

- Attach sanitized evidence with stable `error_code` and trace correlation.

### Approvals

- Plan/diff/tests:
- Production owner:
- Independent audit:

## Residual risks and blockers

- None, or evidence, impact, owner, and next safe action.

