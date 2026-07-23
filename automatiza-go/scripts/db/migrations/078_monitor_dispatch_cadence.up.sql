-- automatiza:migration-mode=transactional
-- Cadência durável entre mensagens espelhadas (Melhoria 7).
--
-- O que isto NÃO é: um sleep. O modo monitoring é event-driven (o ritmo é o do
-- grupo-fonte), então "intervalo mínimo entre mensagens" só existe se a mensagem
-- aceita sobreviver a deploy, crash e limpeza do Redis. Daí um ledger em
-- PostgreSQL — a única primitiva de verdade aqui: Redis/Asynq podem ser zerados
-- e o poller ainda recupera todo pending não terminal.
--
-- Quatro objetos novos + uma coluna. Tudo aditivo: com
-- monitor_interval_minutes = 0 (default) e a feature flag desligada, NADA nesta
-- migração é lido ou escrito e o espelho segue exatamente como hoje.
--
-- Por que cada peça existe:
--
--   automation_monitor_dispatch_state  — sequência e "próximo horário livre" POR
--     automação. É a linha travada (FOR UPDATE) que serializa admissões
--     concorrentes de duas réplicas; automações distintas não se bloqueiam.
--
--   automation_monitor_payloads        — a cópia DURÁVEL da mensagem-fonte,
--     incluindo os bytes da mídia. media.url/media.s3 da WAHA são fontes de
--     AQUISIÇÃO (o contrato não promete retenção), nunca a referência guardada:
--     um item agendado para daqui a 40min não pode depender de um arquivo que
--     expira.
--
--   automation_monitor_pending         — um item da fila por (automação,
--     mensagem). sequence_no é imutável e é a ORDEM; not_before_at é o horário,
--     que muda (retry, janela fechada). Separar os dois é o que impede o item 1
--     reagendado de ultrapassar o item 2.
--
--   ..._pending_destinations           — ledger POR GRUPO DE DESTINO. Sem ele,
--     crash depois de enviar para 3 de 10 grupos obriga a repetir os 3 (promoção
--     duplicada) ou a perder os 7. Com ele, cada destino tem desfecho próprio;
--     'sending' sem conclusão vira 'assumed_sent' e NUNCA é reenviado — mesma
--     política de entrega incerta que o espelho já pratica hoje
--     (monitoring_service.go, IsAmbiguousOutcome).
SET lock_timeout = '5s';
SET statement_timeout = '10min';

-- =============================================================================
-- Coluna de configuração
-- =============================================================================

ALTER TABLE automations
    ADD COLUMN IF NOT EXISTS monitor_interval_minutes INTEGER NOT NULL DEFAULT 0;

-- NOT VALID + VALIDATE em bloco idempotente (padrão das migrações anteriores):
-- re-executar a migração num banco onde a constraint já existe não pode falhar.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'chk_automations_monitor_interval_minutes'
    ) THEN
        ALTER TABLE automations
            ADD CONSTRAINT chk_automations_monitor_interval_minutes
            CHECK (monitor_interval_minutes BETWEEN 0 AND 1440) NOT VALID;
        ALTER TABLE automations
            VALIDATE CONSTRAINT chk_automations_monitor_interval_minutes;
    END IF;
END
$$;

-- Alvo do FK composto (automation_id, user_id) das tabelas abaixo. É esse par
-- que torna impossível um pending apontar para a automação de OUTRO dono.
CREATE UNIQUE INDEX IF NOT EXISTS uq_automations_id_user
    ON automations(id, user_id);

-- =============================================================================
-- Estado por automação: sequência + próximo horário livre
-- =============================================================================

