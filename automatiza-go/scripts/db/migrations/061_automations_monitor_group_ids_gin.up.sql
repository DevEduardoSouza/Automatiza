-- Índice GIN para o fan-out do espelho/offer_generate: a cada mensagem de
-- grupo monitorado o worker roda 2x `monitor_group_ids @> '["<jid>"]'::jsonb`
-- (GetMonitoringAutomationsForGroup + GetOfferGenerateAutomationsForGroup),
-- que sem índice era scan de todas as automações ativas por evento.
-- jsonb_path_ops: menor e mais rápido para @>, único operador usado.
CREATE INDEX IF NOT EXISTS idx_automations_monitor_group_ids_gin
    ON automations USING GIN (monitor_group_ids jsonb_path_ops);
