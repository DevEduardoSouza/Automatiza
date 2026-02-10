-- Rollback: Remove partial index for unconsumed ML link list items

DROP INDEX CONCURRENTLY IF EXISTS idx_ml_link_list_items_unconsumed;
