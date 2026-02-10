-- Seed admin settings with default values.
-- Uses ON CONFLICT to avoid overwriting any pre-existing values.

INSERT INTO settings (key, value, is_secret, created_at, updated_at)
VALUES
  -- SMTP / Email
  ('smtp_host',       'smtp.gmail.com', false, NOW(), NOW()),
  ('smtp_port',       '587',            false, NOW(), NOW()),
  ('smtp_username',   '',               false, NOW(), NOW()),
  ('smtp_password',   '',               true,  NOW(), NOW()),
  ('smtp_from_email', '',               false, NOW(), NOW()),
  ('smtp_from_name',  'Automatiza',     false, NOW(), NOW()),

  -- Kiwify
  ('kiwify_webhook_token', '', true, NOW(), NOW()),

  -- WAHA API
  ('waha_api_key', '', true,  NOW(), NOW()),
  ('waha_api_url', '', false, NOW(), NOW()),

  -- AI
  ('ai_api_key',      '', true,  NOW(), NOW()),
  ('ai_api_url',      '', false, NOW(), NOW()),
  ('ai_model',        '', false, NOW(), NOW()),
  ('ai_image_model',  '', false, NOW(), NOW()),

  -- Support & Tutorial
  ('support_whatsapp', '', false, NOW(), NOW()),
  ('tutorial_link',    '', false, NOW(), NOW()),

  -- Link Safety
  ('link_safety_mode',                   'block', false, NOW(), NOW()),
  ('link_safety_blocked_hosts',          '',      false, NOW(), NOW()),
  ('link_safety_allowed_hosts',          's.shopee.com.br,shopee.com.br,www.shopee.com.br', false, NOW(), NOW()),
  ('link_safety_enforce_allowlist',      '0',     false, NOW(), NOW()),
  ('link_safety_trusted_redirect_hosts', '',      false, NOW(), NOW()),
  ('link_safety_max_redirects',          '6',     false, NOW(), NOW()),
  ('link_safety_redirect_timeout_ms',    '3500',  false, NOW(), NOW()),
  ('link_safety_redirect_cache_seconds', '21600', false, NOW(), NOW()),

  -- Load Balancer
  ('load_balancer_server_1_max_users', '100', false, NOW(), NOW()),
  ('load_balancer_server_2_max_users', '100', false, NOW(), NOW()),
  ('load_balancer_server_3_max_users', '100', false, NOW(), NOW())
ON CONFLICT (key) DO NOTHING;
