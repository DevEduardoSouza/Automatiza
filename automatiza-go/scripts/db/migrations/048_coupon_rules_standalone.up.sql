-- Cupons Avançados standalone (paridade com AffiliPro).
--
-- Modelo da 046: a regra apontava pra um cupom do catálogo (FK
-- coupon_id). Pra criar uma regra o operador precisava primeiro
-- cadastrar um Cupom Simples e depois selecioná-lo no Avançado —
-- fluxo confuso e diferente do que o concorrente entrega.
--
-- A partir daqui a regra carrega seu próprio código + mensagem +
-- desconto. Cupons Simples seguem como catálogo independente para
-- uso manual em mensagens (mesma separação do AffiliPro).
--
-- coupon_id segue na tabela como NULLABLE só pra preservar histórico
-- e permitir rollback rápido caso algo dê errado em produção. O código
-- novo não lê nem grava nessa coluna; um follow-up pode dropar.

ALTER TABLE coupon_rules
    ADD COLUMN IF NOT EXISTS code VARCHAR(100),
    ADD COLUMN IF NOT EXISTS message TEXT,
    ADD COLUMN IF NOT EXISTS discount_type VARCHAR(50),
    ADD COLUMN IF NOT EXISTS discount_value DECIMAL(10, 2),
    ADD COLUMN IF NOT EXISTS max_discount DECIMAL(10, 2);

-- Backfill: regras existentes herdam código/mensagem/desconto do cupom
-- linkado. COALESCE preserva valores já gravados (idempotência) e o
-- WHERE evita reprocessar linhas que já passaram por aqui.
UPDATE coupon_rules cr
SET code = COALESCE(cr.code, c.code),
    message = COALESCE(cr.message, c.coupon_text),
    discount_type = COALESCE(cr.discount_type, c.discount_type),
    discount_value = COALESCE(cr.discount_value, c.discount_value),
    max_discount = COALESCE(cr.max_discount, c.max_discount)
FROM coupons c
WHERE c.id = cr.coupon_id
  AND (cr.code IS NULL OR cr.message IS NULL);

-- Permite criar regras sem FK pra cupom — a regra agora é autocontida.
ALTER TABLE coupon_rules ALTER COLUMN coupon_id DROP NOT NULL;
