-- Reverte a 078. APENAS para banco efêmero/local (teste de schema).
--
-- Em produção a rota é desligar os kill switches (INSTAGRAM_AI_REPLY_ENABLED e
-- instagram_ai_configs.is_active) — nunca rodar isto: apaga o app da Meta que
-- cada cliente cadastrou, e recadastrar exige que ele volte ao painel de
-- desenvolvedores da Meta.
DROP INDEX IF EXISTS idx_instagram_replies_user_recent;
DROP INDEX IF EXISTS idx_instagram_inbound_events_retention;
DROP INDEX IF EXISTS idx_instagram_inbound_events_user_recent;

DROP TABLE IF EXISTS instagram_replies;
DROP TABLE IF EXISTS instagram_inbound_events;
DROP TABLE IF EXISTS instagram_ai_configs;
DROP TABLE IF EXISTS instagram_apps;

ALTER TABLE plans DROP COLUMN IF EXISTS max_ai_replies_per_day;
