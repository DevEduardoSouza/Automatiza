-- 076: fundação histórica de membros de grupo (Melhoria 2, Fase A).
--
-- Aditiva e idempotente. NÃO altera nem remove a tabela legada `group_members`
-- (que guarda telefone bruto e não tem user_id) — ela continua intocada.
--
-- Três objetos novos:
--   group_member_events        ledger append-only de FATOS de webhook
--   group_memberships          projeção do estado atual, reconciliável
--   group_member_daily_snapshots agregado diário (bucket America/Sao_Paulo)
--
-- Privacidade: nenhum identificador bruto (id/pn/telefone) é persistido — só
-- HMAC-SHA256 com segredo dedicado (GROUP_MEMBER_HASH_SECRET). `hash_version`
-- torna o contrato explícito; o segredo é imutável após o primeiro rollout com
-- escrita (trocá-lo criaria identidades novas).

-- =============================================================================
-- Ledger — todo registro aqui é fato observado em webhook group.v2.participants
-- =============================================================================

CREATE TABLE IF NOT EXISTS group_member_events (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    group_id UUID NULL REFERENCES groups(id) ON DELETE SET NULL,
    wa_group_id VARCHAR(255) NOT NULL,
    instance_id INTEGER NULL REFERENCES instances(id) ON DELETE SET NULL,
    -- ID original do envelope WAHA (ULID minúsculo, prefixo evt_ opcional).
    provider_event_id VARCHAR(64) NOT NULL,
    -- timestamp (ms) do envelope: 2º critério de desempate da ordem total.
    provider_created_at TIMESTAMPTZ NOT NULL,
    participant_hash CHAR(64) NOT NULL,
    phone_hash CHAR(64) NULL,
    hash_version SMALLINT NOT NULL DEFAULT 1 CHECK (hash_version = 1),
    id_type VARCHAR(8) NOT NULL CHECK (id_type IN ('c_us', 'lid')),
    ddd CHAR(2) NULL CHECK (ddd IS NULL OR ddd ~ '^[0-9]{2}$'),
    event_type VARCHAR(16) NOT NULL CHECK (event_type IN ('join', 'leave', 'promote', 'demote')),
    role VARCHAR(16) NOT NULL CHECK (role IN ('left', 'participant', 'admin', 'superadmin')),
    -- timestamp (s) do payload: quando o fato aconteceu no WhatsApp.
    occurred_at TIMESTAMPTZ NOT NULL,
    -- quando NÓS aplicamos: guard anti-clobber da reconciliação concorrente.
    applied_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Chave idempotente por participante. O ULID do envelope é o "unique
    -- identifier for the event" do contrato WAHA, então redelivery é no-op e
    -- dois fatos legítimos do mesmo tipo no mesmo segundo continuam distintos.
    CONSTRAINT group_member_events_provider_participant_key
        UNIQUE (user_id, provider_event_id, participant_hash)
);

CREATE INDEX IF NOT EXISTS idx_group_member_events_user_group_occurred
    ON group_member_events (user_id, group_id, occurred_at)
    WHERE group_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_group_member_events_user_wagroup_occurred
    ON group_member_events (user_id, wa_group_id, occurred_at);

CREATE INDEX IF NOT EXISTS idx_group_member_events_user_phone_occurred
    ON group_member_events (user_id, phone_hash, occurred_at)
    WHERE phone_hash IS NOT NULL;

-- Retenção (cleanup_group_member_tracking, 180 dias).
CREATE INDEX IF NOT EXISTS idx_group_member_events_occurred_at
    ON group_member_events (occurred_at);

-- =============================================================================
-- Projeção — estado atual por (tenant, grupo, identidade)
-- =============================================================================

