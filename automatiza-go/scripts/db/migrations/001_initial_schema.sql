-- =============================================================================
-- PostgreSQL Schema for Automatiza
-- Migration: 001_initial_schema.sql
-- This extends the auth schema (init.sql) with all business tables
-- =============================================================================

-- Enable required extensions (idempotent)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- =============================================================================
-- Profiles Extension (add billing fields if not exist)
-- =============================================================================

-- Add billing columns to profiles if they don't exist
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'profiles' AND column_name = 'phone_number') THEN
        ALTER TABLE profiles ADD COLUMN phone_number VARCHAR(20);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'profiles' AND column_name = 'max_automations') THEN
        ALTER TABLE profiles ADD COLUMN max_automations INTEGER DEFAULT 5;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'profiles' AND column_name = 'billing_status') THEN
        ALTER TABLE profiles ADD COLUMN billing_status VARCHAR(50) DEFAULT 'active';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'profiles' AND column_name = 'billing_suspension_kind') THEN
        ALTER TABLE profiles ADD COLUMN billing_suspension_kind VARCHAR(50);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'profiles' AND column_name = 'billing_suspended_at') THEN
        ALTER TABLE profiles ADD COLUMN billing_suspended_at TIMESTAMPTZ;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'profiles' AND column_name = 'billing_suspension_reason') THEN
        ALTER TABLE profiles ADD COLUMN billing_suspension_reason TEXT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'profiles' AND column_name = 'billing_last_event_at') THEN
        ALTER TABLE profiles ADD COLUMN billing_last_event_at TIMESTAMPTZ;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'profiles' AND column_name = 'billing_last_event_type') THEN
        ALTER TABLE profiles ADD COLUMN billing_last_event_type VARCHAR(100);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'profiles' AND column_name = 'billing_next_payment_at') THEN
        ALTER TABLE profiles ADD COLUMN billing_next_payment_at TIMESTAMPTZ;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'profiles' AND column_name = 'plan_access_until') THEN
        ALTER TABLE profiles ADD COLUMN plan_access_until TIMESTAMPTZ;
    END IF;
END
$$;

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
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, wa_group_id)
);

CREATE INDEX IF NOT EXISTS idx_groups_user_id ON groups(user_id);
CREATE INDEX IF NOT EXISTS idx_groups_instance_id ON groups(instance_id);
CREATE INDEX IF NOT EXISTS idx_groups_wa_group_id ON groups(wa_group_id);
CREATE INDEX IF NOT EXISTS idx_groups_status ON groups(status);

-- =============================================================================
-- Group Members (for detailed tracking)
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
    content TEXT NOT NULL,
    variables JSONB DEFAULT '[]'::jsonb,
    is_default BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dispatch_texts_user_id ON dispatch_texts(user_id);

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

    -- Search/Filter config
    shopee_link TEXT,
    keyword TEXT,
    categories JSONB DEFAULT '[]'::jsonb,
    stores JSONB DEFAULT '[]'::jsonb,
    min_price DECIMAL(10, 2),
    max_price DECIMAL(10, 2),
    min_discount DECIMAL(5, 2),
    priority INTEGER DEFAULT 0,
    filter_type VARCHAR(50),

    -- Target groups
    group_ids JSONB DEFAULT '[]'::jsonb,
    send_groups JSONB DEFAULT '[]'::jsonb,

    -- Monitoring config
    monitor_group_ids JSONB DEFAULT '[]'::jsonb,
    monitoring_text_rules JSONB,

    -- Message config
    custom_message TEXT,
    include_image BOOLEAN DEFAULT TRUE,
    dispatch_text_id UUID REFERENCES dispatch_texts(id) ON DELETE SET NULL,

    -- Schedule config
    schedule_time TIME,
    schedule_days JSONB DEFAULT '[]'::jsonb,
    start_time TIME,
    end_time TIME,
    interval_minutes INTEGER DEFAULT 60,

    -- Execution config
    delay_seconds INTEGER DEFAULT 1,
    dedupe_hours INTEGER DEFAULT 168,

    -- Tracking
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_automations_user_id ON automations(user_id);
CREATE INDEX IF NOT EXISTS idx_automations_type ON automations(type);
CREATE INDEX IF NOT EXISTS idx_automations_status ON automations(status);
CREATE INDEX IF NOT EXISTS idx_automations_next_run_at ON automations(next_run_at);

-- =============================================================================
-- Automation Runs (Execution Locks)
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
    UNIQUE(automation_id, started_at)
);

CREATE INDEX IF NOT EXISTS idx_automation_runs_automation_id ON automation_runs(automation_id);
CREATE INDEX IF NOT EXISTS idx_automation_runs_status ON automation_runs(status);

-- =============================================================================
-- Dispatch Logs (Message History)
-- =============================================================================

