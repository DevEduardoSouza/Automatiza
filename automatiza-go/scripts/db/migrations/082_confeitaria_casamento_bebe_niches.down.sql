-- Reverte o seed dos nichos de 24/07. subcategories caem por FK ON DELETE CASCADE.
DELETE FROM categories WHERE slug IN ('confeitariaDoces', 'casamento', 'bebeMaternidade');
