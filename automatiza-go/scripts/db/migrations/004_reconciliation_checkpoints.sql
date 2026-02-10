-- =============================================================================
-- PostgreSQL Migration for Reconciliation Checkpoints
-- Migration: 004_reconciliation_checkpoints.sql
-- Tracks last processed message per chat for backfill/reconciliation
-- =============================================================================
-- As per CLAUDE.md: Implementar reconciliacao/backfill com filtros por timestamp
-- This table enables gap detection and recovery when webhook events are missed.
-- The consumer uses these checkpoints to request missing messages from WAHA API.
-- =============================================================================

BEGIN;

-- =============================================================================
-- Reconciliation Checkpoints Table (alternative schema for WAHA reconciliation)
-- Only creates if table does not already exist (000 creates user-based version)
-- =============================================================================

CREATE TABLE IF NOT EXISTS reconciliation_checkpoints (
    id SERIAL PRIMARY KEY,
    session_name VARCHAR(128) NOT NULL,
    chat_id VARCHAR(128) NOT NULL,
    last_message_timestamp BIGINT NOT NULL,
    last_message_id VARCHAR(128),
    last_run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(20) NOT NULL DEFAULT 'idle' CHECK (status IN ('idle', 'running', 'failed')),
    UNIQUE(session_name, chat_id)
);

-- =============================================================================
-- Indexes - only create if the referenced columns exist
-- =============================================================================

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'reconciliation_checkpoints' AND column_name = 'session_name'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_reconciliation_checkpoints_session
            ON reconciliation_checkpoints(session_name);
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'reconciliation_checkpoints' AND column_name = 'status'
        AND table_schema = 'public'
    ) THEN
        -- Only create partial index if status column has the expected check constraint
        BEGIN
            CREATE INDEX IF NOT EXISTS idx_reconciliation_checkpoints_status
                ON reconciliation_checkpoints(status) WHERE status != 'idle';
        EXCEPTION WHEN OTHERS THEN
            NULL; -- Ignore if index cannot be created (different column type/constraint)
        END;
    END IF;
END $$;

COMMIT;
