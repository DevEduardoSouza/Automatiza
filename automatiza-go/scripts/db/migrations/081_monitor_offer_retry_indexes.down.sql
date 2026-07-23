-- Down da 081 — SOMENTE banco local descartável.
SET lock_timeout = '3s';

DROP INDEX CONCURRENTLY IF EXISTS monitor_offer_jobs_terminal_idx;
DROP INDEX CONCURRENTLY IF EXISTS monitor_offer_items_canonical_key_uidx;
DROP INDEX CONCURRENTLY IF EXISTS monitor_offer_items_pending_idx;
DROP INDEX CONCURRENTLY IF EXISTS monitor_offer_items_job_ordinal_idx;
DROP INDEX CONCURRENTLY IF EXISTS monitor_offer_deliveries_sending_idx;
