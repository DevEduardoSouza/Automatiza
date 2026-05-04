DROP INDEX IF EXISTS uniq_instances_user_default;
ALTER TABLE instances DROP COLUMN IF EXISTS is_default;
