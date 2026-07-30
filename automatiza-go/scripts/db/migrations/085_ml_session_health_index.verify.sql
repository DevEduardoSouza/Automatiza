-- Verificador da 085 (exigido para migração nontransactional).
--
-- ATENÇÃO: arquivos .verify.sql RODAM EM PRODUÇÃO no deploy e travam o
-- pipeline quando migration_verified sai false. Manter estritamente como
-- verificação da 085 — nunca acrescentar aqui teste de invariante de negócio.
--
-- Checa indisvalid, não só existência: um CREATE INDEX CONCURRENTLY
-- interrompido deixa o índice INVALID, que o planner ignora enquanto a escrita
-- continua pagando por ele.
SELECT
    count(*) FILTER (WHERE i.indisvalid)     AS valid_indexes,
    count(*) FILTER (WHERE NOT i.indisvalid) AS invalid_indexes,
    (count(*) FILTER (WHERE i.indisvalid) = 1
     AND count(*) FILTER (WHERE NOT i.indisvalid) = 0) AS migration_verified
FROM pg_class c
JOIN pg_index i ON i.indexrelid = c.oid
WHERE c.relname = 'idx_ml_credentials_session_health' \gset
