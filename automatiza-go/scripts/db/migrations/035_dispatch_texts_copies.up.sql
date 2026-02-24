-- 035: Add copies TEXT[] to dispatch_texts for copy variation support.
-- Each dispatch text can have multiple copy variations; the system randomly selects one per message.
-- Existing body is migrated to copies[0] for backward compatibility.

ALTER TABLE dispatch_texts ADD COLUMN IF NOT EXISTS copies TEXT[] NOT NULL DEFAULT '{}';

-- Migrate existing body into copies[0] for all rows that have a non-empty body but empty copies
UPDATE dispatch_texts
SET copies = ARRAY[body]
WHERE body IS NOT NULL AND body != '' AND (copies IS NULL OR copies = '{}');

-- Add copy_index to dispatch_logs to track which variation was used per dispatch
ALTER TABLE dispatch_logs ADD COLUMN IF NOT EXISTS copy_index SMALLINT;
