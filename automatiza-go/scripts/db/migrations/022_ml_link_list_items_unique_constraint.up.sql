-- Migration: 022_ml_link_list_items_unique_constraint.up.sql
-- Adds UNIQUE constraint to prevent duplicate URLs per list
-- Safe: checks which column name exists (origin_url vs product_url)

BEGIN;

DO $$
BEGIN
    -- Check for origin_url column (009 schema)
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
    -- Check for product_url column (000 schema)
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
