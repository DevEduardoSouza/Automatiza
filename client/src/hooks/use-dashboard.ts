import { Link2, type LucideIcon, TrendingUp, UserCheck, Users } from 'lucide-react';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';

import { useDashboardConnect } from './use-dashboard-connect';

export interface DashboardStats {
  totalGroups: number;
  fullGroups: number;
  totalMembers: number;
  activeLinks: number;
}

export interface Announcement {
  enabled: boolean;
  title: string;
  body: string;
}

export interface StatCard {
  title: string;
  value: number;
  icon: LucideIcon;
  iconColor: string;
}

interface UseDashboardReturn {
  stats: DashboardStats;
  announcement: Announcement;
  statCards: StatCard[];
  isLoading: boolean;
  isError: boolean;
}

/**
 * Hook for dashboard page logic.
 * Fetches stats and announcement data via ConnectRPC from Go backend.
 */
export function useDashboard(): UseDashboardReturn {
  const { t } = useTranslation();
  const { stats, announcement, isLoading, isError } = useDashboardConnect();

  const statCards = useMemo<StatCard[]>(
    () => [
      {
        title: t('pages.dashboard.stats.totalGroups'),
        value: stats.totalGroups,
        icon: Users,
        iconColor: 'bg-primary',
      },
      {
        title: t('pages.dashboard.stats.fullGroups'),
        value: stats.fullGroups,
        icon: UserCheck,
        iconColor: 'bg-primary',
      },
      {
        title: t('pages.dashboard.stats.totalMembers'),
        value: stats.totalMembers,
        icon: TrendingUp,
        iconColor: 'bg-primary',
      },
      {
        title: t('pages.dashboard.stats.activeLinks'),
        value: stats.activeLinks,
        icon: Link2,
        iconColor: 'bg-primary',
      },
    ],
    [t, stats]
  );

  return {
    stats,
    announcement,
    statCards,
    isLoading,
    isError,
  };
}
