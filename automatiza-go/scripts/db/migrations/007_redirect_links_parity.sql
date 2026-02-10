-- =============================================================================
-- PostgreSQL Migration for Redirect Links Parity
-- Migration: 007_redirect_links_parity.sql
-- Adds group_clicks JSONB column for tracking clicks per group
-- =============================================================================

-- Add group_clicks column to track individual group click counts
-- This allows tracking how many times each target group received traffic
-- Format: {"group_id": click_count, "group_id2": click_count2, ...}
ALTER TABLE saved_redirect_links
ADD COLUMN IF NOT EXISTS group_clicks JSONB DEFAULT '{}'::jsonb;

-- Add comment for documentation
COMMENT ON COLUMN saved_redirect_links.group_clicks IS 'JSONB object tracking click counts per target group. Format: {"group_id": count}';

-- Create index on group_clicks for efficient querying if needed in the future
-- Using GIN index for JSONB to support queries like @> operator
CREATE INDEX IF NOT EXISTS idx_saved_redirect_links_group_clicks ON saved_redirect_links USING GIN (group_clicks);
