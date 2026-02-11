import type { ReactNode } from 'react';
import { Package, TrendingUp, Users } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
  iconBgHover: 'rgba(141, 45, 83, 0.15)',
};

interface DashboardStats {
  totalUsers: number;
  activeUsers: number;
  inactiveUsers: number;
  planStats: { plan: string; count: number }[];
  growthData: { date: string; users: number }[];
}

interface StatsCardsProps {
  stats: DashboardStats;
}

interface StatCardItemProps {
  label: string;
  value: number;
  subtitle: string;
  icon: ReactNode;
}

function StatCardItem({ label, value, subtitle, icon }: StatCardItemProps) {
  const [hovered, setHovered] = useState(false);

  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-3 xs:p-4 sm:p-5 transition-colors"
      style={{
        backgroundColor: hovered ? colors.wineBgHover : colors.wineBg,
        borderColor: colors.wineBorder,
      }}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      <div className="flex items-center justify-between pb-2">
        <span
          className="text-xs md:text-sm font-semibold uppercase tracking-[0.1em]"
          style={{ color: colors.wineTextLight }}
        >
          {label}
        </span>
        <div
          className="p-1.5 xs:p-2 rounded-lg transition-colors"
          style={{ backgroundColor: hovered ? colors.iconBgHover : colors.iconBg }}
        >
          {icon}
        </div>
      </div>
      <div className="text-xl xs:text-2xl md:text-3xl font-semibold">{value}</div>
      <div className="text-xs mt-0.5" style={{ color: colors.wineTextMuted }}>
        {subtitle}
      </div>
    </div>
  );
}

/**
 * Cards de estatísticas do dashboard admin.
 * Exibe total de usuários, usuários ativos e planos contratados.
 */
export function StatsCards({ stats }: StatsCardsProps) {
  const { t } = useTranslation();

  return (
    <div className="grid gap-3 xs:gap-4 grid-cols-1 xs:grid-cols-2 md:grid-cols-3">
      <StatCardItem
        label={t('admin.dashboard.stats.totalUsers')}
        value={stats.totalUsers}
        subtitle={`${stats.activeUsers} ${t('common.active').toLowerCase()}, ${stats.inactiveUsers} ${t('admin.dashboard.stats.inactiveUsers')}`}
        icon={<Users className="h-4 w-4" style={{ color: colors.wine }} aria-hidden="true" />}
      />
      <StatCardItem
        label={t('admin.dashboard.stats.activeUsers')}
        value={stats.activeUsers}
        subtitle={t('admin.dashboard.stats.activeLast30Days')}
        icon={<TrendingUp className="h-4 w-4" style={{ color: colors.wine }} aria-hidden="true" />}
      />
      <StatCardItem
        label={t('admin.dashboard.stats.contractedPlans')}
        value={stats.planStats.length}
        subtitle={t('admin.dashboard.stats.differentPlans')}
        icon={<Package className="h-4 w-4" style={{ color: colors.wine }} aria-hidden="true" />}
      />
    </div>
  );
}
