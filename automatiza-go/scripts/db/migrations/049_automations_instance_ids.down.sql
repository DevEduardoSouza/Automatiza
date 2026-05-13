-- Reverte 049: drop instance_ids array e seu index.
--
-- Rows criadas/editadas pelo código novo que populam SÓ instance_ids
-- (sem instance_id) ficariam sem chip ao reverter — execute antes:
--   UPDATE automations SET instance_id = instance_ids[1]
--   WHERE instance_id IS NULL AND instance_ids IS NOT NULL;
-- pra preservar pelo menos o primeiro chip da lista.

DROP INDEX IF EXISTS idx_automations_instance_ids;

ALTER TABLE automations
    DROP COLUMN IF EXISTS instance_ids;
