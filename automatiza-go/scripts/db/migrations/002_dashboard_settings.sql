-- =============================================================================
-- PostgreSQL Migration for Dashboard Feature
-- Migration: 002_dashboard_settings.sql
-- Adds dashboard-specific indexes and default settings
-- =============================================================================

-- =============================================================================
-- Optimized Index for Dashboard Groups Query
-- =============================================================================

-- Index for efficient dashboard stats query (user groups where user is admin)
CREATE INDEX IF NOT EXISTS idx_groups_user_admin ON groups(user_id, is_admin);

-- Index for efficient active redirect links count
CREATE INDEX IF NOT EXISTS idx_saved_redirect_links_user_active ON saved_redirect_links(user_id, is_active);

-- =============================================================================
-- Dashboard Announcement Settings
-- =============================================================================

INSERT INTO settings (key, value, description) VALUES
    ('dashboard_announcement_enabled', '0', 'Enable dashboard announcement banner (0=disabled, 1=enabled)'),
    ('dashboard_announcement_title', '', 'Dashboard announcement title'),
    ('dashboard_announcement_body', '', 'Dashboard announcement body/message')
ON CONFLICT (key) DO NOTHING;
