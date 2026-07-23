\set ON_ERROR_STOP on

BEGIN TRANSACTION READ ONLY;
SET LOCAL search_path = public, pg_catalog;

SELECT
    extname,
    extversion
FROM pg_extension
WHERE extname = 'pg_trgm';

SELECT
    idx_ns.nspname AS index_schema,
    idx.relname AS index_name,
    tbl_ns.nspname AS table_schema,
    tbl.relname AS table_name,
    i.indisvalid,
    i.indisready,
    pg_size_pretty(pg_relation_size(i.indexrelid)) AS index_size,
    pg_get_indexdef(i.indexrelid) AS index_definition
FROM pg_index i
JOIN pg_class idx ON idx.oid = i.indexrelid
JOIN pg_namespace idx_ns ON idx_ns.oid = idx.relnamespace
JOIN pg_class tbl ON tbl.oid = i.indrelid
JOIN pg_namespace tbl_ns ON tbl_ns.oid = tbl.relnamespace
WHERE idx_ns.nspname = 'public'
  AND idx.relname = 'idx_dispatch_logs_product_url_trgm';

SELECT to_regclass('public._deploy_migrations') IS NOT NULL AS legacy_ledger_present
\gset
\if :legacy_ledger_present
SELECT
    filename,
    applied_at
FROM _deploy_migrations
WHERE filename IN (
    '019_dispatch_logs_performance_indexes.up.sql',
    '037_drop_unused_trgm_index.up.sql'
)
ORDER BY filename;
\else
\echo event=wp002_legacy_ledger_missing
\endif

SELECT to_regclass('public._deploy_migration_attempts') IS NOT NULL AS attempt_ledger_present
\gset
\if :attempt_ledger_present
SELECT
    filename,
    checksum,
    mode,
    status,
    trace_id,
    started_at,
    finished_at,
    error_code
FROM _deploy_migration_attempts
WHERE filename IN (
    '019_dispatch_logs_performance_indexes.up.sql',
    '037_drop_unused_trgm_index.up.sql'
)
ORDER BY filename;
\else
\echo event=wp002_attempt_ledger_not_initialized
\endif

SELECT
    p.pid,
    p.phase,
    p.blocks_done,
    p.blocks_total,
    p.tuples_done,
    p.tuples_total,
    p.partitions_done,
    p.partitions_total
FROM pg_stat_progress_create_index p
WHERE p.relid = to_regclass('public.dispatch_logs');

COMMIT;
