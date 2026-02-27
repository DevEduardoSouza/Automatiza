-- =============================================================================
-- Rollback: 036_password_reset_tokens_audit_columns.down.sql
-- =============================================================================

BEGIN;

ALTER TABLE password_reset_tokens
    DROP COLUMN IF EXISTS ip_address,
    DROP COLUMN IF EXISTS user_agent;

COMMIT;
