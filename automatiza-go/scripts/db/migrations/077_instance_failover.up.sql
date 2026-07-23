-- 077: chip reserva e failover de instância WhatsApp (Melhoria 5).
--
-- Aditiva e idempotente. NÃO altera nenhuma tabela existente além de adicionar
-- uma UNIQUE em `instances` (necessária como alvo de FK composta) — nenhuma
-- coluna é removida, renomeada ou tem semântica alterada.
--
-- Sete objetos novos:
--   instance_failover_policies       política primária→reserva por usuário
--   instance_failover_roles          papel de cada chip (1 papel global por chip)
--   instance_failover_dry_runs       recibo append-only de ensaio (só GET no WAHA)
--   instance_failover_dry_run_items  cobertura calculada por automação
--   instance_failover_operations     operação mutável (failover/reversão)
--   instance_failover_operation_items snapshot antes/depois por automação
--   instance_failover_dispatch_leases lease efêmera do gate de produtor
--   instance_failover_events         ledger append-only de auditoria
--
-- Invariantes que o SCHEMA (não o código) garante:
--   * modo automático é impossível  → ck_failover_mode
--   * primária ≠ reserva            → ck_failover_distinct_instances
--   * política só entre chips do MESMO dono → FK composta (user_id, id)
--   * um chip ocupa no máximo UM papel no sistema → PK de instance_failover_roles
--   * no máximo UMA operação não terminal por usuário → uq_failover_one_nonterminal_per_user
--   * chip em qualquer papel não pode ser apagado → FK ON DELETE RESTRICT
--
-- Este arquivo NUNCA deve rodar contra produção nesta entrega.

-- =============================================================================
-- 4.1 — Ownership composta: alvo das FKs que provam "mesma conta"
-- =============================================================================
-- instances.id é global (SERIAL) e instances.user_id é UUID. Uma FK simples
-- para instances(id) provaria apenas que o chip existe, não que ele pertence ao
-- dono da política. A UNIQUE abaixo é o que torna a FK composta possível.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uq_instances_user_id_id'
          AND conrelid = 'instances'::regclass
    ) THEN
        ALTER TABLE instances
            ADD CONSTRAINT uq_instances_user_id_id UNIQUE (user_id, id);
    END IF;
END
$$;

