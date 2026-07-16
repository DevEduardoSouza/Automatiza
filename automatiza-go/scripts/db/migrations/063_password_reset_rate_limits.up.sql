-- Tabela de rate limit do fluxo de reset de senha (ProfilesService/RequestPasswordReset).
-- O repositório rate_limit_repository.go SEMPRE esperou esta tabela, mas nenhuma migração
-- a criava — em produção toda requisição de reset logava
-- 'relation "password_reset_rate_limits" does not exist (SQLSTATE 42P01)':
--   * checkRateLimit falha-aberto (permite, sem proteção real contra spam/enumeração)
--   * Increment falha e spama Sentry a cada reset
-- Colunas/constraint conforme as queries do repo (GetAttempts/Increment/Reset):
--   ON CONFLICT (identifier, identifier_type) exige a UNIQUE abaixo.
CREATE TABLE IF NOT EXISTS password_reset_rate_limits (
    id BIGSERIAL PRIMARY KEY,
    identifier VARCHAR(255) NOT NULL,
    identifier_type VARCHAR(16) NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 1,
    window_start TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT password_reset_rate_limits_identifier_uniq UNIQUE (identifier, identifier_type)
);

-- Busca por janela: WHERE identifier=$1 AND identifier_type=$2 AND window_start > NOW()-interval
CREATE INDEX IF NOT EXISTS idx_prrl_window ON password_reset_rate_limits (identifier, identifier_type, window_start);
