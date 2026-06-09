-- Seed the 13 group-limit plans (Kiwify offers of the single "AUTOMATISA" product).
-- Each tier = price (monthly) -> group_limit, mapped to its Kiwify offer pay-link
-- slug (pay.kiwify.com.br/<slug>) stored in kiwify_product_id_monthly so the
-- webhook's checkout-link match resolves the purchase to the right plan.
--
-- Idempotent: ON CONFLICT (slug) updates the tunable-to-drift fields so re-running
-- (or editing the tiers here) keeps prod in sync. Legacy `price`/`max_groups`
-- mirror the new fields for any code still reading them.
INSERT INTO plans
    (name, slug, description, price, price_monthly, interval, group_limit, max_groups, is_active, sort_order, kiwify_product_id_monthly, features)
VALUES
    ('5 grupos',   'grupos-5',   'Até 5 grupos',   69.00,   69.00,   'monthly', 5,   5,   TRUE, 1,  'u9AgACI', ARRAY['Até 5 grupos']),
    ('10 grupos',  'grupos-10',  'Até 10 grupos',  109.00,  109.00,  'monthly', 10,  10,  TRUE, 2,  'yADivXk', ARRAY['Até 10 grupos']),
    ('20 grupos',  'grupos-20',  'Até 20 grupos',  159.00,  159.00,  'monthly', 20,  20,  TRUE, 3,  'l9IYlC1', ARRAY['Até 20 grupos']),
    ('30 grupos',  'grupos-30',  'Até 30 grupos',  209.00,  209.00,  'monthly', 30,  30,  TRUE, 4,  'YntB8ht', ARRAY['Até 30 grupos']),
    ('40 grupos',  'grupos-40',  'Até 40 grupos',  259.00,  259.00,  'monthly', 40,  40,  TRUE, 5,  'egLQHLd', ARRAY['Até 40 grupos']),
    ('50 grupos',  'grupos-50',  'Até 50 grupos',  309.00,  309.00,  'monthly', 50,  50,  TRUE, 6,  'jRbBe2P', ARRAY['Até 50 grupos']),
    ('60 grupos',  'grupos-60',  'Até 60 grupos',  359.00,  359.00,  'monthly', 60,  60,  TRUE, 7,  '9f0VCUj', ARRAY['Até 60 grupos']),
    ('70 grupos',  'grupos-70',  'Até 70 grupos',  409.00,  409.00,  'monthly', 70,  70,  TRUE, 8,  'FFGkHKB', ARRAY['Até 70 grupos']),
    ('80 grupos',  'grupos-80',  'Até 80 grupos',  459.00,  459.00,  'monthly', 80,  80,  TRUE, 9,  '8bSRc8O', ARRAY['Até 80 grupos']),
    ('90 grupos',  'grupos-90',  'Até 90 grupos',  509.00,  509.00,  'monthly', 90,  90,  TRUE, 10, 'oyUHC0P', ARRAY['Até 90 grupos']),
    ('100 grupos', 'grupos-100', 'Até 100 grupos', 609.00,  609.00,  'monthly', 100, 100, TRUE, 11, 'oUT2CGV', ARRAY['Até 100 grupos']),
    ('150 grupos', 'grupos-150', 'Até 150 grupos', 1009.00, 1009.00, 'monthly', 150, 150, TRUE, 12, 'Tn8xxrJ', ARRAY['Até 150 grupos']),
    ('200 grupos', 'grupos-200', 'Até 200 grupos', 1700.00, 1700.00, 'monthly', 200, 200, TRUE, 13, 'znzVPLf', ARRAY['Até 200 grupos'])
ON CONFLICT (slug) DO UPDATE SET
    name                      = EXCLUDED.name,
    description               = EXCLUDED.description,
    price                     = EXCLUDED.price,
    price_monthly             = EXCLUDED.price_monthly,
    group_limit               = EXCLUDED.group_limit,
    max_groups                = EXCLUDED.max_groups,
    is_active                 = EXCLUDED.is_active,
    sort_order                = EXCLUDED.sort_order,
    kiwify_product_id_monthly = EXCLUDED.kiwify_product_id_monthly,
    features                  = EXCLUDED.features,
    updated_at                = NOW();
