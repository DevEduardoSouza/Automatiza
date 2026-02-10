-- =============================================================================
-- PostgreSQL Schema for Password Reset
-- Migration: 005_password_reset.sql
-- Adds password_hash to profiles if missing, ensures password_reset_tokens exist
-- =============================================================================

BEGIN;

-- Add password_hash column to profiles (safe: IF NOT EXISTS)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'profiles' AND column_name = 'password_hash') THEN
        ALTER TABLE profiles ADD COLUMN password_hash TEXT;
    END IF;
END
$$;

-- password_reset_tokens already created by 000, but safe with IF NOT EXISTS
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token_hash ON password_reset_tokens(token_hash);

-- Enable RLS (safe to run even if already enabled)
ALTER TABLE password_reset_tokens ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE tablename = 'password_reset_tokens' AND policyname = 'service_role_password_reset_tokens') THEN
        CREATE POLICY service_role_password_reset_tokens
            ON password_reset_tokens FOR ALL TO service_role
            USING (true) WITH CHECK (true);
    END IF;
END
$$;

-- Cleanup function
CREATE OR REPLACE FUNCTION cleanup_expired_password_reset_tokens()
RETURNS void AS $$
BEGIN
    DELETE FROM password_reset_tokens
    WHERE expires_at < NOW() - INTERVAL '1 day'
       OR used_at < NOW() - INTERVAL '1 day';
END;
$$ LANGUAGE plpgsql;

GRANT EXECUTE ON FUNCTION cleanup_expired_password_reset_tokens() TO service_role;
GRANT ALL ON password_reset_tokens TO service_role;

COMMIT;
