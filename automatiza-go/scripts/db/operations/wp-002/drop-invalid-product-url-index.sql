\set ON_ERROR_STOP on
SET search_path = public, pg_catalog;

\if :{?wp002_confirm}
\else
\echo error_code=wp002_operator_confirmation_missing expected=-v wp002_confirm=DROP_INVALID_PRODUCT_URL_INDEX
SELECT 1 / 0;
\endif

SELECT :'wp002_confirm' = 'DROP_INVALID_PRODUCT_URL_INDEX' AS operator_confirmed
\gset
\if :operator_confirmed
\else
\echo error_code=wp002_operator_confirmation_invalid
SELECT 1 / 0;
\endif

SELECT
    count(*) = 1
    AND bool_and(NOT i.indisvalid)
    AND bool_and(
        regexp_replace(lower(pg_get_indexdef(i.indexrelid)), '\s+', ' ', 'g')
        = 'create index idx_dispatch_logs_product_url_trgm on public.dispatch_logs using gin (product_url gin_trgm_ops)'
    ) AS invalid_expected_index
FROM pg_index i
JOIN pg_class idx ON idx.oid = i.indexrelid
JOIN pg_namespace idx_ns ON idx_ns.oid = idx.relnamespace
JOIN pg_class tbl ON tbl.oid = i.indrelid
JOIN pg_namespace tbl_ns ON tbl_ns.oid = tbl.relnamespace
WHERE idx_ns.nspname = 'public'
  AND idx.relname = 'idx_dispatch_logs_product_url_trgm'
  AND tbl_ns.nspname = 'public'
  AND tbl.relname = 'dispatch_logs'
\gset

\if :invalid_expected_index
\else
\echo error_code=wp002_drop_precondition_failed expected=one_invalid_index_with_expected_definition
SELECT 1 / 0;
\endif

SET lock_timeout = '5s';
SET statement_timeout = '10min';
DROP INDEX CONCURRENTLY public.idx_dispatch_logs_product_url_trgm;
RESET statement_timeout;
RESET lock_timeout;

SELECT to_regclass('public.idx_dispatch_logs_product_url_trgm') IS NULL AS index_absent
\gset
\if :index_absent
\echo event=wp002_invalid_index_dropped
\else
\echo error_code=wp002_drop_postcondition_failed
SELECT 1 / 0;
\endif
