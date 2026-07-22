-- Reverte a 074: remove a tabela de subscriptions de Web Push. Só afeta dados
-- da própria feature (que nasce desligada); nada mais depende desta tabela.
DROP TABLE IF EXISTS web_push_subscriptions;
