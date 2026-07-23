-- Down da 080 — SOMENTE banco local descartável. Nunca em produção.
-- DROP CONCURRENTLY pelo mesmo motivo do up: não bloquear escrita.
SET lock_timeout = '3s';

DROP INDEX CONCURRENTLY IF EXISTS idx_failover_policies_user;
DROP INDEX CONCURRENTLY IF EXISTS idx_failover_policies_detector;
DROP INDEX CONCURRENTLY IF EXISTS idx_failover_roles_policy;
DROP INDEX CONCURRENTLY IF EXISTS idx_failover_dry_runs_policy;
DROP INDEX CONCURRENTLY IF EXISTS uq_failover_one_nonterminal_per_user;
DROP INDEX CONCURRENTLY IF EXISTS idx_failover_operations_policy_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_failover_operation_items_automation;
DROP INDEX CONCURRENTLY IF EXISTS idx_failover_leases_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_failover_leases_expires;
DROP INDEX CONCURRENTLY IF EXISTS idx_failover_events_policy_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_failover_events_operation;
DROP INDEX CONCURRENTLY IF EXISTS uq_failover_events_idempotency;
