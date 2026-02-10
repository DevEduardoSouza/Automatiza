-- Migration: 024_plans_features_to_text_array.up.sql
-- Descrição: Cria tabela plans (se não existir) e garante que features é TEXT[]

-- Criar tabela plans se não existir
CREATE TABLE IF NOT EXISTS plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    description TEXT,
    price_monthly NUMERIC(10,2),
    price_semester NUMERIC(10,2),
    price_annual NUMERIC(10,2),
    group_limit INTEGER DEFAULT 1 NOT NULL,
    storage_limit BIGINT DEFAULT 1000000000 NOT NULL,
    features TEXT[] DEFAULT '{}' NOT NULL,
    kiwify_product_id_monthly TEXT,
    kiwify_product_id_semester TEXT,
    kiwify_product_id_annual TEXT,
    is_active BOOLEAN DEFAULT true NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

-- Índices úteis
CREATE INDEX IF NOT EXISTS idx_plans_slug ON plans(slug);
CREATE INDEX IF NOT EXISTS idx_plans_is_active ON plans(is_active);

-- Converte features de jsonb para text[] se necessário
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'plans'
        AND column_name = 'features'
        AND data_type = 'jsonb'
    ) THEN
        ALTER TABLE plans ADD COLUMN features_temp TEXT[] DEFAULT '{}' NOT NULL;

        UPDATE plans
        SET features_temp = (
            SELECT COALESCE(
                ARRAY(SELECT jsonb_array_elements_text(features)),
                '{}'
            )
        );

        ALTER TABLE plans DROP COLUMN features;
        ALTER TABLE plans RENAME COLUMN features_temp TO features;
    END IF;
END $$;

COMMENT ON TABLE plans IS 'Planos de assinatura disponíveis';
