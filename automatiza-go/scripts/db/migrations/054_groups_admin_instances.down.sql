DROP INDEX IF EXISTS idx_groups_admin_instance_ids;
ALTER TABLE groups DROP COLUMN IF EXISTS admin_instance_ids;
