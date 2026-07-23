-- Verifier da 078. O runner só marca a migração como aplicada se
-- migration_verified vier true; qualquer peça faltando bloqueia o deploy em vez
-- de deixar o código novo subir sobre um schema incompleto (o precedente 23514
-- da migração 071).
--
-- Confere as quatro tabelas, a coluna de cota em plans, as duas travas de
-- unicidade que impedem resposta duplicada, e os três índices CONCURRENTLY —
-- estes últimos exigindo indisvalid, porque um CREATE INDEX CONCURRENTLY que
-- falha no meio deixa um índice INVÁLIDO e silencioso.
SELECT COALESCE((
    SELECT
        -- tabelas novas
        (SELECT count(*) FROM information_schema.tables
          WHERE table_schema = 'public'
            AND table_name IN ('instagram_apps', 'instagram_ai_configs',
                               'instagram_inbound_events', 'instagram_replies')) = 4
        -- coluna de cota
        AND EXISTS (SELECT 1 FROM information_schema.columns
                     WHERE table_schema = 'public' AND table_name = 'plans'
                       AND column_name = 'max_ai_replies_per_day')
        -- travas de unicidade
        AND EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uq_instagram_replies_event')
        AND EXISTS (SELECT 1 FROM pg_constraint
                     WHERE conname = 'instagram_inbound_events_provider_event_id_key')
        AND EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'instagram_apps_webhook_token_key')
        -- índices válidos (CONCURRENTLY pode deixar índice inválido)
        AND (SELECT count(*)
               FROM pg_index i
               JOIN pg_class idx ON idx.oid = i.indexrelid
               JOIN pg_namespace ns ON ns.oid = idx.relnamespace
              WHERE ns.nspname = 'public'
                AND i.indisvalid AND i.indisready
                AND idx.relname IN ('idx_instagram_inbound_events_user_recent',
                                    'idx_instagram_inbound_events_retention',
                                    'idx_instagram_replies_user_recent')) = 3
), false) AS migration_verified
\gset
