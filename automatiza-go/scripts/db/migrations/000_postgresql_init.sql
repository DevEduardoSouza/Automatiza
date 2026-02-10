-- =============================================================================
-- PostgreSQL Schema for Automatiza (Pure PostgreSQL - No Supabase)
-- Migration: 000_postgresql_init.sql
-- Updated: 2026-02-04 - Fixed schema compatibility with Go auth code
-- =============================================================================

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- =============================================================================
-- Helper Functions
-- =============================================================================

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- Roles Lookup Table (enum-like table for role definitions)
-- =============================================================================

CREATE TABLE IF NOT EXISTS roles_lookup (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO roles_lookup (name, description) VALUES
    ('admin', 'Administrator with full access'),
    ('user', 'Regular user'),
    ('viewer', 'Read-only access')
ON CONFLICT (name) DO NOTHING;

-- =============================================================================
-- Plans (subscription plans)
-- =============================================================================

CREATE TABLE IF NOT EXISTS plans (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    price DECIMAL(10, 2) DEFAULT 0,
    interval VARCHAR(20) DEFAULT 'monthly',
    max_automations INTEGER DEFAULT 5,
    max_groups INTEGER DEFAULT 50,
    max_instances INTEGER DEFAULT 1,
    features TEXT[],
    is_active BOOLEAN DEFAULT TRUE,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    price_monthly NUMERIC(10, 2),
    price_semester NUMERIC(10, 2),
    price_annual NUMERIC(10, 2),
    group_limit INTEGER NOT NULL DEFAULT 1,
    storage_limit BIGINT NOT NULL DEFAULT 1000000000
);

INSERT INTO plans (name, slug, description, price, max_automations, max_groups, max_instances, features) VALUES
    ('Free', 'free', 'Plano gratuito com recursos basicos', 0, 1, 10, 1, ARRAY['1 automacao', '10 grupos', '1 instancia WhatsApp']),
    ('Starter', 'starter', 'Ideal para comecar', 47, 5, 50, 2, ARRAY['5 automacoes', '50 grupos', '2 instancias WhatsApp']),
    ('Pro', 'pro', 'Para profissionais', 97, 20, 200, 5, ARRAY['20 automacoes', '200 grupos', '5 instancias WhatsApp']),
    ('Enterprise', 'enterprise', 'Para grandes operacoes', 197, 100, 1000, 20, ARRAY['Automacoes ilimitadas', 'Grupos ilimitados', '20 instancias WhatsApp'])
ON CONFLICT (slug) DO NOTHING;

-- =============================================================================
-- Profiles (Users)
-- =============================================================================

CREATE TABLE IF NOT EXISTS profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) NOT NULL UNIQUE,
    -- Legacy password_hash kept for compatibility, but auth uses user_credentials
    password_hash VARCHAR(255) DEFAULT '',
    name VARCHAR(255),
    -- display_name is used by Go auth code
    display_name VARCHAR(255),
    phone_number VARCHAR(20),
    avatar_url TEXT,
    role_id INTEGER REFERENCES roles_lookup(id) DEFAULT 2,
    plan_id UUID REFERENCES plans(id),
    -- plan column used by some code paths
    plan VARCHAR(50) DEFAULT 'free',
    max_automations INTEGER DEFAULT 5,
    max_groups INTEGER DEFAULT 50,
    max_instances INTEGER DEFAULT 1,
    billing_status VARCHAR(50) DEFAULT 'active',
    billing_suspension_kind VARCHAR(50),
    billing_suspended_at TIMESTAMPTZ,
    billing_suspension_reason TEXT,
    billing_last_event_at TIMESTAMPTZ,
    billing_last_event_type VARCHAR(100),
    billing_next_payment_at TIMESTAMPTZ,
    plan_access_until TIMESTAMPTZ,
    is_active BOOLEAN DEFAULT TRUE,
    -- email_verified is a boolean flag used by Go auth code
    email_verified BOOLEAN DEFAULT FALSE,
    -- email_verified_at is timestamp for when email was verified
    email_verified_at TIMESTAMPTZ,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_profiles_email ON profiles(email);
