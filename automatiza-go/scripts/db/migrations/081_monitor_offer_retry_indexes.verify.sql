-- Verificador da 081 (exigido para migração nontransactional).
-- Checa indisvalid, não só existência: CONCURRENTLY interrompido deixa índice
-- INVALID, que o planner ignora e a escrita paga.
SELECT
    count(*) FILTER (WHERE i.indisvalid)     AS valid_indexes,
    count(*) FILTER (WHERE NOT i.indisvalid) AS invalid_indexes,
    (count(*) FILTER (WHERE i.indisvalid) = 5
     AND count(*) FILTER (WHERE NOT i.indisvalid) = 0) AS migration_verified
FROM pg_class c
JOIN pg_index i ON i.indexrelid = c.oid
WHERE c.relname IN ('monitor_offer_jobs_terminal_idx',
    'monitor_offer_items_canonical_key_uidx',
    'monitor_offer_items_pending_idx',
    'monitor_offer_items_job_ordinal_idx',
    'monitor_offer_deliveries_sending_idx') \gset
