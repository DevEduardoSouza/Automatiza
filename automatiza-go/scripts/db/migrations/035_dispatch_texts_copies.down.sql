-- 035 down: Remove copies column from dispatch_texts and copy_index from dispatch_logs

ALTER TABLE dispatch_logs DROP COLUMN IF EXISTS copy_index;
ALTER TABLE dispatch_texts DROP COLUMN IF EXISTS copies;
