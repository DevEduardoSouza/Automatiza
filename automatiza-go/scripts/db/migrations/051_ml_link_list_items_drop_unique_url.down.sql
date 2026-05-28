-- Rollback: 051_ml_link_list_items_drop_unique_url.down.sql
-- Restaura o UNIQUE constraint. ATENÇÃO: vai falhar se houver duplicatas
-- na tabela (que é exatamente o que a migration up permitiu). Antes de
-- rodar este down, deduplicar via:
--
--   DELETE FROM ml_link_list_items WHERE id IN (
--     SELECT id FROM (
--       SELECT id, ROW_NUMBER() OVER (PARTITION BY list_id, product_url
--                                     ORDER BY order_index, created_at) AS rn
--       FROM ml_link_list_items
--     ) t WHERE rn > 1
--   );

BEGIN;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'origin_url'
    ) THEN
        IF NOT EXISTS (
            SELECT 1 FROM pg_constraint
            WHERE conname = 'ml_link_list_items_unique_origin_url_per_list'
        ) THEN
            ALTER TABLE ml_link_list_items
                ADD CONSTRAINT ml_link_list_items_unique_origin_url_per_list
                UNIQUE (list_id, origin_url);
        END IF;
    ELSIF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'product_url'
    ) THEN
        IF NOT EXISTS (
            SELECT 1 FROM pg_constraint
            WHERE conname = 'ml_link_list_items_list_id_product_url_key'
        ) THEN
            ALTER TABLE ml_link_list_items
                ADD CONSTRAINT ml_link_list_items_list_id_product_url_key
                UNIQUE (list_id, product_url);
        END IF;
    END IF;
END $$;

COMMIT;
