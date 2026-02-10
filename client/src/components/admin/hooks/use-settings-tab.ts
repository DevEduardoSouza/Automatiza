import { useAuth } from '@/contexts';
import { useSettingsConnect } from '@/hooks/use-settings-connect';
import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import {
  INITIAL_SETTINGS,
  SETTINGS_KEYS,
  type SettingsState,
  type UseSettingsTabReturn,
} from '../types/settings.types';

/**
 * Hook que gerencia a logica da aba de configuracoes.
 * Permite visualizar e salvar configuracoes do sistema.
 */
export function useSettingsTab(): UseSettingsTabReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const {
    settings: remoteSettings,
    loading,
    updateSetting,
    isUpdating,
  } = useSettingsConnect(SETTINGS_KEYS);
  const [settings, setSettings] = useState<SettingsState>(INITIAL_SETTINGS);
  const [saving, setSaving] = useState(false);

  // Sync remote settings to local state when loaded
  useEffect(() => {
    if (!loading && remoteSettings) {
      setSettings({
        smtp_host: (remoteSettings.smtp_host as string) || 'smtp.gmail.com',
        smtp_port: (remoteSettings.smtp_port as string) || '587',
        smtp_username: (remoteSettings.smtp_username as string) || '',
        smtp_password: (remoteSettings.smtp_password as string) || '',
        smtp_from_email: (remoteSettings.smtp_from_email as string) || '',
        smtp_from_name: (remoteSettings.smtp_from_name as string) || 'Automatiza',
        kiwify_webhook_token: (remoteSettings.kiwify_webhook_token as string) || '',
        waha_api_key: (remoteSettings.waha_api_key as string) || '',
        waha_api_url: (remoteSettings.waha_api_url as string) || '',
        ai_api_key: (remoteSettings.ai_api_key as string) || '',
        ai_api_url: (remoteSettings.ai_api_url as string) || '',
        ai_model: (remoteSettings.ai_model as string) || '',
        ai_image_model: (remoteSettings.ai_image_model as string) || '',
        support_whatsapp: (remoteSettings.support_whatsapp as string) || '',
        tutorial_link: (remoteSettings.tutorial_link as string) || '',
        dashboard_announcement_enabled:
          (remoteSettings.dashboard_announcement_enabled as string) || '1',
        dashboard_announcement_title: (remoteSettings.dashboard_announcement_title as string) || '',
        dashboard_announcement_body: (remoteSettings.dashboard_announcement_body as string) || '',
        link_safety_mode: (remoteSettings.link_safety_mode as string) || 'block',
        link_safety_blocked_hosts: (remoteSettings.link_safety_blocked_hosts as string) || '',
        link_safety_allowed_hosts:
          (remoteSettings.link_safety_allowed_hosts as string) ||
          's.shopee.com.br,shopee.com.br,www.shopee.com.br',
        link_safety_enforce_allowlist:
          (remoteSettings.link_safety_enforce_allowlist as string) || '0',
        link_safety_trusted_redirect_hosts:
          (remoteSettings.link_safety_trusted_redirect_hosts as string) || '',
        link_safety_max_redirects: (remoteSettings.link_safety_max_redirects as string) || '6',
        link_safety_redirect_timeout_ms:
          (remoteSettings.link_safety_redirect_timeout_ms as string) || '3500',
        link_safety_redirect_cache_seconds:
          (remoteSettings.link_safety_redirect_cache_seconds as string) || '21600',
        load_balancer_server_1_max_users:
          (remoteSettings.load_balancer_server_1_max_users as string) || '100',
        load_balancer_server_2_max_users:
          (remoteSettings.load_balancer_server_2_max_users as string) || '100',
        load_balancer_server_3_max_users:
          (remoteSettings.load_balancer_server_3_max_users as string) || '100',
      });
    }
  }, [loading, remoteSettings]);

  const handleSaveSettings = useCallback(async () => {
    if (!user) {
      toast.error(t('common.errors.notAuthenticated'));
      return;
    }

    setSaving(true);
    try {
      const settingsToSave = [
        { key: 'smtp_host', value: settings.smtp_host },
        { key: 'smtp_port', value: settings.smtp_port },
        { key: 'smtp_username', value: settings.smtp_username },
        { key: 'smtp_password', value: settings.smtp_password },
        { key: 'smtp_from_email', value: settings.smtp_from_email },
        { key: 'smtp_from_name', value: settings.smtp_from_name },
        { key: 'kiwify_webhook_token', value: settings.kiwify_webhook_token },
        { key: 'waha_api_key', value: settings.waha_api_key },
        { key: 'waha_api_url', value: settings.waha_api_url },
        { key: 'ai_api_key', value: settings.ai_api_key },
        { key: 'ai_api_url', value: settings.ai_api_url },
        { key: 'ai_model', value: settings.ai_model },
        { key: 'ai_image_model', value: settings.ai_image_model },
        { key: 'support_whatsapp', value: settings.support_whatsapp },
        { key: 'tutorial_link', value: settings.tutorial_link },
        { key: 'dashboard_announcement_enabled', value: settings.dashboard_announcement_enabled },
        { key: 'dashboard_announcement_title', value: settings.dashboard_announcement_title },
        { key: 'dashboard_announcement_body', value: settings.dashboard_announcement_body },
        { key: 'link_safety_mode', value: settings.link_safety_mode },
        { key: 'link_safety_blocked_hosts', value: settings.link_safety_blocked_hosts },
        { key: 'link_safety_allowed_hosts', value: settings.link_safety_allowed_hosts },
        { key: 'link_safety_enforce_allowlist', value: settings.link_safety_enforce_allowlist },
        {
          key: 'link_safety_trusted_redirect_hosts',
          value: settings.link_safety_trusted_redirect_hosts,
        },
        { key: 'link_safety_max_redirects', value: settings.link_safety_max_redirects },
        { key: 'link_safety_redirect_timeout_ms', value: settings.link_safety_redirect_timeout_ms },
        {
          key: 'link_safety_redirect_cache_seconds',
          value: settings.link_safety_redirect_cache_seconds,
        },
        {
          key: 'load_balancer_server_1_max_users',
          value: settings.load_balancer_server_1_max_users,
        },
        {
          key: 'load_balancer_server_2_max_users',
          value: settings.load_balancer_server_2_max_users,
        },
        {
          key: 'load_balancer_server_3_max_users',
          value: settings.load_balancer_server_3_max_users,
        },
      ];

      for (const setting of settingsToSave) {
        await updateSetting(setting.key, setting.value);
      }

      toast.success(t('admin.settings.success.saved'));
    } catch (_error) {
      toast.error(t('admin.settings.errors.saveError'));
    } finally {
      setSaving(false);
    }
  }, [settings, user, t, updateSetting]);

  return {
    settings,
    setSettings,
    loading,
    saving: saving || isUpdating,
    handleSaveSettings,
  };
}
