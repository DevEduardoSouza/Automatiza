-- Categories and Subcategories System
-- Provides dynamic category management for automation configuration.

-- =============================================================================
-- CATEGORIES TABLE
-- =============================================================================

CREATE TABLE IF NOT EXISTS categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    icon VARCHAR(50),
    display_order INT DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    has_refinements BOOLEAN DEFAULT FALSE,
    is_system BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- =============================================================================
-- SUBCATEGORIES TABLE
-- =============================================================================

CREATE TABLE IF NOT EXISTS subcategories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    slug VARCHAR(100) NOT NULL,
    name VARCHAR(150) NOT NULL,
    display_key VARCHAR(200) NOT NULL,
    group_name VARCHAR(100),
    display_order INT DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    is_system BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(category_id, slug)
);

-- =============================================================================
-- INDEXES
-- =============================================================================

CREATE INDEX IF NOT EXISTS idx_categories_slug ON categories(slug);
CREATE INDEX IF NOT EXISTS idx_categories_display_order ON categories(display_order);
CREATE INDEX IF NOT EXISTS idx_categories_is_active ON categories(is_active) WHERE is_active = TRUE;

CREATE INDEX IF NOT EXISTS idx_subcategories_category_id ON subcategories(category_id);
CREATE INDEX IF NOT EXISTS idx_subcategories_display_order ON subcategories(display_order);
CREATE INDEX IF NOT EXISTS idx_subcategories_display_key ON subcategories(display_key);
CREATE INDEX IF NOT EXISTS idx_subcategories_is_active ON subcategories(is_active) WHERE is_active = TRUE;

-- =============================================================================
-- ROW LEVEL SECURITY
-- =============================================================================

ALTER TABLE categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE subcategories ENABLE ROW LEVEL SECURITY;

CREATE POLICY service_role_categories ON categories FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_subcategories ON subcategories FOR ALL TO service_role USING (true) WITH CHECK (true);

-- =============================================================================
-- TRIGGERS
-- =============================================================================

DROP TRIGGER IF EXISTS update_categories_updated_at ON categories;
CREATE TRIGGER update_categories_updated_at BEFORE UPDATE ON categories
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_subcategories_updated_at ON subcategories;
CREATE TRIGGER update_subcategories_updated_at BEFORE UPDATE ON subcategories
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =============================================================================
-- SEED: CATEGORIES (is_system = TRUE for defaults)
-- =============================================================================

INSERT INTO categories (slug, name, icon, display_order, has_refinements, is_system) VALUES
    ('ofertasGerais', 'Ofertas gerais', 'tag', 1, false, true),
    ('eletronicos', 'Eletrônicos', 'monitor', 2, true, true),
    ('eletrodomesticos', 'Eletrodomésticos', 'refrigerator', 3, true, true),
    ('moda', 'Moda', 'shirt', 4, true, true),
    ('homens', 'Homens', 'user', 5, true, true),
    ('casaDecoracao', 'Casa e Decoração', 'home', 6, true, true),
    ('beleza', 'Beleza', 'sparkles', 7, true, true),
    ('esportes', 'Esportes', 'dumbbell', 8, true, true),
    ('livros', 'Livros', 'book-open', 9, false, true),
    ('brinquedos', 'Brinquedos', 'gamepad-2', 10, false, true),
    ('infantil', 'Infantil', 'baby', 11, false, true),
    ('maternidade', 'Maternidade', 'heart', 12, false, true),
    ('alimentos', 'Alimentos', 'utensils', 13, false, true),
    ('automotivo', 'Automotivo', 'car', 14, false, true),
    ('pets', 'Pets', 'paw-print', 15, false, true)
ON CONFLICT (slug) DO NOTHING;

-- =============================================================================
-- SEED: SUBCATEGORIES - ELETRÔNICOS
-- =============================================================================

