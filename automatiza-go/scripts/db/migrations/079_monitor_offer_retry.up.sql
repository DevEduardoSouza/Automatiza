-- automatiza:migration-mode=transactional
-- Melhoria 9 — ledger de ofertas/campanhas do monitor com retry durável.
--
-- ADITIVA: só cria tabelas novas. Nenhum DROP, nenhum ALTER destrutivo, nenhuma
-- reescrita de tabela existente. Com os gates MONITOR_MIRROR_CONTENT e
-- MONITOR_PRODUCT_RETRY em 'off' (default), estas tabelas ficam VAZIAS e o
-- código não as toca — a migração é inerte até alguém ligar o gate.
--
-- NOTA DE NUMERAÇÃO: o plano reservava a 077, mas quando a implementação
-- começou a 077 (instagram_story_hardening) já estava commitada e havia
-- 077_instance_failover e 078_monitor_dispatch_cadence no working tree de outra
-- frente. Esta migração é a 079 para não colidir.
--
-- Por que o ledger não é dispatch_logs: dispatch_logs é histórico POR DESTINO,
-- volumosa (8.6GB) e seu status não representa "esta mensagem já produziu sua
-- oferta". Idempotência e recovery precisam de uma tabela pequena com unique
-- keys próprias; as duas convivem.

-- Teto curto de espera por lock (exigência do guardrail GOV-SQL-003).
-- Esta migração só cria tabelas novas, então na prática não disputa lock com
-- tráfego; o teto é rede de segurança.
SET lock_timeout = '3s';

SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- ---------------------------------------------------------------------------
-- Job: uma mensagem-fonte, para uma automação, num modo.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS monitor_offer_jobs (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id              UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    automation_id        UUID NOT NULL REFERENCES automations(id) ON DELETE CASCADE,
    mode                 TEXT NOT NULL CHECK (mode IN ('monitoring', 'offer_generate')),
    source_group_id      TEXT NOT NULL,
    message_id           TEXT NOT NULL,
    message_timestamp    TIMESTAMPTZ NOT NULL,
    selected_product_key TEXT NULL,
    -- AES-GCM v1 sobre internal/lib/crypto com a chave já existente
    -- (ML_CREDENTIALS_ENCRYPTION_KEY). Guarda o mínimo para refazer
    -- classificação e render num retry; expira em 6h.
    payload_ciphertext   TEXT NULL,
    payload_expires_at   TIMESTAMPTZ NOT NULL,
    reason_code          TEXT NOT NULL DEFAULT 'none',
    status               TEXT NOT NULL CHECK (
        status IN ('active', 'succeeded', 'exhausted', 'blocked', 'superseded', 'ambiguous')
    ),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- A identidade da mensagem. É ela que faz o redelivery do webhook ser
    -- no-op em vez de segunda oferta.
    CONSTRAINT monitor_offer_jobs_identity_key
        UNIQUE (user_id, automation_id, mode, source_group_id, message_id),

    CONSTRAINT monitor_offer_jobs_reason_code_check CHECK (reason_code IN (
        'none','timeout','dns','http_3xx','http_408','http_425','http_429',
        'http_4xx','http_5xx','invalid_response','credential_load_failed',
        'shopee_10020','shopee_10030','remote_rejected','preview_incomplete',
        'parser_inconclusive','response_too_large','invalid_signature',
        'missing_credentials','invalid_token','shopee_10035','shopee_11001',
        'cart','not_found','invalid_url','unsafe_scheme','unsafe_host',
        'private_address','redirect_escape','url_too_long','too_many_links',
        'unsupported_store','duplicate_link','excess_product',
        'automation_inactive','automation_owner_mismatch',
        'source_group_mismatch','mode_mismatch','outside_window',
        'no_destination','no_chip','already_sent','missing_message_id',
        'payload_too_large','payload_encrypt_failed','payload_decrypt_failed',
        'payload_expired','unsupported_media','media_unavailable',
        'media_fingerprint_unavailable','session_restricted',
        'not_participating','delivery_rejected','delivery_ambiguous'
    ))
);

-- Cleanup diário varre só o que já é terminal: índice parcial mantém o
-- varredor fora das linhas ativas.

