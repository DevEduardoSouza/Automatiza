-- Cupons Avançados: regras de aplicação automática.
--
-- Modelo até aqui: cada automação carregava um único cupom via
-- automations.coupon_id, escolhido manualmente no formulário. Vai ser
-- substituído por um sistema de regras: o catálogo `coupons` segue
-- como está (texto, código, valor de desconto, etc.), e esta tabela
-- nova descreve QUANDO cada cupom entra na mensagem.
--
-- Quando o worker for disparar uma oferta, ele consulta as regras
-- ativas do dono filtrando por plataforma + período + valor mínimo
-- do produto, e escolhe a regra com maior min_price (tie-break por
-- priority, depois created_at). O cupom referenciado pela regra é
-- injetado no template.
--
-- automations.coupon_id é mantida no schema (sem dropar) pra permitir
-- rollback rápido; o código deixa de ler/gravar essa coluna.

CREATE TABLE IF NOT EXISTS coupon_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    coupon_id UUID NOT NULL REFERENCES coupons(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL,
    period_start TIMESTAMPTZ,
    period_end TIMESTAMPTZ,
    min_price NUMERIC(10,2) NOT NULL DEFAULT 0,
    priority INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index principal usado pelo matcher: filtra por dono + plataforma e só
-- linhas ativas (partial index pra ficar enxuto).
CREATE INDEX IF NOT EXISTS idx_coupon_rules_user_platform_active
    ON coupon_rules(user_id, platform)
    WHERE is_active = TRUE;

-- Index complementar pra cascade DELETE quando um cupom é removido.
CREATE INDEX IF NOT EXISTS idx_coupon_rules_coupon_id
    ON coupon_rules(coupon_id);

-- Per-automation opt-out for the rule engine.
--
-- Search-mode automations that pull from the Shopee API often hit
-- products that are already inside a Shopee promo (Big Sale, etc.) —
-- piling our own coupon on top reads as redundant or misleading. Same
-- concern can show up in any mode where the operator wants the
-- automation to dispatch "as is". With this flag the operator can
-- silence the rule engine for one automation without touching the
-- global rules table.
--
-- Default TRUE preserves the behavior of the four users we'll backfill
-- below — their coupon stays attached. Operators that want a clean
-- pipeline flip this off later from the form.
ALTER TABLE automations
ADD COLUMN IF NOT EXISTS apply_coupon_rules BOOLEAN NOT NULL DEFAULT TRUE;

-- Backfill: cada automação com coupon_id setado vira uma regra equivalente
-- pro mesmo dono. Sem janela, sem valor mínimo — comportamento igual ao
-- "cupom fixo na automação". DISTINCT ON dedupa quando o mesmo dono tem
-- N automações apontando pro mesmo cupom; o NOT EXISTS torna a migration
-- idempotente caso reaplique. Preserva os 4 usuários que dependem disso
-- hoje sem regressão.
INSERT INTO coupon_rules (user_id, coupon_id, platform, min_price, priority, is_active)
SELECT DISTINCT ON (a.user_id, a.coupon_id)
    a.user_id,
    a.coupon_id,
    COALESCE(c.store, 'shopee'),
    0,
    0,
    TRUE
FROM automations a
JOIN coupons c ON c.id = a.coupon_id
WHERE a.coupon_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM coupon_rules cr
      WHERE cr.user_id = a.user_id AND cr.coupon_id = a.coupon_id
  );
