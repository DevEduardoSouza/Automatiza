-- =============================================================================
-- Migration 014: Update affiliate_credentials for proto compatibility
-- =============================================================================
-- This migration adds:
-- 1. credentials JSONB column (replaces app_id/app_secret for flexibility)
-- 2. auto_generate boolean column (controls automatic link generation)
-- 3. Migrates existing data from app_id/app_secret to credentials JSONB
-- =============================================================================

-- Add new columns
ALTER TABLE affiliate_credentials
    ADD COLUMN IF NOT EXISTS credentials JSONB,
    ADD COLUMN IF NOT EXISTS auto_generate BOOLEAN DEFAULT TRUE;

-- Migrate existing data from app_id/app_secret to credentials JSONB
-- Format: {"app_id": "...", "app_secret": "..."}
UPDATE affiliate_credentials
SET credentials = jsonb_build_object(
    'app_id', app_id,
    'app_secret', app_secret,
    'access_token', COALESCE(access_token, ''),
    'refresh_token', COALESCE(refresh_token, ''),
    'token_expires_at', COALESCE(token_expires_at::text, '')
)
WHERE credentials IS NULL
  AND app_id IS NOT NULL
  AND app_secret IS NOT NULL;

-- Add index for credentials JSONB queries (GIN for contains operators)
CREATE INDEX IF NOT EXISTS idx_affiliate_credentials_credentials
    ON affiliate_credentials USING GIN (credentials);

-- Add index for auto_generate filtering
CREATE INDEX IF NOT EXISTS idx_affiliate_credentials_auto_generate
    ON affiliate_credentials(auto_generate) WHERE auto_generate = TRUE;

-- Add index for is_active filtering (if not exists)
CREATE INDEX IF NOT EXISTS idx_affiliate_credentials_is_active
    ON affiliate_credentials(is_active) WHERE is_active = TRUE;

-- Comment the columns for documentation
COMMENT ON COLUMN affiliate_credentials.credentials IS 'Store-specific credentials as JSONB (app_id, app_secret, tokens, etc.)';
COMMENT ON COLUMN affiliate_credentials.auto_generate IS 'Whether to auto-generate affiliate links for this store';

-- =============================================================================
-- Notes:
-- - We keep app_id and app_secret columns for backward compatibility during migration
-- - After confirming all code uses the credentials JSONB column, we can drop them
-- - The credentials field stores different data per store:
--   - Shopee: {app_id, app_secret}
--   - Mercado Livre: {encrypted_token, site_id}
--   - Amazon: {access_key, secret_key, partner_tag}
--   - AWIN: {publisher_id, api_key}
-- =============================================================================
