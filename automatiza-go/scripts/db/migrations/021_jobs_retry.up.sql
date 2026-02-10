-- Migration: 021_jobs_retry.up.sql
-- Descrição: Adiciona colunas de retry ao jobs para backoff exponencial

ALTER TABLE jobs ADD COLUMN IF NOT EXISTS retry_count INT NOT NULL DEFAULT 0;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS max_retries INT NOT NULL DEFAULT 3;

COMMENT ON COLUMN jobs.retry_count IS 'Número de tentativas já executadas';
COMMENT ON COLUMN jobs.max_retries IS 'Número máximo de tentativas antes de marcar como failed';
