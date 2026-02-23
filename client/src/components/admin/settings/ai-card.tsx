import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Sparkles } from 'lucide-react';
import type { SettingsState } from '../types/settings.types';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

interface AICardProps {
  settings: SettingsState;
  setSettings: (settings: SettingsState) => void;
  t: (key: string) => string;
}

export function AICard({ settings, setSettings, t }: AICardProps) {
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
          <Sparkles
            className="w-4 h-4 xs:w-5 xs:h-5"
            style={{ color: colors.wine }}
            aria-hidden="true"
          />
        </div>
        <span
          className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
          style={{ color: colors.wineText }}
        >
          {t('admin.settings.ai.title')}
        </span>
      </div>
      <p className="text-xs xs:text-sm mb-3 xs:mb-4" style={{ color: colors.wineTextMuted }}>
        {t('admin.settings.ai.description')}
      </p>
      <div className="space-y-3 xs:space-y-4">
        <div className="space-y-2">
          <Label htmlFor="ai_api_url">{t('admin.settings.ai.url')}</Label>
          <Input
            id="ai_api_url"
            type="url"
            placeholder={t('admin.settings.ai.urlPlaceholder')}
            value={settings.ai_api_url}
            onChange={(e) => setSettings({ ...settings, ai_api_url: e.target.value })}
          />
          <p className="text-xs whitespace-pre-line" style={{ color: colors.wineTextMuted }}>
            {t('admin.settings.ai.urlHint')}
          </p>
        </div>
        <div className="space-y-2">
          <Label htmlFor="ai_api_key">{t('admin.settings.ai.apiKey')}</Label>
          <Input
            id="ai_api_key"
            type="password"
            placeholder={t('admin.settings.ai.apiKeyPlaceholder')}
            value={settings.ai_api_key}
            onChange={(e) => setSettings({ ...settings, ai_api_key: e.target.value })}
          />
          <p className="text-xs" style={{ color: colors.wineTextMuted }}>
            {t('admin.settings.ai.apiKeyHint')}
          </p>
        </div>
        <div className="space-y-2">
          <Label htmlFor="ai_model">{t('admin.settings.ai.model')}</Label>
          <Input
            id="ai_model"
            type="text"
            placeholder={t('admin.settings.ai.modelPlaceholder')}
            value={settings.ai_model}
            onChange={(e) => setSettings({ ...settings, ai_model: e.target.value })}
          />
          <p className="text-xs" style={{ color: colors.wineTextMuted }}>
            {t('admin.settings.ai.modelHint')}
          </p>
        </div>
      </div>
    </div>
  );
}
