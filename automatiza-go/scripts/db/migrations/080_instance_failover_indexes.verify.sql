-- Verificador da 080 (exigido para migração nontransactional).
--
-- Duas perguntas, porque CONCURRENTLY falha de um jeito silencioso que a
-- ausência do índice não captura: um índice pode EXISTIR e estar INVALID
-- (criação interrompida no meio). Um índice inválido não é usado pelo planner e
-- ainda pesa na escrita — o pior dos mundos, e invisível para quem só confere
-- se o nome existe.
SELECT
    count(*) FILTER (WHERE i.indisvalid)     AS valid_indexes,
    count(*) FILTER (WHERE NOT i.indisvalid) AS invalid_indexes,
    (count(*) FILTER (WHERE i.indisvalid) = 12
     AND count(*) FILTER (WHERE NOT i.indisvalid) = 0) AS migration_verified
FROM pg_class c
JOIN pg_index i ON i.indexrelid = c.oid
WHERE c.relname IN (
    'idx_failover_policies_user',
    'idx_failover_policies_detector',
    'idx_failover_roles_policy',
    'idx_failover_dry_runs_policy',
    'uq_failover_one_nonterminal_per_user',
    'idx_failover_operations_policy_created',
    'idx_failover_operation_items_automation',
    'idx_failover_leases_active',
    'idx_failover_leases_expires',
    'idx_failover_events_policy_created',
    'idx_failover_events_operation',
    'uq_failover_events_idempotency'
) \gset
