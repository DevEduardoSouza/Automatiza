-- Remove admin settings seeded by 026_admin_settings_seed.up.sql.
-- Only deletes the specific keys that were inserted.

DELETE FROM settings WHERE key IN (
  'smtp_host',
  'smtp_port',
  'smtp_username',
  'smtp_password',
  'smtp_from_email',
  'smtp_from_name',
  'kiwify_webhook_token',
  'waha_api_key',
  'waha_api_url',
  'ai_api_key',
  'ai_api_url',
  'ai_model',
  'ai_image_model',
  'support_whatsapp',
  'tutorial_link',
  'link_safety_mode',
  'link_safety_blocked_hosts',
  'link_safety_allowed_hosts',
  'link_safety_enforce_allowlist',
  'link_safety_trusted_redirect_hosts',
  'link_safety_max_redirects',
  'link_safety_redirect_timeout_ms',
  'link_safety_redirect_cache_seconds',
  'load_balancer_server_1_max_users',
  'load_balancer_server_2_max_users',
  'load_balancer_server_3_max_users'
);
