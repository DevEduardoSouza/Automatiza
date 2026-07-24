-- automatiza:migration-mode=transactional
-- Nichos Confeitaria e Doces / Casamento / Bebê e Maternidade + subnichos (24/07)
-- Aditiva: INSERT em categories (has_refinements=true) + subcategories.
-- name = display = chave do mapa Go; display_key = 'Nicho - Grupo (Subnicho)'.
-- Subnichos SEM productCatId próprio -> busca por keyword (keywordCategories em
-- shopee_service.go usa o termo entre parênteses). ON CONFLICT DO NOTHING = reexecutável.

INSERT INTO categories (slug, name, icon, display_order, has_refinements, is_system) VALUES
    ('confeitariaDoces', 'Confeitaria e Doces', 'cake', 28, true, true),
    ('casamento', 'Casamento', 'heart', 29, true, true),
    ('bebeMaternidade', 'Bebê e Maternidade', 'baby', 30, true, true)
ON CONFLICT (slug) DO NOTHING;

-- ---- Confeitaria e Doces ----
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'chocolates', 'Chocolates', 'Confeitaria e Doces - Ingredientes (Chocolates)', 'Ingredientes', 1, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cacau-e-achocolatados', 'Cacau e achocolatados', 'Confeitaria e Doces - Ingredientes (Cacau e achocolatados)', 'Ingredientes', 2, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'corantes', 'Corantes', 'Confeitaria e Doces - Ingredientes (Corantes)', 'Ingredientes', 3, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'essencias', 'Essências', 'Confeitaria e Doces - Ingredientes (Essências)', 'Ingredientes', 4, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'aromas', 'Aromas', 'Confeitaria e Doces - Ingredientes (Aromas)', 'Ingredientes', 5, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'recheios', 'Recheios', 'Confeitaria e Doces - Ingredientes (Recheios)', 'Ingredientes', 6, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'coberturas', 'Coberturas', 'Confeitaria e Doces - Ingredientes (Coberturas)', 'Ingredientes', 7, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'chantilly', 'Chantilly', 'Confeitaria e Doces - Ingredientes (Chantilly)', 'Ingredientes', 8, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'chantininho', 'Chantininho', 'Confeitaria e Doces - Ingredientes (Chantininho)', 'Ingredientes', 9, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'pasta-americana', 'Pasta americana', 'Confeitaria e Doces - Ingredientes (Pasta americana)', 'Ingredientes', 10, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'pasta-saborizante', 'Pasta saborizante', 'Confeitaria e Doces - Ingredientes (Pasta saborizante)', 'Ingredientes', 11, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'confeitos', 'Confeitos', 'Confeitaria e Doces - Ingredientes (Confeitos)', 'Ingredientes', 12, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'acucares-especiais', 'Açúcares especiais', 'Confeitaria e Doces - Ingredientes (Açúcares especiais)', 'Ingredientes', 13, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'farinhas', 'Farinhas', 'Confeitaria e Doces - Ingredientes (Farinhas)', 'Ingredientes', 14, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'fermentos', 'Fermentos', 'Confeitaria e Doces - Ingredientes (Fermentos)', 'Ingredientes', 15, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'misturas-prontas', 'Misturas prontas', 'Confeitaria e Doces - Ingredientes (Misturas prontas)', 'Ingredientes', 16, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'leite-condensado', 'Leite condensado', 'Confeitaria e Doces - Ingredientes (Leite condensado)', 'Ingredientes', 17, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'creme-de-leite', 'Creme de leite', 'Confeitaria e Doces - Ingredientes (Creme de leite)', 'Ingredientes', 18, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'leite-em-po', 'Leite em pó', 'Confeitaria e Doces - Ingredientes (Leite em pó)', 'Ingredientes', 19, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'leite-ninho', 'Leite ninho', 'Confeitaria e Doces - Ingredientes (Leite ninho)', 'Ingredientes', 20, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'gotas-de-chocolate', 'Gotas de chocolate', 'Confeitaria e Doces - Ingredientes (Gotas de chocolate)', 'Ingredientes', 21, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'chocolate-fracionado', 'Chocolate fracionado', 'Confeitaria e Doces - Ingredientes (Chocolate fracionado)', 'Ingredientes', 22, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'chocolate-nobre', 'Chocolate nobre', 'Confeitaria e Doces - Ingredientes (Chocolate nobre)', 'Ingredientes', 23, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'chocolate-em-po', 'Chocolate em pó', 'Confeitaria e Doces - Ingredientes (Chocolate em pó)', 'Ingredientes', 24, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'granulado', 'Granulado', 'Confeitaria e Doces - Ingredientes (Granulado)', 'Ingredientes', 25, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'ganache', 'Ganache', 'Confeitaria e Doces - Ingredientes (Ganache)', 'Ingredientes', 26, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'doce-de-leite', 'Doce de leite', 'Confeitaria e Doces - Ingredientes (Doce de leite)', 'Ingredientes', 27, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'glucose', 'Glucose', 'Confeitaria e Doces - Ingredientes (Glucose)', 'Ingredientes', 28, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'emulsificante', 'Emulsificante', 'Confeitaria e Doces - Ingredientes (Emulsificante)', 'Ingredientes', 29, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'gelatina', 'Gelatina', 'Confeitaria e Doces - Ingredientes (Gelatina)', 'Ingredientes', 30, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'acucar-de-confeiteiro', 'Açúcar de confeiteiro', 'Confeitaria e Doces - Ingredientes (Açúcar de confeiteiro)', 'Ingredientes', 31, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'coco-ralado', 'Coco ralado', 'Confeitaria e Doces - Ingredientes (Coco ralado)', 'Ingredientes', 32, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'espatulas', 'Espátulas', 'Confeitaria e Doces - Utensílios (Espátulas)', 'Utensílios', 33, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bicos-de-confeitar', 'Bicos de confeitar', 'Confeitaria e Doces - Utensílios (Bicos de confeitar)', 'Utensílios', 34, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'sacos-de-confeitar', 'Sacos de confeitar', 'Confeitaria e Doces - Utensílios (Sacos de confeitar)', 'Utensílios', 35, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bailarina', 'Bailarina', 'Confeitaria e Doces - Utensílios (Bailarina)', 'Utensílios', 36, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'fouet', 'Fouet', 'Confeitaria e Doces - Utensílios (Fouet)', 'Utensílios', 37, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'raspadores', 'Raspadores', 'Confeitaria e Doces - Utensílios (Raspadores)', 'Utensílios', 38, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'pinceis-culinarios', 'Pincéis culinários', 'Confeitaria e Doces - Utensílios (Pincéis culinários)', 'Utensílios', 39, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'termometros', 'Termômetros', 'Confeitaria e Doces - Utensílios (Termômetros)', 'Utensílios', 40, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'balancas', 'Balanças', 'Confeitaria e Doces - Utensílios (Balanças)', 'Utensílios', 41, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'medidores', 'Medidores', 'Confeitaria e Doces - Utensílios (Medidores)', 'Utensílios', 42, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'facas', 'Facas', 'Confeitaria e Doces - Utensílios (Facas)', 'Utensílios', 43, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'tesouras-culinarias', 'Tesouras culinárias', 'Confeitaria e Doces - Utensílios (Tesouras culinárias)', 'Utensílios', 44, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'colher-medidora', 'Colher medidora', 'Confeitaria e Doces - Utensílios (Colher medidora)', 'Utensílios', 45, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'xicara-medidora', 'Xícara medidora', 'Confeitaria e Doces - Utensílios (Xícara medidora)', 'Utensílios', 46, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'rolo-de-macarrao', 'Rolo de macarrão', 'Confeitaria e Doces - Utensílios (Rolo de macarrão)', 'Utensílios', 47, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'espatula-de-silicone', 'Espátula de silicone', 'Confeitaria e Doces - Utensílios (Espátula de silicone)', 'Utensílios', 48, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'espatula-angular', 'Espátula angular', 'Confeitaria e Doces - Utensílios (Espátula angular)', 'Utensílios', 49, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'peneira', 'Peneira', 'Confeitaria e Doces - Utensílios (Peneira)', 'Utensílios', 50, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'ralador', 'Ralador', 'Confeitaria e Doces - Utensílios (Ralador)', 'Utensílios', 51, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'batedor-manual', 'Batedor manual', 'Confeitaria e Doces - Utensílios (Batedor manual)', 'Utensílios', 52, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'tapete-de-silicone', 'Tapete de silicone', 'Confeitaria e Doces - Utensílios (Tapete de silicone)', 'Utensílios', 53, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'tela-de-resfriamento', 'Tela de resfriamento', 'Confeitaria e Doces - Utensílios (Tela de resfriamento)', 'Utensílios', 54, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'alisador-de-bolo', 'Alisador de bolo', 'Confeitaria e Doces - Utensílios (Alisador de bolo)', 'Utensílios', 55, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'nivelador-de-bolo', 'Nivelador de bolo', 'Confeitaria e Doces - Utensílios (Nivelador de bolo)', 'Utensílios', 56, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'boleador', 'Boleador', 'Confeitaria e Doces - Utensílios (Boleador)', 'Utensílios', 57, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'formas-para-bolo', 'Formas para bolo', 'Confeitaria e Doces - Formas (Formas para bolo)', 'Formas', 58, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'formas-de-silicone', 'Formas de silicone', 'Confeitaria e Doces - Formas (Formas de silicone)', 'Formas', 59, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'formas-para-bombom', 'Formas para bombom', 'Confeitaria e Doces - Formas (Formas para bombom)', 'Formas', 60, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'formas-para-pao-de-mel', 'Formas para pão de mel', 'Confeitaria e Doces - Formas (Formas para pão de mel)', 'Formas', 61, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'formas-para-cupcake', 'Formas para cupcake', 'Confeitaria e Doces - Formas (Formas para cupcake)', 'Formas', 62, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'formas-para-biscoitos', 'Formas para biscoitos', 'Confeitaria e Doces - Formas (Formas para biscoitos)', 'Formas', 63, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cortadores', 'Cortadores', 'Confeitaria e Doces - Formas (Cortadores)', 'Formas', 64, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'formas-para-pudim', 'Formas para pudim', 'Confeitaria e Doces - Formas (Formas para pudim)', 'Formas', 65, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'formas-para-torta', 'Formas para torta', 'Confeitaria e Doces - Formas (Formas para torta)', 'Formas', 66, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'formas-redondas', 'Formas redondas', 'Confeitaria e Doces - Formas (Formas redondas)', 'Formas', 67, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'formas-retangulares', 'Formas retangulares', 'Confeitaria e Doces - Formas (Formas retangulares)', 'Formas', 68, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'formas-para-pao', 'Formas para pão', 'Confeitaria e Doces - Formas (Formas para pão)', 'Formas', 69, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'formas-de-gelo', 'Formas de gelo', 'Confeitaria e Doces - Formas (Formas de gelo)', 'Formas', 70, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'formas-para-picole', 'Formas para picolé', 'Confeitaria e Doces - Formas (Formas para picolé)', 'Formas', 71, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'formas-para-trufas', 'Formas para trufas', 'Confeitaria e Doces - Formas (Formas para trufas)', 'Formas', 72, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'aro-para-bolo', 'Aro para bolo', 'Confeitaria e Doces - Formas (Aro para bolo)', 'Formas', 73, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'forma-de-fundo-falso', 'Forma de fundo falso', 'Confeitaria e Doces - Formas (Forma de fundo falso)', 'Formas', 74, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'formas-para-muffin', 'Formas para muffin', 'Confeitaria e Doces - Formas (Formas para muffin)', 'Formas', 75, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'batedeiras', 'Batedeiras', 'Confeitaria e Doces - Equipamentos (Batedeiras)', 'Equipamentos', 76, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'liquidificadores', 'Liquidificadores', 'Confeitaria e Doces - Equipamentos (Liquidificadores)', 'Equipamentos', 77, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'processadores', 'Processadores', 'Confeitaria e Doces - Equipamentos (Processadores)', 'Equipamentos', 78, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'derretedeiras-de-chocolate', 'Derretedeiras de chocolate', 'Confeitaria e Doces - Equipamentos (Derretedeiras de chocolate)', 'Equipamentos', 79, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'fornos', 'Fornos', 'Confeitaria e Doces - Equipamentos (Fornos)', 'Equipamentos', 80, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'fogoes', 'Fogões', 'Confeitaria e Doces - Equipamentos (Fogões)', 'Equipamentos', 81, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'panelas', 'Panelas', 'Confeitaria e Doces - Equipamentos (Panelas)', 'Equipamentos', 82, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'panelas-para-brigadeiro', 'Panelas para brigadeiro', 'Confeitaria e Doces - Equipamentos (Panelas para brigadeiro)', 'Equipamentos', 83, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'panelas-antiaderentes', 'Panelas antiaderentes', 'Confeitaria e Doces - Equipamentos (Panelas antiaderentes)', 'Equipamentos', 84, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'fogareiros', 'Fogareiros', 'Confeitaria e Doces - Equipamentos (Fogareiros)', 'Equipamentos', 85, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'seladoras-de-embalagem', 'Seladoras de embalagem', 'Confeitaria e Doces - Equipamentos (Seladoras de embalagem)', 'Equipamentos', 86, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'batedeira-planetaria', 'Batedeira planetária', 'Confeitaria e Doces - Equipamentos (Batedeira planetária)', 'Equipamentos', 87, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'mixer', 'Mixer', 'Confeitaria e Doces - Equipamentos (Mixer)', 'Equipamentos', 88, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'micro-ondas', 'Micro-ondas', 'Confeitaria e Doces - Equipamentos (Micro-ondas)', 'Equipamentos', 89, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'air-fryer', 'Air fryer', 'Confeitaria e Doces - Equipamentos (Air fryer)', 'Equipamentos', 90, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'forno-eletrico', 'Forno elétrico', 'Confeitaria e Doces - Equipamentos (Forno elétrico)', 'Equipamentos', 91, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cooktop', 'Cooktop', 'Confeitaria e Doces - Equipamentos (Cooktop)', 'Equipamentos', 92, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'balanca-digital', 'Balança digital', 'Confeitaria e Doces - Equipamentos (Balança digital)', 'Equipamentos', 93, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'maquina-de-algodao-doce', 'Máquina de algodão doce', 'Confeitaria e Doces - Equipamentos (Máquina de algodão doce)', 'Equipamentos', 94, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'maquina-de-waffle', 'Máquina de waffle', 'Confeitaria e Doces - Equipamentos (Máquina de waffle)', 'Equipamentos', 95, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'chocolateira', 'Chocolateira', 'Confeitaria e Doces - Equipamentos (Chocolateira)', 'Equipamentos', 96, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'panela-eletrica', 'Panela elétrica', 'Confeitaria e Doces - Equipamentos (Panela elétrica)', 'Equipamentos', 97, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'topos-de-bolo', 'Topos de bolo', 'Confeitaria e Doces - Decoração (Topos de bolo)', 'Decoração', 98, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'flores-de-acucar', 'Flores de açúcar', 'Confeitaria e Doces - Decoração (Flores de açúcar)', 'Decoração', 99, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'papel-arroz', 'Papel arroz', 'Confeitaria e Doces - Decoração (Papel arroz)', 'Decoração', 100, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'papel-wafer', 'Papel wafer', 'Confeitaria e Doces - Decoração (Papel wafer)', 'Decoração', 101, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'glitter-comestivel', 'Glitter comestível', 'Confeitaria e Doces - Decoração (Glitter comestível)', 'Decoração', 102, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'po-para-decoracao', 'Pó para decoração', 'Confeitaria e Doces - Decoração (Pó para decoração)', 'Decoração', 103, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'perolas', 'Pérolas', 'Confeitaria e Doces - Decoração (Pérolas)', 'Decoração', 104, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'sprinkles', 'Sprinkles', 'Confeitaria e Doces - Decoração (Sprinkles)', 'Decoração', 105, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'apliques', 'Apliques', 'Confeitaria e Doces - Decoração (Apliques)', 'Decoração', 106, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'confeitos-metalizados', 'Confeitos metalizados', 'Confeitaria e Doces - Decoração (Confeitos metalizados)', 'Decoração', 107, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'corante-em-gel', 'Corante em gel', 'Confeitaria e Doces - Decoração (Corante em gel)', 'Decoração', 108, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'corante-em-po', 'Corante em pó', 'Confeitaria e Doces - Decoração (Corante em pó)', 'Decoração', 109, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'tinta-comestivel', 'Tinta comestível', 'Confeitaria e Doces - Decoração (Tinta comestível)', 'Decoração', 110, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'spray-comestivel', 'Spray comestível', 'Confeitaria e Doces - Decoração (Spray comestível)', 'Decoração', 111, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'perolados', 'Perolados', 'Confeitaria e Doces - Decoração (Perolados)', 'Decoração', 112, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'granulado-colorido', 'Granulado colorido', 'Confeitaria e Doces - Decoração (Granulado colorido)', 'Decoração', 113, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'micangas-comestiveis', 'Miçangas comestíveis', 'Confeitaria e Doces - Decoração (Miçangas comestíveis)', 'Decoração', 114, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'folha-de-ouro-comestivel', 'Folha de ouro comestível', 'Confeitaria e Doces - Decoração (Folha de ouro comestível)', 'Decoração', 115, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'estencil-para-bolo', 'Estêncil para bolo', 'Confeitaria e Doces - Decoração (Estêncil para bolo)', 'Decoração', 116, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'molde-de-silicone-para-decoracao', 'Molde de silicone para decoração', 'Confeitaria e Doces - Decoração (Molde de silicone para decoração)', 'Decoração', 117, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cortador-de-petalas', 'Cortador de pétalas', 'Confeitaria e Doces - Decoração (Cortador de pétalas)', 'Decoração', 118, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'caixas-para-bolo', 'Caixas para bolo', 'Confeitaria e Doces - Embalagens (Caixas para bolo)', 'Embalagens', 119, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'caixas-para-doces', 'Caixas para doces', 'Confeitaria e Doces - Embalagens (Caixas para doces)', 'Embalagens', 120, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'caixas-para-cupcakes', 'Caixas para cupcakes', 'Confeitaria e Doces - Embalagens (Caixas para cupcakes)', 'Embalagens', 121, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bandejas', 'Bandejas', 'Confeitaria e Doces - Embalagens (Bandejas)', 'Embalagens', 122, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bases-para-bolo', 'Bases para bolo', 'Confeitaria e Doces - Embalagens (Bases para bolo)', 'Embalagens', 123, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'sacolas', 'Sacolas', 'Confeitaria e Doces - Embalagens (Sacolas)', 'Embalagens', 124, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'fitas', 'Fitas', 'Confeitaria e Doces - Embalagens (Fitas)', 'Embalagens', 125, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'lacos', 'Laços', 'Confeitaria e Doces - Embalagens (Laços)', 'Embalagens', 126, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'etiquetas', 'Etiquetas', 'Confeitaria e Doces - Embalagens (Etiquetas)', 'Embalagens', 127, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'tags', 'Tags', 'Confeitaria e Doces - Embalagens (Tags)', 'Embalagens', 128, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'adesivos', 'Adesivos', 'Confeitaria e Doces - Embalagens (Adesivos)', 'Embalagens', 129, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'caixas-para-trufas', 'Caixas para trufas', 'Confeitaria e Doces - Embalagens (Caixas para trufas)', 'Embalagens', 130, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'caixas-para-brigadeiro', 'Caixas para brigadeiro', 'Confeitaria e Doces - Embalagens (Caixas para brigadeiro)', 'Embalagens', 131, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'forminhas-para-doces', 'Forminhas para doces', 'Confeitaria e Doces - Embalagens (Forminhas para doces)', 'Embalagens', 132, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'forminhas-de-papel', 'Forminhas de papel', 'Confeitaria e Doces - Embalagens (Forminhas de papel)', 'Embalagens', 133, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'embalagem-para-presente', 'Embalagem para presente', 'Confeitaria e Doces - Embalagens (Embalagem para presente)', 'Embalagens', 134, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'papel-celofane', 'Papel celofane', 'Confeitaria e Doces - Embalagens (Papel celofane)', 'Embalagens', 135, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'papel-de-seda', 'Papel de seda', 'Confeitaria e Doces - Embalagens (Papel de seda)', 'Embalagens', 136, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cinta-para-bolo', 'Cinta para bolo', 'Confeitaria e Doces - Embalagens (Cinta para bolo)', 'Embalagens', 137, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'blister-para-bombom', 'Blister para bombom', 'Confeitaria e Doces - Embalagens (Blister para bombom)', 'Embalagens', 138, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'saquinhos-plasticos', 'Saquinhos plásticos', 'Confeitaria e Doces - Embalagens (Saquinhos plásticos)', 'Embalagens', 139, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'potinhos-para-sobremesa', 'Potinhos para sobremesa', 'Confeitaria e Doces - Embalagens (Potinhos para sobremesa)', 'Embalagens', 140, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'organizadores', 'Organizadores', 'Confeitaria e Doces - Organização (Organizadores)', 'Organização', 141, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'potes', 'Potes', 'Confeitaria e Doces - Organização (Potes)', 'Organização', 142, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'gaveteiros', 'Gaveteiros', 'Confeitaria e Doces - Organização (Gaveteiros)', 'Organização', 143, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'estantes', 'Estantes', 'Confeitaria e Doces - Organização (Estantes)', 'Organização', 144, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'suportes', 'Suportes', 'Confeitaria e Doces - Organização (Suportes)', 'Organização', 145, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'porta-bicos', 'Porta-bicos', 'Confeitaria e Doces - Organização (Porta-bicos)', 'Organização', 146, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'porta-formas', 'Porta-formas', 'Confeitaria e Doces - Organização (Porta-formas)', 'Organização', 147, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'potes-hermeticos', 'Potes herméticos', 'Confeitaria e Doces - Organização (Potes herméticos)', 'Organização', 148, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'potes-de-mantimentos', 'Potes de mantimentos', 'Confeitaria e Doces - Organização (Potes de mantimentos)', 'Organização', 149, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'organizador-de-bicos', 'Organizador de bicos', 'Confeitaria e Doces - Organização (Organizador de bicos)', 'Organização', 150, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'prateleiras', 'Prateleiras', 'Confeitaria e Doces - Organização (Prateleiras)', 'Organização', 151, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cesto-organizador', 'Cesto organizador', 'Confeitaria e Doces - Organização (Cesto organizador)', 'Organização', 152, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'caixa-organizadora', 'Caixa organizadora', 'Confeitaria e Doces - Organização (Caixa organizadora)', 'Organização', 153, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'divisorias', 'Divisórias', 'Confeitaria e Doces - Organização (Divisórias)', 'Organização', 154, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'porta-temperos', 'Porta-temperos', 'Confeitaria e Doces - Organização (Porta-temperos)', 'Organização', 155, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'nichos', 'Nichos', 'Confeitaria e Doces - Organização (Nichos)', 'Organização', 156, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'topos-de-bolo-personalizados', 'Topos de bolo personalizados', 'Confeitaria e Doces - Personalização (Topos de bolo personalizados)', 'Personalização', 157, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'carimbos-para-biscoitos', 'Carimbos para biscoitos', 'Confeitaria e Doces - Personalização (Carimbos para biscoitos)', 'Personalização', 158, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'moldes', 'Moldes', 'Confeitaria e Doces - Personalização (Moldes)', 'Personalização', 159, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cortadores-personalizados', 'Cortadores personalizados', 'Confeitaria e Doces - Personalização (Cortadores personalizados)', 'Personalização', 160, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'impressao-comestivel', 'Impressão comestível', 'Confeitaria e Doces - Personalização (Impressão comestível)', 'Personalização', 161, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'molde-de-silicone', 'Molde de silicone', 'Confeitaria e Doces - Personalização (Molde de silicone)', 'Personalização', 162, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'carimbo-personalizado', 'Carimbo personalizado', 'Confeitaria e Doces - Personalização (Carimbo personalizado)', 'Personalização', 163, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'papel-arroz-personalizado', 'Papel arroz personalizado', 'Confeitaria e Doces - Personalização (Papel arroz personalizado)', 'Personalização', 164, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'stencil-personalizado', 'Stencil personalizado', 'Confeitaria e Doces - Personalização (Stencil personalizado)', 'Personalização', 165, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'etiqueta-personalizada', 'Etiqueta personalizada', 'Confeitaria e Doces - Personalização (Etiqueta personalizada)', 'Personalização', 166, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'marcador-de-confeitaria', 'Marcador de confeitaria', 'Confeitaria e Doces - Personalização (Marcador de confeitaria)', 'Personalização', 167, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'kit-festa', 'Kit festa', 'Confeitaria e Doces - Festas (Kit festa)', 'Festas', 168, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'lembrancinhas', 'Lembrancinhas', 'Confeitaria e Doces - Festas (Lembrancinhas)', 'Festas', 169, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'displays', 'Displays', 'Confeitaria e Doces - Festas (Displays)', 'Festas', 170, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'baloes', 'Balões', 'Confeitaria e Doces - Festas (Balões)', 'Festas', 171, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'paineis', 'Painéis', 'Confeitaria e Doces - Festas (Painéis)', 'Festas', 172, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'suportes-para-doces', 'Suportes para doces', 'Confeitaria e Doces - Festas (Suportes para doces)', 'Festas', 173, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'velas-de-aniversario', 'Velas de aniversário', 'Confeitaria e Doces - Festas (Velas de aniversário)', 'Festas', 174, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'vela-numeral', 'Vela numeral', 'Confeitaria e Doces - Festas (Vela numeral)', 'Festas', 175, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'toalha-de-mesa', 'Toalha de mesa', 'Confeitaria e Doces - Festas (Toalha de mesa)', 'Festas', 176, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'guardanapos', 'Guardanapos', 'Confeitaria e Doces - Festas (Guardanapos)', 'Festas', 177, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'pratos-descartaveis', 'Pratos descartáveis', 'Confeitaria e Doces - Festas (Pratos descartáveis)', 'Festas', 178, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'copos-descartaveis', 'Copos descartáveis', 'Confeitaria e Doces - Festas (Copos descartáveis)', 'Festas', 179, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'talheres-descartaveis', 'Talheres descartáveis', 'Confeitaria e Doces - Festas (Talheres descartáveis)', 'Festas', 180, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cortina-metalizada', 'Cortina metalizada', 'Confeitaria e Doces - Festas (Cortina metalizada)', 'Festas', 181, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'arco-de-baloes', 'Arco de balões', 'Confeitaria e Doces - Festas (Arco de balões)', 'Festas', 182, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'faixa-de-feliz-aniversario', 'Faixa de feliz aniversário', 'Confeitaria e Doces - Festas (Faixa de feliz aniversário)', 'Festas', 183, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cake-board', 'Cake board', 'Confeitaria e Doces - Festas (Cake board)', 'Festas', 184, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bandeja-para-doces', 'Bandeja para doces', 'Confeitaria e Doces - Festas (Bandeja para doces)', 'Festas', 185, true
FROM categories c WHERE c.slug = 'confeitariaDoces' ON CONFLICT DO NOTHING;

