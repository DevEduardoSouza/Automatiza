-- Per-automation chip preference.
--
-- The automation form lets the user pick which WhatsApp chip the
-- automation runs on, but we never persisted that choice — re-opening
-- the form re-derived the chip from the first send_group's owner
-- (groups.instance_id), so any user-selected chip silently reverted.
-- The dispatcher likewise routed by group ownership, ignoring the form
-- selection.
--
-- This column stores the form selection. NULL = "no preference" (legacy
-- and the default), behaves exactly like before. When set, the worker
-- treats it as the preferred sender chip for every dispatch from this
-- automation, falling back to per-group ownership only if the chip is
-- offline.
--
-- ON DELETE SET NULL keeps the automation alive when its chip is
-- removed; the user can pick a new chip on next edit.

ALTER TABLE automations
ADD COLUMN IF NOT EXISTS instance_id INTEGER REFERENCES instances(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_automations_instance_id
    ON automations(instance_id) WHERE instance_id IS NOT NULL;
