import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Webhook } from 'lucide-react';
import type { SettingsState } from '../types/settings.types';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

interface KiwifyCardProps {
  settings: SettingsState;
  setSettings: (settings: SettingsState) => void;
  t: (key: string) => string;
}

export function KiwifyCard({ settings, setSettings, t }: KiwifyCardProps) {
  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
        <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
          <Webhook
            className="w-4 h-4 xs:w-5 xs:h-5"
            style={{ color: colors.wine }}
            aria-hidden="true"
          />
        </div>
        <span
          className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
          style={{ color: colors.wineText }}
        >
          {t('admin.settings.kiwify.title')}
        </span>
      </div>
      <p className="text-sm mb-3" style={{ color: colors.wineTextMuted }}>
        {t('admin.settings.kiwify.description')}
      </p>
      <div className="space-y-3 xs:space-y-4">
        <div className="space-y-2">
          <Label htmlFor="kiwify_webhook_token">{t('admin.settings.kiwify.token')}</Label>
          <Input
            id="kiwify_webhook_token"
            type="password"
            placeholder={t('admin.settings.kiwify.tokenPlaceholder')}
            value={settings.kiwify_webhook_token}
            onChange={(e) => setSettings({ ...settings, kiwify_webhook_token: e.target.value })}
          />
          <p className="text-xs" style={{ color: colors.wineTextMuted }}>
            {t('admin.settings.kiwify.tokenHint')}
          </p>
        </div>
      </div>
    </div>
  );
}
