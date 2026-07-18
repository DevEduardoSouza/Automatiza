-- Novos nichos do buscador de ofertas (18/07)
-- Aditiva: só INSERT em `categories` (ON CONFLICT DO NOTHING), sem subcategorias.
-- Os IDs de categoria Shopee (productCatId) vivem no backend em
-- initCategoryMapping() (shopee_service.go); aqui só entram os nomes/slugs que
-- alimentam o seletor de categorias. name = display name = chave do mapa Go.

INSERT INTO categories (slug, name, icon, display_order, has_refinements, is_system) VALUES
    ('jardinagem', 'Jardinagem', 'sprout', 16, false, true),
    ('construcaoObra', 'Construção e Obra', 'hard-hat', 17, false, true),
    ('ferramentas', 'Ferramentas', 'wrench', 18, false, true),
    ('aquarismo', 'Aquarismo', 'fish', 19, false, true),
    ('roupasMaternidade', 'Roupas de Maternidade', 'shirt', 20, false, true),
    ('introducaoAlimentar', 'Introdução Alimentar', 'utensils', 21, false, true),
    ('enxovalBebe', 'Enxoval de Bebê', 'baby', 22, false, true),
    ('mamaesBebes', 'Mamães e Bebês', 'baby', 23, false, true),
    ('roupasFitness', 'Roupas Fitness', 'shirt', 24, false, true),
    ('acessoriosFitness', 'Acessórios Fitness', 'dumbbell', 25, false, true),
    ('suplementosAlimentares', 'Suplementos Alimentares', 'pill', 26, false, true),
    ('sexShop', 'Sex Shop', 'heart', 27, false, true)
ON CONFLICT (slug) DO NOTHING;