CREATE TABLE IF NOT EXISTS automation_monitor_dispatch_state (
    automation_id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    next_sequence BIGINT NOT NULL DEFAULT 1 CHECK (next_sequence > 0),
    -- Reserva: horário a partir do qual a PRÓXIMA mensagem admitida pode sair.
    next_available_at TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp(),
    -- Quando o último item começou a despachar DE FATO. É o que impede rajada
    -- de catch-up: depois de downtime, o sucessor conta o intervalo a partir do
    -- início real do anterior, não do horário reservado que já passou.
    last_dispatch_started_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp(),
    CONSTRAINT fk_monitor_state_automation_owner
        FOREIGN KEY (automation_id, user_id)
        REFERENCES automations(id, user_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_monitor_state_user
    ON automation_monitor_dispatch_state(user_id, automation_id);

-- =============================================================================
-- Payload durável da mensagem-fonte (compartilhado por N automações)
-- =============================================================================

CREATE TABLE IF NOT EXISTS automation_monitor_payloads (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    source_session_name TEXT NOT NULL,
    source_chat_id TEXT NOT NULL,
    source_message_id TEXT NOT NULL,
    content_sha256 CHAR(64) NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp(),
    -- Apenas os campos de IncomingMessage necessários para reconstruir o
    -- disparo. NUNCA media.url, media.s3, base64 ou payload WAHA bruto.
    message_snapshot JSONB NOT NULL,
    media_mimetype TEXT,
    media_filename TEXT,
    media_sha256 CHAR(64),
    media_size_bytes INTEGER CHECK (
        media_size_bytes IS NULL OR media_size_bytes BETWEEN 1 AND 15728640
    ),
    media_bytes BYTEA,
    media_purged_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp(),
    -- Redelivery do mesmo webhook (a WAHA entrega o mesmo key.id por N chips do
    -- mesmo dono) encontra esta unique e NÃO consome slot novo.
    CONSTRAINT uq_monitor_payload_source
        UNIQUE (user_id, source_chat_id, source_message_id),
    -- Ou os três campos de mídia existem e são coerentes, ou nenhum existe.
    -- Impede "mídia meio gravada" virar item agendado sem imagem.
    CONSTRAINT chk_monitor_payload_media_complete CHECK (
        (media_bytes IS NULL AND media_size_bytes IS NULL AND media_sha256 IS NULL)
        OR
        (media_bytes IS NOT NULL
            AND media_size_bytes = octet_length(media_bytes)
            AND octet_length(media_bytes) BETWEEN 1 AND 15728640
            AND media_sha256 IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_monitor_payload_content_dedup
    ON automation_monitor_payloads(user_id, source_chat_id, content_sha256, accepted_at DESC);
CREATE INDEX IF NOT EXISTS idx_monitor_payload_cleanup
    ON automation_monitor_payloads(created_at, id);

-- =============================================================================
-- Fila por automação
-- =============================================================================

CREATE TABLE IF NOT EXISTS automation_monitor_pending (
    id BIGSERIAL PRIMARY KEY,
    -- FK viva: vira NULL quando a automação é apagada (ver FK abaixo).
    automation_id UUID,
    -- Cópia imutável: é por ela que a fila, a ordem e a auditoria continuam
    -- fazendo sentido depois que a automação some.
    automation_id_snapshot UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    payload_id BIGINT NOT NULL
        REFERENCES automation_monitor_payloads(id) ON DELETE CASCADE,
    sequence_no BIGINT NOT NULL CHECK (sequence_no > 0),
    interval_minutes_snapshot INTEGER NOT NULL
        CHECK (interval_minutes_snapshot BETWEEN 0 AND 1440),
    window_snapshot JSONB NOT NULL,
    delay_seconds_snapshot INTEGER NOT NULL CHECK (delay_seconds_snapshot >= 0),
    -- Horário RESERVADO na admissão (imutável, auditoria).
    scheduled_at TIMESTAMPTZ NOT NULL,
    -- Horário efetivo de liberação (muda em retry/janela fechada).
    not_before_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (
        status IN (
            'pending','enqueuing','enqueued','processing',
            'completed','failed','canceled'
        )
    ),
    -- Geração + token matam ABA: uma task velha reemitida não pode mexer numa
    -- lease nova (o CAS não casa e ela vira no-op).
    enqueue_generation INTEGER NOT NULL DEFAULT 0 CHECK (enqueue_generation >= 0),
    enqueue_token UUID,
    enqueue_claimed_at TIMESTAMPTZ,
    enqueued_at TIMESTAMPTZ,
    processing_attempts INTEGER NOT NULL DEFAULT 0 CHECK (processing_attempts >= 0),
    processing_token UUID,
    processing_started_at TIMESTAMPTZ,
    first_started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    result JSONB NOT NULL DEFAULT '{}'::jsonb,
    -- Código fechado (enum), nunca texto livre: erro bruto pode carregar URL,
    -- grupo ou corpo da mensagem.
    last_error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp(),
    CONSTRAINT uq_monitor_pending_message
        UNIQUE (automation_id_snapshot, payload_id),
    CONSTRAINT uq_monitor_pending_sequence
        UNIQUE (automation_id_snapshot, sequence_no),
    -- Apagar a automação NÃO apaga a fila: zera o vínculo e deixa o
    -- maintenance cancelar de forma auditável. CASCADE aqui destruiria o
    -- histórico de por que aqueles envios não saíram.
    CONSTRAINT fk_monitor_pending_automation_owner
        FOREIGN KEY (automation_id, user_id)
        REFERENCES automations(id, user_id)
        ON DELETE SET NULL (automation_id),
    CONSTRAINT chk_monitor_pending_enqueue_lease CHECK (
        status NOT IN ('enqueuing','enqueued')
        OR (enqueue_token IS NOT NULL AND enqueue_claimed_at IS NOT NULL)
    ),
    CONSTRAINT chk_monitor_pending_processing_lease CHECK (
        status <> 'processing'
        OR (processing_token IS NOT NULL AND processing_started_at IS NOT NULL)
    )
);

-- Head da fila: só o MENOR sequence_no não terminal de cada automação pode ser
-- enfileirado. Este índice é o que torna essa escolha barata.
CREATE INDEX IF NOT EXISTS idx_monitor_pending_head
    ON automation_monitor_pending(automation_id_snapshot, sequence_no)
    WHERE status IN ('pending','enqueuing','enqueued','processing');
CREATE INDEX IF NOT EXISTS idx_monitor_pending_due
    ON automation_monitor_pending(not_before_at, automation_id_snapshot, sequence_no)
    WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_monitor_pending_enqueue_lease
    ON automation_monitor_pending(enqueue_claimed_at, id)
    WHERE status IN ('enqueuing','enqueued');
CREATE INDEX IF NOT EXISTS idx_monitor_pending_processing_lease
    ON automation_monitor_pending(processing_started_at, id)
    WHERE status = 'processing';
CREATE INDEX IF NOT EXISTS idx_monitor_pending_user_summary
    ON automation_monitor_pending(user_id, automation_id_snapshot, status, sequence_no);
CREATE INDEX IF NOT EXISTS idx_monitor_pending_cleanup
    ON automation_monitor_pending(completed_at, id)
    WHERE status IN ('completed','failed','canceled');

-- =============================================================================
-- Ledger por destino
-- =============================================================================

CREATE TABLE IF NOT EXISTS automation_monitor_pending_destinations (
    id BIGSERIAL PRIMARY KEY,
    pending_id BIGINT NOT NULL
        REFERENCES automation_monitor_pending(id) ON DELETE CASCADE,
    -- Ordem congelada do fan-out (o shuffle anti-ban acontece UMA vez, na
    -- materialização): retry não re-embaralha e não pula ninguém.
    ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
    destination_group_id TEXT NOT NULL,
    canonical_key TEXT,
    dispatch_snapshot JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (
        status IN (
            'pending','claimed','sending','sent','assumed_sent',
            'skipped','failed','canceled'
        )
    ),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    not_before_at TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp(),
    claim_token UUID,
    claimed_at TIMESTAMPTZ,
    -- Fronteira do "não sei se entregou": marcada ANTES do POST.
    sending_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    dispatch_log_id BIGINT,
    outcome_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp(),
    CONSTRAINT uq_monitor_destination_group UNIQUE (pending_id, destination_group_id),
    CONSTRAINT uq_monitor_destination_ordinal UNIQUE (pending_id, ordinal),
    CONSTRAINT chk_monitor_destination_claim CHECK (
        status NOT IN ('claimed','sending')
        OR (claim_token IS NOT NULL AND claimed_at IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_monitor_destination_work
    ON automation_monitor_pending_destinations(pending_id, ordinal, status, not_before_at);
CREATE INDEX IF NOT EXISTS idx_monitor_destination_claimed
    ON automation_monitor_pending_destinations(claimed_at, id)
    WHERE status = 'claimed';
CREATE INDEX IF NOT EXISTS idx_monitor_destination_sending
    ON automation_monitor_pending_destinations(sending_at, id)
    WHERE status = 'sending';
