-- Index for the dedup query: isDuplicate checks (user_id, group_id, canonical_key, status, created_at)
-- Without this index the query does a full table scan on 200K+ rows, causing timeouts
-- that silently bypass dedup and allow duplicate promotions.
-- Drop any previous version of this index (may have been created without the status column).
DROP INDEX IF EXISTS idx_dispatch_logs_dedup;
CREATE INDEX IF NOT EXISTS idx_dispatch_logs_dedup
    ON dispatch_logs (user_id, group_id, canonical_key, status, created_at DESC);
