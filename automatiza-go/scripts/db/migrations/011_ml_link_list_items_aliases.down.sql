-- =============================================================================
-- Migration: 011_ml_link_list_items_aliases.down.sql
-- Removes alias columns added for API compatibility
-- =============================================================================

BEGIN;

-- Drop sync trigger
DROP TRIGGER IF EXISTS trigger_sync_ml_link_list_items_aliases ON ml_link_list_items;
DROP FUNCTION IF EXISTS sync_ml_link_list_items_aliases();

-- Drop indexes
DROP INDEX IF EXISTS idx_ml_link_list_items_unconsumed_position;
DROP INDEX IF EXISTS idx_ml_link_list_items_automation_id;

-- Remove alias columns (keeping original columns from 009)
ALTER TABLE ml_link_list_items DROP COLUMN IF EXISTS ml_link_list_id;
ALTER TABLE ml_link_list_items DROP COLUMN IF EXISTS url;
ALTER TABLE ml_link_list_items DROP COLUMN IF EXISTS position;
ALTER TABLE ml_link_list_items DROP COLUMN IF EXISTS automation_id;

COMMIT;
