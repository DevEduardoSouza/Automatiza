-- Reverte o seed dos novos nichos (18/07). Remove apenas as linhas semeadas
-- pela 072 up; não toca nas 15 categorias originais nem em subcategorias.
DELETE FROM categories WHERE slug IN (
    'jardinagem', 'construcaoObra', 'ferramentas', 'aquarismo',
    'roupasMaternidade', 'introducaoAlimentar', 'enxovalBebe', 'mamaesBebes',
    'roupasFitness', 'acessoriosFitness', 'suplementosAlimentares', 'sexShop'
);
