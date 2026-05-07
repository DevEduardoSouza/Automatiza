-- Reverte 048: volta as regras pro modelo FK-only.
--
-- Cuidado: regras criadas após 048 que NÃO tenham coupon_id setado
-- vão ficar inválidas (FK NOT NULL volta). Se já houver regras assim
-- em produção, o ALTER abaixo falha — manualmente limpar antes:
--   DELETE FROM coupon_rules WHERE coupon_id IS NULL;
-- Os campos inline (code/message/discount_*) são descartados; o FK
-- pra coupons.id segue como única fonte da verdade.

ALTER TABLE coupon_rules ALTER COLUMN coupon_id SET NOT NULL;

ALTER TABLE coupon_rules
    DROP COLUMN IF EXISTS max_discount,
    DROP COLUMN IF EXISTS discount_value,
    DROP COLUMN IF EXISTS discount_type,
    DROP COLUMN IF EXISTS message,
    DROP COLUMN IF EXISTS code;
