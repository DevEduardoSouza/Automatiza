-- =============================================================================
-- Migration: 010_add_lock_until_to_automation_runs.up.sql
-- Description: Add lock_until column for execution locking mechanism
-- =============================================================================

-- Add lock_until column for execution locking
ALTER TABLE automation_runs ADD COLUMN IF NOT EXISTS lock_until TIMESTAMPTZ;

-- Partial index for active locks (only index non-null values)
-- This allows efficient queries to find locked runs without indexing all rows
CREATE INDEX IF NOT EXISTS idx_automation_runs_active_lock
ON automation_runs (automation_id, lock_until)
WHERE lock_until IS NOT NULL;

-- Comment for documentation
COMMENT ON COLUMN automation_runs.lock_until IS 'Timestamp until which this automation run is locked for execution';
