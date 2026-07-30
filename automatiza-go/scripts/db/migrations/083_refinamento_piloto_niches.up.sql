-- automatiza:migration-mode=transactional
-- Refinamento do buscador: nichos de Pets / Ferramentas / Automotivo / Brinquedos (30/07)
-- Aditiva: INSERT em subcategories + has_refinements=true nas 4 categorias, que JA
-- existem (migracoes 028 e 072). Nao cria nem altera categoria.
--
-- Como estes nichos filtram (nenhum tem productCatId proprio):
--   keywordCategories() em shopee_service.go extrai o texto entre PARENTESES do
--   display_key e o usa como KEYWORD da busca; parentCategoryID() casa essa
--   keyword com o catId da categoria-MAE (prefixo antes do primeiro " - ",
--   que TEM de bater com a chave de initCategoryMapping()).
--   Por isso o texto entre parenteses e uma consulta de busca de verdade, nao um
--   rotulo: o rotulo que o cliente le e a coluna `name`.
-- Reexecutavel: ON CONFLICT DO NOTHING.

UPDATE categories SET has_refinements = true, updated_at = NOW()
WHERE slug IN ('pets', 'ferramentas', 'automotivo', 'brinquedos');

-- Segura Confeitaria/Casamento/Bebe: os 405 nichos deles foram cadastrados em
-- 24/07 e NUNCA apareceram na tela (o painel de refinamento era travado em 7
-- categorias no React). Ao destravar o painel eles apareceriam de uma vez, e a
-- medicao de 30/07 contra a API da Shopee mostrou que 155 dos 405 usam termo de
-- uma palavra so, que sem contexto devolve produto fora do nicho ("Touca" ->
-- touca de cetim adulto, "Potes" -> marmita fitness, "Batedeiras" -> batedor de
-- leite). Deixar em false mantem EXATAMENTE o que o cliente ve hoje (a categoria
-- segue selecionavel, sem painel) ate os termos serem curados.
-- Para religar depois da curadoria: UPDATE ... SET has_refinements = true.
UPDATE categories SET has_refinements = false, updated_at = NOW()
WHERE slug IN ('confeitariaDoces', 'casamento', 'bebeMaternidade');


-- ---- Pets (22 nichos) ----
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'racao-cachorro', 'Ração', 'Pets - Cães (Ração para cachorro)', 'Cães', 1, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'petisco-cachorro', 'Petiscos', 'Pets - Cães (Petisco para cachorro)', 'Cães', 2, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'coleira-cachorro', 'Coleira e guia', 'Pets - Cães (Coleira para cachorro)', 'Cães', 3, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cama-cachorro', 'Caminha', 'Pets - Cães (Cama para cachorro)', 'Cães', 4, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'brinquedo-cachorro', 'Brinquedos', 'Pets - Cães (Brinquedo para cachorro)', 'Cães', 5, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'roupa-cachorro', 'Roupinha', 'Pets - Cães (Roupa para cachorro)', 'Cães', 6, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'racao-gato', 'Ração', 'Pets - Gatos (Ração para gato)', 'Gatos', 7, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'areia-gato', 'Areia higiênica', 'Pets - Gatos (Areia higiênica para gato)', 'Gatos', 8, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'arranhador-gato', 'Arranhador', 'Pets - Gatos (Arranhador para gato)', 'Gatos', 9, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'brinquedo-gato', 'Brinquedos', 'Pets - Gatos (Brinquedo para gato)', 'Gatos', 10, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'caixa-transporte-gato', 'Caixa de transporte', 'Pets - Gatos (Caixa de transporte para gato)', 'Gatos', 11, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'shampoo-pet', 'Shampoo', 'Pets - Higiene e Saúde (Shampoo para cachorro)', 'Higiene e Saúde', 12, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'tapete-higienico-pet', 'Tapete higiênico', 'Pets - Higiene e Saúde (Tapete higiênico para cachorro)', 'Higiene e Saúde', 13, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'antipulgas-pet', 'Antipulgas', 'Pets - Higiene e Saúde (Antipulgas para cães e gatos)', 'Higiene e Saúde', 14, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'escova-pelos-pet', 'Escova de pelos', 'Pets - Higiene e Saúde (Escova para tirar pelos de pet)', 'Higiene e Saúde', 15, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cortador-unha-pet', 'Cortador de unha', 'Pets - Higiene e Saúde (Cortador de unha para pet)', 'Higiene e Saúde', 16, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'comedouro-pet', 'Comedouro', 'Pets - Acessórios (Comedouro para pet)', 'Acessórios', 17, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bebedouro-pet', 'Bebedouro', 'Pets - Acessórios (Bebedouro para pet)', 'Acessórios', 18, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'fonte-agua-gato', 'Fonte de água', 'Pets - Acessórios (Fonte de água para gatos)', 'Acessórios', 19, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'casinha-cachorro', 'Casinha', 'Pets - Acessórios (Casinha para cachorro)', 'Acessórios', 20, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'gaiola-passaro', 'Gaiola', 'Pets - Aves e Roedores (Gaiola para pássaros)', 'Aves e Roedores', 21, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'racao-hamster', 'Ração de roedor', 'Pets - Aves e Roedores (Ração para hamster)', 'Aves e Roedores', 22, true
FROM categories c WHERE c.slug = 'pets' ON CONFLICT DO NOTHING;

