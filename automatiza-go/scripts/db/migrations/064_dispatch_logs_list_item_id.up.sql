-- Rastreabilidade de entrega por item de lista (L2 da auditoria de estabilidade).
--
-- O LIST mode marca ml_link_list_items.consumed_at NO ENFILEIRAMENTO; se o envio
-- falha terminalmente no worker (chip caído, WAHA fora), o item ficava queimado
-- sem nunca ter sido entregue e sem forma de detectar. Esta coluna liga cada
-- dispatch_log do LIST ao item que o originou, permitindo ao reconciliador
-- (reconcile_list_deliveries) detectar "consumido mas nunca entregue" e devolver
-- o item (consumed_at=NULL) para retry no próximo ciclo.
ALTER TABLE dispatch_logs ADD COLUMN IF NOT EXISTS list_item_id UUID;

-- Autocura: um CREATE INDEX CONCURRENTLY interrompido (crash/cancel) deixa o
-- índice INVALID — e o IF NOT EXISTS abaixo o veria como "existente" para
-- sempre. Dropa só se inválido; as migrações re-rodam a cada deploy.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_index i
        JOIN pg_class c ON c.oid = i.indexrelid
        WHERE c.relname = 'idx_dispatch_logs_list_item' AND NOT i.indisvalid
    ) THEN
        EXECUTE 'DROP INDEX idx_dispatch_logs_list_item';
    END IF;
END$$;

-- Índice parcial: só linhas do LIST mode entram (fração pequena da tabela).
-- CONCURRENTLY é OBRIGATÓRIO aqui: dispatch_logs tem ~8.6GB/6M+ linhas em
-- produção e a migração roda ANTES do rolling deploy, com o worker antigo
-- servindo tráfego — um CREATE INDEX comum segura ShareLock na tabela por
-- dezenas de segundos e bloqueia TODO INSERT/UPDATE de dispatch (updates de
-- 'sent' perdidos viram falso-negativo de entrega). O runner
-- (run-migrations.sh) usa psql em autocommit, sem transação — CONCURRENTLY
-- é válido neste contexto.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dispatch_logs_list_item
ON dispatch_logs (list_item_id, created_at)
WHERE list_item_id IS NOT NULL;
