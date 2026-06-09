-- Remove the 13 seeded group-limit plans. Users still referencing these slugs
-- (profiles.plan) will fall back to the model's default limit until reassigned.
DELETE FROM plans WHERE slug IN (
    'grupos-5','grupos-10','grupos-20','grupos-30','grupos-40','grupos-50',
    'grupos-60','grupos-70','grupos-80','grupos-90','grupos-100','grupos-150','grupos-200'
);
