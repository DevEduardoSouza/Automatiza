-- Track every chip that has access to a group.
--
-- Background: groups.instance_id is the single "owner" — the first chip
-- that discovered the group during sync (see PR #94). When two chips
-- share the same WhatsApp group, only the owner is recorded, so chip B
-- filtering its own list cannot find groups that chip A discovered first.
--
-- This array is the union of every chip whose sync has seen the group.
-- It augments instance_id (kept as the primary chip) without changing
-- the unique constraint or the existing single-owner semantics. Filters
-- and the per-row chip column read from this array; sync writers append.

ALTER TABLE groups
ADD COLUMN IF NOT EXISTS participating_instance_ids INTEGER[] NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_groups_participating_instance_ids
    ON groups USING GIN (participating_instance_ids);

-- Backfill: every existing row has one known chip — its current owner.
UPDATE groups
SET participating_instance_ids = ARRAY[instance_id]
WHERE instance_id IS NOT NULL
  AND NOT (instance_id = ANY(participating_instance_ids));