-- ---- Ferramentas (20 nichos) ----
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'furadeira', 'Furadeira', 'Ferramentas - Elétricas (Furadeira)', 'Elétricas', 1, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'parafusadeira', 'Parafusadeira', 'Ferramentas - Elétricas (Parafusadeira)', 'Elétricas', 2, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'serra-eletrica', 'Serra', 'Ferramentas - Elétricas (Serra elétrica)', 'Elétricas', 3, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'lixadeira', 'Lixadeira', 'Ferramentas - Elétricas (Lixadeira)', 'Elétricas', 4, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'esmerilhadeira', 'Esmerilhadeira', 'Ferramentas - Elétricas (Esmerilhadeira)', 'Elétricas', 5, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'jogo-chaves', 'Jogo de chaves', 'Ferramentas - Manuais (Jogo de chaves)', 'Manuais', 6, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'alicate', 'Alicate', 'Ferramentas - Manuais (Alicate)', 'Manuais', 7, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'martelo', 'Martelo', 'Ferramentas - Manuais (Martelo)', 'Manuais', 8, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'chave-fenda', 'Chave de fenda', 'Ferramentas - Manuais (Chave de fenda)', 'Manuais', 9, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'chave-catraca', 'Chave catraca', 'Ferramentas - Manuais (Chave catraca)', 'Manuais', 10, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'trena', 'Trena', 'Ferramentas - Medição (Trena)', 'Medição', 11, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'nivel-laser', 'Nível a laser', 'Ferramentas - Medição (Nível a laser)', 'Medição', 12, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'multimetro', 'Multímetro', 'Ferramentas - Medição (Multímetro)', 'Medição', 13, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'paquimetro', 'Paquímetro', 'Ferramentas - Medição (Paquímetro)', 'Medição', 14, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'caixa-ferramentas', 'Caixa de ferramentas', 'Ferramentas - Organização (Caixa de ferramentas)', 'Organização', 15, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bancada-ferramentas', 'Bancada', 'Ferramentas - Organização (Bancada de ferramentas)', 'Organização', 16, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cinto-ferramentas', 'Cinto porta-ferramentas', 'Ferramentas - Organização (Cinto porta ferramentas)', 'Organização', 17, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'maquina-solda', 'Máquina de solda', 'Ferramentas - Solda e Pintura (Máquina de solda)', 'Solda e Pintura', 18, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'compressor-ar', 'Compressor de ar', 'Ferramentas - Solda e Pintura (Compressor de ar)', 'Solda e Pintura', 19, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'pistola-pintura', 'Pistola de pintura', 'Ferramentas - Solda e Pintura (Pistola de pintura)', 'Solda e Pintura', 20, true
FROM categories c WHERE c.slug = 'ferramentas' ON CONFLICT DO NOTHING;

-- ---- Automotivo (19 nichos) ----
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'capa-banco-carro', 'Capa de banco', 'Automotivo - Acessórios Internos (Capa de banco automotivo)', 'Acessórios Internos', 1, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'tapete-carro', 'Tapete', 'Automotivo - Acessórios Internos (Tapete para carro)', 'Acessórios Internos', 2, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'organizador-porta-malas', 'Organizador', 'Automotivo - Acessórios Internos (Organizador de porta-malas)', 'Acessórios Internos', 3, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'suporte-celular-carro', 'Suporte de celular', 'Automotivo - Acessórios Internos (Suporte de celular para carro)', 'Acessórios Internos', 4, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'capa-volante', 'Capa de volante', 'Automotivo - Acessórios Internos (Capa de volante)', 'Acessórios Internos', 5, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'central-multimidia', 'Central multimídia', 'Automotivo - Som e Multimídia (Central multimídia automotiva)', 'Som e Multimídia', 6, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'alto-falante-carro', 'Alto-falante', 'Automotivo - Som e Multimídia (Alto falante automotivo)', 'Som e Multimídia', 7, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'camera-re', 'Câmera de ré', 'Automotivo - Som e Multimídia (Câmera de ré para carro)', 'Som e Multimídia', 8, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'cera-automotiva', 'Cera', 'Automotivo - Limpeza (Cera automotiva)', 'Limpeza', 9, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'aspirador-carro', 'Aspirador', 'Automotivo - Limpeza (Aspirador de carro)', 'Limpeza', 10, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'aromatizante-carro', 'Aromatizante', 'Automotivo - Limpeza (Aromatizante automotivo)', 'Limpeza', 11, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'calota', 'Calota', 'Automotivo - Pneus e Rodas (Calota para roda)', 'Pneus e Rodas', 12, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'calibrador-pneu', 'Calibrador de pneu', 'Automotivo - Pneus e Rodas (Calibrador de pneu)', 'Pneus e Rodas', 13, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'palheta-limpador', 'Palheta', 'Automotivo - Manutenção (Palheta limpador de para-brisa)', 'Manutenção', 14, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'lampada-farol', 'Lâmpada de farol', 'Automotivo - Manutenção (Lâmpada de farol automotivo)', 'Manutenção', 15, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'oleo-motor', 'Óleo de motor', 'Automotivo - Manutenção (Óleo de motor)', 'Manutenção', 16, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'capacete-moto', 'Capacete', 'Automotivo - Moto (Capacete de moto)', 'Moto', 17, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bau-moto', 'Baú', 'Automotivo - Moto (Baú para moto)', 'Moto', 18, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'capa-moto', 'Capa de moto', 'Automotivo - Moto (Capa para moto)', 'Moto', 19, true
FROM categories c WHERE c.slug = 'automotivo' ON CONFLICT DO NOTHING;

