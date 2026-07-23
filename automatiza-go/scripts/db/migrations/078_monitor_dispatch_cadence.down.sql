-- automatiza:migration-mode=transactional
-- Rollback da 078. EXISTE POR CONVENÇÃO E NUNCA É EXECUTADO EM PRODUÇÃO:
-- o pipeline não roda .down.sql, e rodá-lo aqui apagaria fila de envio ainda
-- não despachada. Rollback operacional real é forward-only — desligar a flag
-- (FEATURE_MONITOR_DISPATCH_CADENCE=false), deixar o ledger existente drenar em
-- ordem e corrigir por migração aditiva.
SET lock_timeout = '5s';
SET statement_timeout = '10min';

DROP TABLE IF EXISTS automation_monitor_pending_destinations;
DROP TABLE IF EXISTS automation_monitor_pending;
DROP TABLE IF EXISTS automation_monitor_payloads;
DROP TABLE IF EXISTS automation_monitor_dispatch_state;

DROP INDEX IF EXISTS uq_automations_id_user;

ALTER TABLE automations
    DROP CONSTRAINT IF EXISTS chk_automations_monitor_interval_minutes;
ALTER TABLE automations
    DROP COLUMN IF EXISTS monitor_interval_minutes;
