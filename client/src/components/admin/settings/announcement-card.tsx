import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { BookOpen } from 'lucide-react';
import type { SettingsState } from '../types/settings.types';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

interface AnnouncementCardProps {
  settings: SettingsState;
  setSettings: (settings: SettingsState) => void;
  t: (key: string) => string;
}

export function AnnouncementCard({ settings, setSettings, t }: AnnouncementCardProps) {
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
          <BookOpen
            className="w-4 h-4 xs:w-5 xs:h-5"
            style={{ color: colors.wine }}
            aria-hidden="true"
          />
        </div>
        <span
          className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
          style={{ color: colors.wineText }}
        >
          {t('admin.settings.announcement.title')}
        </span>
      </div>
      <p className="text-xs xs:text-sm mb-3 xs:mb-4" style={{ color: colors.wineTextMuted }}>
        {t('admin.settings.announcement.description')}
      </p>
      <div className="space-y-3 xs:space-y-4">
        <div className="flex items-center justify-between gap-4">
          <div>
            <Label htmlFor="announcement-enabled-switch">
              {t('admin.settings.announcement.enabled')}
            </Label>
            <p className="text-xs" style={{ color: colors.wineTextMuted }}>
              {t('admin.settings.announcement.enabledHint')}
            </p>
          </div>
          <Switch
            id="announcement-enabled-switch"
            checked={settings.dashboard_announcement_enabled !== '0'}
            onCheckedChange={(checked) =>
              setSettings({ ...settings, dashboard_announcement_enabled: checked ? '1' : '0' })
            }
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="dashboard_announcement_title">
            {t('admin.settings.announcement.titleLabel')}
          </Label>
          <Input
            id="dashboard_announcement_title"
            type="text"
            placeholder={t('admin.settings.announcement.titlePlaceholder')}
            value={settings.dashboard_announcement_title}
            onChange={(e) =>
              setSettings({ ...settings, dashboard_announcement_title: e.target.value })
            }
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="dashboard_announcement_body">
            {t('admin.settings.announcement.messageLabel')}
          </Label>
          <Textarea
            id="dashboard_announcement_body"
            placeholder={t('admin.settings.announcement.messagePlaceholder')}
            value={settings.dashboard_announcement_body}
            onChange={(e) =>
              setSettings({ ...settings, dashboard_announcement_body: e.target.value })
            }
            rows={5}
          />
        </div>
      </div>
    </div>
  );
}
