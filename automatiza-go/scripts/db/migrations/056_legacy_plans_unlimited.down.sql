-- Restore the original seeded group_limit (1) on the legacy plans.
UPDATE plans
SET group_limit = 1, updated_at = NOW()
WHERE slug IN ('free', 'starter', 'pro', 'enterprise');
