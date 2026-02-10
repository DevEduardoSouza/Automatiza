-- Migration: 016_sync_jobs.down.sql
-- Description: Rollback sync_jobs table

DROP TRIGGER IF EXISTS update_sync_jobs_updated_at ON sync_jobs;
DROP TABLE IF EXISTS sync_jobs;
