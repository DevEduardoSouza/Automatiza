-- Remove extended fields from plans table
ALTER TABLE plans DROP COLUMN IF EXISTS description;
ALTER TABLE plans DROP COLUMN IF EXISTS price_monthly;
ALTER TABLE plans DROP COLUMN IF EXISTS price_semester;
ALTER TABLE plans DROP COLUMN IF EXISTS price_annual;
ALTER TABLE plans DROP COLUMN IF EXISTS group_limit;
ALTER TABLE plans DROP COLUMN IF EXISTS storage_limit;
ALTER TABLE plans DROP COLUMN IF EXISTS features;
