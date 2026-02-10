-- =============================================================================
-- PostgreSQL Migration for WAHA Webhook Events
-- Migration: 003_webhook_events.sql
-- Creates webhook_events table for idempotent event processing
-- =============================================================================
--
-- Purpose:
--   This table ensures idempotent processing of WAHA webhook events.
--   Per CLAUDE.md: "Consumer deve ser idempotente por payload.id com unique
--   constraint/tabela de dedupe; aceitar eventos duplicados."
--
-- The UNIQUE constraint on event_id is CRITICAL for idempotency - it prevents
-- duplicate processing of the same webhook event even if delivered multiple times.
--
-- Status workflow:
--   received -> processing -> processed (success)
--                          -> failed (error, will retry)
--                          -> dlq (max retries exceeded)
-- =============================================================================

BEGIN;

-- =============================================================================
-- Webhook Events Table (Idempotency)
-- =============================================================================

CREATE TABLE IF NOT EXISTS webhook_events (
    id SERIAL PRIMARY KEY,

    -- WAHA event identifier (ULID from payload.id)
    -- This is the CRITICAL field for idempotency
    event_id VARCHAR(128) NOT NULL,

    -- Session identifier from WAHA
    session_name VARCHAR(128) NOT NULL,

    -- Event type: message, message.ack, session.status, etc.
    event_type VARCHAR(50) NOT NULL,

    -- Timestamps for tracking
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,

    -- Processing status with allowed values
    status VARCHAR(20) NOT NULL DEFAULT 'received'
        CHECK (status IN ('received', 'processing', 'processed', 'failed', 'dlq')),

    -- Error tracking for failed events
    error_message TEXT,

    -- Retry counter for exponential backoff
    retry_count INTEGER NOT NULL DEFAULT 0,

    -- UNIQUE CONSTRAINT - CRITICAL FOR IDEMPOTENCY
    -- This prevents duplicate processing of the same event
    CONSTRAINT uq_webhook_events_event_id UNIQUE (event_id)
);

-- =============================================================================
-- Indexes for Webhook Events
-- =============================================================================

-- Index for filtering by session (common query pattern)
CREATE INDEX IF NOT EXISTS idx_webhook_events_session
    ON webhook_events(session_name);

-- Partial index for events that need processing
-- Only indexes rows with actionable statuses (smaller, faster)
CREATE INDEX IF NOT EXISTS idx_webhook_events_status
    ON webhook_events(status)
    WHERE status IN ('received', 'processing', 'failed');

-- Index for time-based queries (cleanup, monitoring, reconciliation)
CREATE INDEX IF NOT EXISTS idx_webhook_events_received_at
    ON webhook_events(received_at);

-- =============================================================================
-- Comments for Documentation
-- =============================================================================

COMMENT ON TABLE webhook_events IS
    'Stores WAHA webhook events for idempotent processing. The event_id unique constraint ensures events are processed exactly once.';

COMMENT ON COLUMN webhook_events.event_id IS
    'ULID from WAHA payload.id - UNIQUE constraint ensures idempotency';

COMMENT ON COLUMN webhook_events.status IS
    'Processing status: received (new), processing (in progress), processed (done), failed (error), dlq (dead letter)';

COMMENT ON COLUMN webhook_events.retry_count IS
    'Number of retry attempts. Used with exponential backoff. Max retries before DLQ defined in application config.';

COMMIT;
