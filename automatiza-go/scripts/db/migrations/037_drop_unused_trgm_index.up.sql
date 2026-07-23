-- automatiza:migration-mode=transactional
-- Tombstone for the retired idx_dispatch_logs_product_url_trgm migration.
--
-- Never drop an unexpected valid/invalid index from the automatic deploy path.
-- The WP-002 operator runbook owns diagnosis and DROP INDEX CONCURRENTLY.
SELECT 1;
