-- =============================================================================
-- 031: Add kiwify product ID columns to plans + fix user_roles composite unique
-- =============================================================================

-- Plans: add kiwify product ID columns
ALTER TABLE plans ADD COLUMN IF NOT EXISTS kiwify_product_id_monthly TEXT;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS kiwify_product_id_semester TEXT;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS kiwify_product_id_annual TEXT;

-- User Roles: drop single-column unique, add composite unique (user_id, role)
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname LIKE '%user_roles_user_id_key%' AND conrelid = 'user_roles'::regclass
  ) THEN
    ALTER TABLE user_roles DROP CONSTRAINT user_roles_user_id_key;
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_roles_user_id_role ON user_roles(user_id, role);
