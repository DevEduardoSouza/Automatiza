-- Reverte a 073: remove as tabelas do Instagram Stories. Só afeta dados da
-- própria feature (que nasce desligada); nada mais depende destas tabelas.
DROP TABLE IF EXISTS instagram_story_posts;
DROP TABLE IF EXISTS instagram_credentials;
