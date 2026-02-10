-- Rollback: Remove coupon tracking fields from dispatch_logs table

ALTER TABLE dispatch_logs
DROP COLUMN IF EXISTS coupon_skip_reason,
DROP COLUMN IF EXISTS coupon_applied,
DROP COLUMN IF EXISTS coupon_id;
