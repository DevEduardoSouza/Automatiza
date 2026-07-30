-- Reverte o seed de 30/07. As CATEGORIAS nao sao tocadas (ja existiam); so caem
-- as subcategorias criadas aqui e o flag de refinamento volta a false.
DELETE FROM subcategories
WHERE category_id IN (SELECT id FROM categories WHERE slug IN ('pets', 'ferramentas', 'automotivo', 'brinquedos'));

UPDATE categories SET has_refinements = false, updated_at = NOW()
WHERE slug IN ('pets', 'ferramentas', 'automotivo', 'brinquedos');

-- Devolve o flag das 3 categorias de 24/07 ao estado anterior a esta migracao.
UPDATE categories SET has_refinements = true, updated_at = NOW()
WHERE slug IN ('confeitariaDoces', 'casamento', 'bebeMaternidade');
