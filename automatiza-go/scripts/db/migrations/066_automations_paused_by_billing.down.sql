DROP INDEX IF EXISTS idx_automations_paused_by_billing;
ALTER TABLE automations DROP COLUMN IF EXISTS paused_by_billing;
