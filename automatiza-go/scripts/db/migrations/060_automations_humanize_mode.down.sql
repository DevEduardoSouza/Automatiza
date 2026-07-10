-- Rollback: 060_automations_humanize_mode.down.sql

ALTER TABLE automations
    DROP COLUMN IF EXISTS humanize_mode;
