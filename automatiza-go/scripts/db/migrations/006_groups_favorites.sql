-- =============================================================================
-- PostgreSQL Schema for Groups Favorites
-- Migration: 006_groups_favorites.sql
-- Adds partial index for is_favorite column on groups table
-- =============================================================================

BEGIN;

-- =============================================================================
-- Add is_favorite column if not exists (idempotent)
-- =============================================================================

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'groups' AND column_name = 'is_favorite') THEN
        ALTER TABLE groups ADD COLUMN is_favorite BOOLEAN DEFAULT FALSE;
    END IF;
END
$$;

-- =============================================================================
-- Create partial index for favorite groups (only where is_favorite = TRUE)
-- This optimizes queries that filter for favorite groups
-- =============================================================================

CREATE INDEX IF NOT EXISTS idx_groups_is_favorite ON groups(is_favorite) WHERE is_favorite = TRUE;

-- =============================================================================
-- Create composite index for user favorites queries
-- Optimizes: SELECT * FROM groups WHERE user_id = $1 AND is_favorite = TRUE
-- =============================================================================

CREATE INDEX IF NOT EXISTS idx_groups_user_favorite ON groups(user_id, is_favorite) WHERE is_favorite = TRUE;

COMMIT;
