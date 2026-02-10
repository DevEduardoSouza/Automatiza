-- Rollback: Remove performance indexes for dispatch_logs

DROP INDEX IF EXISTS idx_dispatch_logs_product_url_trgm;
DROP INDEX IF EXISTS idx_dispatch_logs_store;
DROP INDEX IF EXISTS idx_dispatch_logs_group;
DROP INDEX IF EXISTS idx_dispatch_logs_user_created;
