import { Button } from '@/components/ui/button';
import { Save } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useSettingsTab } from './hooks';
import {
  AICard,
  AnnouncementCard,
  AppSettingsCard,
  KiwifyCard,
  LinkSafetyCard,
  LoadBalancerSettingsCard,
  SMTPCard,
  WahaCard,
} from './settings';

/**
 * Aba de configurações do sistema.
 * Permite gerenciar chaves de API, webhooks e outras configurações.
 */
export default function SettingsTab() {
  const { t } = useTranslation();
  const { settings, setSettings, loading, saving, handleSaveSettings } = useSettingsTab();

  if (loading) {
    return (
      <div className="text-center py-8" role="status" aria-live="polite">
        {t('admin.settings.loading')}
      </div>
    );
  }

  return (
    <div className="space-y-3 xs:space-y-4 sm:space-y-5 lg:space-y-6">
      <SMTPCard settings={settings} setSettings={setSettings} t={t} />
      <KiwifyCard settings={settings} setSettings={setSettings} t={t} />
      <WahaCard settings={settings} setSettings={setSettings} t={t} />
      <AICard settings={settings} setSettings={setSettings} t={t} />
      <AppSettingsCard settings={settings} setSettings={setSettings} t={t} />
      <LoadBalancerSettingsCard settings={settings} setSettings={setSettings} t={t} />
      <LinkSafetyCard settings={settings} setSettings={setSettings} t={t} />
      <AnnouncementCard settings={settings} setSettings={setSettings} t={t} />

      <Button
        onClick={handleSaveSettings}
        disabled={saving}
        className="w-full lg:w-auto"
        aria-label={
          saving ? t('admin.settings.savingAriaLabel') : t('admin.settings.saveAriaLabel')
        }
        aria-busy={saving}
      >
        {saving ? (
          t('admin.settings.saving')
        ) : (
          <>
            <Save className="h-4 w-4 mr-2" />
            {t('admin.settings.saveButton')}
          </>
        )}
      </Button>
    </div>
  );
}
