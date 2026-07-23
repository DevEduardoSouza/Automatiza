-- automatiza:migration-mode=nontransactional
-- Performance indexes for dispatch_logs queries.
--
-- The retired product_url trigram index must not return here. Migration 037
-- removed it after production evidence showed no useful workload, and the old
-- runner otherwise rebuilt and dropped it on every deploy.
SET lock_timeout = '5s';
SET statement_timeout = '10min';

-- Adds missing indexes for the automation-logs page filters and pagination.
-- Existing indexes from 001_initial_schema.sql:
--   idx_dispatch_logs_user_id (user_id)
--   idx_dispatch_logs_automation_id (automation_id)
--   idx_dispatch_logs_status (status)
--   idx_dispatch_logs_canonical_key (canonical_key)
--   idx_dispatch_logs_created_at (created_at)
--   idx_dispatch_logs_dedup (user_id, group_id, canonical_key, created_at)

-- Compound index for main listing query (user_id + created_at DESC)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dispatch_logs_user_created
  ON dispatch_logs(user_id, created_at DESC);

-- Index for group filter
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dispatch_logs_group
  ON dispatch_logs(group_id);

-- Index for store filter
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dispatch_logs_store
  ON dispatch_logs(store);

RESET statement_timeout;
RESET lock_timeout;
