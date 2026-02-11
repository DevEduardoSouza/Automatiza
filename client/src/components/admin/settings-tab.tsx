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

const colors = {
  wine: '#8D2D53',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

/**
 * Aba de configurações do sistema.
 * Permite gerenciar chaves de API, webhooks e outras configurações.
 */
export default function SettingsTab() {
  const { t } = useTranslation();
  const { settings, setSettings, loading, saving, handleSaveSettings } = useSettingsTab();

  if (loading) {
    return (
      <div className="flex items-center justify-center py-6 xs:py-8" role="status" aria-live="polite">
        <div className="text-center space-y-4">
          <div
            className="h-8 w-8 xs:h-10 xs:w-10 animate-spin rounded-full border-[3px] xs:border-4 border-t-transparent mx-auto"
            style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
          />
          <p className="text-xs xs:text-sm" style={{ color: colors.wineTextMuted }}>
            {t('admin.settings.loading')}
          </p>
        </div>
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

      <button
        type="button"
        onClick={handleSaveSettings}
        disabled={saving}
        className="auth-btn-glow relative inline-flex w-full lg:w-auto cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
        aria-label={
          saving ? t('admin.settings.savingAriaLabel') : t('admin.settings.saveAriaLabel')
        }
        aria-busy={saving}
      >
        <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
          <div className="auth-btn-glow__glow absolute inset-0" />
        </div>
        <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
          {saving ? (
            t('admin.settings.saving')
          ) : (
            <>
              <Save className="h-4 w-4" />
              {t('admin.settings.saveButton')}
            </>
          )}
        </span>
      </button>
    </div>
  );
}