CREATE TABLE IF NOT EXISTS group_memberships (
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    group_id UUID NULL REFERENCES groups(id) ON DELETE SET NULL,
    wa_group_id VARCHAR(255) NOT NULL,
    participant_hash CHAR(64) NOT NULL,
    phone_hash CHAR(64) NULL,
    hash_version SMALLINT NOT NULL DEFAULT 1 CHECK (hash_version = 1),
    id_type VARCHAR(8) NOT NULL CHECK (id_type IN ('c_us', 'lid')),
    ddd CHAR(2) NULL CHECK (ddd IS NULL OR ddd ~ '^[0-9]{2}$'),
    role VARCHAR(16) NULL CHECK (role IS NULL OR role IN ('left', 'participant', 'admin', 'superadmin')),
    is_active BOOLEAN NOT NULL,
    -- NULL quando a entrada só foi VISTA (baseline de reconciliação): nunca
    -- inventamos o instante de entrada/saída que não observamos.
    joined_at TIMESTAMPTZ NULL,
    left_at TIMESTAMPTZ NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    first_seen_source VARCHAR(16) NOT NULL
        CHECK (first_seen_source IN ('webhook', 'reconciliation')),
    -- Ordem total aplicada: (last_event_at, last_provider_created_at, last_provider_event_id).
    last_event_at TIMESTAMPTZ NULL,
    last_provider_created_at TIMESTAMPTZ NULL,
    last_provider_event_id VARCHAR(64) NULL,
    last_event_type VARCHAR(16) NULL,
    last_event_applied_at TIMESTAMPTZ NULL,
    last_reconciled_at TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, wa_group_id, participant_hash)
);

CREATE INDEX IF NOT EXISTS idx_group_memberships_active
    ON group_memberships (user_id, wa_group_id, participant_hash)
    WHERE is_active;

CREATE INDEX IF NOT EXISTS idx_group_memberships_active_phone
    ON group_memberships (user_id, phone_hash)
    WHERE is_active AND phone_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_group_memberships_active_group
    ON group_memberships (user_id, group_id)
    WHERE is_active AND group_id IS NOT NULL;

-- =============================================================================
-- Agregado diário — mantido transacionalmente pelos dois writers legítimos
-- =============================================================================

CREATE TABLE IF NOT EXISTS group_member_daily_snapshots (
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    group_id UUID NULL REFERENCES groups(id) ON DELETE SET NULL,
    wa_group_id VARCHAR(255) NOT NULL,
    -- Bucket local: America/Sao_Paulo, sempre.
    snapshot_date DATE NOT NULL,
    -- Observação PONTUAL (não "fim do dia"): quantidade de projeções ativas no
    -- instante members_observed_at. NULL enquanto o grupo não tiver baseline.
    members_count INTEGER NULL CHECK (members_count IS NULL OR members_count >= 0),
    members_observed_at TIMESTAMPTZ NULL,
    joins_count INTEGER NOT NULL DEFAULT 0 CHECK (joins_count >= 0),
    leaves_count INTEGER NOT NULL DEFAULT 0 CHECK (leaves_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, wa_group_id, snapshot_date)
);

CREATE INDEX IF NOT EXISTS idx_group_member_daily_snapshots_user_date
    ON group_member_daily_snapshots (user_id, snapshot_date, wa_group_id);

-- Retenção (400 dias).
CREATE INDEX IF NOT EXISTS idx_group_member_daily_snapshots_date
    ON group_member_daily_snapshots (snapshot_date);

-- =============================================================================
-- Controle de reconciliação em `groups`
-- =============================================================================

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS member_tracking_started_at TIMESTAMPTZ NULL;

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS members_reconciled_at TIMESTAMPTZ NULL;

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS members_reconcile_claimed_at TIMESTAMPTZ NULL;

-- Ordem do claim: nunca reconciliado primeiro, depois o mais antigo.
CREATE INDEX IF NOT EXISTS idx_groups_member_reconcile_claim
    ON groups (members_reconcile_claimed_at, members_reconciled_at ASC NULLS FIRST, id);
