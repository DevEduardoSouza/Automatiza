-- =============================================================================
-- Migration: 030_fix_reconciliation_checkpoints.up.sql
-- Fix: reconciliation_checkpoints table had wrong schema (user-based instead
--      of session-based). Drop and recreate with correct WAHA session schema.
-- =============================================================================

BEGIN;

-- Drop the old table (may have user_id/checkpoint_type columns from 000)
DROP TABLE IF EXISTS reconciliation_checkpoints CASCADE;

-- Recreate with correct session-based schema
CREATE TABLE reconciliation_checkpoints (
    id SERIAL PRIMARY KEY,
    session_name VARCHAR(128) NOT NULL,
    chat_id VARCHAR(128) NOT NULL,
    last_message_timestamp BIGINT NOT NULL DEFAULT 0,
    last_message_id VARCHAR(128),
    last_run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(20) NOT NULL DEFAULT 'idle' CHECK (status IN ('idle', 'running', 'failed')),
    UNIQUE(session_name, chat_id)
);

CREATE INDEX idx_reconciliation_checkpoints_session
    ON reconciliation_checkpoints(session_name);

CREATE INDEX idx_reconciliation_checkpoints_status
    ON reconciliation_checkpoints(status) WHERE status != 'idle';

COMMIT;