-- Computadores
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'computadores-notebooks', 'Notebooks', 'Eletrônicos - Computadores (Notebooks)', 'Computadores', 1, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'computadores-ssd', 'SSD', 'Eletrônicos - Computadores (SSD)', 'Computadores', 2, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'computadores-ram', 'Memória RAM', 'Eletrônicos - Computadores (RAM)', 'Computadores', 3, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'computadores-teclados', 'Teclados', 'Eletrônicos - Computadores (Teclados)', 'Computadores', 4, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'computadores-monitores', 'Monitores', 'Eletrônicos - Computadores (Monitores)', 'Computadores', 5, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

-- Celulares
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'celulares-smartphones', 'Smartphones', 'Eletrônicos - Celulares (Smartphones)', 'Celulares', 10, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'celulares-capas', 'Capas/Películas', 'Eletrônicos - Celulares (Capas)', 'Celulares', 11, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'celulares-carregadores', 'Carregadores/Cabos', 'Eletrônicos - Celulares (Carregadores)', 'Celulares', 12, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'celulares-powerbank', 'Powerbank', 'Eletrônicos - Celulares (Powerbank)', 'Celulares', 13, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

-- Áudio
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'audio-fones-headsets', 'Fones/Headsets', 'Eletrônicos - Áudio (Fones/Headsets)', 'Áudio', 20, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'audio-caixas-de-som', 'Caixas de som', 'Eletrônicos - Áudio (Caixas de Som)', 'Áudio', 21, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

-- Games
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'games-consoles', 'Consoles', 'Eletrônicos - Games (Consoles)', 'Games', 30, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'games-acessorios-controles', 'Controles/Acessórios', 'Eletrônicos - Games (Acessórios/Controles)', 'Games', 31, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

-- Câmeras & Drones
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cameras-webcams', 'Webcams', 'Eletrônicos - Câmeras (Webcams)', 'Câmeras & Drones', 40, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'drones', 'Drones', 'Eletrônicos - Drones', 'Câmeras & Drones', 41, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

-- Wearables
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'wearables-smartwatch', 'Smartwatches', 'Eletrônicos - Wearables (Smartwatch)', 'Wearables', 50, true
FROM categories c WHERE c.slug = 'eletronicos' ON CONFLICT DO NOTHING;

-- =============================================================================
-- SEED: SUBCATEGORIES - ELETRODOMÉSTICOS
-- =============================================================================

-- Cozinha
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cozinha-airfryer', 'Airfryer', 'Eletrodomésticos - Cozinha (Airfryer)', 'Cozinha', 1, true
FROM categories c WHERE c.slug = 'eletrodomesticos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cozinha-liquidificador', 'Liquidificador/Espremedor', 'Eletrodomésticos - Cozinha (Liquidificador/Espremedor)', 'Cozinha', 2, true
FROM categories c WHERE c.slug = 'eletrodomesticos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cozinha-cafeteira', 'Cafeteira', 'Eletrodomésticos - Cozinha (Cafeteira)', 'Cozinha', 3, true
FROM categories c WHERE c.slug = 'eletrodomesticos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cozinha-micro-ondas', 'Micro-ondas', 'Eletrodomésticos - Cozinha (Micro-ondas)', 'Cozinha', 4, true
FROM categories c WHERE c.slug = 'eletrodomesticos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cozinha-torradeira', 'Torradeira/Sanduicheira', 'Eletrodomésticos - Cozinha (Torradeira/Sanduicheira)', 'Cozinha', 5, true
FROM categories c WHERE c.slug = 'eletrodomesticos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cozinha-batedeira', 'Batedeira/Mixer', 'Eletrodomésticos - Cozinha (Batedeira/Mixer)', 'Cozinha', 6, true
FROM categories c WHERE c.slug = 'eletrodomesticos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cozinha-geladeira', 'Geladeira', 'Eletrodomésticos - Cozinha (Geladeira)', 'Cozinha', 7, true
FROM categories c WHERE c.slug = 'eletrodomesticos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cozinha-filtro-agua', 'Filtro/Purificador/Água', 'Eletrodomésticos - Cozinha (Filtro/Água)', 'Cozinha', 8, true
FROM categories c WHERE c.slug = 'eletrodomesticos' ON CONFLICT DO NOTHING;

