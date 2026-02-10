-- Rollback: 008_jobs_table.sql
DROP TRIGGER IF EXISTS update_jobs_updated_at ON jobs;
DROP TABLE IF EXISTS jobs;
