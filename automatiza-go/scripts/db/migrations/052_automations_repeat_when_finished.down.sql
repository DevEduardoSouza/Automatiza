-- Rollback: 052_automations_repeat_when_finished.down.sql

ALTER TABLE automations
    DROP COLUMN IF EXISTS repeat_when_finished;
