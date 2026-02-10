-- =============================================================================
-- Seed Data for Automatiza Development
-- =============================================================================

-- Test user: test@example.com / Test123456
-- Password hash generated with: go run ./cmd/hashpw/main.go "Test123456"

DO $$
DECLARE
    test_user_id UUID;
    -- Argon2id hash for "Test123456" with default parameters
    test_password_hash TEXT := '$argon2id$v=19$m=65536,t=3,p=2$+Co1fdUnkatbLazibnXUjg$1v2WhlRTqri8rBbeYRLelLJDgicfJUm9dtfaawhGKxE';
BEGIN
    -- Insert test user profile
    INSERT INTO profiles (id, email, display_name, email_verified, plan)
    VALUES (
        'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
        'test@example.com',
        'Test User',
        TRUE,
        'pro'
    )
    ON CONFLICT (email) DO UPDATE SET
        display_name = EXCLUDED.display_name,
        email_verified = EXCLUDED.email_verified,
        plan = EXCLUDED.plan
    RETURNING id INTO test_user_id;

    -- Insert credentials
    INSERT INTO user_credentials (user_id, password_hash)
    VALUES (test_user_id, test_password_hash)
    ON CONFLICT (user_id) DO UPDATE SET
        password_hash = EXCLUDED.password_hash,
        failed_attempts = 0,
        locked_until = NULL;

    -- Insert admin role
    INSERT INTO user_roles (user_id, role)
    VALUES (test_user_id, 'admin')
    ON CONFLICT (user_id) DO UPDATE SET role = EXCLUDED.role;

    RAISE NOTICE 'Test user created: test@example.com / Test123456';
END
$$;

-- Read-only user for testing
DO $$
DECLARE
    readonly_user_id UUID;
    readonly_password_hash TEXT := '$argon2id$v=19$m=65536,t=3,p=2$+Co1fdUnkatbLazibnXUjg$1v2WhlRTqri8rBbeYRLelLJDgicfJUm9dtfaawhGKxE';
BEGIN
    INSERT INTO profiles (id, email, display_name, email_verified, plan)
    VALUES (
        'b1ffcd00-0d1c-5f09-cc7e-7cc0ce491b22',
        'readonly@example.com',
        'Read Only User',
        TRUE,
        'free'
    )
    ON CONFLICT (email) DO UPDATE SET
        display_name = EXCLUDED.display_name
    RETURNING id INTO readonly_user_id;

    INSERT INTO user_credentials (user_id, password_hash)
    VALUES (readonly_user_id, readonly_password_hash)
    ON CONFLICT (user_id) DO UPDATE SET
        password_hash = EXCLUDED.password_hash;

    INSERT INTO user_roles (user_id, role)
    VALUES (readonly_user_id, 'user')
    ON CONFLICT (user_id) DO UPDATE SET role = EXCLUDED.role;

    RAISE NOTICE 'Read-only user created: readonly@example.com / Test123456';
END
$$;

-- Admin user: bruno.clara@yahoo.com / semSenha123!
-- Password hash generated with: go run ./cmd/hashpw/main.go "semSenha123!"

DO $$
DECLARE
    admin_user_id UUID;
    admin_password_hash TEXT := '$argon2id$v=19$m=65536,t=3,p=2$uHM9YnhIt8oYI+VOxCpeQw$+MnNc5gy42aG71qdG0cT7etSrMmfnJ1arMXfj+trsro';
BEGIN
    INSERT INTO profiles (id, email, display_name, email_verified, plan)
    VALUES (
        gen_random_uuid(),
        'bruno.clara@yahoo.com',
        'Bruno Clara',
        TRUE,
        'pro'
    )
    ON CONFLICT (email) DO UPDATE SET
        display_name = EXCLUDED.display_name,
        email_verified = EXCLUDED.email_verified,
        plan = EXCLUDED.plan
    RETURNING id INTO admin_user_id;

    INSERT INTO user_credentials (user_id, password_hash)
    VALUES (admin_user_id, admin_password_hash)
    ON CONFLICT (user_id) DO UPDATE SET
        password_hash = EXCLUDED.password_hash,
        failed_attempts = 0,
        locked_until = NULL;

    INSERT INTO user_roles (user_id, role)
    VALUES (admin_user_id, 'admin')
    ON CONFLICT (user_id) DO UPDATE SET role = EXCLUDED.role;

    RAISE NOTICE 'Admin user created: bruno.clara@yahoo.com / semSenha123!';
END
$$;