-- ---------------------------------------------------------------------------
-- Item: um link comercial dentro do job.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS monitor_offer_items (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id               UUID NOT NULL REFERENCES monitor_offer_jobs(id) ON DELETE CASCADE,
    -- Posição da primeira ocorrência no corpo. O teto 20 é o mesmo
    -- models.MaxCommercialLinks: mensagem com mais links é bloqueada INTEIRA
    -- no job (too_many_links) em vez de virar linhas sem fim.
    ordinal              SMALLINT NOT NULL CHECK (ordinal >= 0 AND ordinal < 20),
    paragraph_ordinal    SMALLINT NOT NULL CHECK (paragraph_ordinal >= 0),
    -- SHA-256 completo. A URL crua NUNCA é persistida aqui: log mostra só os
    -- 8 primeiros bytes em hex.
    source_link_hash     CHAR(64) NOT NULL,
    host                 TEXT NOT NULL,
    -- Nasce NULL e pode ser preenchida depois da expansão. O unique parcial
    -- abaixo é o que funde duas ocorrências do mesmo produto.
    canonical_key        TEXT NULL,
    duplicate_of_item_id UUID NULL REFERENCES monitor_offer_items(id) ON DELETE SET NULL,
    store                TEXT NOT NULL CHECK (store IN ('shopee', 'mercadolivre', 'unsupported')),
    url_class            TEXT NOT NULL CHECK (
        url_class IN ('product', 'campaign_or_coupon', 'non_commerce', 'unsupported_commerce', 'unknown')
    ),
    error_class          TEXT NOT NULL CHECK (
        error_class IN ('none', 'transient', 'permanent_config', 'permanent_content', 'security_block')
    ),
    reason_code          TEXT NOT NULL DEFAULT 'none',
    output_key           TEXT NULL,
    output_kind          TEXT NULL CHECK (output_kind IN ('product_offer', 'campaign_notice')),
    outcome              TEXT NOT NULL DEFAULT 'none' CHECK (
        outcome IN ('none', 'converted', 'owner_passthrough', 'removed', 'included')
    ),
    status               TEXT NOT NULL CHECK (
        status IN ('pending', 'processing', 'succeeded', 'exhausted', 'blocked', 'superseded', 'ambiguous')
    ),
    attempts             SMALLINT NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
    total_attempts       INTEGER NOT NULL DEFAULT 0 CHECK (total_attempts >= 0),
    replay_count         INTEGER NOT NULL DEFAULT 0 CHECK (replay_count >= 0),
    -- Geração de enfileiramento: uma task com geração obsoleta é no-op. É o
    -- que torna seguro perder o ack do enqueue (o sweep cria geração nova).
    enqueue_generation   INTEGER NOT NULL DEFAULT 0,
    next_attempt_at      TIMESTAMPTZ NULL,
    lease_until          TIMESTAMPTZ NULL,
    attempt_started_at   TIMESTAMPTZ NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT monitor_offer_items_ordinal_key UNIQUE (job_id, ordinal),

    CONSTRAINT monitor_offer_items_reason_code_check CHECK (reason_code IN (
        'none','timeout','dns','http_3xx','http_408','http_425','http_429',
        'http_4xx','http_5xx','invalid_response','credential_load_failed',
        'shopee_10020','shopee_10030','remote_rejected','preview_incomplete',
        'parser_inconclusive','response_too_large','invalid_signature',
        'missing_credentials','invalid_token','shopee_10035','shopee_11001',
        'cart','not_found','invalid_url','unsafe_scheme','unsafe_host',
        'private_address','redirect_escape','url_too_long','too_many_links',
        'unsupported_store','duplicate_link','excess_product',
        'automation_inactive','automation_owner_mismatch',
        'source_group_mismatch','mode_mismatch','outside_window',
        'no_destination','no_chip','already_sent','missing_message_id',
        'payload_too_large','payload_encrypt_failed','payload_decrypt_failed',
        'payload_expired','unsupported_media','media_unavailable',
        'media_fingerprint_unavailable','session_restricted',
        'not_participating','delivery_rejected','delivery_ambiguous'
    ))
);

-- Funde duas ocorrências do mesmo produto no mesmo job. Parcial porque
-- canonical_key nasce NULL e vários itens podem ficar sem ela.

-- Fila do sweep: só linhas pending entram no índice.

-- ---------------------------------------------------------------------------
-- Delivery: a autoridade de idempotência POR DESTINO.
-- ---------------------------------------------------------------------------
-- Um claim no Redis não fecha o crash "WAHA aceitou e o processo morreu antes
-- do sent" — perder a chave permite duplicata. Esta linha, sim: a PK cobre
-- (job, saída, destino), e 'sending' órfã vira 'ambiguous', nunca pending.
CREATE TABLE IF NOT EXISTS monitor_offer_deliveries (
    job_id               UUID NOT NULL REFERENCES monitor_offer_jobs(id) ON DELETE CASCADE,
    output_key           TEXT NOT NULL,
    output_kind          TEXT NOT NULL CHECK (
        output_kind IN ('product_offer', 'campaign_notice', 'common_relay')
    ),
    destination_group_id TEXT NOT NULL,
    status               TEXT NOT NULL CHECK (
        status IN ('prepared', 'sending', 'sent', 'failed', 'ambiguous', 'skipped')
    ),
    attempt_token        UUID NULL,
    provider_message_id  TEXT NULL,
    reason_code          TEXT NOT NULL DEFAULT 'none',
    sending_at           TIMESTAMPTZ NULL,
    sent_at              TIMESTAMPTZ NULL,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (job_id, output_key, destination_group_id),

    CONSTRAINT monitor_offer_deliveries_reason_code_check CHECK (reason_code IN (
        'none','timeout','dns','http_3xx','http_408','http_425','http_429',
        'http_4xx','http_5xx','invalid_response','credential_load_failed',
        'shopee_10020','shopee_10030','remote_rejected','preview_incomplete',
        'parser_inconclusive','response_too_large','invalid_signature',
        'missing_credentials','invalid_token','shopee_10035','shopee_11001',
        'cart','not_found','invalid_url','unsafe_scheme','unsafe_host',
        'private_address','redirect_escape','url_too_long','too_many_links',
        'unsupported_store','duplicate_link','excess_product',
        'automation_inactive','automation_owner_mismatch',
        'source_group_mismatch','mode_mismatch','outside_window',
        'no_destination','no_chip','already_sent','missing_message_id',
        'payload_too_large','payload_encrypt_failed','payload_decrypt_failed',
        'payload_expired','unsupported_media','media_unavailable',
        'media_fingerprint_unavailable','session_restricted',
        'not_participating','delivery_rejected','delivery_ambiguous'
    ))
);

-- O reaper de 'sending' órfã (>3min vira ambiguous) varre só estas linhas.

