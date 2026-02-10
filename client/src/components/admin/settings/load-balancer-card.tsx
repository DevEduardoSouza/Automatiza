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

interface LoadBalancerSettingsCardProps {
  settings: SettingsState;
  setSettings: (settings: SettingsState) => void;
  t: (key: string) => string;
}

export function LoadBalancerSettingsCard({
  settings,
  setSettings,
  t,
}: LoadBalancerSettingsCardProps) {
  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
        <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
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
          {t('admin.settings.loadBalancer.title')}
        </span>
      </div>
      <p className="text-sm mb-3" style={{ color: colors.wineTextMuted }}>
        {t('admin.settings.loadBalancer.description')}
      </p>
      <div className="space-y-3 xs:space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div className="space-y-2">
            <Label htmlFor="lb_srv1">{t('admin.settings.loadBalancer.server1')}</Label>
            <Input
              id="lb_srv1"
              type="number"
              min={1}
              step={1}
              value={settings.load_balancer_server_1_max_users}
              onChange={(e) =>
                setSettings({ ...settings, load_balancer_server_1_max_users: e.target.value })
              }
            />
            <p className="text-xs" style={{ color: colors.wineTextMuted }}>
              {t('admin.settings.loadBalancer.example')}
            </p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="lb_srv2">{t('admin.settings.loadBalancer.server2')}</Label>
            <Input
              id="lb_srv2"
              type="number"
              min={1}
              step={1}
              value={settings.load_balancer_server_2_max_users}
              onChange={(e) =>
                setSettings({ ...settings, load_balancer_server_2_max_users: e.target.value })
              }
            />
            <p className="text-xs" style={{ color: colors.wineTextMuted }}>
              {t('admin.settings.loadBalancer.example')}
            </p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="lb_srv3">{t('admin.settings.loadBalancer.server3')}</Label>
            <Input
              id="lb_srv3"
              type="number"
              min={1}
              step={1}
              value={settings.load_balancer_server_3_max_users}
              onChange={(e) =>
                setSettings({ ...settings, load_balancer_server_3_max_users: e.target.value })
              }
            />
            <p className="text-xs" style={{ color: colors.wineTextMuted }}>
              {t('admin.settings.loadBalancer.example')}
            </p>
          </div>
        </div>
        <p className="text-xs" style={{ color: colors.wineTextMuted }}>
          {t('admin.settings.loadBalancer.tip')}
        </p>
      </div>
    </div>
  );
}