-- ---- Brinquedos (22 nichos) ----
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'blocos-montar', 'Blocos de montar', 'Brinquedos - Educativos (Blocos de montar)', 'Educativos', 1, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'quebra-cabeca', 'Quebra-cabeça', 'Brinquedos - Educativos (Quebra-cabeça)', 'Educativos', 2, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'brinquedo-pedagogico', 'Pedagógicos', 'Brinquedos - Educativos (Brinquedo pedagógico)', 'Educativos', 3, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'kit-robotica', 'Robótica', 'Brinquedos - Educativos (Kit de robótica infantil)', 'Educativos', 4, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'boneca', 'Bonecas', 'Brinquedos - Bonecas e Bonecos (Boneca)', 'Bonecas e Bonecos', 5, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'boneca-reborn', 'Boneca reborn', 'Brinquedos - Bonecas e Bonecos (Boneca reborn)', 'Bonecas e Bonecos', 6, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'action-figure', 'Action figure', 'Brinquedos - Bonecas e Bonecos (Action figure)', 'Bonecas e Bonecos', 7, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'boneco-colecionavel', 'Colecionáveis', 'Brinquedos - Bonecas e Bonecos (Boneco colecionável)', 'Bonecas e Bonecos', 8, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'jogo-tabuleiro', 'Tabuleiro', 'Brinquedos - Jogos (Jogo de tabuleiro)', 'Jogos', 9, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'jogo-cartas', 'Cartas', 'Brinquedos - Jogos (Jogo de cartas)', 'Jogos', 10, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'jogo-eletronico-infantil', 'Eletrônicos', 'Brinquedos - Jogos (Jogo eletrônico infantil)', 'Jogos', 11, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'bicicleta-infantil', 'Bicicleta infantil', 'Brinquedos - Ar Livre (Bicicleta infantil)', 'Ar Livre', 12, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'piscina-bolinhas', 'Piscina de bolinhas', 'Brinquedos - Ar Livre (Piscina de bolinhas)', 'Ar Livre', 13, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'escorregador-infantil', 'Escorregador', 'Brinquedos - Ar Livre (Escorregador infantil)', 'Ar Livre', 14, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'pipa', 'Pipa', 'Brinquedos - Ar Livre (Pipa infantil)', 'Ar Livre', 15, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'carrinho-controle-remoto', 'Carrinho', 'Brinquedos - Controle Remoto (Carrinho de controle remoto)', 'Controle Remoto', 16, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'drone-brinquedo', 'Drone', 'Brinquedos - Controle Remoto (Drone de brinquedo)', 'Controle Remoto', 17, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'chocalho', 'Chocalho', 'Brinquedos - Bebê (Chocalho para bebê)', 'Bebê', 18, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'mordedor', 'Mordedor', 'Brinquedos - Bebê (Mordedor para bebê)', 'Bebê', 19, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'tapete-atividades', 'Tapete de atividades', 'Brinquedos - Bebê (Tapete de atividades para bebê)', 'Bebê', 20, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'pelucia', 'Pelúcia', 'Brinquedos - Pelúcias (Pelúcia)', 'Pelúcias', 21, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_system)
SELECT c.id, 'fidget-antiestresse', 'Antiestresse', 'Brinquedos - Pelúcias (Brinquedo antiestresse)', 'Pelúcias', 22, true
FROM categories c WHERE c.slug = 'brinquedos' ON CONFLICT DO NOTHING;