-- ---- Casamento ----
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'casamento', 'Casamento', 'Casamento - Termos Gerais (Casamento)', 'Termos Gerais', 1, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'noiva', 'Noiva', 'Casamento - Termos Gerais (Noiva)', 'Termos Gerais', 2, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'festa-de-casamento', 'Festa de Casamento', 'Casamento - Termos Gerais (Festa de Casamento)', 'Termos Gerais', 3, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cerimonia', 'Cerimônia', 'Casamento - Termos Gerais (Cerimônia)', 'Termos Gerais', 4, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'recepcao', 'Recepção', 'Casamento - Termos Gerais (Recepção)', 'Termos Gerais', 5, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'lua-de-mel', 'Lua de Mel', 'Casamento - Termos Gerais (Lua de Mel)', 'Termos Gerais', 6, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'wedding', 'Wedding', 'Casamento - Termos Gerais (Wedding)', 'Termos Gerais', 7, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bride', 'Bride', 'Casamento - Termos Gerais (Bride)', 'Termos Gerais', 8, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'noivado', 'Noivado', 'Casamento - Termos Gerais (Noivado)', 'Termos Gerais', 9, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'pedido-de-casamento', 'Pedido de casamento', 'Casamento - Termos Gerais (Pedido de casamento)', 'Termos Gerais', 10, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'save-the-date', 'Save the date', 'Casamento - Termos Gerais (Save the date)', 'Termos Gerais', 11, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'padrinhos', 'Padrinhos', 'Casamento - Termos Gerais (Padrinhos)', 'Termos Gerais', 12, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'madrinha', 'Madrinha', 'Casamento - Termos Gerais (Madrinha)', 'Termos Gerais', 13, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'daminha', 'Daminha', 'Casamento - Termos Gerais (Daminha)', 'Termos Gerais', 14, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'pajem', 'Pajem', 'Casamento - Termos Gerais (Pajem)', 'Termos Gerais', 15, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bodas', 'Bodas', 'Casamento - Termos Gerais (Bodas)', 'Termos Gerais', 16, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'convites', 'Convites', 'Casamento - Convites e Papelaria (Convites)', 'Convites e Papelaria', 17, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'papelaria', 'Papelaria', 'Casamento - Convites e Papelaria (Papelaria)', 'Convites e Papelaria', 18, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'papelaria-personalizada', 'Papelaria Personalizada', 'Casamento - Convites e Papelaria (Papelaria Personalizada)', 'Convites e Papelaria', 19, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'convites-de-casamento', 'Convites de Casamento', 'Casamento - Convites e Papelaria (Convites de Casamento)', 'Convites e Papelaria', 20, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'plaquinhas', 'Plaquinhas', 'Casamento - Convites e Papelaria (Plaquinhas)', 'Convites e Papelaria', 21, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'menus', 'Menus', 'Casamento - Convites e Papelaria (Menus)', 'Convites e Papelaria', 22, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'lacre-de-cera', 'Lacre de Cera', 'Casamento - Convites e Papelaria (Lacre de Cera)', 'Convites e Papelaria', 23, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'envelopes', 'Envelopes', 'Casamento - Convites e Papelaria (Envelopes)', 'Convites e Papelaria', 24, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'convite-digital', 'Convite digital', 'Casamento - Convites e Papelaria (Convite digital)', 'Convites e Papelaria', 25, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'papel-para-convite', 'Papel para convite', 'Casamento - Convites e Papelaria (Papel para convite)', 'Convites e Papelaria', 26, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'caligrafia', 'Caligrafia', 'Casamento - Convites e Papelaria (Caligrafia)', 'Convites e Papelaria', 27, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'selo-personalizado', 'Selo personalizado', 'Casamento - Convites e Papelaria (Selo personalizado)', 'Convites e Papelaria', 28, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cartao-de-agradecimento', 'Cartão de agradecimento', 'Casamento - Convites e Papelaria (Cartão de agradecimento)', 'Convites e Papelaria', 29, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'placa-de-boas-vindas', 'Placa de boas-vindas', 'Casamento - Convites e Papelaria (Placa de boas-vindas)', 'Convites e Papelaria', 30, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'numeracao-de-mesa', 'Numeração de mesa', 'Casamento - Convites e Papelaria (Numeração de mesa)', 'Convites e Papelaria', 31, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'livro-de-mensagens', 'Livro de mensagens', 'Casamento - Convites e Papelaria (Livro de mensagens)', 'Convites e Papelaria', 32, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'lembrancinhas', 'Lembrancinhas', 'Casamento - Lembrancinhas (Lembrancinhas)', 'Lembrancinhas', 33, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bem-casado', 'Bem Casado', 'Casamento - Lembrancinhas (Bem Casado)', 'Lembrancinhas', 34, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'caixa-mdf', 'Caixa MDF', 'Casamento - Lembrancinhas (Caixa MDF)', 'Lembrancinhas', 35, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'caixinhas-personalizadas', 'Caixinhas Personalizadas', 'Casamento - Lembrancinhas (Caixinhas Personalizadas)', 'Lembrancinhas', 36, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'sacolas-personalizadas', 'Sacolas Personalizadas', 'Casamento - Lembrancinhas (Sacolas Personalizadas)', 'Lembrancinhas', 37, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'sabonete-artesanal', 'Sabonete artesanal', 'Casamento - Lembrancinhas (Sabonete artesanal)', 'Lembrancinhas', 38, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'vela-aromatica', 'Vela aromática', 'Casamento - Lembrancinhas (Vela aromática)', 'Lembrancinhas', 39, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'mini-suculenta', 'Mini suculenta', 'Casamento - Lembrancinhas (Mini suculenta)', 'Lembrancinhas', 40, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'marca-pagina', 'Marca-página', 'Casamento - Lembrancinhas (Marca-página)', 'Lembrancinhas', 41, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'chaveiro-personalizado', 'Chaveiro personalizado', 'Casamento - Lembrancinhas (Chaveiro personalizado)', 'Lembrancinhas', 42, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'ima-de-geladeira', 'Ímã de geladeira', 'Casamento - Lembrancinhas (Ímã de geladeira)', 'Lembrancinhas', 43, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'porta-guardanapo', 'Porta-guardanapo', 'Casamento - Lembrancinhas (Porta-guardanapo)', 'Lembrancinhas', 44, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'potinho-personalizado', 'Potinho personalizado', 'Casamento - Lembrancinhas (Potinho personalizado)', 'Lembrancinhas', 45, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'decoracao-casamento', 'Decoração Casamento', 'Casamento - Decoração (Decoração Casamento)', 'Decoração', 46, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'flores-artificiais', 'Flores Artificiais', 'Casamento - Decoração (Flores Artificiais)', 'Decoração', 47, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'arranjos', 'Arranjos', 'Casamento - Decoração (Arranjos)', 'Decoração', 48, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'velas', 'Velas', 'Casamento - Decoração (Velas)', 'Decoração', 49, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'casticais', 'Castiçais', 'Casamento - Decoração (Castiçais)', 'Decoração', 50, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'mesa-posta', 'Mesa Posta', 'Casamento - Decoração (Mesa Posta)', 'Decoração', 51, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'sousplat', 'Sousplat', 'Casamento - Decoração (Sousplat)', 'Decoração', 52, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'trilho-de-mesa', 'Trilho de Mesa', 'Casamento - Decoração (Trilho de Mesa)', 'Decoração', 53, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'centro-de-mesa', 'Centro de Mesa', 'Casamento - Decoração (Centro de Mesa)', 'Decoração', 54, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'vaso-decorativo', 'Vaso decorativo', 'Casamento - Decoração (Vaso decorativo)', 'Decoração', 55, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'toalha-de-mesa', 'Toalha de mesa', 'Casamento - Decoração (Toalha de mesa)', 'Decoração', 56, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'caminho-de-mesa', 'Caminho de mesa', 'Casamento - Decoração (Caminho de mesa)', 'Decoração', 57, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'suporte-de-vela', 'Suporte de vela', 'Casamento - Decoração (Suporte de vela)', 'Decoração', 58, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'lanterna-decorativa', 'Lanterna decorativa', 'Casamento - Decoração (Lanterna decorativa)', 'Decoração', 59, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cortina-de-led', 'Cortina de LED', 'Casamento - Decoração (Cortina de LED)', 'Decoração', 60, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'painel-de-flores', 'Painel de flores', 'Casamento - Decoração (Painel de flores)', 'Decoração', 61, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'arco-decorativo', 'Arco decorativo', 'Casamento - Decoração (Arco decorativo)', 'Decoração', 62, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'passadeira', 'Passadeira', 'Casamento - Decoração (Passadeira)', 'Decoração', 63, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'placa-decorativa', 'Placa decorativa', 'Casamento - Decoração (Placa decorativa)', 'Decoração', 64, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'guardanapo-de-tecido', 'Guardanapo de tecido', 'Casamento - Decoração (Guardanapo de tecido)', 'Decoração', 65, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cha-de-panela', 'Chá de Panela', 'Casamento - Chá de Panela e Enxoval (Chá de Panela)', 'Chá de Panela e Enxoval', 66, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cha-bar', 'Chá Bar', 'Casamento - Chá de Panela e Enxoval (Chá Bar)', 'Chá de Panela e Enxoval', 67, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cha-de-cozinha', 'Chá de Cozinha', 'Casamento - Chá de Panela e Enxoval (Chá de Cozinha)', 'Chá de Panela e Enxoval', 68, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'utensilios-de-cozinha', 'Utensílios de Cozinha', 'Casamento - Chá de Panela e Enxoval (Utensílios de Cozinha)', 'Chá de Panela e Enxoval', 69, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'nova-casa', 'Nova Casa', 'Casamento - Chá de Panela e Enxoval (Nova Casa)', 'Chá de Panela e Enxoval', 70, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'enxoval', 'Enxoval', 'Casamento - Chá de Panela e Enxoval (Enxoval)', 'Chá de Panela e Enxoval', 71, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cama-mesa-banho', 'Cama Mesa Banho', 'Casamento - Chá de Panela e Enxoval (Cama Mesa Banho)', 'Chá de Panela e Enxoval', 72, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'casa-e-decoracao', 'Casa e Decoração', 'Casamento - Chá de Panela e Enxoval (Casa e Decoração)', 'Chá de Panela e Enxoval', 73, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'organizadores', 'Organizadores', 'Casamento - Chá de Panela e Enxoval (Organizadores)', 'Chá de Panela e Enxoval', 74, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cozinha', 'Cozinha', 'Casamento - Chá de Panela e Enxoval (Cozinha)', 'Chá de Panela e Enxoval', 75, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'banheiro', 'Banheiro', 'Casamento - Chá de Panela e Enxoval (Banheiro)', 'Chá de Panela e Enxoval', 76, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'lavanderia', 'Lavanderia', 'Casamento - Chá de Panela e Enxoval (Lavanderia)', 'Chá de Panela e Enxoval', 77, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'jogo-de-panelas', 'Jogo de panelas', 'Casamento - Chá de Panela e Enxoval (Jogo de panelas)', 'Chá de Panela e Enxoval', 78, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'jogo-de-copos', 'Jogo de copos', 'Casamento - Chá de Panela e Enxoval (Jogo de copos)', 'Chá de Panela e Enxoval', 79, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'jogo-de-talheres', 'Jogo de talheres', 'Casamento - Chá de Panela e Enxoval (Jogo de talheres)', 'Chá de Panela e Enxoval', 80, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'jogo-de-pratos', 'Jogo de pratos', 'Casamento - Chá de Panela e Enxoval (Jogo de pratos)', 'Chá de Panela e Enxoval', 81, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'toalha-de-banho', 'Toalha de banho', 'Casamento - Chá de Panela e Enxoval (Toalha de banho)', 'Chá de Panela e Enxoval', 82, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'jogo-de-cama', 'Jogo de cama', 'Casamento - Chá de Panela e Enxoval (Jogo de cama)', 'Chá de Panela e Enxoval', 83, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'edredom', 'Edredom', 'Casamento - Chá de Panela e Enxoval (Edredom)', 'Chá de Panela e Enxoval', 84, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cortina', 'Cortina', 'Casamento - Chá de Panela e Enxoval (Cortina)', 'Chá de Panela e Enxoval', 85, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'tapete', 'Tapete', 'Casamento - Chá de Panela e Enxoval (Tapete)', 'Chá de Panela e Enxoval', 86, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'potes-hermeticos', 'Potes herméticos', 'Casamento - Chá de Panela e Enxoval (Potes herméticos)', 'Chá de Panela e Enxoval', 87, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'escorredor', 'Escorredor', 'Casamento - Chá de Panela e Enxoval (Escorredor)', 'Chá de Panela e Enxoval', 88, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'organizador-de-armario', 'Organizador de armário', 'Casamento - Chá de Panela e Enxoval (Organizador de armário)', 'Chá de Panela e Enxoval', 89, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'aromatizador', 'Aromatizador', 'Casamento - Chá de Panela e Enxoval (Aromatizador)', 'Chá de Panela e Enxoval', 90, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'kit-banheiro', 'Kit banheiro', 'Casamento - Chá de Panela e Enxoval (Kit banheiro)', 'Chá de Panela e Enxoval', 91, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'maquiagem', 'Maquiagem', 'Casamento - Beleza da Noiva (Maquiagem)', 'Beleza da Noiva', 92, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'skincare', 'Skincare', 'Casamento - Beleza da Noiva (Skincare)', 'Beleza da Noiva', 93, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'perfumes', 'Perfumes', 'Casamento - Beleza da Noiva (Perfumes)', 'Beleza da Noiva', 94, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cilios', 'Cílios', 'Casamento - Beleza da Noiva (Cílios)', 'Beleza da Noiva', 95, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cosmeticos', 'Cosméticos', 'Casamento - Beleza da Noiva (Cosméticos)', 'Beleza da Noiva', 96, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cuidados-com-a-pele', 'Cuidados com a Pele', 'Casamento - Beleza da Noiva (Cuidados com a Pele)', 'Beleza da Noiva', 97, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'batom', 'Batom', 'Casamento - Beleza da Noiva (Batom)', 'Beleza da Noiva', 98, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'base', 'Base', 'Casamento - Beleza da Noiva (Base)', 'Beleza da Noiva', 99, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'paleta-de-sombras', 'Paleta de sombras', 'Casamento - Beleza da Noiva (Paleta de sombras)', 'Beleza da Noiva', 100, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'mascara-de-cilios', 'Máscara de cílios', 'Casamento - Beleza da Noiva (Máscara de cílios)', 'Beleza da Noiva', 101, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cilios-posticos', 'Cílios postiços', 'Casamento - Beleza da Noiva (Cílios postiços)', 'Beleza da Noiva', 102, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'pinceis-de-maquiagem', 'Pincéis de maquiagem', 'Casamento - Beleza da Noiva (Pincéis de maquiagem)', 'Beleza da Noiva', 103, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'iluminador', 'Iluminador', 'Casamento - Beleza da Noiva (Iluminador)', 'Beleza da Noiva', 104, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'fixador-de-maquiagem', 'Fixador de maquiagem', 'Casamento - Beleza da Noiva (Fixador de maquiagem)', 'Beleza da Noiva', 105, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'hidratante', 'Hidratante', 'Casamento - Beleza da Noiva (Hidratante)', 'Beleza da Noiva', 106, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'protetor-solar', 'Protetor solar', 'Casamento - Beleza da Noiva (Protetor solar)', 'Beleza da Noiva', 107, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'serum-facial', 'Sérum facial', 'Casamento - Beleza da Noiva (Sérum facial)', 'Beleza da Noiva', 108, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'kit-skincare', 'Kit skincare', 'Casamento - Beleza da Noiva (Kit skincare)', 'Beleza da Noiva', 109, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'vestido-de-noiva', 'Vestido de Noiva', 'Casamento - Vestuário da Noiva (Vestido de Noiva)', 'Vestuário da Noiva', 110, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'veu', 'Véu', 'Casamento - Vestuário da Noiva (Véu)', 'Vestuário da Noiva', 111, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'grinalda', 'Grinalda', 'Casamento - Vestuário da Noiva (Grinalda)', 'Vestuário da Noiva', 112, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'tiara', 'Tiara', 'Casamento - Vestuário da Noiva (Tiara)', 'Vestuário da Noiva', 113, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'acessorios-para-noiva', 'Acessórios para Noiva', 'Casamento - Vestuário da Noiva (Acessórios para Noiva)', 'Vestuário da Noiva', 114, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'sapato-de-noiva', 'Sapato de Noiva', 'Casamento - Vestuário da Noiva (Sapato de Noiva)', 'Vestuário da Noiva', 115, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'sapatilha-de-noiva', 'Sapatilha de noiva', 'Casamento - Vestuário da Noiva (Sapatilha de noiva)', 'Vestuário da Noiva', 116, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'robe-de-noiva', 'Robe de noiva', 'Casamento - Vestuário da Noiva (Robe de noiva)', 'Vestuário da Noiva', 117, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'liga-de-noiva', 'Liga de noiva', 'Casamento - Vestuário da Noiva (Liga de noiva)', 'Vestuário da Noiva', 118, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'luvas', 'Luvas', 'Casamento - Vestuário da Noiva (Luvas)', 'Vestuário da Noiva', 119, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bolero', 'Bolero', 'Casamento - Vestuário da Noiva (Bolero)', 'Vestuário da Noiva', 120, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'brinco-de-noiva', 'Brinco de noiva', 'Casamento - Vestuário da Noiva (Brinco de noiva)', 'Vestuário da Noiva', 121, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'colar-de-noiva', 'Colar de noiva', 'Casamento - Vestuário da Noiva (Colar de noiva)', 'Vestuário da Noiva', 122, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'buque-de-noiva', 'Buquê de noiva', 'Casamento - Vestuário da Noiva (Buquê de noiva)', 'Vestuário da Noiva', 123, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'terco-de-noiva', 'Terço de noiva', 'Casamento - Vestuário da Noiva (Terço de noiva)', 'Vestuário da Noiva', 124, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'lingerie', 'Lingerie', 'Casamento - Vestuário da Noiva (Lingerie)', 'Vestuário da Noiva', 125, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'porta-aliancas', 'Porta Alianças', 'Casamento - Alianças e Acessórios (Porta Alianças)', 'Alianças e Acessórios', 126, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'caixa-para-aliancas', 'Caixa para Alianças', 'Casamento - Alianças e Acessórios (Caixa para Alianças)', 'Alianças e Acessórios', 127, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'joias', 'Joias', 'Casamento - Alianças e Acessórios (Joias)', 'Alianças e Acessórios', 128, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'semijoias', 'Semijoias', 'Casamento - Alianças e Acessórios (Semijoias)', 'Alianças e Acessórios', 129, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'alianca-de-casamento', 'Aliança de casamento', 'Casamento - Alianças e Acessórios (Aliança de casamento)', 'Alianças e Acessórios', 130, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'almofada-para-aliancas', 'Almofada para alianças', 'Casamento - Alianças e Acessórios (Almofada para alianças)', 'Alianças e Acessórios', 131, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'anel-de-noivado', 'Anel de noivado', 'Casamento - Alianças e Acessórios (Anel de noivado)', 'Alianças e Acessórios', 132, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bijuteria', 'Bijuteria', 'Casamento - Alianças e Acessórios (Bijuteria)', 'Alianças e Acessórios', 133, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'pulseira', 'Pulseira', 'Casamento - Alianças e Acessórios (Pulseira)', 'Alianças e Acessórios', 134, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'colar', 'Colar', 'Casamento - Alianças e Acessórios (Colar)', 'Alianças e Acessórios', 135, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'brinco', 'Brinco', 'Casamento - Alianças e Acessórios (Brinco)', 'Alianças e Acessórios', 136, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'relogio', 'Relógio', 'Casamento - Alianças e Acessórios (Relógio)', 'Alianças e Acessórios', 137, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'planner-casamento', 'Planner Casamento', 'Casamento - Organização do Casamento (Planner Casamento)', 'Organização do Casamento', 138, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'agenda-casamento', 'Agenda Casamento', 'Casamento - Organização do Casamento (Agenda Casamento)', 'Organização do Casamento', 139, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'checklist-casamento', 'Checklist Casamento', 'Casamento - Organização do Casamento (Checklist Casamento)', 'Organização do Casamento', 140, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'album-de-casamento', 'Álbum de Casamento', 'Casamento - Organização do Casamento (Álbum de Casamento)', 'Organização do Casamento', 141, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'livro-de-assinaturas', 'Livro de Assinaturas', 'Casamento - Organização do Casamento (Livro de Assinaturas)', 'Organização do Casamento', 142, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'porta-retrato', 'Porta Retrato', 'Casamento - Organização do Casamento (Porta Retrato)', 'Organização do Casamento', 143, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'caixa-de-recordacoes', 'Caixa de recordações', 'Casamento - Organização do Casamento (Caixa de recordações)', 'Organização do Casamento', 144, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'convite-de-padrinhos', 'Convite de padrinhos', 'Casamento - Organização do Casamento (Convite de padrinhos)', 'Organização do Casamento', 145, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cronograma', 'Cronograma', 'Casamento - Organização do Casamento (Cronograma)', 'Organização do Casamento', 146, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'placa-dos-noivos', 'Placa dos noivos', 'Casamento - Organização do Casamento (Placa dos noivos)', 'Organização do Casamento', 147, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'moldura', 'Moldura', 'Casamento - Organização do Casamento (Moldura)', 'Organização do Casamento', 148, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'porta-lembrancas', 'Porta-lembranças', 'Casamento - Organização do Casamento (Porta-lembranças)', 'Organização do Casamento', 149, true
FROM categories c WHERE c.slug = 'casamento' ON CONFLICT DO NOTHING;

