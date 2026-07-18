-- Rollback: volta a constraint de 4 valores da migração 023.
-- Antes, mapeia os valores novos para o equivalente antigo mais próximo
-- (mesmo racional da 023: nenhuma linha pode violar a constraint recriada).

ALTER TABLE action_history DROP CONSTRAINT IF EXISTS action_history_status_check;

UPDATE action_history SET status = 'completed' WHERE status = 'assumedSent';
UPDATE action_history SET status = 'failed' WHERE status = 'skipped';

ALTER TABLE action_history ADD CONSTRAINT action_history_status_check
    CHECK (status IN ('pending', 'running', 'completed', 'failed'));