CREATE INDEX IF NOT EXISTS idx_profiles_role_id ON profiles(role_id);
CREATE INDEX IF NOT EXISTS idx_profiles_plan_id ON profiles(plan_id);
CREATE INDEX IF NOT EXISTS idx_profiles_billing_status ON profiles(billing_status);

-- =============================================================================
-- User Credentials (separate table for auth - used by Go auth code)
-- =============================================================================

CREATE TABLE IF NOT EXISTS user_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    failed_attempts INTEGER DEFAULT 0,
    locked_until TIMESTAMPTZ,
    password_changed_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_credentials_user_id ON user_credentials(user_id);

-- =============================================================================
-- User Roles (mapping table user_id -> role - used by Go auth code)
-- =============================================================================

CREATE TABLE IF NOT EXISTS user_roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE UNIQUE,
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);

-- =============================================================================
-- Sessions (User Authentication Sessions)
-- =============================================================================

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    -- token_hash is optional in new auth system
    token_hash VARCHAR(255) DEFAULT '',
    refresh_token_hash VARCHAR(255),
    -- csrf_token_hash used by Go auth code
    csrf_token_hash VARCHAR(255),
    -- device_fingerprint used by Go auth code
    device_fingerprint VARCHAR(255),
    user_agent TEXT,
    ip_address VARCHAR(45),
    expires_at TIMESTAMPTZ NOT NULL,
    refresh_expires_at TIMESTAMPTZ,
    -- last_activity_at used by Go auth code
    last_activity_at TIMESTAMPTZ DEFAULT NOW(),
    -- revoked_at used by Go auth code for session revocation
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- =============================================================================
-- Password Reset Tokens
-- =============================================================================

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token_hash ON password_reset_tokens(token_hash);


-- =============================================================================
-- WhatsApp Instances
-- =============================================================================

CREATE TABLE IF NOT EXISTS instances (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    instance_id VARCHAR(255) NOT NULL,
    friendly_name VARCHAR(255),
    status VARCHAR(50) DEFAULT 'disconnected',
    qr_code TEXT,
    phone_number VARCHAR(20),
    server_id VARCHAR(100),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, instance_id)
);

CREATE INDEX IF NOT EXISTS idx_instances_user_id ON instances(user_id);
CREATE INDEX IF NOT EXISTS idx_instances_instance_id ON instances(instance_id);
CREATE INDEX IF NOT EXISTS idx_instances_status ON instances(status);

-- =============================================================================
-- WhatsApp Groups
-- =============================================================================

CREATE TABLE IF NOT EXISTS groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    instance_id INTEGER NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    wa_group_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    picture_url TEXT,
    members_count INTEGER DEFAULT 0,
    member_limit INTEGER DEFAULT 1024,
    status VARCHAR(50) DEFAULT 'active',
    is_admin BOOLEAN DEFAULT FALSE,
    invite_code VARCHAR(100),
    admin_only BOOLEAN DEFAULT FALSE,
    is_favorite BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, wa_group_id)
);

CREATE INDEX IF NOT EXISTS idx_groups_user_id ON groups(user_id);
CREATE INDEX IF NOT EXISTS idx_groups_instance_id ON groups(instance_id);
CREATE INDEX IF NOT EXISTS idx_groups_wa_group_id ON groups(wa_group_id);
CREATE INDEX IF NOT EXISTS idx_groups_status ON groups(status);
CREATE INDEX IF NOT EXISTS idx_groups_user_admin ON groups(user_id, is_admin);

-- =============================================================================
-- Group Members
-- =============================================================================

CREATE TABLE IF NOT EXISTS group_members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    wa_user_id VARCHAR(255) NOT NULL,
    phone_number VARCHAR(20),
    push_name VARCHAR(255),
    is_admin BOOLEAN DEFAULT FALSE,
    is_super_admin BOOLEAN DEFAULT FALSE,
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(group_id, wa_user_id)
);

CREATE INDEX IF NOT EXISTS idx_group_members_group_id ON group_members(group_id);
CREATE INDEX IF NOT EXISTS idx_group_members_wa_user_id ON group_members(wa_user_id);

