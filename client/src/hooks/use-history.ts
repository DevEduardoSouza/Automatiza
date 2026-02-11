import { useHistoryConnect } from '@/hooks/use-history-connect';
import { useHistoryRealtime } from '@/hooks/use-history-realtime';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import type { ActionHistory, ActionHistoryStatus, HistoryStats } from '@/types/action-history';
import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

export interface HistoryFilters {
  eventType: string | null;
  status: ActionHistoryStatus | null;
}

interface UseHistoryReturn {
  history: ActionHistory[];
  loading: boolean;
  stats: HistoryStats;
  loadHistory: () => Promise<void>;
  getActionTypeLabel: (actionType: string) => string;
  filters: HistoryFilters;
  setFilters: (filters: HistoryFilters) => void;
  clearFilters: () => void;
  clearHistory: () => Promise<number>;
  clearingHistory: boolean;
  // Pagination
  currentPage: number;
  pageSize: number;
  totalCount: number;
  totalPages: number;
  hasMore: boolean;
  setCurrentPage: (page: number) => void;
  setPageSize: (size: number) => void;
}

const DEFAULT_FILTERS: HistoryFilters = {
  eventType: null,
  status: null,
};

const DEFAULT_PAGE_SIZE = 20;

/**
 * Hook for history page logic.
 * Uses ConnectRPC useHistoryConnect for data fetching.
 * Stats are calculated server-side for consistency.
 */
export function useHistory(): UseHistoryReturn {
  const { t } = useTranslation();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  // Filter state
  const [filters, setFiltersState] = useState<HistoryFilters>(DEFAULT_FILTERS);

  // Pagination state
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);

  // Build connect options from filters
  const connectOptions = useMemo(
    () => ({
      page: currentPage,
      pageSize,
      eventType: filters.eventType as
        | 'send_message'
        | 'message_sent'
        | 'close_groups'
        | 'open_groups'
        | 'change_name'
        | 'change_photo'
        | 'change_description'
        | 'import_groups'
        | 'automation_run'
        | 'coupon_used'
        | 'link_clicked'
        | 'group_sync'
        | 'error'
        | 'send_campaign'
        | 'generate_image'
        | 'add_member'
        | 'remove_member'
        | 'promote_member'
        | 'demote_member'
        | 'leave_group'
        | undefined,
      status: filters.status ?? undefined,
    }),
    [filters, currentPage, pageSize]
  );

  // Use ConnectRPC hook for history data - stats come from server
  const {
    events,
    loading,
    stats: serverStats,
    clearHistory: clearHistoryFn,
    clearingHistory,
    refetch,
    totalCount,
    totalPages,
    hasMore,
  } = useHistoryConnect(connectOptions);

  // Auto-refetch when history changes via SSE
  useHistoryRealtime(refetch);

  // Map history events to ActionHistory format
  const history: ActionHistory[] = useMemo(() => {
    return events.map((event) => ({
      id: event.id,
      action_type: event.event_type,
      description: event.description,
      status: (event.status as ActionHistory['status']) || 'completed',
      target_count: event.target_count ?? 0,
      success_count: event.success_count ?? 0,
      error_count: event.error_count ?? 0,
      error_message: event.error_message ?? null,
      created_at: event.created_at,
      completed_at: event.completed_at ?? null,
    }));
  }, [events]);

  const loadHistory = useCallback(async () => {
    try {
      // Invalidate the connect-query cache to trigger refetch
      await queryClient.invalidateQueries({
        queryKey: ['connectRpc', 'history.v1.HistoryService'],
      });
    } catch (error: unknown) {
      captureHookError(error, 'useHistory', 'loadHistory');
      const errorMessage = error instanceof Error ? error.message : t('common.error');
      toast({
        variant: 'destructive',
        title: t('pages.history.errors.load'),
        description: errorMessage,
      });
    }
  }, [queryClient, t, toast]);

  // Use stats from server (calculated in Go handler)
  const stats: HistoryStats = serverStats;

  const getActionTypeLabel = useCallback(
    (actionType: string): string => {
      const labels: Record<string, string> = {
        close_groups: t('pages.history.actions.closeGroups'),
        open_groups: t('pages.history.actions.openGroups'),
        send_message: t('pages.history.actions.sendMessage'),
        change_name: t('pages.history.actions.changeName'),
        change_photo: t('pages.history.actions.changePhoto'),
        change_description: t('pages.history.actions.changeDescription'),
        import_groups: t('pages.history.actions.importGroups'),
        message_sent: t('pages.history.actions.sendMessage'),
        automation_run: t('pages.history.actions.automationRun'),
        coupon_used: t('pages.history.actions.couponUsed'),
        link_clicked: t('pages.history.actions.linkClicked'),
        group_sync: t('pages.history.actions.importGroups'),
        error: t('pages.history.actions.error'),
        send_campaign: t('pages.history.actions.sendCampaign'),
        generate_image: t('pages.history.actions.generateImage'),
        add_member: t('pages.history.actions.addMember'),
        remove_member: t('pages.history.actions.removeMember'),
        promote_member: t('pages.history.actions.promoteMember'),
        demote_member: t('pages.history.actions.demoteMember'),
        leave_group: t('pages.history.actions.leaveGroup'),
      };
      return labels[actionType] || actionType;
    },
    [t]
  );

  // Reset to page 1 when filters change
  const setFilters = useCallback((newFilters: HistoryFilters) => {
    setFiltersState(newFilters);
    setCurrentPage(1);
  }, []);

  const clearFilters = useCallback(() => {
    setFiltersState(DEFAULT_FILTERS);
    setCurrentPage(1);
  }, []);

  const clearHistory = useCallback(async (): Promise<number> => {
    try {
      const deletedCount = await clearHistoryFn();
      toast({
        title: t('pages.history.clearSuccess'),
        description: t('pages.history.clearSuccessDescription', { count: deletedCount }),
      });
      return deletedCount;
    } catch (error: unknown) {
      captureHookError(error, 'useHistory', 'clearHistory');
      const errorMessage = error instanceof Error ? error.message : t('common.error');
      toast({
        variant: 'destructive',
        title: t('pages.history.errors.clear'),
        description: errorMessage,
      });
      return 0;
    }
  }, [clearHistoryFn, t, toast]);

  return {
    history,
    loading,
    stats,
    loadHistory,
    getActionTypeLabel,
    filters,
    setFilters,
    clearFilters,
    clearHistory,
    clearingHistory,
    // Pagination
    currentPage,
    pageSize,
    totalCount,
    totalPages,
    hasMore,
    setCurrentPage,
    setPageSize,
  };
}