-- Climatização
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'climatizacao-ventilador', 'Ventilador', 'Eletrodomésticos - Climatização (Ventilador)', 'Climatização', 20, true
FROM categories c WHERE c.slug = 'eletrodomesticos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'climatizacao-ar-condicionado', 'Ar-condicionado', 'Eletrodomésticos - Climatização (Ar-condicionado)', 'Climatização', 21, true
FROM categories c WHERE c.slug = 'eletrodomesticos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'climatizacao-portatil', 'Portátil/Climatizador', 'Eletrodomésticos - Climatização (Portátil/Climatizador)', 'Climatização', 22, true
FROM categories c WHERE c.slug = 'eletrodomesticos' ON CONFLICT DO NOTHING;

-- Lavanderia
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'lavanderia-lava-e-seca', 'Lava e seca', 'Eletrodomésticos - Lavanderia (Lava e Seca)', 'Lavanderia', 30, true
FROM categories c WHERE c.slug = 'eletrodomesticos' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'lavanderia-ferro', 'Ferro/Vaporizador', 'Eletrodomésticos - Lavanderia (Ferro/Vaporizador)', 'Lavanderia', 31, true
FROM categories c WHERE c.slug = 'eletrodomesticos' ON CONFLICT DO NOTHING;

-- Limpeza
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'limpeza-aspirador', 'Aspirador/Robô', 'Eletrodomésticos - Limpeza (Aspirador/Robô)', 'Limpeza', 40, true
FROM categories c WHERE c.slug = 'eletrodomesticos' ON CONFLICT DO NOTHING;

-- TVs
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'tvs-tv', 'TVs', 'Eletrodomésticos - TVs (TV)', 'TVs', 50, true
FROM categories c WHERE c.slug = 'eletrodomesticos' ON CONFLICT DO NOTHING;

-- =============================================================================
-- SEED: SUBCATEGORIES - MODA
-- =============================================================================

-- Feminina
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'feminina-vestidos', 'Vestidos', 'Moda Feminina - Vestidos', 'Feminina', 1, true
FROM categories c WHERE c.slug = 'moda' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'feminina-shorts', 'Shorts', 'Moda Feminina - Shorts', 'Feminina', 2, true
FROM categories c WHERE c.slug = 'moda' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'feminina-calcados-tenis', 'Calçados: Tênis', 'Moda Feminina - Calçados (Tênis)', 'Feminina', 3, true
FROM categories c WHERE c.slug = 'moda' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'feminina-calcados-plataformas', 'Calçados: Plataformas', 'Moda Feminina - Calçados (Plataformas)', 'Feminina', 4, true
FROM categories c WHERE c.slug = 'moda' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'feminina-calcas', 'Calças', 'Moda Feminina - Calças', 'Feminina', 5, true
FROM categories c WHERE c.slug = 'moda' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'feminina-jeans', 'Jeans', 'Moda Feminina - Jeans', 'Feminina', 6, true
FROM categories c WHERE c.slug = 'moda' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'feminina-camisetas', 'Camisetas', 'Moda Feminina - Camisetas', 'Feminina', 7, true
FROM categories c WHERE c.slug = 'moda' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'feminina-blusas', 'Blusas', 'Moda Feminina - Blusas', 'Feminina', 8, true
FROM categories c WHERE c.slug = 'moda' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'feminina-bolsas', 'Bolsas', 'Moda Feminina - Bolsas', 'Feminina', 9, true
FROM categories c WHERE c.slug = 'moda' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'feminina-lingerie-calcinhas', 'Lingerie: Calcinhas', 'Moda Feminina - Lingerie (Calcinhas)', 'Feminina', 10, true
FROM categories c WHERE c.slug = 'moda' ON CONFLICT DO NOTHING;

