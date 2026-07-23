\set ON_ERROR_STOP on
SET search_path = public, pg_catalog;

\if :{?wp002_confirm}
\else
\echo error_code=wp002_operator_confirmation_missing expected=-v wp002_confirm=CREATE_PRODUCT_URL_INDEX_RECOVERY
SELECT 1 / 0;
\endif

SELECT :'wp002_confirm' = 'CREATE_PRODUCT_URL_INDEX_RECOVERY' AS operator_confirmed
\gset
\if :operator_confirmed
\else
\echo error_code=wp002_operator_confirmation_invalid
SELECT 1 / 0;
\endif

SELECT EXISTS (
    SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm'
) AS pg_trgm_present
\gset
\if :pg_trgm_present
\else
\echo error_code=wp002_pg_trgm_missing action=stop_and_review_extension_change_separately
SELECT 1 / 0;
\endif

SELECT to_regclass('public.idx_dispatch_logs_product_url_trgm') IS NULL AS index_absent
\gset
\if :index_absent
\else
\echo error_code=wp002_create_precondition_failed expected=index_absent
SELECT 1 / 0;
\endif

SET lock_timeout = '5s';
SET statement_timeout = '2h';
CREATE INDEX CONCURRENTLY idx_dispatch_logs_product_url_trgm
    ON public.dispatch_logs USING gin (product_url gin_trgm_ops);
RESET statement_timeout;
RESET lock_timeout;

SELECT COALESCE((
    SELECT
        i.indisvalid
        AND i.indisready
        AND regexp_replace(lower(pg_get_indexdef(i.indexrelid)), '\s+', ' ', 'g')
            = 'create index idx_dispatch_logs_product_url_trgm on public.dispatch_logs using gin (product_url gin_trgm_ops)'
    FROM pg_index i
    JOIN pg_class idx ON idx.oid = i.indexrelid
    JOIN pg_namespace idx_ns ON idx_ns.oid = idx.relnamespace
    JOIN pg_class tbl ON tbl.oid = i.indrelid
    JOIN pg_namespace tbl_ns ON tbl_ns.oid = tbl.relnamespace
    WHERE idx_ns.nspname = 'public'
      AND idx.relname = 'idx_dispatch_logs_product_url_trgm'
      AND tbl_ns.nspname = 'public'
      AND tbl.relname = 'dispatch_logs'
), false) AS index_valid
\gset
\if :index_valid
\echo event=wp002_recovery_index_created
\else
\echo error_code=wp002_create_postcondition_failed action=do_not_retry_use_invalid_index_drop_change
SELECT 1 / 0;
\endif
