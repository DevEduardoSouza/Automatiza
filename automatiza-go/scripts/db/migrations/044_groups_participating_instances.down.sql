DROP INDEX IF EXISTS idx_groups_participating_instance_ids;
ALTER TABLE groups DROP COLUMN IF EXISTS participating_instance_ids;
