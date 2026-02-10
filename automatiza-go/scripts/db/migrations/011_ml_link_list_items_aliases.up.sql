-- =============================================================================
-- Migration: 011_ml_link_list_items_aliases.up.sql
-- Adds aliased columns to ml_link_list_items for API compatibility
-- Safe: only runs if 009 schema (origin_url column) exists
-- If 000 ran first (product_url schema), this is a no-op
-- =============================================================================

BEGIN;

-- Only apply aliases if 009 schema is detected (has origin_url column)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'origin_url'
    ) THEN
        RAISE NOTICE 'ml_link_list_items uses 000 schema (product_url), skipping 011 aliases';
        RETURN;
    END IF;

    -- Add ml_link_list_id alias for list_id
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'ml_link_list_id'
    ) THEN
        ALTER TABLE ml_link_list_items ADD COLUMN ml_link_list_id UUID;
        UPDATE ml_link_list_items SET ml_link_list_id = list_id WHERE ml_link_list_id IS NULL;
    END IF;

    -- Add url alias for origin_url
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'url'
    ) THEN
        ALTER TABLE ml_link_list_items ADD COLUMN url TEXT;
        UPDATE ml_link_list_items SET url = origin_url WHERE url IS NULL;
    END IF;

    -- Add position alias for order_index
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'position'
    ) THEN
        ALTER TABLE ml_link_list_items ADD COLUMN position INT DEFAULT 0;
        UPDATE ml_link_list_items SET position = order_index WHERE position IS NULL OR position = 0;
    END IF;

    -- Add automation_id reference
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'automation_id'
    ) THEN
        ALTER TABLE ml_link_list_items ADD COLUMN automation_id UUID REFERENCES automations(id) ON DELETE SET NULL;
    END IF;
END $$;

-- Safe index creation (only if columns exist)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'ml_link_list_id'
    ) AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'position'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_ml_link_list_items_unconsumed_position
        ON ml_link_list_items (ml_link_list_id, position)
        WHERE consumed_at IS NULL;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'automation_id'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_ml_link_list_items_automation_id
        ON ml_link_list_items (automation_id)
        WHERE automation_id IS NOT NULL;
    END IF;
END $$;

-- Sync trigger (only if 009 schema columns exist)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'origin_url'
    ) THEN
        CREATE OR REPLACE FUNCTION sync_ml_link_list_items_aliases()
        RETURNS TRIGGER AS $fn$
        BEGIN
            IF NEW.ml_link_list_id IS NOT NULL AND NEW.list_id IS NULL THEN
                NEW.list_id := NEW.ml_link_list_id;
            ELSIF NEW.list_id IS NOT NULL AND NEW.ml_link_list_id IS NULL THEN
                NEW.ml_link_list_id := NEW.list_id;
            END IF;

            IF NEW.url IS NOT NULL AND NEW.origin_url IS NULL THEN
                NEW.origin_url := NEW.url;
            ELSIF NEW.origin_url IS NOT NULL AND NEW.url IS NULL THEN
                NEW.url := NEW.origin_url;
            END IF;

            IF NEW.position IS NOT NULL AND NEW.position != 0 AND (NEW.order_index IS NULL OR NEW.order_index = 0) THEN
                NEW.order_index := NEW.position;
            ELSIF NEW.order_index IS NOT NULL AND NEW.order_index != 0 AND (NEW.position IS NULL OR NEW.position = 0) THEN
                NEW.position := NEW.order_index;
            END IF;

            RETURN NEW;
        END;
        $fn$ LANGUAGE plpgsql;

        DROP TRIGGER IF EXISTS trigger_sync_ml_link_list_items_aliases ON ml_link_list_items;
        CREATE TRIGGER trigger_sync_ml_link_list_items_aliases
            BEFORE INSERT OR UPDATE ON ml_link_list_items
            FOR EACH ROW
            EXECUTE FUNCTION sync_ml_link_list_items_aliases();
    END IF;
END $$;

COMMIT;
