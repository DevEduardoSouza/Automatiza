-- Grandfather existing users: the legacy plans (free/starter/pro/enterprise),
-- where all current ~366 users live, were seeded with group_limit = 1. With the
-- per-plan group cap (PR #184) those users would suddenly be capped at 1 group
-- the moment FEATURE_ENFORCE_PLAN_GROUP_LIMITS is turned on.
--
-- Product decision: do NOT limit current users — only NEW buyers on the 13
-- grupos-* tiers are capped. group_limit = 0 is the "no limit" sentinel that
-- validatePlanGroupLimit treats as unlimited (<= 0 → skip). So existing users
-- stay unlimited; only profiles.plan set to a grupos-* slug enforces a cap.
UPDATE plans
SET group_limit = 0, updated_at = NOW()
WHERE slug IN ('free', 'starter', 'pro', 'enterprise');
