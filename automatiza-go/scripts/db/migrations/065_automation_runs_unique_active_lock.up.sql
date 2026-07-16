-- Exclusão mútua REAL do lock de execução de automação (L1 da auditoria).
--
-- AcquireLock fazia check-then-insert sem constraint única: duas transações
-- concorrentes (cron + "Rodar agora" da UI) viam "sem lock" e AMBAS inseriam
-- uma linha de lock — a automação rodava 2x e o LIST entregava o mesmo item
-- duas vezes. O índice UNIQUE parcial abaixo garante no máximo UMA linha de
-- lock ativo (lock_until IS NOT NULL) por automação; o AcquireLock passa a
-- usar INSERT ... ON CONFLICT DO NOTHING e trata conflito como "lock ocupado".

-- Saneamento pré-índice: locks EXPIRADOS mas não liberados (run morto antes do
-- release; o reaper diário limpa com atraso) não podem bloquear o índice nem o
-- próximo acquire. Mesma semântica do ReapStaleRunningLocks.
UPDATE automation_runs
SET status = 'failed', lock_until = NULL, completed_at = COALESCE(completed_at, NOW())
WHERE status = 'running' AND lock_until IS NOT NULL AND lock_until <= NOW();

UPDATE automation_runs
SET lock_until = NULL
WHERE lock_until IS NOT NULL AND lock_until <= NOW();

-- Se a própria race já tiver deixado 2+ locks ATIVOS para a mesma automação,
-- mantém só o mais recente (o run mais novo é o que está de fato executando).
UPDATE automation_runs
SET lock_until = NULL, status = 'failed', completed_at = COALESCE(completed_at, NOW())
WHERE id IN (
    SELECT id FROM (
        SELECT id, ROW_NUMBER() OVER (PARTITION BY automation_id ORDER BY lock_until DESC, started_at DESC) AS rn
        FROM automation_runs
        WHERE lock_until IS NOT NULL
    ) ranked
    WHERE ranked.rn > 1
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_automation_runs_active_lock
ON automation_runs (automation_id)
WHERE lock_until IS NOT NULL;
