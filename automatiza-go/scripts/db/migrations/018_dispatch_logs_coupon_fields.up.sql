-- Add coupon tracking fields to dispatch_logs table
-- These fields track which coupon was applied (or skipped) for each dispatch.
-- Required for parity with the legacy Supabase schema.

ALTER TABLE dispatch_logs
ADD COLUMN IF NOT EXISTS coupon_id UUID REFERENCES coupons(id) ON DELETE SET NULL,
ADD COLUMN IF NOT EXISTS coupon_applied BOOLEAN,
ADD COLUMN IF NOT EXISTS coupon_skip_reason TEXT;

COMMENT ON COLUMN dispatch_logs.coupon_id IS 'Reference to the coupon that was evaluated for this dispatch';
COMMENT ON COLUMN dispatch_logs.coupon_applied IS 'Whether the coupon was successfully applied to the offer';
COMMENT ON COLUMN dispatch_logs.coupon_skip_reason IS 'Reason why the coupon was not applied (e.g., price too low, wrong store)';
