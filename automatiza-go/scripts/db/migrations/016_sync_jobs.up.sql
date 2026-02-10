-- Migration: 016_sync_jobs.up.sql
-- Description: Table for tracking group synchronization jobs
-- Safe: checks column existence before creating indexes

CREATE TABLE IF NOT EXISTS sync_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    instance_id INTEGER REFERENCES instances(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    progress INTEGER DEFAULT 0,
    total INTEGER DEFAULT 0,
    error TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Safe indexes (only create if columns exist - handles both 000 and 016 schemas)
CREATE INDEX IF NOT EXISTS idx_sync_jobs_user_id ON sync_jobs(user_id);

DO $$
BEGIN
    -- 000 schema uses instance_id (INTEGER, nullable)
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sync_jobs' AND column_name = 'instance_id'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_sync_jobs_instance_id ON sync_jobs(instance_id);
    END IF;

    -- 000 schema uses status, old 016 schema uses state
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sync_jobs' AND column_name = 'status'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_sync_jobs_status ON sync_jobs(status);
    ELSIF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sync_jobs' AND column_name = 'state'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_sync_jobs_state ON sync_jobs(state) WHERE state IN ('pending', 'in_progress');
    END IF;

    -- Old schema has job_id
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sync_jobs' AND column_name = 'job_id'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_sync_jobs_job_id ON sync_jobs(job_id);
    END IF;
END $$;

-- Trigger for updated_at (safe: drop first)
DROP TRIGGER IF EXISTS update_sync_jobs_updated_at ON sync_jobs;
CREATE TRIGGER update_sync_jobs_updated_at
    BEFORE UPDATE ON sync_jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
