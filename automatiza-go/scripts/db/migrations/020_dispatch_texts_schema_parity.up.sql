-- =============================================================================
-- Fix dispatch_texts schema to match Go code, proto, and Node.js parity
-- Safe: checks column existence before renaming/modifying
-- =============================================================================

-- 1. Rename content -> body (only if content exists and body does not)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'dispatch_texts' AND column_name = 'content'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'dispatch_texts' AND column_name = 'body'
    ) THEN
        ALTER TABLE dispatch_texts RENAME COLUMN content TO body;
    END IF;
END $$;

-- 2. Add is_active column (safe: IF NOT EXISTS via DO block)
ALTER TABLE dispatch_texts ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;

-- 3. Migrate data if is_default exists
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'dispatch_texts' AND column_name = 'is_default'
    ) THEN
        UPDATE dispatch_texts SET is_active = TRUE;
    END IF;
END $$;

-- 4. Drop unused columns (safe: IF EXISTS)
ALTER TABLE dispatch_texts DROP COLUMN IF EXISTS variables;
ALTER TABLE dispatch_texts DROP COLUMN IF EXISTS is_default;

-- 5. Ensure NOT NULL constraints on body/created_at/updated_at
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'dispatch_texts' AND column_name = 'body'
    ) THEN
        ALTER TABLE dispatch_texts ALTER COLUMN body SET NOT NULL;
    END IF;
END $$;

ALTER TABLE dispatch_texts ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE dispatch_texts ALTER COLUMN created_at SET DEFAULT NOW();
ALTER TABLE dispatch_texts ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE dispatch_texts ALTER COLUMN updated_at SET DEFAULT NOW();
