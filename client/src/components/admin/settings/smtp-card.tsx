import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Mail } from 'lucide-react';
import type { SettingsState } from '../types/settings.types';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

interface SMTPCardProps {
  settings: SettingsState;
  setSettings: (settings: SettingsState) => void;
  t: (key: string) => string;
}

export function SMTPCard({ settings, setSettings, t }: SMTPCardProps) {
  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
        <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
          <Mail
            className="w-4 h-4 xs:w-5 xs:h-5"
            style={{ color: colors.wine }}
            aria-hidden="true"
          />
        </div>
        <span
          className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
          style={{ color: colors.wineText }}
        >
          {t('admin.settings.smtp.title')}
        </span>
      </div>
      <p className="text-sm mb-3" style={{ color: colors.wineTextMuted }}>
        {t('admin.settings.smtp.description')}
      </p>
      <div className="space-y-3 xs:space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <Label htmlFor="smtp_host">{t('admin.settings.smtp.host')}</Label>
            <Input
              id="smtp_host"
              type="text"
              placeholder={t('admin.settings.smtp.hostPlaceholder')}
              value={settings.smtp_host}
              onChange={(e) => setSettings({ ...settings, smtp_host: e.target.value })}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="smtp_port">{t('admin.settings.smtp.port')}</Label>
            <Input
              id="smtp_port"
              type="text"
              placeholder={t('admin.settings.smtp.portPlaceholder')}
              value={settings.smtp_port}
              onChange={(e) => setSettings({ ...settings, smtp_port: e.target.value })}
            />
          </div>
        </div>
        <div className="space-y-2">
          <Label htmlFor="smtp_username">{t('admin.settings.smtp.username')}</Label>
          <Input
            id="smtp_username"
            type="email"
            placeholder={t('admin.settings.smtp.usernamePlaceholder')}
            value={settings.smtp_username}
            onChange={(e) => setSettings({ ...settings, smtp_username: e.target.value })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="smtp_password">{t('admin.settings.smtp.password')}</Label>
          <Input
            id="smtp_password"
            type="password"
            placeholder="••••••••••••••••"
            value={settings.smtp_password}
            onChange={(e) => setSettings({ ...settings, smtp_password: e.target.value })}
          />
          <p className="text-xs" style={{ color: colors.wineTextMuted }}>
            {t('admin.settings.smtp.passwordHint')}
          </p>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <Label htmlFor="smtp_from_email">{t('admin.settings.smtp.fromEmail')}</Label>
            <Input
              id="smtp_from_email"
              type="email"
              placeholder="noreply@seudominio.com"
              value={settings.smtp_from_email}
              onChange={(e) => setSettings({ ...settings, smtp_from_email: e.target.value })}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="smtp_from_name">{t('admin.settings.smtp.fromName')}</Label>
            <Input
              id="smtp_from_name"
              type="text"
              placeholder="Automatiza"
              value={settings.smtp_from_name}
              onChange={(e) => setSettings({ ...settings, smtp_from_name: e.target.value })}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