-- =============================================================================
-- 4.2 — Política
-- =============================================================================
CREATE TABLE IF NOT EXISTS instance_failover_policies (
    id                           BIGSERIAL PRIMARY KEY,
    user_id                      UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    primary_instance_id          INTEGER NOT NULL,
    backup_instance_id           INTEGER NOT NULL,
    mode                         TEXT NOT NULL DEFAULT 'manual',
    state                        TEXT NOT NULL DEFAULT 'active',
    -- version é a versão de CONFIGURAÇÃO da política (optimistic locking do
    -- wire). Telemetria de probe NÃO a incrementa — senão um detector rodando a
    -- cada minuto invalidaria todo dry-run em andamento.
    version                      BIGINT NOT NULL DEFAULT 1,
    current_operation_id         UUID NULL,
    unhealthy_after_seconds      INTEGER NOT NULL DEFAULT 900,
    required_consecutive_probes  INTEGER NOT NULL DEFAULT 3,
    drain_deadline_seconds       INTEGER NOT NULL DEFAULT 120,
    proposal_ttl_seconds         INTEGER NOT NULL DEFAULT 1800,
    primary_unhealthy_since      TIMESTAMPTZ NULL,
    consecutive_unhealthy_probes INTEGER NOT NULL DEFAULT 0,
    last_probe_at                TIMESTAMPTZ NULL,
    last_probe_status            TEXT NULL,
    proposal_expires_at          TIMESTAMPTZ NULL,
    created_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_failover_primary_owner
        FOREIGN KEY (user_id, primary_instance_id)
        REFERENCES instances (user_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_failover_backup_owner
        FOREIGN KEY (user_id, backup_instance_id)
        REFERENCES instances (user_id, id) ON DELETE RESTRICT,
    CONSTRAINT ck_failover_distinct_instances
        CHECK (primary_instance_id <> backup_instance_id),
    -- 'automatic' é impossível no banco, não só no wire.
    CONSTRAINT ck_failover_mode
        CHECK (mode IN ('manual', 'assisted')),
    CONSTRAINT ck_failover_state
        CHECK (state IN ('active', 'proposed', 'switching', 'active_on_backup', 'reverting', 'failed')),
    -- Estado estável ⇒ sem operação corrente; estado de operação ⇒ operação
    -- corrente obrigatória. Elimina ABA e "policy switching sem dono".
    CONSTRAINT ck_failover_current_operation
        CHECK (
            (state IN ('active', 'proposed', 'active_on_backup') AND current_operation_id IS NULL)
            OR
            (state IN ('switching', 'reverting', 'failed') AND current_operation_id IS NOT NULL)
        ),
    CONSTRAINT ck_failover_version CHECK (version > 0),
    -- 900s dá ao monitor.py (*/5min) pelo menos três ciclos para recuperar a
    -- sessão antes de qualquer proposta.
    CONSTRAINT ck_failover_unhealthy_after
        CHECK (unhealthy_after_seconds BETWEEN 900 AND 86400),
    CONSTRAINT ck_failover_probe_count
        CHECK (required_consecutive_probes BETWEEN 3 AND 60),
    CONSTRAINT ck_failover_drain_deadline
        CHECK (drain_deadline_seconds BETWEEN 30 AND 1800),
    CONSTRAINT ck_failover_proposal_ttl
        CHECK (proposal_ttl_seconds BETWEEN 300 AND 86400),
    -- Um chip é primária de no máximo uma política e reserva de no máximo uma.
    CONSTRAINT uq_failover_primary_instance UNIQUE (primary_instance_id),
    CONSTRAINT uq_failover_backup_instance UNIQUE (backup_instance_id)
);

CREATE INDEX IF NOT EXISTS idx_failover_policies_user
    ON instance_failover_policies (user_id);

CREATE INDEX IF NOT EXISTS idx_failover_policies_detector
    ON instance_failover_policies (mode, state)
    WHERE mode = 'assisted';

-- =============================================================================
-- 4.3 — Papéis
-- =============================================================================
-- instance_id é a PK GLOBAL da tabela: é ela que torna impossível, no banco,
-- que um chip apareça em duas políticas (inclusive em papel oposto). Isso
-- elimina A↔B, cadeias A→B/B→C e blast radius compartilhado na v1.
CREATE TABLE IF NOT EXISTS instance_failover_roles (
    instance_id INTEGER PRIMARY KEY,
    policy_id   BIGINT NOT NULL REFERENCES instance_failover_policies(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL,
    role        TEXT NOT NULL CHECK (role IN ('primary', 'backup')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_failover_role_owner
        FOREIGN KEY (user_id, instance_id)
        REFERENCES instances (user_id, id) ON DELETE RESTRICT,
    CONSTRAINT uq_failover_role_per_policy UNIQUE (policy_id, role)
);

CREATE INDEX IF NOT EXISTS idx_failover_roles_policy
    ON instance_failover_roles (policy_id);

-- Coerência papéis × política, verificada no COMMIT (DEFERRABLE INITIALLY
-- DEFERRED) para que criar policy + 2 roles na mesma transação seja possível
-- em qualquer ordem.
--
-- O validador é uma função comum; cada tabela tem seu próprio wrapper de
-- trigger porque NEW/OLD têm tipos diferentes nas duas (a linha de policies não
-- tem policy_id).
CREATE OR REPLACE FUNCTION instance_failover_check_roles(p_policy_id BIGINT)
RETURNS VOID AS $$
DECLARE
    v_policy instance_failover_policies%ROWTYPE;
    v_count  INTEGER;
BEGIN
    IF p_policy_id IS NULL THEN
        RETURN;
    END IF;

    SELECT * INTO v_policy FROM instance_failover_policies WHERE id = p_policy_id;
    IF NOT FOUND THEN
        -- Política apagada na mesma transação (DisablePolicy → cascade nas
        -- roles): não há contrato a verificar.
        RETURN;
    END IF;

    SELECT count(*) INTO v_count
      FROM instance_failover_roles WHERE policy_id = p_policy_id;
    IF v_count <> 2 THEN
        RAISE EXCEPTION 'instance_failover_roles: policy % deve ter exatamente 2 papeis, tem %',
            p_policy_id, v_count;
    END IF;

    PERFORM 1 FROM instance_failover_roles r
     WHERE r.policy_id = p_policy_id
       AND r.role = 'primary'
       AND r.instance_id = v_policy.primary_instance_id
       AND r.user_id = v_policy.user_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'instance_failover_roles: papel primary da policy % nao aponta para primary_instance_id/user_id da politica',
            p_policy_id;
    END IF;

    PERFORM 1 FROM instance_failover_roles r
     WHERE r.policy_id = p_policy_id
       AND r.role = 'backup'
       AND r.instance_id = v_policy.backup_instance_id
       AND r.user_id = v_policy.user_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'instance_failover_roles: papel backup da policy % nao aponta para backup_instance_id/user_id da politica',
            p_policy_id;
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION instance_failover_roles_match_policy_fn()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM instance_failover_check_roles(OLD.policy_id);
    ELSE
        PERFORM instance_failover_check_roles(NEW.policy_id);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION instance_failover_policies_match_roles_fn()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM instance_failover_check_roles(NEW.id);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS instance_failover_roles_match_policy ON instance_failover_roles;
CREATE CONSTRAINT TRIGGER instance_failover_roles_match_policy
    AFTER INSERT OR UPDATE OR DELETE ON instance_failover_roles
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION instance_failover_roles_match_policy_fn();

-- Espelho do trigger acima do lado da POLÍTICA. Sem ele, um UPDATE que mexesse
-- em backup_instance_id sem reescrever as roles não dispararia verificação
-- nenhuma e deixaria papéis apontando para o chip antigo — o invariante da
-- seção 4.3 do plano ("papel backup aponta para policy.backup_instance_id")
-- ficaria dependendo só da disciplina do serviço. Mesmo validador, mesmo
-- momento (commit). DELETE não é coberto de propósito: a política já não existe.
DROP TRIGGER IF EXISTS instance_failover_policies_match_roles ON instance_failover_policies;
CREATE CONSTRAINT TRIGGER instance_failover_policies_match_roles
    AFTER INSERT OR UPDATE ON instance_failover_policies
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION instance_failover_policies_match_roles_fn();

-- =============================================================================
-- 4.4 — Dry-runs (recibo append-only; só GET no WAHA produziu isto)
-- =============================================================================
CREATE TABLE IF NOT EXISTS instance_failover_dry_runs (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- policy_id vira NULL se a política for removida; policy_id_snapshot
    -- preserva a identidade histórica para auditoria.
    policy_id          BIGINT NULL REFERENCES instance_failover_policies(id) ON DELETE SET NULL,
    policy_id_snapshot BIGINT NOT NULL,
    user_id            UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    direction          TEXT NOT NULL CHECK (direction IN ('failover', 'revert')),
    idempotency_key    TEXT NOT NULL,
    request_hash       CHAR(64) NOT NULL,
    policy_version     BIGINT NOT NULL,
    dry_run_ref        CHAR(64) NOT NULL,
    status             TEXT NOT NULL CHECK (status IN ('pass', 'blocked')),
    report             JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at         TIMESTAMPTZ NOT NULL,

    CONSTRAINT ck_failover_dry_run_key
        CHECK (octet_length(idempotency_key) BETWEEN 16 AND 128
               AND idempotency_key ~ '^[\x20-\x7E]+$'),
    CONSTRAINT ck_failover_dry_run_expiry CHECK (expires_at > created_at),
    CONSTRAINT uq_failover_dry_run_idempotency UNIQUE (user_id, direction, idempotency_key),
    CONSTRAINT uq_failover_dry_run_ref UNIQUE (user_id, dry_run_ref)
);

CREATE INDEX IF NOT EXISTS idx_failover_dry_runs_policy
    ON instance_failover_dry_runs (policy_id_snapshot, created_at DESC);

-- =============================================================================
-- 4.5 — Itens do dry-run
-- =============================================================================
CREATE TABLE IF NOT EXISTS instance_failover_dry_run_items (
    dry_run_id             UUID NOT NULL REFERENCES instance_failover_dry_runs(id) ON DELETE CASCADE,
    automation_id          UUID NULL REFERENCES automations(id) ON DELETE SET NULL,
    automation_id_snapshot UUID NOT NULL,
    config_fingerprint     CHAR(64) NOT NULL,
    snapshot_before        JSONB NOT NULL,
    snapshot_after         JSONB NOT NULL,
    required_source_groups TEXT[] NOT NULL DEFAULT '{}',
    required_target_groups TEXT[] NOT NULL DEFAULT '{}',
    admin_required_groups  TEXT[] NOT NULL DEFAULT '{}',
    missing_groups         TEXT[] NOT NULL DEFAULT '{}',
    missing_admin_groups   TEXT[] NOT NULL DEFAULT '{}',
    result                 TEXT NOT NULL CHECK (result IN ('covered', 'blocked', 'deleted_noop')),
    reason_codes           TEXT[] NOT NULL DEFAULT '{}',
    PRIMARY KEY (dry_run_id, automation_id_snapshot)
);

-- =============================================================================
-- 4.6 — Operações mutáveis
-- =============================================================================
CREATE TABLE IF NOT EXISTS instance_failover_operations (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_id            BIGINT NULL REFERENCES instance_failover_policies(id) ON DELETE SET NULL,
    policy_id_snapshot   BIGINT NOT NULL,
    user_id              UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    kind                 TEXT NOT NULL CHECK (kind IN ('failover', 'revert')),
    status               TEXT NOT NULL
        CHECK (status IN ('preparing', 'draining', 'verifying', 'completed', 'aborted', 'failed')),
    -- Estado ao qual a política volta se esta operação abortar. Gravado ANTES
    -- do gate fechar: recovery restaura sem inferir.
    return_policy_state  TEXT NOT NULL
        CHECK (return_policy_state IN ('active', 'proposed', 'active_on_backup')),
    idempotency_key      TEXT NOT NULL,
    request_hash         CHAR(64) NOT NULL,
    -- RESTRICT: o recibo do ensaio que autorizou a troca nunca some.
    dry_run_id           UUID NOT NULL REFERENCES instance_failover_dry_runs(id) ON DELETE RESTRICT,
    source_operation_id  UUID NULL REFERENCES instance_failover_operations(id) ON DELETE RESTRICT,
    start_policy_version BIGINT NOT NULL,
    gate_closed_at       TIMESTAMPTZ NULL,
    drain_deadline_at    TIMESTAMPTZ NULL,
    applied_at           TIMESTAMPTZ NULL,
    completed_at         TIMESTAMPTZ NULL,
    report               JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT ck_failover_operation_key
        CHECK (octet_length(idempotency_key) BETWEEN 16 AND 128
               AND idempotency_key ~ '^[\x20-\x7E]+$'),
    CONSTRAINT uq_failover_operation_idempotency UNIQUE (user_id, kind, idempotency_key),
    -- Alvo da FK composta de current_operation_id: prova que a operação
    -- referenciada pertence à MESMA política.
    CONSTRAINT uq_failover_operation_policy UNIQUE (policy_id, id)
);

-- No máximo UMA operação não terminal por usuário. Duas políticas do mesmo
-- dono não conseguem trocar chip ao mesmo tempo.
CREATE UNIQUE INDEX IF NOT EXISTS uq_failover_one_nonterminal_per_user
    ON instance_failover_operations (user_id)
    WHERE status IN ('preparing', 'draining', 'verifying');

CREATE INDEX IF NOT EXISTS idx_failover_operations_policy_created
    ON instance_failover_operations (policy_id, created_at DESC);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_failover_current_operation'
          AND conrelid = 'instance_failover_policies'::regclass
    ) THEN
        ALTER TABLE instance_failover_policies
            ADD CONSTRAINT fk_failover_current_operation
            FOREIGN KEY (id, current_operation_id)
            REFERENCES instance_failover_operations (policy_id, id)
            DEFERRABLE INITIALLY DEFERRED;
    END IF;
END
$$;

-- =============================================================================
-- 4.7 — Itens da operação (snapshot antes/depois — base do recovery)
-- =============================================================================
CREATE TABLE IF NOT EXISTS instance_failover_operation_items (
    operation_id           UUID NOT NULL REFERENCES instance_failover_operations(id) ON DELETE CASCADE,
    automation_id          UUID NULL REFERENCES automations(id) ON DELETE SET NULL,
    automation_id_snapshot UUID NOT NULL,
    fingerprint_before     CHAR(64) NOT NULL,
    fingerprint_after      CHAR(64) NOT NULL,
    snapshot_before        JSONB NOT NULL,
    snapshot_after         JSONB NOT NULL,
    result                 TEXT NOT NULL DEFAULT 'pending'
        CHECK (result IN ('pending', 'applied', 'restored', 'deleted_noop')),
    PRIMARY KEY (operation_id, automation_id_snapshot)
);

CREATE INDEX IF NOT EXISTS idx_failover_operation_items_automation
    ON instance_failover_operation_items (automation_id_snapshot);

-- =============================================================================
-- 4.8 — Leases de dispatch (efêmeras)
-- =============================================================================
-- SEM FK para automations de propósito: um delete concorrente de automação não
-- pode apagar a prova de que há trabalho em voo — é justamente essa prova que
-- segura o gate.
CREATE TABLE IF NOT EXISTS instance_failover_dispatch_leases (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL,
    automation_id UUID NOT NULL,
    -- <hostname>:<pid>:<uuid> — nunca carrega dado de usuário.
    holder_id     TEXT NOT NULL,
    acquired_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    heartbeat_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL,
    released_at   TIMESTAMPTZ NULL,

    CONSTRAINT ck_failover_lease_expiry CHECK (expires_at > acquired_at)
);

CREATE INDEX IF NOT EXISTS idx_failover_leases_active
    ON instance_failover_dispatch_leases (user_id, automation_id, expires_at)
    WHERE released_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_failover_leases_expires
    ON instance_failover_dispatch_leases (expires_at);

-- =============================================================================
-- 4.9 — Ledger append-only
-- =============================================================================
CREATE TABLE IF NOT EXISTS instance_failover_events (
    id                 BIGSERIAL PRIMARY KEY,
    policy_id          BIGINT NULL REFERENCES instance_failover_policies(id) ON DELETE SET NULL,
    policy_id_snapshot BIGINT NOT NULL,
    operation_id       UUID NULL REFERENCES instance_failover_operations(id) ON DELETE SET NULL,
    dry_run_id         UUID NULL REFERENCES instance_failover_dry_runs(id) ON DELETE SET NULL,
    user_id            UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    event_type         TEXT NOT NULL,
    -- actor_user_id vem EXCLUSIVAMENTE do contexto autenticado; nenhuma request
    -- carrega ator.
    actor_type         TEXT NOT NULL CHECK (actor_type IN ('user', 'cron', 'system')),
    actor_user_id      UUID NULL,
    idempotency_scope  TEXT NULL,
    idempotency_key    TEXT NULL,
    request_hash       CHAR(64) NULL,
    policy_version     BIGINT NULL,
    snapshot_before    JSONB NULL,
    snapshot_after     JSONB NULL,
    report             JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT ck_failover_event_type CHECK (event_type IN (
        'policy_created', 'policy_updated', 'policy_disabled',
        'dry_run_passed', 'dry_run_blocked',
        'proposal_created', 'proposal_recovered', 'proposal_expired',
        'switch_requested', 'switch_gate_closed', 'switch_drain_timeout',
        'switch_applied', 'switch_completed', 'switch_aborted', 'switch_failed',
        'revert_requested', 'revert_gate_closed', 'revert_drain_timeout',
        'revert_applied', 'revert_completed', 'revert_aborted', 'revert_failed',
        'recovery_completed', 'recovery_aborted', 'recovery_failed'
    )),
    CONSTRAINT ck_failover_event_idempotency_scope
        CHECK (idempotency_scope IS NULL OR idempotency_scope IN ('set_policy', 'disable_policy'))
);

CREATE INDEX IF NOT EXISTS idx_failover_events_policy_created
    ON instance_failover_events (policy_id_snapshot, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_failover_events_operation
    ON instance_failover_events (operation_id, created_at, id);

-- Recibo idempotente de SetPolicy/DisablePolicy: replay com a mesma chave
-- devolve o report anterior em vez de mutar de novo.
CREATE UNIQUE INDEX IF NOT EXISTS uq_failover_events_idempotency
    ON instance_failover_events (user_id, idempotency_scope, idempotency_key)
    WHERE idempotency_scope IS NOT NULL;
