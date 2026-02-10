-- Rollback: Remove counting fields from action_history

-- Remover constraint de status
ALTER TABLE action_history DROP CONSTRAINT IF EXISTS action_history_status_check;

-- Remover índice de status
DROP INDEX IF EXISTS idx_action_history_status;

-- Remover campos de contagem
ALTER TABLE action_history DROP COLUMN IF EXISTS error_count;
ALTER TABLE action_history DROP COLUMN IF EXISTS success_count;
ALTER TABLE action_history DROP COLUMN IF EXISTS target_count;
