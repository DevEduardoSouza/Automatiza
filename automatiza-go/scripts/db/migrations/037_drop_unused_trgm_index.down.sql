CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_dispatch_logs_product_url_trgm
  ON dispatch_logs USING gin(product_url gin_trgm_ops);
