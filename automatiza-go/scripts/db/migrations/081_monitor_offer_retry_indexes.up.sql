-- automatiza:migration-mode=nontransactional
-- 081: índices da Melhoria 9 (ledger de ofertas do monitor), fora da 079.
--
-- Mesma razão da 080: o guardrail exige CREATE INDEX CONCURRENTLY, que não roda
-- dentro de transação. A 079 fica transacional (cria 3 tabelas de forma atômica)
-- e os índices vêm aqui.
--
-- Idempotente: todos IF NOT EXISTS. Reexecutar é no-op.

SET lock_timeout = '3s';

CREATE INDEX CONCURRENTLY IF NOT EXISTS monitor_offer_jobs_terminal_idx
    ON monitor_offer_jobs (updated_at)
    WHERE status IN ('succeeded', 'exhausted', 'blocked', 'superseded', 'ambiguous');

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS monitor_offer_items_canonical_key_uidx
    ON monitor_offer_items (job_id, canonical_key)
    WHERE canonical_key IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS monitor_offer_items_pending_idx
    ON monitor_offer_items (status, next_attempt_at)
    WHERE status = 'pending';

CREATE INDEX CONCURRENTLY IF NOT EXISTS monitor_offer_items_job_ordinal_idx
    ON monitor_offer_items (job_id, ordinal);

CREATE INDEX CONCURRENTLY IF NOT EXISTS monitor_offer_deliveries_sending_idx
    ON monitor_offer_deliveries (status, sending_at)
    WHERE status = 'sending';