CREATE TABLE IF NOT EXISTS dispatch_logs (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    instance_id INTEGER REFERENCES instances(id) ON DELETE SET NULL,
    group_id UUID REFERENCES groups(id) ON DELETE SET NULL,
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
    dedup_hash VARCHAR(64),
    dedup_window_hours INTEGER DEFAULT 168,
    type VARCHAR(50),
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
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
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_action_history_user_id ON action_history(user_id);
CREATE INDEX IF NOT EXISTS idx_action_history_action_type ON action_history(action_type);
CREATE INDEX IF NOT EXISTS idx_action_history_created_at ON action_history(created_at);

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

-- =============================================================================
-- Affiliate Credentials (Shopee, Amazon, etc.)
-- =============================================================================

CREATE TABLE IF NOT EXISTS affiliate_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    store VARCHAR(50) NOT NULL,
    app_id VARCHAR(255) NOT NULL,
    app_secret TEXT NOT NULL,
    access_token TEXT,
    refresh_token TEXT,
    token_expires_at TIMESTAMPTZ,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, store)
);

CREATE INDEX IF NOT EXISTS idx_affiliate_credentials_user_id ON affiliate_credentials(user_id);
CREATE INDEX IF NOT EXISTS idx_affiliate_credentials_store ON affiliate_credentials(store);

-- =============================================================================
-- ML Credentials (Mercado Livre - encrypted)
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
-- ML Link Lists (Mercado Livre product lists)
-- =============================================================================

CREATE TABLE IF NOT EXISTS ml_link_lists (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    automation_id UUID REFERENCES automations(id) ON DELETE SET NULL,
    product_url TEXT NOT NULL,
    affiliate_link TEXT,
    status VARCHAR(50) DEFAULT 'pending',
    processed_at TIMESTAMPTZ,
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ml_link_lists_user_id ON ml_link_lists(user_id);
CREATE INDEX IF NOT EXISTS idx_ml_link_lists_automation_id ON ml_link_lists(automation_id);
CREATE INDEX IF NOT EXISTS idx_ml_link_lists_status ON ml_link_lists(status);

-- =============================================================================
-- Saved Redirect Links (URL shortener/redirect)
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
    UNIQUE(slug)
);

CREATE INDEX IF NOT EXISTS idx_saved_redirect_links_user_id ON saved_redirect_links(user_id);
CREATE INDEX IF NOT EXISTS idx_saved_redirect_links_slug ON saved_redirect_links(slug);
CREATE INDEX IF NOT EXISTS idx_saved_redirect_links_is_active ON saved_redirect_links(is_active);

-- =============================================================================
-- System Events (logging)
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
CREATE INDEX IF NOT EXISTS idx_system_events_severity ON system_events(severity);
CREATE INDEX IF NOT EXISTS idx_system_events_created_at ON system_events(created_at);
CREATE INDEX IF NOT EXISTS idx_system_events_user_id ON system_events(user_id);

-- =============================================================================
-- Settings (key-value store)
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
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_logs_source ON webhook_logs(source);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_event_type ON webhook_logs(event_type);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_status ON webhook_logs(status);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_created_at ON webhook_logs(created_at);

-- =============================================================================
-- User Servers (load balancing mapping)
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
-- Contacts (for CRM-like features)
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
CREATE INDEX IF NOT EXISTS idx_contacts_phone_number ON contacts(phone_number);

-- =============================================================================
-- Email Templates
-- =============================================================================

CREATE TABLE IF NOT EXISTS email_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL UNIQUE,
    subject VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    variables JSONB DEFAULT '[]'::jsonb,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- =============================================================================
-- Triggers for updated_at
-- =============================================================================

-- Apply update trigger to all tables with updated_at
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
            DROP TRIGGER IF EXISTS update_%s_updated_at ON %s;
            CREATE TRIGGER update_%s_updated_at
                BEFORE UPDATE ON %s
                FOR EACH ROW
                EXECUTE FUNCTION update_updated_at_column();
        ', t, t, t, t);
    END LOOP;
END
$$;

-- =============================================================================
-- Row Level Security
-- =============================================================================

-- Enable RLS on all business tables
ALTER TABLE instances ENABLE ROW LEVEL SECURITY;
ALTER TABLE groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE group_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE dispatch_texts ENABLE ROW LEVEL SECURITY;
ALTER TABLE automations ENABLE ROW LEVEL SECURITY;
ALTER TABLE automation_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE dispatch_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE action_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE coupons ENABLE ROW LEVEL SECURITY;
ALTER TABLE affiliate_credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE ml_credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE ml_link_lists ENABLE ROW LEVEL SECURITY;
ALTER TABLE saved_redirect_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE system_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_servers ENABLE ROW LEVEL SECURITY;
ALTER TABLE contacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE email_templates ENABLE ROW LEVEL SECURITY;