-- =============================================================================
-- Dispatch Texts (Message Templates)
-- =============================================================================

CREATE TABLE IF NOT EXISTS dispatch_texts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    media_url TEXT,
    media_type VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX IF NOT EXISTS idx_dispatch_texts_user_id ON dispatch_texts(user_id);

-- =============================================================================
-- Categories (system-wide product categories)
-- =============================================================================

CREATE TABLE IF NOT EXISTS categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    icon VARCHAR(50),
    display_order INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    has_refinements BOOLEAN DEFAULT FALSE,
    is_system BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_categories_slug ON categories(slug);
CREATE INDEX IF NOT EXISTS idx_categories_display_order ON categories(display_order);
CREATE INDEX IF NOT EXISTS idx_categories_is_active ON categories(is_active) WHERE is_active = TRUE;

-- =============================================================================
-- Subcategories
-- =============================================================================

CREATE TABLE IF NOT EXISTS subcategories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    slug VARCHAR(100) NOT NULL,
    name VARCHAR(150) NOT NULL,
    display_key VARCHAR(200) NOT NULL,
    group_name VARCHAR(100),
    display_order INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    is_system BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(category_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_subcategories_category_id ON subcategories(category_id);
CREATE INDEX IF NOT EXISTS idx_subcategories_display_order ON subcategories(display_order);
CREATE INDEX IF NOT EXISTS idx_subcategories_display_key ON subcategories(display_key);
CREATE INDEX IF NOT EXISTS idx_subcategories_is_active ON subcategories(is_active) WHERE is_active = TRUE;

-- =============================================================================
-- Automations
-- =============================================================================

CREATE TABLE IF NOT EXISTS automations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) DEFAULT 'search',
    status VARCHAR(50) DEFAULT 'inactive',
    mode VARCHAR(50),
    category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    coupon_id UUID,
    shopee_link TEXT,
    keyword TEXT,
    categories JSONB DEFAULT '[]'::jsonb,
    stores JSONB DEFAULT '[]'::jsonb,
    min_price DECIMAL(10, 2),
    max_price DECIMAL(10, 2),
    min_discount DECIMAL(5, 2),
    priority INTEGER DEFAULT 0,
    filter_type VARCHAR(50),
    group_ids JSONB DEFAULT '[]'::jsonb,
    send_groups JSONB DEFAULT '[]'::jsonb,
    monitor_group_ids JSONB DEFAULT '[]'::jsonb,
    monitoring_text_rules JSONB,
    custom_message TEXT,
    include_image BOOLEAN DEFAULT TRUE,
    dispatch_text_id UUID REFERENCES dispatch_texts(id) ON DELETE SET NULL,
    schedule_time TIME,
    schedule_days JSONB DEFAULT '[]'::jsonb,
    start_time TIME,
    end_time TIME,
    interval_minutes INTEGER DEFAULT 60,
    delay_seconds INTEGER DEFAULT 1,
    dedupe_hours INTEGER DEFAULT 168,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    texts TEXT[] DEFAULT '{}',
    ctas TEXT[] DEFAULT '{}',
    last_error TEXT,
    last_error_at TIMESTAMPTZ,
    ml_link_list_id UUID,
    list_items_per_run INTEGER,
    list_include_image BOOLEAN DEFAULT FALSE,
    priority_type TEXT
);

CREATE INDEX IF NOT EXISTS idx_automations_user_id ON automations(user_id);
CREATE INDEX IF NOT EXISTS idx_automations_type ON automations(type);
CREATE INDEX IF NOT EXISTS idx_automations_status ON automations(status);
CREATE INDEX IF NOT EXISTS idx_automations_next_run_at ON automations(next_run_at);
CREATE INDEX IF NOT EXISTS idx_automations_category_id ON automations(category_id);

-- =============================================================================
-- Automation Runs
-- =============================================================================

CREATE TABLE IF NOT EXISTS automation_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    automation_id UUID NOT NULL REFERENCES automations(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    status VARCHAR(50) DEFAULT 'running',
    offers_found INTEGER DEFAULT 0,
    messages_sent INTEGER DEFAULT 0,
    messages_failed INTEGER DEFAULT 0,
    error_message TEXT,
    lock_until TIMESTAMPTZ,
    UNIQUE(automation_id, started_at)
);

