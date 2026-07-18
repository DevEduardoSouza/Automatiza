-- Migration: aceitar os status 'assumedSent' e 'skipped' em action_history
--
-- O MessageWorker grava 'assumedSent' (timeout/desfecho ambíguo de envio —
-- message_worker.go:402/426) e 'skipped' (descarte de mensagem atrasada —
-- message_worker.go:97), mas o CHECK criado na migração 023 só permite
-- pending/running/completed/failed: todo UPDATE desses caminhos falha com
-- SQLSTATE 23514 (~86 erros/10min medidos em produção em 18/07/2026) e o
-- desfecho real da execução se perde (linha fica presa em running/pending).

ALTER TABLE action_history DROP CONSTRAINT IF EXISTS action_history_status_check;

-- NOT VALID: o conjunto novo é superset do antigo, então toda linha existente
-- já satisfaz a constraint — dispensa o scan de ~1.2M linhas sob ACCESS
-- EXCLUSIVE durante o deploy. Linhas novas são validadas normalmente.
ALTER TABLE action_history ADD CONSTRAINT action_history_status_check
    CHECK (status IN ('pending', 'running', 'completed', 'failed', 'assumedSent', 'skipped'))
    NOT VALID;
