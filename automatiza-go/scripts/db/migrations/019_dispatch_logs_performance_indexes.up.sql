-- Performance indexes for dispatch_logs queries
-- Adds missing indexes for the automation-logs page filters and pagination.
-- Existing indexes from 001_initial_schema.sql:
--   idx_dispatch_logs_user_id (user_id)
--   idx_dispatch_logs_automation_id (automation_id)
--   idx_dispatch_logs_status (status)
--   idx_dispatch_logs_canonical_key (canonical_key)
--   idx_dispatch_logs_created_at (created_at)
--   idx_dispatch_logs_dedup (user_id, group_id, canonical_key, created_at)

-- Compound index for main listing query (user_id + created_at DESC)
CREATE INDEX IF NOT EXISTS idx_dispatch_logs_user_created
  ON dispatch_logs(user_id, created_at DESC);

-- Index for group filter
CREATE INDEX IF NOT EXISTS idx_dispatch_logs_group
  ON dispatch_logs(group_id);

-- Index for store filter
CREATE INDEX IF NOT EXISTS idx_dispatch_logs_store
  ON dispatch_logs(store);

-- Trigram index for ILIKE search on product_url
-- Requires pg_trgm extension
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_dispatch_logs_product_url_trgm
  ON dispatch_logs USING gin(product_url gin_trgm_ops);
