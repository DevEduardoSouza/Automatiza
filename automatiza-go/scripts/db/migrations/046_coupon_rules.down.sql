DROP INDEX IF EXISTS idx_coupon_rules_coupon_id;
DROP INDEX IF EXISTS idx_coupon_rules_user_platform_active;
DROP TABLE IF EXISTS coupon_rules;
ALTER TABLE automations DROP COLUMN IF EXISTS apply_coupon_rules;
