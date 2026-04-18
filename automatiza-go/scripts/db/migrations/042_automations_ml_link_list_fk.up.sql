-- Add FK on automations.ml_link_list_id with ON DELETE SET NULL to prevent
-- orphaned references when a user deletes a ML link list that is still
-- referenced by an active LIST-mode automation.
--
-- Observed symptom before this fix: a deleted list left the automation in
-- "active" state with ml_link_list_id pointing at a non-existent row.
-- GetNextUnconsumed returned [], processListMode logged success with
-- "Lista sem itens disponiveis" and the automation ran forever doing nothing.
--
-- After this FK, deleting a list auto-NULLs the column; the existing guard
-- at process_list.go:33-40 then surfaces a clear error ("Selecione uma Lista
-- de Links para usar no modo Lista.") on the next run.

-- Step 1: clean existing orphans (ml_link_list_id pointing to a deleted list).
UPDATE automations a
SET ml_link_list_id = NULL
WHERE a.ml_link_list_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM ml_link_lists l WHERE l.id = a.ml_link_list_id);

-- Step 2: add FK constraint (idempotent — skip if already present).
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_automations_ml_link_list_id'
          AND conrelid = 'automations'::regclass
    ) THEN
        ALTER TABLE automations
        ADD CONSTRAINT fk_automations_ml_link_list_id
        FOREIGN KEY (ml_link_list_id)
        REFERENCES ml_link_lists(id)
        ON DELETE SET NULL;
    END IF;
END$$;
