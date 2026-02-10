-- =============================================================================
-- Migration: 010_add_lock_until_to_automation_runs.down.sql
-- Description: Remove lock_until column from automation_runs
-- =============================================================================

-- Remove partial index first (must be done before dropping the column)
DROP INDEX IF EXISTS idx_automation_runs_active_lock;

-- Remove column
ALTER TABLE automation_runs DROP COLUMN IF EXISTS lock_until;
