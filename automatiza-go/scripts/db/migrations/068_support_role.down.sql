-- 068 down: remove the 'support' role from roles_lookup.
-- Does not touch user_roles rows (removing a support grant is an admin action, not a schema rollback).
DELETE FROM roles_lookup WHERE name = 'support';
