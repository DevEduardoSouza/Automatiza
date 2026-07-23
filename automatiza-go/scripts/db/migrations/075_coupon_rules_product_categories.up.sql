-- 075: cupom por categoria oficial Shopee.
--
-- product_category_ids guarda os IDs de categoria da Shopee
-- (ProductOfferV2.productCatIds, GraphQL Int = 32 bits; BIGINT[] é
-- alargamento sem perda). NULL e '{}' significam "regra genérica" — toda
-- escrita nova normaliza lista vazia para NULL. Sem DEFAULT e sem NOT NULL
-- para não reescrever a tabela nem mudar a semântica das regras legadas,
-- que continuam genéricas.
--
-- A constraint fecha a corrida entre a leitura da plataforma atual e o
-- UPDATE no handler, e protege writers futuros: array não vazio só existe
-- em platform = 'shopee' (Mercado Livre não expõe categoria oficial pela
-- API). A validação de aplicação continua obrigatória — ela é quem produz
-- InvalidArgument legível; a constraint é a rede de segurança.
--
-- Adicionar coluna nullable não reescreve a tabela, mas exige lock de
-- catálogo breve (ACCESS EXCLUSIVE momentâneo) — não é "zero lock".
ALTER TABLE coupon_rules
    ADD COLUMN IF NOT EXISTS product_category_ids BIGINT[];

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'coupon_rules_product_categories_shopee_only'
          AND conrelid = 'coupon_rules'::regclass
    ) THEN
        ALTER TABLE coupon_rules
            ADD CONSTRAINT coupon_rules_product_categories_shopee_only
            CHECK (
                COALESCE(cardinality(product_category_ids), 0) = 0
                OR platform = 'shopee'
            ) NOT VALID;
    END IF;
END
$$;

ALTER TABLE coupon_rules
    VALIDATE CONSTRAINT coupon_rules_product_categories_shopee_only;
