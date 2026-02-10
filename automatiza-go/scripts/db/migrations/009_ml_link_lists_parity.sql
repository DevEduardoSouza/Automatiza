-- =============================================================================
-- Migration: 009_ml_link_lists_parity.sql
-- Updates ml_link_lists table and creates ml_link_list_items table
-- Safe: only migrates if old schema (automation_id column) is detected
-- =============================================================================

-- Only rename old table if it has the old schema (automation_id column from 001)
-- If 000 ran first, ml_link_lists already has the correct schema
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_lists' AND column_name = 'automation_id'
    ) THEN
        EXECUTE 'DROP POLICY IF EXISTS service_role_ml_link_lists ON ml_link_lists';
        DROP INDEX IF EXISTS idx_ml_link_lists_automation_id;
        DROP INDEX IF EXISTS idx_ml_link_lists_status;
        ALTER TABLE ml_link_lists RENAME TO ml_link_lists_old;
    END IF;
END $$;

-- =============================================================================
-- Create new ml_link_lists table (IF NOT EXISTS handles both scenarios)
-- =============================================================================

CREATE TABLE IF NOT EXISTS ml_link_lists (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ml_link_lists_user_id ON ml_link_lists(user_id);
CREATE INDEX IF NOT EXISTS idx_ml_link_lists_is_active ON ml_link_lists(is_active);

-- =============================================================================
-- Create ml_link_list_items table (safe: IF NOT EXISTS)
-- If 000 ran, this is a no-op. If old schema, creates with 009 columns.
-- =============================================================================

DO $$
BEGIN
    -- Only create the 009-style table if it doesn't exist yet
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'ml_link_list_items') THEN
        CREATE TABLE ml_link_list_items (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            list_id UUID NOT NULL REFERENCES ml_link_lists(id) ON DELETE CASCADE,
            origin_url TEXT NOT NULL,
            order_index INTEGER DEFAULT 0,
            is_active BOOLEAN DEFAULT TRUE,
            consumed_at TIMESTAMPTZ,
            consumed_by_automation_id UUID REFERENCES automations(id) ON DELETE SET NULL,
            consumed_by_group_id UUID REFERENCES groups(id) ON DELETE SET NULL,
            product_title VARCHAR(500),
            product_price DECIMAL(10, 2),
            product_image_url TEXT,
            metadata JSONB DEFAULT '{}'::jsonb,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW()
        );
    END IF;
END $$;

-- Safe index creation (checks column existence)
CREATE INDEX IF NOT EXISTS idx_ml_link_list_items_list_id ON ml_link_list_items(list_id);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'order_index'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_ml_link_list_items_order_index ON ml_link_list_items(list_id, order_index);
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'is_active'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_ml_link_list_items_is_active ON ml_link_list_items(is_active);
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ml_link_list_items' AND column_name = 'consumed_at'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_ml_link_list_items_consumed_at ON ml_link_list_items(consumed_at);
    END IF;
END $$;

-- =============================================================================
-- Row Level Security
-- =============================================================================

ALTER TABLE ml_link_lists ENABLE ROW LEVEL SECURITY;
ALTER TABLE ml_link_list_items ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE tablename = 'ml_link_lists' AND policyname = 'service_role_ml_link_lists') THEN
        CREATE POLICY service_role_ml_link_lists ON ml_link_lists FOR ALL TO service_role USING (true) WITH CHECK (true);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE tablename = 'ml_link_list_items' AND policyname = 'service_role_ml_link_list_items') THEN
        CREATE POLICY service_role_ml_link_list_items ON ml_link_list_items FOR ALL TO service_role USING (true) WITH CHECK (true);
    END IF;
END $$;

-- =============================================================================
-- Triggers for updated_at
-- =============================================================================

DROP TRIGGER IF EXISTS update_ml_link_lists_updated_at ON ml_link_lists;
CREATE TRIGGER update_ml_link_lists_updated_at
    BEFORE UPDATE ON ml_link_lists
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_ml_link_list_items_updated_at ON ml_link_list_items;
CREATE TRIGGER update_ml_link_list_items_updated_at
    BEFORE UPDATE ON ml_link_list_items
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Grant Permissions
GRANT ALL ON ml_link_lists TO service_role;
GRANT ALL ON ml_link_list_items TO service_role;
