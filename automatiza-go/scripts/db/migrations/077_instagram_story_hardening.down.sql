-- Reverte a 077 e deixa o schema exatamente como a 073 o entregou.
--
-- APENAS para banco efêmero/local (teste de schema). Em produção a rota é
-- desligar o kill switch INSTAGRAM_STORIES_ENABLED — nunca rodar este down:
-- ele descarta app_scoped_user_id e container_id, e sem eles o vínculo com a
-- conta da Meta e a reconciliação de publicação se perdem.
DROP TABLE IF EXISTS instagram_data_deletion_requests;

DROP INDEX IF EXISTS uq_instagram_story_posts_source_event;
DROP INDEX IF EXISTS idx_instagram_story_posts_retention;

ALTER TABLE instagram_story_posts
    DROP CONSTRAINT IF EXISTS instagram_story_posts_attempt_count_nonnegative;

ALTER TABLE instagram_story_posts
    DROP COLUMN IF EXISTS container_id,
    DROP COLUMN IF EXISTS processing_started_at,
    DROP COLUMN IF EXISTS attempt_count,
    DROP COLUMN IF EXISTS error_code,
    DROP COLUMN IF EXISTS published_reconciled;

DROP INDEX IF EXISTS uq_instagram_credentials_app_scoped_user_id;
DROP INDEX IF EXISTS uq_instagram_credentials_ig_user_id;

ALTER TABLE instagram_credentials
    DROP COLUMN IF EXISTS app_scoped_user_id;
