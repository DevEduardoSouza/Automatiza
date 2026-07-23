-- Rollback APENAS de desenvolvimento. Em produção o rollback da feature é
-- desligar o gate COUPON_CATEGORY_RULES_ENABLED — dropar a coluna apaga a
-- segmentação por categoria que o operador configurou.
ALTER TABLE coupon_rules
    DROP CONSTRAINT IF EXISTS coupon_rules_product_categories_shopee_only;

ALTER TABLE coupon_rules
    DROP COLUMN IF EXISTS product_category_ids;
