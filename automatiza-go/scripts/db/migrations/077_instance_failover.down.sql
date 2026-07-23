-- Down da 077 — SOMENTE para banco local descartável.
--
-- NUNCA executar em produção. Rollback operacional da Melhoria 5 é
-- FEATURE_INSTANCE_FAILOVER=false (bloqueia operação nova) + migração
-- corretiva forward-only. As tabelas são aditivas e podem ficar.
--
-- Ordem: FK de operação corrente → tabelas em ordem inversa de dependência →
-- trigger/função → UNIQUE de ownership.

ALTER TABLE IF EXISTS instance_failover_policies
    DROP CONSTRAINT IF EXISTS fk_failover_current_operation;

DROP TABLE IF EXISTS instance_failover_events;
DROP TABLE IF EXISTS instance_failover_dispatch_leases;
DROP TABLE IF EXISTS instance_failover_operation_items;
DROP TABLE IF EXISTS instance_failover_operations;
DROP TABLE IF EXISTS instance_failover_dry_run_items;
DROP TABLE IF EXISTS instance_failover_dry_runs;

DROP TRIGGER IF EXISTS instance_failover_policies_match_roles ON instance_failover_policies;
DROP TRIGGER IF EXISTS instance_failover_roles_match_policy ON instance_failover_roles;

DROP TABLE IF EXISTS instance_failover_roles;
DROP TABLE IF EXISTS instance_failover_policies;

DROP FUNCTION IF EXISTS instance_failover_policies_match_roles_fn();
DROP FUNCTION IF EXISTS instance_failover_roles_match_policy_fn();
DROP FUNCTION IF EXISTS instance_failover_check_roles(BIGINT);

ALTER TABLE IF EXISTS instances
    DROP CONSTRAINT IF EXISTS uq_instances_user_id_id;
