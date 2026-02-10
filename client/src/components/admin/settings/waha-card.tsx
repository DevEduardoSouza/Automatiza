import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Key } from 'lucide-react';
import type { SettingsState } from '../types/settings.types';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

interface WahaCardProps {
  settings: SettingsState;
  setSettings: (settings: SettingsState) => void;
  t: (key: string) => string;
}

export function WahaCard({ settings, setSettings, t }: WahaCardProps) {
  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
        <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
          <Key
            className="w-4 h-4 xs:w-5 xs:h-5"
            style={{ color: colors.wine }}
            aria-hidden="true"
          />
        </div>
        <span
          className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
          style={{ color: colors.wineText }}
        >
          {t('admin.settings.waha.title')}
        </span>
      </div>
      <p className="text-sm mb-3" style={{ color: colors.wineTextMuted }}>
        {t('admin.settings.waha.description')}
      </p>
      <div className="space-y-3 xs:space-y-4">
        <div className="space-y-2">
          <Label htmlFor="waha_api_url">{t('admin.settings.waha.url')}</Label>
          <Input
            id="waha_api_url"
            type="url"
            placeholder={t('admin.settings.waha.urlPlaceholder')}
            value={settings.waha_api_url}
            onChange={(e) => setSettings({ ...settings, waha_api_url: e.target.value })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="waha_api_key">{t('admin.settings.waha.apiKey')}</Label>
          <Input
            id="waha_api_key"
            type="password"
            placeholder={t('admin.settings.waha.apiKeyPlaceholder')}
            value={settings.waha_api_key}
            onChange={(e) => setSettings({ ...settings, waha_api_key: e.target.value })}
          />
        </div>
      </div>
    </div>
  );
}
