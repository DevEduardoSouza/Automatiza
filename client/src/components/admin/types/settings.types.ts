/**
 * Types for the SettingsTab component and useSettingsTab hook.
 * Manages system-wide settings.
 */

export interface Setting {
  key: string;
  value: string | null;
  description: string | null;
}

export interface SettingsState {
  smtp_host: string;
  smtp_port: string;
  smtp_username: string;
  smtp_password: string;
  smtp_from_email: string;
  smtp_from_name: string;
  kiwify_webhook_token: string;
  waha_api_key: string;
  waha_api_url: string;
  ai_api_key: string;
  ai_api_url: string;
  ai_model: string;
  support_whatsapp: string;
  tutorial_link: string;
  dashboard_announcement_enabled: string;
  dashboard_announcement_title: string;
  dashboard_announcement_body: string;
  link_safety_mode: string;
  link_safety_blocked_hosts: string;
  link_safety_allowed_hosts: string;
  link_safety_enforce_allowlist: string;
  link_safety_trusted_redirect_hosts: string;
  link_safety_max_redirects: string;
  link_safety_redirect_timeout_ms: string;
  link_safety_redirect_cache_seconds: string;
}

export const INITIAL_SETTINGS: SettingsState = {
  smtp_host: 'smtp.gmail.com',
  smtp_port: '587',
  smtp_username: '',
  smtp_password: '',
  smtp_from_email: '',
  smtp_from_name: 'Automatiza',
  kiwify_webhook_token: '',
  waha_api_key: '',
  waha_api_url: '',
  ai_api_key: '',
  ai_api_url: '',
  ai_model: '',
  support_whatsapp: '',
  tutorial_link: '',
  dashboard_announcement_enabled: '1',
  dashboard_announcement_title: '',
  dashboard_announcement_body: '',
  link_safety_mode: 'block',
  link_safety_blocked_hosts: '',
  link_safety_allowed_hosts: 's.shopee.com.br,shopee.com.br,www.shopee.com.br',
  link_safety_enforce_allowlist: '0',
  link_safety_trusted_redirect_hosts: '',
  link_safety_max_redirects: '6',
  link_safety_redirect_timeout_ms: '3500',
  link_safety_redirect_cache_seconds: '21600',
};

export const SETTINGS_KEYS = [
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
  'support_whatsapp',
  'tutorial_link',
  'dashboard_announcement_enabled',
  'dashboard_announcement_title',
  'dashboard_announcement_body',
  'link_safety_mode',
  'link_safety_blocked_hosts',
  'link_safety_allowed_hosts',
  'link_safety_enforce_allowlist',
  'link_safety_trusted_redirect_hosts',
  'link_safety_max_redirects',
  'link_safety_redirect_timeout_ms',
  'link_safety_redirect_cache_seconds',
] as const;

export interface UseSettingsTabReturn {
  settings: SettingsState;
  setSettings: (settings: SettingsState) => void;
  loading: boolean;
  saving: boolean;
  handleSaveSettings: () => Promise<void>;
}
