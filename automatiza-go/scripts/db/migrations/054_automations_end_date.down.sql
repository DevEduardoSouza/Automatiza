-- Rollback: 054_automations_end_date.down.sql

ALTER TABLE automations
    DROP COLUMN IF EXISTS end_date;
