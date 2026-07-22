-- Web Push (PWA): subscriptions de navegador por dispositivo (22/07).
-- Aditiva: tabela NOVA, nenhum ALTER destrutivo. A feature nasce desligada
-- (sem VAPID_PRIVATE_KEY no ambiente o backend nem envia nem escreve aqui).
CREATE TABLE IF NOT EXISTS web_push_subscriptions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL,
    endpoint     TEXT NOT NULL UNIQUE,
    p256dh       TEXT NOT NULL,
    auth         TEXT NOT NULL,
    user_agent   TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    failed_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_web_push_subscriptions_user
    ON web_push_subscriptions (user_id);
CREATE INDEX IF NOT EXISTS idx_web_push_subscriptions_failed
    ON web_push_subscriptions (failed_at)
    WHERE failed_at IS NOT NULL;