-- Masculina
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'masculina-bermudas', 'Bermudas', 'Moda Masculina - Bermudas', 'Masculina', 20, true
FROM categories c WHERE c.slug = 'moda' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'masculina-camisas', 'Camisas', 'Moda Masculina - Camisas', 'Masculina', 21, true
FROM categories c WHERE c.slug = 'moda' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'masculina-jeans', 'Jeans', 'Moda Masculina - Jeans', 'Masculina', 22, true
FROM categories c WHERE c.slug = 'moda' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'masculina-calcados-tenis', 'Calçados: Tênis', 'Moda Masculina - Calçados (Tênis)', 'Masculina', 23, true
FROM categories c WHERE c.slug = 'moda' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'masculina-calcados-chinelos', 'Calçados: Chinelos', 'Moda Masculina - Calçados (Chinelos)', 'Masculina', 24, true
FROM categories c WHERE c.slug = 'moda' ON CONFLICT DO NOTHING;

-- =============================================================================
-- SEED: SUBCATEGORIES - HOMENS
-- =============================================================================

-- Roupas
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'roupas-camisetas', 'Camisetas', 'Homens - Roupas (Camisetas)', 'Roupas', 1, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'roupas-bermudas', 'Bermudas/Shorts', 'Homens - Roupas (Bermudas)', 'Roupas', 2, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'roupas-jeans', 'Jeans', 'Homens - Roupas (Jeans)', 'Roupas', 3, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'roupas-calcas-cargo', 'Calças (Cargo)', 'Homens - Roupas (Calças Cargo)', 'Roupas', 4, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'roupas-moletons', 'Moletons', 'Homens - Roupas (Moletons)', 'Roupas', 5, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'roupas-jaquetas', 'Jaquetas', 'Homens - Roupas (Jaquetas)', 'Roupas', 6, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

-- Acessórios
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'acessorios-bones', 'Bonés', 'Homens - Acessórios (Bonés)', 'Acessórios', 10, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'acessorios-relogios', 'Relógios', 'Homens - Acessórios (Relógios)', 'Acessórios', 11, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'acessorios-carteiras', 'Carteiras/Porta-cartões', 'Homens - Acessórios (Carteiras/Porta-cartões)', 'Acessórios', 12, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'acessorios-cintos', 'Cintos', 'Homens - Acessórios (Cintos)', 'Acessórios', 13, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'acessorios-oculos', 'Óculos de sol', 'Homens - Acessórios (Óculos de sol)', 'Acessórios', 14, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

-- Calçados
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'calcados-tenis', 'Tênis', 'Homens - Calçados (Tênis)', 'Calçados', 20, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'calcados-chinelos', 'Chinelos', 'Homens - Calçados (Chinelos)', 'Calçados', 21, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'calcados-botas', 'Botas', 'Homens - Calçados (Botas)', 'Calçados', 22, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'calcados-sociais', 'Sociais/Oxford', 'Homens - Calçados (Sociais/Oxford)', 'Calçados', 23, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

-- Íntimas
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'intimas-cuecas', 'Cuecas', 'Homens - Íntimas (Cuecas)', 'Íntimas', 30, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'intimas-meias', 'Meias', 'Homens - Íntimas (Meias)', 'Íntimas', 31, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

-- Barbearia
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'barbearia-aparadores', 'Aparadores/Máquinas', 'Homens - Barbearia (Aparadores)', 'Barbearia', 40, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'barbearia-cremes', 'Cremes/Espumas/Géis', 'Homens - Barbearia (Cremes/Géis)', 'Barbearia', 41, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

-- Perfumes
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'perfumes', 'Perfumes', 'Homens - Perfumes', 'Perfumes', 50, true
FROM categories c WHERE c.slug = 'homens' ON CONFLICT DO NOTHING;

-- =============================================================================
-- SEED: SUBCATEGORIES - CASA E DECORAÇÃO
-- =============================================================================

