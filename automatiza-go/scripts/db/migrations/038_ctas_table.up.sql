-- Create CTAs (Call-to-Action) table as a dedicated entity per user
CREATE TABLE IF NOT EXISTS ctas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    message TEXT NOT NULL,
    platform TEXT NOT NULL DEFAULT 'all',
    cta_type TEXT NOT NULL DEFAULT 'all',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ctas_user_active ON ctas (user_id, is_active) WHERE is_active = true;
CREATE INDEX idx_ctas_user_created ON ctas (user_id, created_at DESC);

-- Add cta_ids column to automations for referencing user's CTAs
ALTER TABLE automations ADD COLUMN IF NOT EXISTS cta_ids UUID[] DEFAULT '{}';
