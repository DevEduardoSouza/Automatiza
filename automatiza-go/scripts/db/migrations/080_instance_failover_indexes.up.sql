-- automatiza:migration-mode=nontransactional
-- 080: índices da Melhoria 5 (chip reserva), fora da transação da 077.
--
-- Por que separado da 077: o guardrail exige CREATE INDEX CONCURRENTLY, e
-- CONCURRENTLY não roda dentro de transação. Colocar as 7 tabelas numa
-- migração nontransactional seria o pior dos dois mundos — uma falha no meio
-- deixaria o schema pela metade, sem rollback. Então a 077 cria tabelas e
-- constraints de forma atômica, e os índices vêm aqui, um a um, sem bloquear
-- escrita.
--
-- Todos são IF NOT EXISTS e idempotentes. Reexecutar é no-op.
--
-- ATENÇÃO ao operar: CONCURRENTLY pode deixar um índice INVALID se falhar no
-- meio (ex.: deadlock). Diagnóstico:
--   SELECT indexrelid::regclass FROM pg_index WHERE NOT indisvalid;
-- O conserto é DROP INDEX + reexecutar esta migração.

SET lock_timeout = '3s';

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_failover_policies_user
    ON instance_failover_policies (user_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_failover_policies_detector
    ON instance_failover_policies (mode, state)
    WHERE mode = 'assisted';

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_failover_roles_policy
    ON instance_failover_roles (policy_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_failover_dry_runs_policy
    ON instance_failover_dry_runs (policy_id_snapshot, created_at DESC);

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_failover_one_nonterminal_per_user
    ON instance_failover_operations (user_id)
    WHERE status IN ('preparing', 'draining', 'verifying');

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_failover_operations_policy_created
    ON instance_failover_operations (policy_id, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_failover_operation_items_automation
    ON instance_failover_operation_items (automation_id_snapshot);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_failover_leases_active
    ON instance_failover_dispatch_leases (user_id, automation_id, expires_at)
    WHERE released_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_failover_leases_expires
    ON instance_failover_dispatch_leases (expires_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_failover_events_policy_created
    ON instance_failover_events (policy_id_snapshot, created_at DESC, id DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_failover_events_operation
    ON instance_failover_events (operation_id, created_at, id);

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_failover_events_idempotency
    ON instance_failover_events (user_id, idempotency_scope, idempotency_key)
    WHERE idempotency_scope IS NOT NULL;
