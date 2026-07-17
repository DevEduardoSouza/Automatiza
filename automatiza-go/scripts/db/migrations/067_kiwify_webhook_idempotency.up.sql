-- Idempotência + proteção contra processamento duplicado dos webhooks Kiwify.
--
-- O webhook é processado de forma síncrona nos handlers HTTP, sem nenhuma trava
-- contra reentrega. A Kiwify reenvia o mesmo evento em timeout/erro; sem isto,
-- um reenvio de "compra aprovada" recriava usuário/reenviava e-mail, e um
-- reenvio de "reembolso" re-suspendia. A PRIMARY KEY (idempotency_key) faz o
-- INSERT ... ON CONFLICT DO NOTHING servir de trava atômica: a 1ª entrega
-- "reivindica" a chave e processa; entregas concorrentes/repetidas veem
-- conflito e são ignoradas. O status permite reprocessar em caso de falha
-- (a entrega que falha faz DELETE da chave, liberando o retry da Kiwify).
CREATE TABLE IF NOT EXISTS kiwify_webhook_events (
    idempotency_key TEXT PRIMARY KEY,
    email           TEXT NOT NULL,
    event           TEXT NOT NULL,
    user_id         UUID,
    status          TEXT NOT NULL DEFAULT 'processing', -- processing | done
    result          TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_kiwify_webhook_events_email
    ON kiwify_webhook_events (email, created_at DESC);
