-- Rollback categories and subcategories system

-- Drop triggers
DROP TRIGGER IF EXISTS update_subcategories_updated_at ON subcategories;
DROP TRIGGER IF EXISTS update_categories_updated_at ON categories;

-- Drop policies
DROP POLICY IF EXISTS service_role_subcategories ON subcategories;
DROP POLICY IF EXISTS service_role_categories ON categories;

-- Drop tables (cascade will handle foreign keys)
DROP TABLE IF EXISTS subcategories CASCADE;
DROP TABLE IF EXISTS categories CASCADE;
