-- Revert dispatch_texts schema changes

-- 1. Drop trigger if exists
DROP TRIGGER IF EXISTS set_dispatch_texts_updated_at ON dispatch_texts;

-- 2. Re-add old columns
ALTER TABLE dispatch_texts ADD COLUMN variables JSONB DEFAULT '[]'::jsonb;
ALTER TABLE dispatch_texts ADD COLUMN is_default BOOLEAN DEFAULT FALSE;

-- 3. Rename body back to content
ALTER TABLE dispatch_texts RENAME COLUMN body TO content;

-- 4. Drop is_active
ALTER TABLE dispatch_texts DROP COLUMN IF EXISTS is_active;
