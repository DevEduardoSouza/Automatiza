SELECT
    (
        SELECT count(*) = 3
           AND bool_and(i.indisvalid)
           AND bool_and(i.indisready)
           AND bool_and(
               regexp_replace(lower(pg_get_indexdef(i.indexrelid)), '\s+', ' ', 'g')
               = expected.definition
           )
        FROM (
            VALUES
                (
                    'idx_dispatch_logs_user_created',
                    'create index idx_dispatch_logs_user_created on public.dispatch_logs using btree (user_id, created_at desc)'
                ),
                (
                    'idx_dispatch_logs_group',
                    'create index idx_dispatch_logs_group on public.dispatch_logs using btree (group_id)'
                ),
                (
                    'idx_dispatch_logs_store',
                    'create index idx_dispatch_logs_store on public.dispatch_logs using btree (store)'
                )
        ) AS expected(index_name, definition)
        JOIN pg_class idx ON idx.relname = expected.index_name
        JOIN pg_namespace idx_ns ON idx_ns.oid = idx.relnamespace AND idx_ns.nspname = 'public'
        JOIN pg_index i ON i.indexrelid = idx.oid
        JOIN pg_class tbl ON tbl.oid = i.indrelid AND tbl.relname = 'dispatch_logs'
        JOIN pg_namespace tbl_ns ON tbl_ns.oid = tbl.relnamespace AND tbl_ns.nspname = 'public'
    )
    AND to_regclass('public.idx_dispatch_logs_product_url_trgm') IS NULL
    AS migration_verified
\gset
