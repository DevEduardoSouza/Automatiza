-- =============================================================================
-- Migration 014 Rollback: Revert affiliate_credentials update
-- =============================================================================

-- Remove indices first
DROP INDEX IF EXISTS idx_affiliate_credentials_credentials;
DROP INDEX IF EXISTS idx_affiliate_credentials_auto_generate;
DROP INDEX IF EXISTS idx_affiliate_credentials_is_active;

-- Remove new columns (data in credentials JSONB will be lost - ensure backup)
ALTER TABLE affiliate_credentials
    DROP COLUMN IF EXISTS credentials,
    DROP COLUMN IF EXISTS auto_generate;

-- =============================================================================
-- WARNING: This rollback will delete the credentials JSONB data.
-- Ensure you have a backup before running this migration.
-- The app_id and app_secret columns still have the original data.
-- =============================================================================
