DROP INDEX IF EXISTS idx_automations_instance_id;
ALTER TABLE automations DROP COLUMN IF EXISTS instance_id;
