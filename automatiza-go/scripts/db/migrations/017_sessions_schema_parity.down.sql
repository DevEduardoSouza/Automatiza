-- =============================================================================
-- Rollback Migration 017: Revert sessions table to original schema
-- =============================================================================

-- 1. Drop new indexes
DROP INDEX IF EXISTS idx_sessions_revoked_at;
DROP INDEX IF EXISTS idx_sessions_refresh_token_hash;

-- 2. Drop new columns
ALTER TABLE sessions DROP COLUMN IF EXISTS revoked_at;
ALTER TABLE sessions DROP COLUMN IF EXISTS device_fingerprint;
ALTER TABLE sessions DROP COLUMN IF EXISTS csrf_token_hash;

-- 3. Rename columns back
ALTER TABLE sessions RENAME COLUMN last_activity_at TO last_active_at;
ALTER TABLE sessions RENAME COLUMN refresh_token_hash TO token_hash;

-- 4. Recreate original index
CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);
