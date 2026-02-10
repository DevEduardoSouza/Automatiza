import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
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

interface LinkSafetyCardProps {
  settings: SettingsState;
  setSettings: (settings: SettingsState) => void;
  t: (key: string) => string;
}

export function LinkSafetyCard({ settings, setSettings, t }: LinkSafetyCardProps) {
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
          {t('admin.settings.linkSafety.title')}
        </span>
      </div>
      <p className="text-xs xs:text-sm mb-3 xs:mb-4" style={{ color: colors.wineTextMuted }}>
        {t('admin.settings.linkSafety.description')}
      </p>
      <div className="space-y-3 xs:space-y-4">
        <div className="space-y-2">
          <Label>{t('admin.settings.linkSafety.mode')}</Label>
          <Select
            value={settings.link_safety_mode}
            onValueChange={(v) => setSettings({ ...settings, link_safety_mode: v })}
          >
            <SelectTrigger>
              <SelectValue placeholder={t('common.select')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="block">{t('admin.settings.linkSafety.modeBlock')}</SelectItem>
              <SelectItem value="redact">{t('admin.settings.linkSafety.modeRedact')}</SelectItem>
              <SelectItem value="off">{t('admin.settings.linkSafety.modeOff')}</SelectItem>
            </SelectContent>
          </Select>
          <p className="text-xs" style={{ color: colors.wineTextMuted }}>
            {t('admin.settings.linkSafety.modeHint')}
          </p>
        </div>

        <div className="flex items-center justify-between rounded-md border p-3">
          <div className="space-y-1">
            <Label htmlFor="enforce-allowlist-switch">
              {t('admin.settings.linkSafety.enforceAllowlist')}
            </Label>
            <p className="text-xs" style={{ color: colors.wineTextMuted }}>
              {t('admin.settings.linkSafety.enforceAllowlistHint')}
            </p>
          </div>
          <Switch
            id="enforce-allowlist-switch"
            checked={settings.link_safety_enforce_allowlist === '1'}
            onCheckedChange={(checked) =>
              setSettings({ ...settings, link_safety_enforce_allowlist: checked ? '1' : '0' })
            }
          />
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-3 xs:gap-4">
          <div className="space-y-2">
            <Label>{t('admin.settings.linkSafety.allowlist')}</Label>
            <Textarea
              placeholder={t('admin.settings.linkSafety.allowlistPlaceholder')}
              value={settings.link_safety_allowed_hosts}
              onChange={(e) =>
                setSettings({ ...settings, link_safety_allowed_hosts: e.target.value })
              }
            />
            <p className="text-xs" style={{ color: colors.wineTextMuted }}>
              {t('admin.settings.linkSafety.allowlistHint')}
            </p>
          </div>
          <div className="space-y-2">
            <Label>{t('admin.settings.linkSafety.blocklist')}</Label>
            <Textarea
              placeholder={t('admin.settings.linkSafety.blocklistPlaceholder')}
              value={settings.link_safety_blocked_hosts}
              onChange={(e) =>
                setSettings({ ...settings, link_safety_blocked_hosts: e.target.value })
              }
            />
            <p className="text-xs" style={{ color: colors.wineTextMuted }}>
              {t('admin.settings.linkSafety.blocklistHint')}
            </p>
          </div>
        </div>

        <div className="space-y-2">
          <Label>{t('admin.settings.linkSafety.trustedRedirects')}</Label>
          <Textarea
            placeholder={t('admin.settings.linkSafety.trustedRedirectsPlaceholder')}
            value={settings.link_safety_trusted_redirect_hosts}
            onChange={(e) =>
              setSettings({ ...settings, link_safety_trusted_redirect_hosts: e.target.value })
            }
          />
          <p className="text-xs" style={{ color: colors.wineTextMuted }}>
            {t('admin.settings.linkSafety.trustedRedirectsHint')}
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div className="space-y-2">
            <Label>{t('admin.settings.linkSafety.maxRedirects')}</Label>
            <Input
              type="number"
              value={settings.link_safety_max_redirects}
              onChange={(e) =>
                setSettings({ ...settings, link_safety_max_redirects: e.target.value })
              }
            />
          </div>
          <div className="space-y-2">
            <Label>{t('admin.settings.linkSafety.timeout')}</Label>
            <Input
              type="number"
              value={settings.link_safety_redirect_timeout_ms}
              onChange={(e) =>
                setSettings({ ...settings, link_safety_redirect_timeout_ms: e.target.value })
              }
            />
          </div>
          <div className="space-y-2">
            <Label>{t('admin.settings.linkSafety.cache')}</Label>
            <Input
              type="number"
              value={settings.link_safety_redirect_cache_seconds}
              onChange={(e) =>
                setSettings({ ...settings, link_safety_redirect_cache_seconds: e.target.value })
              }
            />
          </div>
        </div>
      </div>
    </div>
  );
}
