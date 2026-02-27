-- =============================================================================
-- Migration: 036_password_reset_tokens_audit_columns.up.sql
-- Adds ip_address and user_agent columns to password_reset_tokens for audit
-- =============================================================================

BEGIN;

ALTER TABLE password_reset_tokens
    ADD COLUMN IF NOT EXISTS ip_address VARCHAR(45),
    ADD COLUMN IF NOT EXISTS user_agent TEXT;

COMMIT;
