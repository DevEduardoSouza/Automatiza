-- Lançamentos financeiros manuais do painel de suporte (aba Financeiro).
--
-- A receita de assinatura (Kiwify) NÃO é rastreada por transação no banco
-- (kiwify_webhook_events só guarda idempotência, sem valor; não há client da
-- API Kiwify). Esta tabela cobre o outro lado: despesas e receitas/lucros
-- lançados à mão pela equipe, com suporte a recorrência. A projeção de lucro
-- da página expande as entradas recorrentes pelo horizonte:
--   recurring = false            -> entrada única em entry_date
--   recurring = true,  N meses   -> vale de entry_date por recurrence_months meses
--   recurring = true,  N = NULL  -> recorrência infinita (sem fim)
--
-- amount em numeric(12,2) (mesma escolha dos preços em `plans`), sempre >= 0;
-- o sinal do fluxo vem de `kind`, não do valor.
CREATE TABLE IF NOT EXISTS finance_entries (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind              TEXT NOT NULL CHECK (kind IN ('expense', 'revenue')),
    description       TEXT NOT NULL,
    amount            NUMERIC(12, 2) NOT NULL CHECK (amount >= 0),
    category          TEXT,
    entry_date        DATE NOT NULL,
    recurring         BOOLEAN NOT NULL DEFAULT false,
    recurrence_months INTEGER,  -- NULL + recurring => infinito; N > 0 => N meses
    created_by        UUID,     -- profile do usuário de suporte que lançou
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- recorrência só faz sentido com meses NULL (infinito) ou > 0
    CONSTRAINT finance_entries_recurrence_ck
        CHECK (recurrence_months IS NULL OR recurrence_months > 0)
);

CREATE INDEX IF NOT EXISTS idx_finance_entries_date
    ON finance_entries (entry_date);

CREATE INDEX IF NOT EXISTS idx_finance_entries_kind
    ON finance_entries (kind);
