import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { MessageSquare } from 'lucide-react';
import type { SettingsState } from '../types/settings.types';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

interface AppSettingsCardProps {
  settings: SettingsState;
  setSettings: (settings: SettingsState) => void;
  t: (key: string) => string;
}

export function AppSettingsCard({ settings, setSettings, t }: AppSettingsCardProps) {
  return (
    <div
      className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4 sm:mb-5">
        <div
          className="p-1.5 xs:p-2 sm:p-2.5 rounded-lg"
          style={{ backgroundColor: colors.iconBg }}
        >
          <MessageSquare
            className="w-4 h-4 xs:w-5 xs:h-5"
            style={{ color: colors.wine }}
            aria-hidden="true"
          />
        </div>
        <span
          className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
          style={{ color: colors.wineText }}
        >
          {t('admin.settings.app.title')}
        </span>
      </div>
      <p className="text-xs xs:text-sm mb-3 xs:mb-4" style={{ color: colors.wineTextMuted }}>
        {t('admin.settings.app.description')}
      </p>
      <div className="space-y-3 xs:space-y-4">
        <div className="space-y-2">
          <Label htmlFor="support_whatsapp">{t('admin.settings.app.supportWhatsapp')}</Label>
          <Input
            id="support_whatsapp"
            type="text"
            placeholder={t('admin.settings.app.supportWhatsappPlaceholder')}
            value={settings.support_whatsapp}
            onChange={(e) => setSettings({ ...settings, support_whatsapp: e.target.value })}
          />
          <p className="text-xs" style={{ color: colors.wineTextMuted }}>
            {t('admin.settings.app.supportWhatsappHint')}
          </p>
        </div>
        <div className="space-y-2">
          <Label htmlFor="tutorial_link">{t('admin.settings.app.tutorialLink')}</Label>
          <Input
            id="tutorial_link"
            type="url"
            placeholder={t('admin.settings.app.tutorialLinkPlaceholder')}
            value={settings.tutorial_link}
            onChange={(e) => setSettings({ ...settings, tutorial_link: e.target.value })}
          />
          <p className="text-xs" style={{ color: colors.wineTextMuted }}>
            {t('admin.settings.app.tutorialLinkHint')}
          </p>
        </div>
      </div>
    </div>
  );
}
