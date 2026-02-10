-- =============================================================================
-- Migration 017: Align sessions table with Go auth service expectations
-- Safe: checks column existence before renaming/adding
-- =============================================================================

-- 1. Rename token_hash → refresh_token_hash (only if token_hash exists and refresh_token_hash does not)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sessions' AND column_name = 'token_hash'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sessions' AND column_name = 'refresh_token_hash'
    ) THEN
        ALTER TABLE sessions RENAME COLUMN token_hash TO refresh_token_hash;
    END IF;
END $$;

-- 2. Rename last_active_at → last_activity_at (only if last_active_at exists and last_activity_at does not)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sessions' AND column_name = 'last_active_at'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sessions' AND column_name = 'last_activity_at'
    ) THEN
        ALTER TABLE sessions RENAME COLUMN last_active_at TO last_activity_at;
    END IF;
END $$;

-- 3. Add csrf_token_hash (nullable for existing rows)
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS csrf_token_hash TEXT;

-- 4. Add device_fingerprint
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS device_fingerprint VARCHAR(255);

-- 5. Add revoked_at for session revocation
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;

-- 6. Update index to match renamed column (safe: drop only if exists)
DROP INDEX IF EXISTS idx_sessions_token_hash;
CREATE INDEX IF NOT EXISTS idx_sessions_refresh_token_hash ON sessions(refresh_token_hash);

-- 7. Add index for revoked_at (useful for cleanup queries)
CREATE INDEX IF NOT EXISTS idx_sessions_revoked_at ON sessions(revoked_at) WHERE revoked_at IS NULL;
