-- Migration: Add partial index for unconsumed ML link list items
-- Purpose: Optimize queries that fetch unconsumed items for automation consumption
-- Safe: checks if required columns exist before creating index

DO $$
BEGIN
    -- Check if the table uses the 009 schema (origin_url, is_active, order_index)
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'is_active'
    ) AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'order_index'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_ml_link_list_items_unconsumed
            ON ml_link_list_items (list_id, order_index, created_at)
            WHERE is_active = true AND consumed_at IS NULL;
    -- Check if the table uses the 000 schema (product_url, status)
    ELSIF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'status'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_ml_link_list_items_unconsumed
            ON ml_link_list_items (list_id, status)
            WHERE status = 'ready' AND consumed_at IS NULL;
    END IF;
END $$;
