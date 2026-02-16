-- =============================================================================
-- 032: Create magic_link_tokens table for passwordless authentication
-- =============================================================================

CREATE TABLE IF NOT EXISTS magic_link_tokens (
    id SERIAL PRIMARY KEY,
    email TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_magic_link_tokens_email ON magic_link_tokens(email);
CREATE INDEX idx_magic_link_tokens_expires ON magic_link_tokens(expires_at) WHERE used_at IS NULL;
