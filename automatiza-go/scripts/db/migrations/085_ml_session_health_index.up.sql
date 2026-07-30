-- automatiza:migration-mode=nontransactional
-- Índice da fila do health check de sessão do Mercado Livre.
--
-- Separado da 084 porque o guardrail GOV-SQL-002 exige CREATE INDEX
-- CONCURRENTLY em migração de rollout, e CONCURRENTLY não pode rodar dentro de
-- uma transação — a 084 é transactional por causa dos ALTER TABLE.
--
-- ADITIVA: só cria índice. Depende das colunas da 084, que roda antes.
--
-- Ordena a fila "quem está há mais tempo sem verificação". NULLS FIRST porque
-- credencial nunca verificada tem prioridade máxima. O índice é parcial
-- (exclui 'expired') porque uma sessão já sabidamente morta não é reverificada:
-- ela não ressuscita sozinha e só gastaria request autenticada contra o ML.

SET lock_timeout = '5s';

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ml_credentials_session_health
    ON ml_credentials (last_verified_at NULLS FIRST)
    WHERE session_status <> 'expired';
