-- automatiza:migration-mode=nontransactional
-- Instagram como canal completo: app da Meta POR CLIENTE + entrada (webhook) +
-- resposta da IA. Puramente aditiva — quatro tabelas novas e uma coluna nova em
-- plans. Nada aqui é lido por código antes da Fase 1 subir; as tabelas nascem
-- vazias e inertes.
--
-- Por que app por cliente: a doc de App Review da Meta isenta de análise o app
-- criado "para um negócio que eu possuo ou gerencio" (Standard Access). Com um
-- app nosso atendendo conta de terceiro, seria Advanced Access + App Review.
-- Guardar o app de cada cliente é o que permite lançar sem depender da Meta.
SET lock_timeout = '5s';
SET statement_timeout = '10min';

-- instagram_apps: as credenciais do app da Meta de cada cliente.
--
-- webhook_token é o coração do desenho. A Meta assina o corpo do webhook com o
-- app secret, mas quem diz de qual cliente é o evento está DENTRO do corpo —
-- ovo e galinha. Dando a cada cliente uma URL própria
-- (/api/v1/instagram/webhook/<webhook_token>), o servidor sabe qual segredo usar
-- ANTES de ler qualquer byte do corpo.
--
-- O secret vive cifrado (mesmo cofre AES de instagram_credentials) e NUNCA volta
-- ao frontend: uma vez salvo, a UI só mostra que existe.
CREATE TABLE IF NOT EXISTS instagram_apps (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id              UUID NOT NULL UNIQUE,
    app_id               TEXT NOT NULL,
    encrypted_app_secret TEXT NOT NULL,
    webhook_token        CHAR(64) NOT NULL UNIQUE,
    verify_token         CHAR(64) NOT NULL,
    is_verified          BOOLEAN NOT NULL DEFAULT false,
    verified_at          TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- instagram_ai_configs: o "treinamento" que o cliente escreve.
--
-- Tudo nasce DESLIGADO (is_active e os três reply_* em false): salvar a
-- configuração não pode, sozinho, fazer a IA começar a responder em nome de
-- alguém. Ligar é ato explícito.
CREATE TABLE IF NOT EXISTS instagram_ai_configs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL UNIQUE,
    persona          TEXT NOT NULL DEFAULT '',
    business_context TEXT NOT NULL DEFAULT '',
    tone             TEXT NOT NULL DEFAULT '',
    blocked_topics   TEXT NOT NULL DEFAULT '',
    fallback_message TEXT NOT NULL DEFAULT '',
    reply_comments   BOOLEAN NOT NULL DEFAULT false,
    reply_dms        BOOLEAN NOT NULL DEFAULT false,
    reply_mentions   BOOLEAN NOT NULL DEFAULT false,
    is_active        BOOLEAN NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- instagram_inbound_events: ingestão idempotente, no molde de webhook_events
-- (migração 003). provider_event_id ÚNICO é obrigatório: a Meta reenvia o mesmo
-- evento quando não recebe o ACK a tempo, e sem esta trava o cliente responderia
-- o mesmo comentário duas ou três vezes.
--
-- content guarda o texto recebido porque a IA precisa dele para responder e o
-- histórico precisa dele para o cliente conferir. É dado de terceiro (quem
-- comentou/mandou DM), então tem retenção de 30 dias e NUNCA vai para log.
CREATE TABLE IF NOT EXISTS instagram_inbound_events (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL,
    provider_event_id TEXT NOT NULL UNIQUE,
    event_type        TEXT NOT NULL
        CHECK (event_type IN ('comment', 'message', 'mention')),
    ig_account_id     TEXT NOT NULL DEFAULT '',
    sender_id         TEXT NOT NULL DEFAULT '',
    thread_id         TEXT NOT NULL DEFAULT '',
    content           TEXT NOT NULL DEFAULT '',
    occurred_at       TIMESTAMPTZ,
    received_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    status            TEXT NOT NULL DEFAULT 'received'
        CHECK (status IN ('received', 'replied', 'skipped', 'failed')),
    error_code        TEXT NOT NULL DEFAULT ''
);

-- instagram_replies: o que a IA respondeu, e o custo disso.
--
-- O índice único (user_id, provider_event_id) é a SEGUNDA trava contra resposta
-- dupla — a primeira é o unique do inbound. Duas camadas porque aqui o efeito
-- colateral é público: uma resposta duplicada aparece no perfil do cliente.
--
-- prompt_tokens/completion_tokens existem porque a chave de IA é NOSSA: sem
-- medir, não há como cobrar nem como cortar um cliente que estourou o custo.
CREATE TABLE IF NOT EXISTS instagram_replies (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL,
    provider_event_id TEXT NOT NULL,
    event_type        TEXT NOT NULL DEFAULT '',
    reply_text        TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'sent', 'failed', 'skipped')),
    error_code        TEXT NOT NULL DEFAULT '',
    prompt_tokens     INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_instagram_replies_event UNIQUE (user_id, provider_event_id),
    CONSTRAINT instagram_replies_tokens_nonnegative
        CHECK (prompt_tokens >= 0 AND completion_tokens >= 0)
);

-- Cota diária de respostas da IA por plano. Hoje o serviço de IA não tem cota
-- nenhuma (internal/services/ai/ai_service.go); com resposta automática, um
-- único post viral de um cliente viraria custo ilimitado na nossa chave.
-- Default conservador: quem precisar de mais, sobe no plano.
ALTER TABLE plans
    ADD COLUMN IF NOT EXISTS max_ai_replies_per_day INTEGER NOT NULL DEFAULT 50;

-- Índices de consulta. CONCURRENTLY por política do runner; as tabelas nascem
-- vazias, então aqui é barato.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_instagram_inbound_events_user_recent
    ON instagram_inbound_events (user_id, received_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_instagram_inbound_events_retention
    ON instagram_inbound_events (received_at, id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_instagram_replies_user_recent
    ON instagram_replies (user_id, created_at DESC);
