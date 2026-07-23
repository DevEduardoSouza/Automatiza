# ADR-0001: Safe production change governance

- Status: Proposed
- Date: 2026-07-23
- Owners: Automatiza production owner
- Decision scope: all production-bound changes

## Context

The application deploys server, worker, frontend, router, and database migrations through one production workflow. PostgreSQL is persistent state; Redis carries queues and coordination; WAHA owns live WhatsApp connection state. The migration runner executes before the new application and supports autocommit operations such as `CREATE INDEX CONCURRENTLY`. Tests for both application stacks currently tolerate known failures, so a green workflow alone is not complete rollout evidence.

Production changes need a shared record that survives agent sessions and makes compatibility, external effects, canaries, abort thresholds, and rollback reviewable before merge.

## Decision

1. Every production-bound item has one versioned change record based on `docs/change-records/TEMPLATE.md`.
2. Each PR and rollout has exactly one primary risk axis: functional, schema, data-backfill, data-cleanup, dependency, infrastructure, security, or governance.
3. Schema and contract changes use expand → migrate → observe → contract. Contract is a later rollout after N-1 compatibility and observation are proven.
4. Application rollback never depends on schema rollback. Destructive contraction is not an incident response.
5. Critical rollouts start with a non-customer canary and explicit abort thresholds.
6. Queue or external-effect changes must implement independently operable global, user, automation, and session kill switches, evaluated at the last boundary before the effect.
7. Production commands, destructive operations, snapshot export, restore, repack, cleanup, lifecycle changes, and contraction require a named human operator and approved window.
8. Evidence contains commands/results, before/after metrics, low-cardinality error codes, trace correlation, and no secrets or PII.
9. CI validation is incremental: it inspects changed migration and change-record files. Existing legacy files are not rewritten or grandfathered into safety claims.

## Consequences

- Review gains an auditable rollout contract and explicit human gates.
- New migrations that introduce known destructive or blocking patterns fail static validation.
- Operational work takes an additional review step, intentionally.
- Static checks cannot prove runtime safety, staging realism, ownership, or metric quality; human review and staging evidence remain mandatory.

## Alternatives rejected

- A documentation-only wiki: it is not versioned with the code and cannot gate a PR.
- Automatically executing production validation from CI: it would require broad credentials and violate the human production gate.
- Enforcing rules across all historical migrations immediately: it would block unrelated changes and mix legacy cleanup into WP-000.

## Revisit

After five production PRs, review validator findings and change-record completeness. Tightening enforcement requires its own governance change record and rollback plan.

