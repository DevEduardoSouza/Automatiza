-- Rollback: 053_automations_start_date.down.sql

ALTER TABLE automations
    DROP COLUMN IF EXISTS start_date;
