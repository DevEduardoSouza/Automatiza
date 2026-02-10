import { Package, TrendingUp, Users } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
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

/**
 * Cards de estatísticas do dashboard admin.
 * Exibe total de usuários, usuários ativos e planos contratados.
 */
export function StatsCards({ stats }: StatsCardsProps) {
  const { t } = useTranslation();

  return (
    <div className="grid gap-4 grid-cols-1 md:grid-cols-3">
      <div
        className="rounded-xl border p-3 xs:p-4"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <div className="flex items-center justify-between pb-2">
          <div className="text-xs md:text-sm font-medium" style={{ color: colors.wineText }}>
            {t('admin.dashboard.stats.totalUsers')}
          </div>
          <div className="p-1.5 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
            <Users className="h-4 w-4" style={{ color: colors.wine }} aria-hidden="true" />
          </div>
        </div>
        <div className="text-xl md:text-2xl font-bold">{stats.totalUsers}</div>
        <div className="text-xs mt-0.5" style={{ color: colors.wineTextMuted }}>
          {stats.activeUsers} {t('common.active').toLowerCase()}, {stats.inactiveUsers}{' '}
          {t('admin.dashboard.stats.inactiveUsers')}
        </div>
      </div>

      <div
        className="rounded-xl border p-3 xs:p-4"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <div className="flex items-center justify-between pb-2">
          <div className="text-xs md:text-sm font-medium" style={{ color: colors.wineText }}>
            {t('admin.dashboard.stats.activeUsers')}
          </div>
          <div className="p-1.5 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
            <TrendingUp className="h-4 w-4" style={{ color: colors.wine }} aria-hidden="true" />
          </div>
        </div>
        <div className="text-xl md:text-2xl font-bold">{stats.activeUsers}</div>
        <div className="text-xs mt-0.5" style={{ color: colors.wineTextMuted }}>
          {t('admin.dashboard.stats.activeLast30Days')}
        </div>
      </div>

      <div
        className="rounded-xl border p-3 xs:p-4"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <div className="flex items-center justify-between pb-2">
          <div className="text-xs md:text-sm font-medium" style={{ color: colors.wineText }}>
            {t('admin.dashboard.stats.contractedPlans')}
          </div>
          <div className="p-1.5 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
            <Package className="h-4 w-4" style={{ color: colors.wine }} aria-hidden="true" />
          </div>
        </div>
        <div className="text-xl md:text-2xl font-bold">{stats.planStats.length}</div>
        <div className="text-xs mt-0.5" style={{ color: colors.wineTextMuted }}>
          {t('admin.dashboard.stats.differentPlans')}
        </div>
      </div>
    </div>
  );
}
