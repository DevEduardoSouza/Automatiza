-- Migration: 027_webhook_logs_kiwify_fields.up.sql
-- Adds Kiwify-specific columns to webhook_logs
-- Safe: IF NOT EXISTS (all columns already exist in 000 schema)

ALTER TABLE webhook_logs ADD COLUMN IF NOT EXISTS kiwify_order_id VARCHAR(100);
ALTER TABLE webhook_logs ADD COLUMN IF NOT EXISTS kiwify_product_id VARCHAR(100);
ALTER TABLE webhook_logs ADD COLUMN IF NOT EXISTS kiwify_customer_email VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_kiwify ON webhook_logs(kiwify_order_id);
