-- Migration: 021_jobs_retry.down.sql
-- Descrição: Remove colunas de retry do jobs

ALTER TABLE jobs DROP COLUMN IF EXISTS retry_count;
ALTER TABLE jobs DROP COLUMN IF EXISTS max_retries;
