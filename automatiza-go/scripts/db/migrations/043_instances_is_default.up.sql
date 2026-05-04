-- Add is_default flag to mark the user's primary WhatsApp instance.
-- Used as the fallback chip when an automation references groups whose
-- instance is disconnected, and as the implicit chip for legacy automations
-- created before the multi-instance UI.

ALTER TABLE instances
ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT FALSE;

-- Only one default instance per user.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_instances_user_default
    ON instances(user_id) WHERE is_default = TRUE;

-- Backfill: mark each user's oldest connected instance as default.
-- Falls back to the oldest instance regardless of status when none is connected,
-- so every existing user ends up with exactly one default.
UPDATE instances i
SET is_default = TRUE
FROM (
    SELECT DISTINCT ON (user_id) id FROM instances
    ORDER BY user_id, (status IN ('connected', 'open')) DESC, created_at ASC
) first_inst
WHERE i.id = first_inst.id;
