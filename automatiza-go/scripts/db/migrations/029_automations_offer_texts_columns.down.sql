-- Revert: Remove offer texts and list mode columns from automations

ALTER TABLE automations DROP COLUMN IF EXISTS texts;
ALTER TABLE automations DROP COLUMN IF EXISTS ctas;
ALTER TABLE automations DROP COLUMN IF EXISTS last_error;
ALTER TABLE automations DROP COLUMN IF EXISTS last_error_at;
ALTER TABLE automations DROP COLUMN IF EXISTS ml_link_list_id;
ALTER TABLE automations DROP COLUMN IF EXISTS list_items_per_run;
ALTER TABLE automations DROP COLUMN IF EXISTS list_include_image;
ALTER TABLE automations DROP COLUMN IF EXISTS priority_type;
