import { useAuth } from '@/contexts';
import {
  getAnnouncement,
  getDashboardHistory,
  getDashboardStats,
} from '@/gen/dashboard/v1/dashboard-DashboardService_connectquery';
import { Period } from '@/gen/dashboard/v1/dashboard_pb';
import { useQuery } from '@connectrpc/connect-query';
import { useMemo } from 'react';

export interface DashboardStatsData {
  totalGroups: number;
  fullGroups: number;
  totalMembers: number;
  activeLinks: number;
}

export interface AnnouncementData {
  enabled: boolean;
  title: string;
  body: string;
}

interface UseDashboardConnectReturn {
  stats: DashboardStatsData;
  announcement: AnnouncementData;
  isLoading: boolean;
  isError: boolean;
}

/**
 * Hook for fetching dashboard data via ConnectRPC.
 * Uses gRPC calls to Go backend for stats and announcements.
 */
export function useDashboardConnect(): UseDashboardConnectReturn {
  const { user } = useAuth();

  // Query for dashboard stats using ConnectRPC Query
  const {
    data: statsResponse,
    isLoading: statsLoading,
    isError: statsError,
  } = useQuery(
    getDashboardStats,
    {},
    {
      enabled: !!user,
      staleTime: 30_000,
    }
  );

  // Query for announcement using ConnectRPC Query
  const {
    data: announcementResponse,
    isLoading: announcementLoading,
    isError: announcementError,
  } = useQuery(
    getAnnouncement,
    {},
    {
      enabled: !!user,
      staleTime: 60_000,
    }
  );

  // Transform stats response
  const stats: DashboardStatsData = useMemo(() => {
    if (!statsResponse?.stats) {
      return {
        totalGroups: 0,
        fullGroups: 0,
        totalMembers: 0,
        activeLinks: 0,
      };
    }

    return {
      totalGroups: statsResponse.stats.totalGroups ?? 0,
      fullGroups: statsResponse.stats.fullGroups ?? 0,
      totalMembers: statsResponse.stats.totalMembers ?? 0,
      activeLinks: statsResponse.stats.activeLinks ?? 0,
    };
  }, [statsResponse]);

  // Transform announcement response
  const announcement: AnnouncementData = useMemo(() => {
    if (!announcementResponse?.announcement) {
      return {
        enabled: false,
        title: '',
        body: '',
      };
    }

    const { enabled, title, body } = announcementResponse.announcement;
    return {
      enabled: enabled && Boolean(title) && Boolean(body),
      title: title ?? '',
      body: body ?? '',
    };
  }, [announcementResponse]);

  return {
    stats,
    announcement,
    isLoading: statsLoading || announcementLoading,
    isError: statsError || announcementError,
  };
}

export type HistoryPeriod = 'weekly' | 'monthly' | 'yearly';

export interface HistoryDataPoint {
  label: string;
  members: number;
  groups: number;
  newMembers: number;
  newGroups: number;
}

interface UseDashboardHistoryReturn {
  data: HistoryDataPoint[];
  isLoading: boolean;
  isError: boolean;
}

/**
 * Hook for fetching dashboard historical data via ConnectRPC.
 * Supports weekly, monthly, and yearly periods.
 */
export function useDashboardHistory(period: HistoryPeriod): UseDashboardHistoryReturn {
  const { user } = useAuth();

  // Convert period string to proto enum
  const periodToProto = (p: HistoryPeriod): Period => {
    switch (p) {
      case 'weekly':
        return Period.WEEKLY;
      case 'monthly':
        return Period.MONTHLY;
      case 'yearly':
        return Period.YEARLY;
      default:
        return Period.WEEKLY;
    }
  };

  const {
    data: historyResponse,
    isLoading,
    isError,
  } = useQuery(
    getDashboardHistory,
    { period: periodToProto(period) },
    {
      enabled: !!user,
      staleTime: 60_000, // 1 minute
    }
  );

  // Transform response to HistoryDataPoint array
  const data: HistoryDataPoint[] = useMemo(() => {
    if (!historyResponse?.dataPoints) {
      return [];
    }

    return historyResponse.dataPoints.map((dp) => ({
      label: dp.label,
      members: dp.members,
      groups: dp.groups,
      newMembers: dp.newMembers,
      newGroups: dp.newGroups,
    }));
  }, [historyResponse]);

  return {
    data,
    isLoading,
    isError,
  };
}
