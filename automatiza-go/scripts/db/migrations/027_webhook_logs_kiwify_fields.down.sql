DROP INDEX IF EXISTS idx_webhook_logs_email;
ALTER TABLE webhook_logs DROP COLUMN IF EXISTS plan;
ALTER TABLE webhook_logs DROP COLUMN IF EXISTS email;