-- Cozinha
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cozinha-armazenamento', 'Cozinha: Armazenamento (potes/marmitas)', 'Casa - Cozinha (Armazenamento)', 'Cozinha', 1, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cozinha-cortadores', 'Cozinha: Cortadores/Raladores', 'Casa - Cozinha (Cortadores)', 'Cozinha', 2, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

-- Quarto
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'quarto-cama', 'Quarto: Cama/Edredom/Colchas', 'Casa - Quarto (Cama/Edredom)', 'Quarto', 10, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

-- Decoração
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'decoracao-tapetes', 'Decoração: Tapetes', 'Casa - Decoração (Tapetes)', 'Decoração', 20, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'decoracao-cortinas', 'Decoração: Cortinas/Persianas', 'Casa - Decoração (Cortinas)', 'Decoração', 21, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'decoracao-vasos', 'Decoração: Vasos', 'Casa - Decoração (Vasos)', 'Decoração', 22, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'decoracao-parede', 'Decoração: Parede/Molduras', 'Casa - Decoração (Parede/Molduras)', 'Decoração', 23, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

-- Banheiro
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'banheiro-suportes', 'Banheiro: Suportes/Saboneteiras', 'Casa - Banheiro (Suportes/Sabonete)', 'Banheiro', 30, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

-- Organização
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'organizacao-guarda-roupa', 'Organização: Guarda-roupa', 'Casa - Organização (Guarda-roupa)', 'Organização', 40, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'organizacao-sapateiras', 'Organização: Sapateiras', 'Casa - Organização (Sapateiras)', 'Organização', 41, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

-- Móveis
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'moveis-guarda-roupas', 'Móveis: Guarda-roupas', 'Casa - Móveis (Guarda-roupas)', 'Móveis', 50, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'moveis-armarios', 'Móveis: Armários/Gabinetes', 'Casa - Móveis (Armários/Gabinetes)', 'Móveis', 51, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'moveis-camas', 'Móveis: Camas/Cabeceiras', 'Casa - Móveis (Camas/Cabeceiras)', 'Móveis', 52, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'moveis-sofas', 'Móveis: Sofás', 'Casa - Móveis (Sofás)', 'Móveis', 53, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'moveis-cadeiras', 'Móveis: Cadeiras/Banquetas', 'Casa - Móveis (Cadeiras/Banquetas)', 'Móveis', 54, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'moveis-mesas', 'Móveis: Mesas/Escrivaninhas', 'Casa - Móveis (Mesas/Escrivaninhas)', 'Móveis', 55, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'moveis-prateleiras', 'Móveis: Prateleiras/Racks', 'Casa - Móveis (Prateleiras/Racks)', 'Móveis', 56, true
FROM categories c WHERE c.slug = 'casaDecoracao' ON CONFLICT DO NOTHING;

-- =============================================================================
-- SEED: SUBCATEGORIES - BELEZA
-- =============================================================================

-- Perfumes
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'perfumes-fragrancias', 'Perfumes e Fragrâncias', 'Beleza - Perfumes e Fragrâncias', 'Perfumes', 1, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

-- Skincare
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'skincare-hidratante', 'Skincare: Hidratante facial', 'Beleza - Skincare (Hidratante Facial)', 'Skincare', 10, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'skincare-protetor', 'Skincare: Protetor solar facial', 'Beleza - Skincare (Protetor Solar Facial)', 'Skincare', 11, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'skincare-soro', 'Skincare: Soro/Essência', 'Beleza - Skincare (Soro/Essência)', 'Skincare', 12, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'skincare-mascaras', 'Skincare: Máscaras', 'Beleza - Skincare (Máscaras)', 'Skincare', 13, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'skincare-massagem', 'Skincare: Massagem facial', 'Beleza - Skincare (Massagem Facial)', 'Skincare', 14, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