CREATE INDEX IF NOT EXISTS idx_automation_runs_automation_id ON automation_runs(automation_id);
CREATE INDEX IF NOT EXISTS idx_automation_runs_status ON automation_runs(status);

-- =============================================================================
-- Dispatch Logs
-- =============================================================================

CREATE TABLE IF NOT EXISTS dispatch_logs (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    instance_id INTEGER REFERENCES instances(id) ON DELETE SET NULL,
    group_id TEXT,
    automation_id UUID REFERENCES automations(id) ON DELETE SET NULL,
    automation_name VARCHAR(255),
    store VARCHAR(50),
    status VARCHAR(50) DEFAULT 'pending',
    message TEXT,
    product_url TEXT,
    product_page_url TEXT,
    canonical_key VARCHAR(255),
    product_category VARCHAR(100),
    product_price DECIMAL(10, 2),
    product_discount DECIMAL(5, 2),
    affiliate_link TEXT,
    coupon_code VARCHAR(50),
    coupon_discount VARCHAR(50),
    coupon_id UUID,
    coupon_name VARCHAR(255),
    dedup_hash VARCHAR(64),
    dedup_window_hours INTEGER DEFAULT 168,
    type VARCHAR(50),
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    coupon_applied BOOLEAN,
    coupon_skip_reason TEXT
);

CREATE INDEX IF NOT EXISTS idx_dispatch_logs_user_id ON dispatch_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_dispatch_logs_automation_id ON dispatch_logs(automation_id);
CREATE INDEX IF NOT EXISTS idx_dispatch_logs_status ON dispatch_logs(status);
CREATE INDEX IF NOT EXISTS idx_dispatch_logs_canonical_key ON dispatch_logs(canonical_key);
CREATE INDEX IF NOT EXISTS idx_dispatch_logs_created_at ON dispatch_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_dispatch_logs_dedup ON dispatch_logs(user_id, group_id, canonical_key, created_at);

-- =============================================================================
-- Action History
-- =============================================================================

