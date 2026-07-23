-- Tombstone: the retired product_url GIN must never be recreated by rollback.
-- Exceptional recovery is operator-only; see operations/wp-002 and its runbook.
SELECT 1;
