DROP INDEX IF EXISTS idx_dispatch_logs_list_item;
ALTER TABLE dispatch_logs DROP COLUMN IF EXISTS list_item_id;
