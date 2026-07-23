SELECT COALESCE((
    SELECT
        i.indisvalid
        AND i.indisready
        AND regexp_replace(lower(pg_get_indexdef(i.indexrelid)), '\s+', ' ', 'g')
            = 'create index idx_dispatch_logs_list_item on public.dispatch_logs using btree (list_item_id, created_at) where (list_item_id is not null)'
    FROM pg_index i
    JOIN pg_class idx ON idx.oid = i.indexrelid
    JOIN pg_namespace idx_ns ON idx_ns.oid = idx.relnamespace
    JOIN pg_class tbl ON tbl.oid = i.indrelid
    JOIN pg_namespace tbl_ns ON tbl_ns.oid = tbl.relnamespace
    WHERE idx_ns.nspname = 'public'
      AND idx.relname = 'idx_dispatch_logs_list_item'
      AND tbl_ns.nspname = 'public'
      AND tbl.relname = 'dispatch_logs'
), false) AS migration_verified
\gset