-- Service role has full access
CREATE POLICY service_role_instances ON instances FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_groups ON groups FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_group_members ON group_members FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_dispatch_texts ON dispatch_texts FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_automations ON automations FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_automation_runs ON automation_runs FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_dispatch_logs ON dispatch_logs FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_action_history ON action_history FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_coupons ON coupons FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_affiliate_credentials ON affiliate_credentials FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_ml_credentials ON ml_credentials FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_ml_link_lists ON ml_link_lists FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_saved_redirect_links ON saved_redirect_links FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_system_events ON system_events FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_settings ON settings FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_webhook_logs ON webhook_logs FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_user_servers ON user_servers FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_contacts ON contacts FOR ALL TO service_role USING (true) WITH CHECK (true);
CREATE POLICY service_role_email_templates ON email_templates FOR ALL TO service_role USING (true) WITH CHECK (true);

-- =============================================================================
-- Grant Permissions
-- =============================================================================

GRANT ALL ON ALL TABLES IN SCHEMA public TO service_role;
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO service_role;

-- =============================================================================
-- Default Settings
-- =============================================================================

INSERT INTO settings (key, value, description) VALUES
    ('waha_api_url', 'http://waha:3000', 'WAHA Plus API URL'),
    ('default_delay_seconds', '1', 'Default delay between messages'),
    ('default_dedupe_hours', '168', 'Default deduplication window in hours'),
    ('max_messages_per_minute', '30', 'Rate limit for messages per minute'),
    ('enable_url_safety', 'true', 'Enable URL safety checks'),
    ('enable_backpressure', 'true', 'Enable queue backpressure')
ON CONFLICT (key) DO NOTHING;

-- =============================================================================
-- Default Email Templates
-- =============================================================================

INSERT INTO email_templates (name, subject, body, variables) VALUES
    ('welcome', 'Bem-vindo ao Automatiza!',
     '<h1>Bem-vindo, {{name}}!</h1><p>Sua conta foi criada com sucesso.</p><p>Sua senha temporaria e: <strong>{{password}}</strong></p><p>Por favor, altere sua senha apos o primeiro login.</p>',
     '["name", "password"]'),
    ('password_reset', 'Redefinir Senha - Automatiza',
     '<h1>Redefinir Senha</h1><p>Clique no link abaixo para redefinir sua senha:</p><p><a href="{{reset_link}}">Redefinir Senha</a></p><p>Este link expira em 1 hora.</p>',
     '["reset_link"]'),
    ('plan_upgrade', 'Parabens! Plano Atualizado - Automatiza',
     '<h1>Plano Atualizado!</h1><p>Ola {{name}},</p><p>Seu plano foi atualizado para <strong>{{plan}}</strong>.</p><p>Aproveite todos os novos recursos!</p>',
     '["name", "plan"]'),
    ('suspension_warning', 'Aviso: Sua conta sera suspensa - Automatiza',
     '<h1>Aviso de Suspensao</h1><p>Ola {{name}},</p><p>Identificamos um problema com seu pagamento. Sua conta sera suspensa em {{days}} dias se o problema nao for resolvido.</p>',
     '["name", "days"]')
ON CONFLICT (name) DO NOTHING;

-- =============================================================================
-- Cleanup Functions
-- =============================================================================

-- Function to clean old dispatch logs (keep last 30 days)
CREATE OR REPLACE FUNCTION cleanup_old_dispatch_logs()
RETURNS void AS $$
BEGIN
    DELETE FROM dispatch_logs WHERE created_at < NOW() - INTERVAL '30 days';
END;
$$ LANGUAGE plpgsql;

-- Function to clean old system events (keep last 7 days)
CREATE OR REPLACE FUNCTION cleanup_old_system_events()
RETURNS void AS $$
BEGIN
    DELETE FROM system_events WHERE created_at < NOW() - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;

-- Function to clean old webhook logs (keep last 7 days)
CREATE OR REPLACE FUNCTION cleanup_old_webhook_logs()
RETURNS void AS $$
BEGIN
    DELETE FROM webhook_logs WHERE created_at < NOW() - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;

-- Function to clean expired sessions
CREATE OR REPLACE FUNCTION cleanup_expired_sessions()
RETURNS void AS $$
BEGIN
    DELETE FROM sessions WHERE expires_at < NOW();
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- Deduplication Helper Function
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

GRANT EXECUTE ON FUNCTION check_dispatch_dedup(UUID, TEXT, VARCHAR, INTEGER) TO service_role;
GRANT EXECUTE ON FUNCTION cleanup_old_dispatch_logs() TO service_role;
GRANT EXECUTE ON FUNCTION cleanup_old_system_events() TO service_role;
GRANT EXECUTE ON FUNCTION cleanup_old_webhook_logs() TO service_role;
GRANT EXECUTE ON FUNCTION cleanup_expired_sessions() TO service_role;
