-- Rollback: Remove coupon_id column from automations table

DROP INDEX IF EXISTS idx_automations_coupon_id;
ALTER TABLE automations DROP COLUMN IF EXISTS coupon_id;