CREATE TABLE IF NOT EXISTS action_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    action_type VARCHAR(100) NOT NULL,
    description TEXT NOT NULL,
    details JSONB,
    status VARCHAR(50) DEFAULT 'pending',
    error_message TEXT,
    total_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    target_count INTEGER DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    CONSTRAINT action_history_status_check CHECK (status IN ('pending', 'running', 'completed', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_action_history_user_id ON action_history(user_id);
CREATE INDEX IF NOT EXISTS idx_action_history_action_type ON action_history(action_type);
CREATE INDEX IF NOT EXISTS idx_action_history_status ON action_history(status);
CREATE INDEX IF NOT EXISTS idx_action_history_created_at ON action_history(created_at);

-- =============================================================================
-- Jobs (Standalone scheduled jobs)
-- =============================================================================

CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    type VARCHAR(100) NOT NULL,
    -- action_type used by Go jobs code
    action_type VARCHAR(100),
    status VARCHAR(50) DEFAULT 'pending',
    payload JSONB,
    result JSONB,
    error TEXT,
    -- error_message and last_error used by Go jobs code
    error_message TEXT,
    last_error TEXT,
    scheduled_for TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,
    -- retry_count used by Go jobs code
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    retry_after TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_jobs_user_id ON jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs(type);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_scheduled_for ON jobs(scheduled_for);

-- =============================================================================
-- Sync Jobs
-- =============================================================================

CREATE TABLE IF NOT EXISTS sync_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    instance_id INTEGER REFERENCES instances(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    progress INTEGER DEFAULT 0,
    total INTEGER DEFAULT 0,
    error TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sync_jobs_user_id ON sync_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_sync_jobs_instance_id ON sync_jobs(instance_id);
CREATE INDEX IF NOT EXISTS idx_sync_jobs_status ON sync_jobs(status);

-- =============================================================================
-- Coupons
-- =============================================================================

CREATE TABLE IF NOT EXISTS coupons (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    code VARCHAR(100) NOT NULL,
    title VARCHAR(255) NOT NULL,
    store VARCHAR(50) NOT NULL,
    affiliate_link TEXT,
    expires_at TIMESTAMPTZ,
    is_active BOOLEAN DEFAULT TRUE,
    category VARCHAR(100),
    discount_type VARCHAR(50),
    discount_value DECIMAL(10, 2),
    max_discount DECIMAL(10, 2),
    min_product_price DECIMAL(10, 2),
    coupon_text TEXT,
    usage_count INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_coupons_user_id ON coupons(user_id);
CREATE INDEX IF NOT EXISTS idx_coupons_store ON coupons(store);
CREATE INDEX IF NOT EXISTS idx_coupons_is_active ON coupons(is_active);
CREATE INDEX IF NOT EXISTS idx_coupons_code ON coupons(code);

-- Now add coupon_id FK to automations
ALTER TABLE automations DROP CONSTRAINT IF EXISTS automations_coupon_id_fkey;
ALTER TABLE automations ADD CONSTRAINT automations_coupon_id_fkey
    FOREIGN KEY (coupon_id) REFERENCES coupons(id) ON DELETE SET NULL;

-- =============================================================================
-- Affiliate Credentials
-- =============================================================================

CREATE TABLE IF NOT EXISTS affiliate_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    store VARCHAR(50) NOT NULL,
    app_id VARCHAR(255),
    app_secret TEXT,
    access_token TEXT,
    refresh_token TEXT,
    token_expires_at TIMESTAMPTZ,
    affiliate_id VARCHAR(255),
    tracking_id VARCHAR(255),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    credentials JSONB,
    auto_generate BOOLEAN DEFAULT TRUE,
    UNIQUE(user_id, store)
);

CREATE INDEX IF NOT EXISTS idx_affiliate_credentials_user_id ON affiliate_credentials(user_id);
CREATE INDEX IF NOT EXISTS idx_affiliate_credentials_store ON affiliate_credentials(store);
CREATE INDEX IF NOT EXISTS idx_affiliate_credentials_is_active ON affiliate_credentials(is_active) WHERE is_active = TRUE;
CREATE INDEX IF NOT EXISTS idx_affiliate_credentials_auto_generate ON affiliate_credentials(auto_generate) WHERE auto_generate = TRUE;
CREATE INDEX IF NOT EXISTS idx_affiliate_credentials_credentials ON affiliate_credentials USING GIN (credentials);

-- =============================================================================
-- ML Credentials (Mercado Livre)
-- =============================================================================

CREATE TABLE IF NOT EXISTS ml_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE UNIQUE,
    encrypted_data TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ml_credentials_user_id ON ml_credentials(user_id);

-- =============================================================================
-- ML Link Lists
-- =============================================================================

CREATE TABLE IF NOT EXISTS ml_link_lists (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ml_link_lists_user_id ON ml_link_lists(user_id);

-- =============================================================================
-- ML Link List Items
-- =============================================================================

CREATE TABLE IF NOT EXISTS ml_link_list_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    list_id UUID NOT NULL REFERENCES ml_link_lists(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    product_url TEXT NOT NULL,
    affiliate_link TEXT,
    product_id VARCHAR(100),
    product_title VARCHAR(500),
    product_price DECIMAL(10, 2),
    product_image_url TEXT,
    aliases TEXT[],
    status VARCHAR(50) DEFAULT 'pending',
    consumed_at TIMESTAMPTZ,
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(list_id, product_url)
);

CREATE INDEX IF NOT EXISTS idx_ml_link_list_items_list_id ON ml_link_list_items(list_id);
CREATE INDEX IF NOT EXISTS idx_ml_link_list_items_user_id ON ml_link_list_items(user_id);
CREATE INDEX IF NOT EXISTS idx_ml_link_list_items_status ON ml_link_list_items(status);
CREATE INDEX IF NOT EXISTS idx_ml_link_list_items_unconsumed ON ml_link_list_items(list_id, status) WHERE status = 'ready' AND consumed_at IS NULL;

-- =============================================================================
-- Saved Redirect Links
-- =============================================================================

CREATE TABLE IF NOT EXISTS saved_redirect_links (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    slug VARCHAR(100) NOT NULL,
    name VARCHAR(255),
    strategy VARCHAR(50) DEFAULT 'member_limit',
    target_groups JSONB DEFAULT '[]'::jsonb,
    click_count INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    group_clicks JSONB DEFAULT '{}'::jsonb,
    UNIQUE(slug)
);

CREATE INDEX IF NOT EXISTS idx_saved_redirect_links_user_id ON saved_redirect_links(user_id);
CREATE INDEX IF NOT EXISTS idx_saved_redirect_links_slug ON saved_redirect_links(slug);
CREATE INDEX IF NOT EXISTS idx_saved_redirect_links_user_active ON saved_redirect_links(user_id, is_active);
CREATE INDEX IF NOT EXISTS idx_saved_redirect_links_group_clicks ON saved_redirect_links USING GIN (group_clicks);

-- =============================================================================
-- System Events
-- =============================================================================

CREATE TABLE IF NOT EXISTS system_events (
    id SERIAL PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB,
    severity VARCHAR(20) DEFAULT 'info',
    source VARCHAR(100),
    user_id UUID REFERENCES profiles(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_system_events_event_type ON system_events(event_type);
CREATE INDEX IF NOT EXISTS idx_system_events_created_at ON system_events(created_at);

-- =============================================================================
-- Settings
-- =============================================================================

CREATE TABLE IF NOT EXISTS settings (
    key VARCHAR(255) PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT,
    is_secret BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- =============================================================================
-- Webhook Logs
-- =============================================================================

CREATE TABLE IF NOT EXISTS webhook_logs (
    id SERIAL PRIMARY KEY,
    source VARCHAR(100) NOT NULL,
    event_type VARCHAR(100),
    payload JSONB,
    status VARCHAR(50) DEFAULT 'received',
    error TEXT,
    processed_at TIMESTAMPTZ,
    kiwify_order_id VARCHAR(100),
    kiwify_product_id VARCHAR(100),
    kiwify_customer_email VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_logs_source ON webhook_logs(source);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_event_type ON webhook_logs(event_type);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_created_at ON webhook_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_kiwify ON webhook_logs(kiwify_order_id);

-- =============================================================================
-- Webhook Events (WAHA idempotency)
-- =============================================================================

CREATE TABLE IF NOT EXISTS webhook_events (
    id SERIAL PRIMARY KEY,
    event_id VARCHAR(128) NOT NULL,
    session_name VARCHAR(128) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'received'
        CHECK (status IN ('received', 'processing', 'processed', 'failed', 'dlq')),
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT uq_webhook_events_event_id UNIQUE (event_id)
);

CREATE INDEX IF NOT EXISTS idx_webhook_events_session ON webhook_events(session_name);
CREATE INDEX IF NOT EXISTS idx_webhook_events_status ON webhook_events(status) WHERE status IN ('received', 'processing', 'failed');
CREATE INDEX IF NOT EXISTS idx_webhook_events_received_at ON webhook_events(received_at);

-- =============================================================================
-- User Servers (load balancing)
-- =============================================================================

CREATE TABLE IF NOT EXISTS user_servers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE UNIQUE,
    server_id VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_servers_user_id ON user_servers(user_id);
CREATE INDEX IF NOT EXISTS idx_user_servers_server_id ON user_servers(server_id);

-- =============================================================================
-- Contacts
-- =============================================================================

CREATE TABLE IF NOT EXISTS contacts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    wa_user_id VARCHAR(255) NOT NULL,
    phone_number VARCHAR(20),
    name VARCHAR(255),
    push_name VARCHAR(255),
    profile_picture_url TEXT,
    tags JSONB DEFAULT '[]'::jsonb,
    notes TEXT,
    is_blocked BOOLEAN DEFAULT FALSE,
    last_interaction_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, wa_user_id)
);

CREATE INDEX IF NOT EXISTS idx_contacts_user_id ON contacts(user_id);
CREATE INDEX IF NOT EXISTS idx_contacts_wa_user_id ON contacts(wa_user_id);

-- =============================================================================
-- Email Templates
-- =============================================================================

CREATE TABLE IF NOT EXISTS email_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL UNIQUE,
    subject VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    description TEXT,
    variables JSONB DEFAULT '[]'::jsonb,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- =============================================================================
-- Reconciliation Checkpoints
-- =============================================================================

CREATE TABLE IF NOT EXISTS reconciliation_checkpoints (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    checkpoint_type VARCHAR(50) NOT NULL,
    last_processed_id VARCHAR(255),
    last_processed_at TIMESTAMPTZ,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, checkpoint_type)
);

CREATE INDEX IF NOT EXISTS idx_reconciliation_checkpoints_user_id ON reconciliation_checkpoints(user_id);

-- =============================================================================
-- Apply updated_at Triggers
-- =============================================================================

DO $$
DECLARE
    t text;
BEGIN
    FOR t IN
        SELECT table_name
        FROM information_schema.columns
        WHERE column_name = 'updated_at'
        AND table_schema = 'public'
    LOOP
        EXECUTE format('
            DROP TRIGGER IF EXISTS update_%s_updated_at ON %I;
            CREATE TRIGGER update_%s_updated_at
                BEFORE UPDATE ON %I
                FOR EACH ROW
                EXECUTE FUNCTION update_updated_at_column();
        ', t, t, t, t);
    END LOOP;
END
$$;

-- =============================================================================
-- Default Settings
-- =============================================================================

INSERT INTO settings (key, value, description) VALUES
    ('waha_api_url', 'http://localhost:3001', 'WAHA API URL'),
    ('waha_api_key', '', 'WAHA API Key'),
    ('default_delay_seconds', '1', 'Default delay between messages'),
    ('default_dedupe_hours', '168', 'Default deduplication window in hours'),
    ('max_messages_per_minute', '30', 'Rate limit for messages per minute'),
    ('enable_url_safety', 'true', 'Enable URL safety checks'),
    ('enable_backpressure', 'true', 'Enable queue backpressure'),
    ('smtp_host', '', 'SMTP Host'),
    ('smtp_port', '587', 'SMTP Port'),
    ('smtp_user', '', 'SMTP User'),
    ('smtp_password', '', 'SMTP Password (encrypted)'),
    ('smtp_from_email', '', 'SMTP From Email'),
    ('smtp_from_name', 'Automatiza', 'SMTP From Name'),
    ('ai_api_key', '', 'AI API Key'),
    ('ai_model', 'gemini-1.5-flash', 'AI Model'),
    ('platform_name', 'Automatiza Grupos', 'Platform Name'),
    ('platform_url', 'https://astecwms.com.br', 'Platform URL'),
    ('support_email', 'suporte@astecwms.com.br', 'Support Email'),
    ('max_free_automations', '1', 'Max automations for free plan'),
    ('max_free_groups', '10', 'Max groups for free plan'),
    ('trial_days', '7', 'Trial period in days'),
    ('maintenance_mode', 'false', 'Maintenance mode enabled'),
    ('registration_enabled', 'true', 'User registration enabled'),
    ('require_email_verification', 'false', 'Require email verification'),
    ('default_timezone', 'America/Sao_Paulo', 'Default timezone')
ON CONFLICT (key) DO NOTHING;

-- =============================================================================
-- Default Email Templates
-- =============================================================================

INSERT INTO email_templates (name, subject, body, description, variables) VALUES
    ('welcome', 'Bem-vindo ao Automatiza!',
     '<h1>Bem-vindo, {{name}}!</h1><p>Sua conta foi criada com sucesso.</p><p>Sua senha temporaria e: <strong>{{password}}</strong></p><p>Por favor, altere sua senha apos o primeiro login.</p>',
     'Email enviado quando um novo usuario e criado',
     '["name", "password"]'),
    ('password_reset', 'Redefinir Senha - Automatiza',
     '<h1>Redefinir Senha</h1><p>Ola {{name}},</p><p>Clique no link abaixo para redefinir sua senha:</p><p><a href="{{reset_link}}">Redefinir Senha</a></p><p>Este link expira em 1 hora.</p>',
     'Email para redefinicao de senha',
     '["name", "reset_link"]'),
    ('plan_upgrade', 'Parabens! Plano Atualizado - Automatiza',
     '<h1>Plano Atualizado!</h1><p>Ola {{name}},</p><p>Seu plano foi atualizado para <strong>{{plan}}</strong>.</p><p>Aproveite todos os novos recursos!</p>',
     'Email enviado quando o plano do usuario e atualizado',
     '["name", "plan"]'),
    ('suspension_warning', 'Aviso: Sua conta sera suspensa - Automatiza',
     '<h1>Aviso de Suspensao</h1><p>Ola {{name}},</p><p>Identificamos um problema com seu pagamento. Sua conta sera suspensa em {{days}} dias se o problema nao for resolvido.</p>',
     'Email de aviso de suspensao por falta de pagamento',
     '["name", "days"]')
ON CONFLICT (name) DO NOTHING;

-- =============================================================================
-- Helper Functions
-- =============================================================================

CREATE OR REPLACE FUNCTION check_dispatch_dedup(
    p_user_id UUID,
    p_group_id TEXT,
    p_canonical_key VARCHAR,
    p_window_hours INTEGER DEFAULT 168
)
RETURNS BOOLEAN AS $$
DECLARE
    v_exists BOOLEAN;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM dispatch_logs
        WHERE user_id = p_user_id
        AND group_id = p_group_id
        AND canonical_key = p_canonical_key
        AND status IN ('sent', 'queued', 'pending')
        AND created_at > NOW() - (p_window_hours || ' hours')::INTERVAL
    ) INTO v_exists;
    RETURN v_exists;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION cleanup_old_dispatch_logs()
RETURNS void AS $$
BEGIN
    DELETE FROM dispatch_logs WHERE created_at < NOW() - INTERVAL '30 days';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION cleanup_old_system_events()
RETURNS void AS $$
BEGIN
    DELETE FROM system_events WHERE created_at < NOW() - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION cleanup_old_webhook_logs()
RETURNS void AS $$
BEGIN
    DELETE FROM webhook_logs WHERE created_at < NOW() - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION cleanup_expired_sessions()
RETURNS void AS $$
BEGIN
    DELETE FROM sessions WHERE expires_at < NOW();
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- Create Admin User
-- Password: admin123 (bcrypt hash pre-computed for compatibility)
-- IMPORTANT: Change password in production!
-- =============================================================================

-- Create admin profile
INSERT INTO profiles (
    id,
    email,
    password_hash,
    name,
    display_name,
    role_id,
    plan,
    max_automations,
    max_groups,
    max_instances,
    email_verified
)
SELECT
    uuid_generate_v4(),
    'admin@astecwms.com.br',
    -- bcrypt hash for 'admin123' (cost 10)
    '$2a$10$vH6t09grEgKBPA/eY3zBAOeF4kbMNM6tRcO2qfwvjdzX2PlSiJxau',
    'Administrador',
    'Administrador',
    (SELECT id FROM roles_lookup WHERE name = 'admin'),
    'enterprise',
    1000,
    10000,
    100,
    TRUE
WHERE NOT EXISTS (SELECT 1 FROM profiles WHERE email = 'admin@astecwms.com.br')
RETURNING id;

-- Create admin credentials in user_credentials table (used by Go auth code)
INSERT INTO user_credentials (user_id, password_hash)
SELECT id, '$2a$10$vH6t09grEgKBPA/eY3zBAOeF4kbMNM6tRcO2qfwvjdzX2PlSiJxau'
FROM profiles WHERE email = 'admin@astecwms.com.br'
ON CONFLICT (user_id) DO NOTHING;

-- Create admin role mapping in user_roles table (used by Go auth code)
INSERT INTO user_roles (user_id, role)
SELECT id, 'admin'
FROM profiles WHERE email = 'admin@astecwms.com.br'
ON CONFLICT (user_id) DO UPDATE SET role = 'admin';

-- =============================================================================
-- Done!
-- =============================================================================