-- ---- Bebê e Maternidade ----
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'jogo-de-quarto-para-bebe', 'Jogo de quarto para bebê', 'Bebê e Maternidade - Quarto do Bebê (Jogo de quarto para bebê)', 'Quarto do Bebê', 1, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'comoda', 'Cômoda', 'Bebê e Maternidade - Quarto do Bebê (Cômoda)', 'Quarto do Bebê', 2, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'moises', 'Moisés', 'Bebê e Maternidade - Quarto do Bebê (Moisés)', 'Quarto do Bebê', 3, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cercadinho', 'Cercadinho', 'Bebê e Maternidade - Quarto do Bebê (Cercadinho)', 'Quarto do Bebê', 4, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'berco', 'Berço', 'Bebê e Maternidade - Quarto do Bebê (Berço)', 'Quarto do Bebê', 5, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'mini-berco', 'Mini berço', 'Bebê e Maternidade - Quarto do Bebê (Mini berço)', 'Quarto do Bebê', 6, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'berco-portatil', 'Berço portátil', 'Bebê e Maternidade - Quarto do Bebê (Berço portátil)', 'Quarto do Bebê', 7, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'kit-berco', 'Kit berço', 'Bebê e Maternidade - Quarto do Bebê (Kit berço)', 'Quarto do Bebê', 8, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'mosquiteiro', 'Mosquiteiro', 'Bebê e Maternidade - Quarto do Bebê (Mosquiteiro)', 'Quarto do Bebê', 9, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'dossel', 'Dossel', 'Bebê e Maternidade - Quarto do Bebê (Dossel)', 'Quarto do Bebê', 10, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'trocador', 'Trocador', 'Bebê e Maternidade - Quarto do Bebê (Trocador)', 'Quarto do Bebê', 11, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'guarda-roupa-infantil', 'Guarda-roupa infantil', 'Bebê e Maternidade - Quarto do Bebê (Guarda-roupa infantil)', 'Quarto do Bebê', 12, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'nicho-para-quarto', 'Nicho para quarto', 'Bebê e Maternidade - Quarto do Bebê (Nicho para quarto)', 'Quarto do Bebê', 13, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'mobile', 'Mobile', 'Bebê e Maternidade - Quarto do Bebê (Mobile)', 'Quarto do Bebê', 14, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'tapete-de-atividades', 'Tapete de atividades', 'Bebê e Maternidade - Quarto do Bebê (Tapete de atividades)', 'Quarto do Bebê', 15, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cabaninha', 'Cabaninha', 'Bebê e Maternidade - Quarto do Bebê (Cabaninha)', 'Quarto do Bebê', 16, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'carrinho', 'Carrinho', 'Bebê e Maternidade - Passeio e Transporte (Carrinho)', 'Passeio e Transporte', 17, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'kit-bolsa-maternidade', 'Kit Bolsa maternidade', 'Bebê e Maternidade - Passeio e Transporte (Kit Bolsa maternidade)', 'Passeio e Transporte', 18, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'carrinho-de-bebe', 'Carrinho de bebê', 'Bebê e Maternidade - Passeio e Transporte (Carrinho de bebê)', 'Passeio e Transporte', 19, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bebe-conforto', 'Bebê conforto', 'Bebê e Maternidade - Passeio e Transporte (Bebê conforto)', 'Passeio e Transporte', 20, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cadeirinha-para-carro', 'Cadeirinha para carro', 'Bebê e Maternidade - Passeio e Transporte (Cadeirinha para carro)', 'Passeio e Transporte', 21, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bolsa-maternidade', 'Bolsa maternidade', 'Bebê e Maternidade - Passeio e Transporte (Bolsa maternidade)', 'Passeio e Transporte', 22, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'mochila-maternidade', 'Mochila maternidade', 'Bebê e Maternidade - Passeio e Transporte (Mochila maternidade)', 'Passeio e Transporte', 23, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'canguru', 'Canguru', 'Bebê e Maternidade - Passeio e Transporte (Canguru)', 'Passeio e Transporte', 24, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'sling', 'Sling', 'Bebê e Maternidade - Passeio e Transporte (Sling)', 'Passeio e Transporte', 25, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'carrinho-guarda-chuva', 'Carrinho guarda-chuva', 'Bebê e Maternidade - Passeio e Transporte (Carrinho guarda-chuva)', 'Passeio e Transporte', 26, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'necessaire-de-bebe', 'Necessaire de bebê', 'Bebê e Maternidade - Passeio e Transporte (Necessaire de bebê)', 'Passeio e Transporte', 27, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'trocador-portatil', 'Trocador portátil', 'Bebê e Maternidade - Passeio e Transporte (Trocador portátil)', 'Passeio e Transporte', 28, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'mamadeira', 'Mamadeira', 'Bebê e Maternidade - Alimentação (Mamadeira)', 'Alimentação', 29, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'kit-mamadeira', 'Kit mamadeira', 'Bebê e Maternidade - Alimentação (Kit mamadeira)', 'Alimentação', 30, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'chupeta', 'Chupeta', 'Bebê e Maternidade - Alimentação (Chupeta)', 'Alimentação', 31, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'esterilizador', 'Esterilizador', 'Bebê e Maternidade - Alimentação (Esterilizador)', 'Alimentação', 32, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'aquecedor-de-mamadeira', 'Aquecedor de mamadeira', 'Bebê e Maternidade - Alimentação (Aquecedor de mamadeira)', 'Alimentação', 33, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'babador', 'Babador', 'Bebê e Maternidade - Alimentação (Babador)', 'Alimentação', 34, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cadeira-de-alimentacao', 'Cadeira de alimentação', 'Bebê e Maternidade - Alimentação (Cadeira de alimentação)', 'Alimentação', 35, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'porta-papinha', 'Porta-papinha', 'Bebê e Maternidade - Alimentação (Porta-papinha)', 'Alimentação', 36, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'copo-de-transicao', 'Copo de transição', 'Bebê e Maternidade - Alimentação (Copo de transição)', 'Alimentação', 37, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'escova-para-mamadeira', 'Escova para mamadeira', 'Bebê e Maternidade - Alimentação (Escova para mamadeira)', 'Alimentação', 38, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bico-de-mamadeira', 'Bico de mamadeira', 'Bebê e Maternidade - Alimentação (Bico de mamadeira)', 'Alimentação', 39, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'camera-para-berco', 'Câmera para berço', 'Bebê e Maternidade - Monitoramento e Segurança (Câmera para berço)', 'Monitoramento e Segurança', 40, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'monitor-bebe-ultrassom', 'Monitor bebê ultrassom', 'Bebê e Maternidade - Monitoramento e Segurança (Monitor bebê ultrassom)', 'Monitoramento e Segurança', 41, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'maquina-de-ruido-branco', 'Máquina de ruído branco', 'Bebê e Maternidade - Monitoramento e Segurança (Máquina de ruído branco)', 'Monitoramento e Segurança', 42, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'baba-eletronica', 'Babá eletrônica', 'Bebê e Maternidade - Monitoramento e Segurança (Babá eletrônica)', 'Monitoramento e Segurança', 43, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'camera-baba', 'Câmera babá', 'Bebê e Maternidade - Monitoramento e Segurança (Câmera babá)', 'Monitoramento e Segurança', 44, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'monitor-de-respiracao', 'Monitor de respiração', 'Bebê e Maternidade - Monitoramento e Segurança (Monitor de respiração)', 'Monitoramento e Segurança', 45, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'sensor-de-movimento', 'Sensor de movimento', 'Bebê e Maternidade - Monitoramento e Segurança (Sensor de movimento)', 'Monitoramento e Segurança', 46, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'protetor-de-berco', 'Protetor de berço', 'Bebê e Maternidade - Monitoramento e Segurança (Protetor de berço)', 'Monitoramento e Segurança', 47, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'grade-de-protecao', 'Grade de proteção', 'Bebê e Maternidade - Monitoramento e Segurança (Grade de proteção)', 'Monitoramento e Segurança', 48, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'protetor-de-tomada', 'Protetor de tomada', 'Bebê e Maternidade - Monitoramento e Segurança (Protetor de tomada)', 'Monitoramento e Segurança', 49, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'trava-de-seguranca', 'Trava de segurança', 'Bebê e Maternidade - Monitoramento e Segurança (Trava de segurança)', 'Monitoramento e Segurança', 50, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'termometro-infantil', 'Termômetro infantil', 'Bebê e Maternidade - Monitoramento e Segurança (Termômetro infantil)', 'Monitoramento e Segurança', 51, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'banheira', 'Banheira', 'Bebê e Maternidade - Higiene e Cuidados (Banheira)', 'Higiene e Cuidados', 52, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'banheira-dobravel', 'Banheira dobrável', 'Bebê e Maternidade - Higiene e Cuidados (Banheira dobrável)', 'Higiene e Cuidados', 53, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'ofuro-para-bebe', 'Ofurô para bebê', 'Bebê e Maternidade - Higiene e Cuidados (Ofurô para bebê)', 'Higiene e Cuidados', 54, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'kit-higiene', 'Kit higiene', 'Bebê e Maternidade - Higiene e Cuidados (Kit higiene)', 'Higiene e Cuidados', 55, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'fralda', 'Fralda', 'Bebê e Maternidade - Higiene e Cuidados (Fralda)', 'Higiene e Cuidados', 56, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'kit-toalha', 'Kit toalha', 'Bebê e Maternidade - Higiene e Cuidados (Kit toalha)', 'Higiene e Cuidados', 57, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'toalha-com-capuz', 'Toalha com capuz', 'Bebê e Maternidade - Higiene e Cuidados (Toalha com capuz)', 'Higiene e Cuidados', 58, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cortador-de-unha-bebe', 'Cortador de unha bebê', 'Bebê e Maternidade - Higiene e Cuidados (Cortador de unha bebê)', 'Higiene e Cuidados', 59, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'aspirador-nasal', 'Aspirador nasal', 'Bebê e Maternidade - Higiene e Cuidados (Aspirador nasal)', 'Higiene e Cuidados', 60, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'termometro-de-banheira', 'Termômetro de banheira', 'Bebê e Maternidade - Higiene e Cuidados (Termômetro de banheira)', 'Higiene e Cuidados', 61, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'body', 'Body', 'Bebê e Maternidade - Enxoval e Roupinhas (Body)', 'Enxoval e Roupinhas', 62, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'macacao', 'Macacão', 'Bebê e Maternidade - Enxoval e Roupinhas (Macacão)', 'Enxoval e Roupinhas', 63, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'manta', 'Manta', 'Bebê e Maternidade - Enxoval e Roupinhas (Manta)', 'Enxoval e Roupinhas', 64, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cueiro', 'Cueiro', 'Bebê e Maternidade - Enxoval e Roupinhas (Cueiro)', 'Enxoval e Roupinhas', 65, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'saco-de-dormir', 'Saco de dormir', 'Bebê e Maternidade - Enxoval e Roupinhas (Saco de dormir)', 'Enxoval e Roupinhas', 66, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'meias', 'Meias', 'Bebê e Maternidade - Enxoval e Roupinhas (Meias)', 'Enxoval e Roupinhas', 67, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'luvas', 'Luvas', 'Bebê e Maternidade - Enxoval e Roupinhas (Luvas)', 'Enxoval e Roupinhas', 68, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'touca', 'Touca', 'Bebê e Maternidade - Enxoval e Roupinhas (Touca)', 'Enxoval e Roupinhas', 69, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'sapatinho', 'Sapatinho', 'Bebê e Maternidade - Enxoval e Roupinhas (Sapatinho)', 'Enxoval e Roupinhas', 70, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'kit-saida-maternidade', 'Kit saída maternidade', 'Bebê e Maternidade - Enxoval e Roupinhas (Kit saída maternidade)', 'Enxoval e Roupinhas', 71, true
FROM categories c WHERE c.slug = 'bebeMaternidade' ON CONFLICT DO NOTHING;

