-- 068: Register the 'support' role in the roles_lookup enum-like table.
-- The auth mechanism reads user_roles.role (VARCHAR, unconstrained), so this is
-- documentation/consistency only — no FK enforces it — but keeps the lookup honest.
INSERT INTO roles_lookup (name, description) VALUES
    ('support', 'Support staff — access limited to the support panel')
ON CONFLICT (name) DO NOTHING;
