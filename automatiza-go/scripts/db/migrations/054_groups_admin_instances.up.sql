-- Per-chip admin tracking for groups.
--
-- A group has ONE row per (user, wa_group_id), but "am I admin" is per-CHIP.
-- The old single is_admin bool was overwritten last-writer-wins by whichever
-- chip synced last: if chip A is admin and chip B (syncing after) is not, the
-- row flipped to is_admin=false and the group vanished from the admin-only
-- list — even though the user could still post via chip A. With N chips this
-- silently hid every group where only a non-last-writer chip was admin.
--
-- admin_instance_ids holds EVERY chip that is admin in the group (mirrors how
-- participating_instance_ids already tracks participation, migration 044).
-- is_admin becomes a derived cache = "admin_instance_ids is non-empty", i.e.
-- admin via ANY chip, so the existing `WHERE is_admin = true` filter is correct
-- again and resets naturally when a chip loses admin.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS admin_instance_ids INTEGER[] NOT NULL DEFAULT '{}';

-- Backfill from the current single-chip snapshot. Imperfect by construction —
-- one bool can't tell us which OTHER chips are admin — so this only seeds the
-- owner chip for rows already flagged admin. The next per-chip sync corrects
-- it by adding each admin chip (and is what fully restores multi-chip counts).
UPDATE groups
SET admin_instance_ids = ARRAY[instance_id]
WHERE is_admin = TRUE
  AND admin_instance_ids = '{}';

CREATE INDEX IF NOT EXISTS idx_groups_admin_instance_ids
    ON groups USING GIN (admin_instance_ids);
