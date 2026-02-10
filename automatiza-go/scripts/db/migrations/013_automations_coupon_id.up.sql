-- Add coupon_id column to automations table for coupon integration
-- This allows automations to have an associated coupon that gets applied to offers

ALTER TABLE automations
ADD COLUMN IF NOT EXISTS coupon_id UUID REFERENCES coupons(id) ON DELETE SET NULL;

-- Create index for faster lookups when filtering automations by coupon
CREATE INDEX IF NOT EXISTS idx_automations_coupon_id ON automations(coupon_id)
WHERE coupon_id IS NOT NULL;

-- Add comment for documentation
COMMENT ON COLUMN automations.coupon_id IS 'Optional coupon to apply to offers sent by this automation';