-- Maquiagem
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'maquiagem-base', 'Maquiagem: Base', 'Beleza - Maquiagem (Base)', 'Maquiagem', 20, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'maquiagem-corretivo', 'Maquiagem: Corretivo', 'Beleza - Maquiagem (Corretivo)', 'Maquiagem', 21, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'maquiagem-po', 'Maquiagem: Pó', 'Beleza - Maquiagem (Pó)', 'Maquiagem', 22, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'maquiagem-kits', 'Maquiagem: Kits', 'Beleza - Maquiagem (Kits)', 'Maquiagem', 23, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

-- Cabelo
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cabelo-shampoo', 'Cabelo: Shampoo', 'Beleza - Cabelo (Shampoo)', 'Cabelo', 30, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cabelo-tratamento', 'Cabelo: Tratamento', 'Beleza - Cabelo (Tratamento)', 'Cabelo', 31, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cabelo-finalizadores', 'Cabelo: Finalizadores', 'Beleza - Cabelo (Finalizadores)', 'Cabelo', 32, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cabelo-tintura', 'Cabelo: Tintura', 'Beleza - Cabelo (Tintura)', 'Cabelo', 33, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

-- Corpo
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'corpo-desodorantes', 'Corpo: Desodorantes/Body splash', 'Beleza - Corpo (Desodorantes/Body Splash)', 'Corpo', 40, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'corpo-hidratante', 'Corpo: Hidratante corporal', 'Beleza - Corpo (Hidratante Corporal)', 'Corpo', 41, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'corpo-sabonetes', 'Corpo: Sabonetes', 'Beleza - Corpo (Sabonetes)', 'Corpo', 42, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'corpo-depilacao', 'Corpo: Depilação', 'Beleza - Corpo (Depilação)', 'Corpo', 43, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

-- Unhas
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'unhas-gel', 'Unhas: Gel', 'Beleza - Unhas (Gel)', 'Unhas', 50, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'unhas-ferramentas', 'Unhas: Ferramentas', 'Beleza - Unhas (Ferramentas)', 'Unhas', 51, true
FROM categories c WHERE c.slug = 'beleza' ON CONFLICT DO NOTHING;

-- =============================================================================
-- SEED: SUBCATEGORIES - ESPORTES
-- =============================================================================

-- Fitness
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'fitness-camisetas', 'Camisetas', 'Esportes - Fitness (Camisetas)', 'Fitness', 1, true
FROM categories c WHERE c.slug = 'esportes' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'fitness-regatas', 'Regatas', 'Esportes - Fitness (Regatas)', 'Fitness', 2, true
FROM categories c WHERE c.slug = 'esportes' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'fitness-musculacao', 'Musculação/Acessórios', 'Esportes - Fitness (Musculação/Acessórios)', 'Fitness', 3, true
FROM categories c WHERE c.slug = 'esportes' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'fitness-maquinas', 'Máquinas (esteira/bike/etc)', 'Esportes - Fitness (Máquinas)', 'Fitness', 4, true
FROM categories c WHERE c.slug = 'esportes' ON CONFLICT DO NOTHING;

-- Outdoor/Ciclismo
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'outdoor-ciclismo', 'Outdoor/Ciclismo (Acessórios)', 'Esportes - Outdoor/Ciclismo (Acessórios)', 'Outdoor/Ciclismo', 10, true
FROM categories c WHERE c.slug = 'esportes' ON CONFLICT DO NOTHING;

-- Futebol
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'futebol-bolas', 'Futebol: Bolas', 'Esportes - Futebol (Bolas)', 'Futebol', 20, true
FROM categories c WHERE c.slug = 'esportes' ON CONFLICT DO NOTHING;

-- Lutas
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'lutas-luvas', 'Lutas: Luvas/Bandagens', 'Esportes - Lutas (Luvas/Bandagens)', 'Lutas', 30, true
FROM categories c WHERE c.slug = 'esportes' ON CONFLICT DO NOTHING;

INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'lutas-sacos', 'Lutas: Sacos de pancada', 'Esportes - Lutas (Sacos de Pancada)', 'Lutas', 31, true
FROM categories c WHERE c.slug = 'esportes' ON CONFLICT DO NOTHING;
