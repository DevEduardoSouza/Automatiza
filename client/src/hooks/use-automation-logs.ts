import { useAuth } from '@/contexts';
import { useAutomationsConnect } from '@/hooks/use-automations-connect';
import { useDebouncedValue } from '@/hooks/use-debounced-value';
import { type LocalDispatchLog, useDispatchLogsConnect } from '@/hooks/use-dispatch-logs-connect';
import { useGroupsConnect } from '@/hooks/use-groups-connect';
import { useJobsConnect } from '@/hooks/use-jobs-connect';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import type {
  DispatchLog,
  DispatchLogAutomation,
  DispatchLogGroup,
  DispatchStatus,
  QueueStatus,
  StoreOption,
} from '@/types/dispatch-log';
import { STORES, isInstanceMissingError } from '@/types/dispatch-log';
import { format } from 'date-fns';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

export type { DispatchLog, DispatchLogAutomation, DispatchLogGroup, DispatchStatus };

interface UseAutomationLogsReturn {
  logs: DispatchLog[];
  automations: DispatchLogAutomation[];
  groups: DispatchLogGroup[];
  loading: boolean;
  queueStatus: QueueStatus | null;
  filterAutomation: string;
  filterStore: string;
  filterStatus: string;
  filterGroup: string;
  searchTerm: string;
  filteredLogs: DispatchLog[];
  stores: StoreOption[];
  totalCount: number;
  hasMore: boolean;
  setFilterAutomation: (value: string) => void;
  setFilterStore: (value: string) => void;
  setFilterStatus: (value: string) => void;
  setFilterGroup: (value: string) => void;
  setSearchTerm: (value: string) => void;
  exportCSV: () => void;
  getProductHref: (log: DispatchLog) => string;
  getGroupName: (groupId: string, groupName?: string) => string;
  isInstanceMissingError: (msg?: string) => boolean;
  getStatusBadgeVariant: (
    status: DispatchStatus
  ) => 'default' | 'secondary' | 'destructive' | 'outline';
  refetchLogs: () => void;
}

/**
 * Converts a LocalDispatchLog from ConnectRPC to the legacy DispatchLog format.
 * This maintains backward compatibility with existing components.
 * Uses all fields from the proto response - no hardcoded values.
 */
export function fromLocalDispatchLog(log: LocalDispatchLog, automationName: string): DispatchLog {
  return {
    id: log.id,
    automation_id: log.automationId,
    automation_name: automationName,
    // Use store from proto, fallback to empty string if not set
    store: (log.store as DispatchLog['store']) || 'shopee',
    group_id: log.groupId,
    group_name: log.groupName,
    product_category: log.productCategory ?? null,
    // Use productUrl if available, fallback to offerUrl then canonicalKey
    product_url: log.productUrl ?? log.offerUrl ?? log.canonicalKey,
    product_page_url: log.productPageUrl ?? log.offerUrl ?? null,
    affiliate_url: log.affiliateLink ?? log.offerUrl,
    status: log.status === 'error' ? 'error' : log.status,
    error: log.errorMessage,
    coupon_id: log.couponId ?? null,
    coupon_code: log.couponCode ?? null,
    coupon_applied: log.couponApplied ?? null,
    coupon_skip_reason: log.couponSkipReason ?? null,
    created_at: log.createdAt.toISOString(),
  };
}

/**
 * Hook for automation logs page logic
 * Manages dispatch logs with filtering and export
 * Uses ConnectRPC hooks for data fetching
 */
export function useAutomationLogs(): UseAutomationLogsReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const { queueStatus: connectQueueStatus } = useJobsConnect();
  const { listAutomations, isListingAutomations } = useAutomationsConnect();
  const { groups: connectGroups, loading: groupsLoading } = useGroupsConnect();

  const [automations, setAutomations] = useState<DispatchLogAutomation[]>([]);
  const [filterAutomation, setFilterAutomation] = useState<string>('all');
  const [filterStore, setFilterStore] = useState<string>('all');
  const [filterStatus, setFilterStatus] = useState<string>('all');
  const [filterGroup, setFilterGroup] = useState<string>('all');
  const [searchTerm, setSearchTerm] = useState('');

  const debouncedSearchTerm = useDebouncedValue(searchTerm, 300);

  // Fetch dispatch logs via ConnectRPC with server-side filters
  // When filterAutomation is 'all', pass empty string to fetch logs from all automations
  const {
    logs: connectLogs,
    totalCount,
    hasMore,
    isLoading: isLoadingLogs,
    refetch: refetchLogs,
  } = useDispatchLogsConnect({
    automationId: filterAutomation !== 'all' ? filterAutomation : undefined,
    limit: 100,
    offset: 0,
    enabled: !!user,
    // Server-side filters
    groupId: filterGroup !== 'all' ? filterGroup : undefined,
    status: filterStatus !== 'all' ? filterStatus : undefined,
    store: filterStore !== 'all' ? filterStore : undefined,
    searchUrl: debouncedSearchTerm || undefined,
  });

  // Map ConnectRPC queue status to local format
  const queueStatus: QueueStatus | null = connectQueueStatus
    ? {
        queue: connectQueueStatus.queue,
        settings: connectQueueStatus.settings,
        estimatedMinutes: connectQueueStatus.estimatedMinutes,
      }
    : null;

  // Map groups from ConnectRPC
  const groups: DispatchLogGroup[] = useMemo(
    () =>
      connectGroups.map((g) => ({
        id: g.id,
        name: g.name,
        wa_group_id: g.wa_group_id,
      })),
    [connectGroups]
  );

  // Convert ConnectRPC logs to legacy format
  const logs: DispatchLog[] = useMemo(() => {
    const automationMap = new Map(automations.map((a) => [a.id, a.name]));
    return connectLogs.map((log) =>
      fromLocalDispatchLog(log, automationMap.get(log.automationId) ?? '')
    );
  }, [connectLogs, automations]);

  const loadAutomations = useCallback(async () => {
    if (!user) return;
    try {
      const automationsList = await listAutomations();
      setAutomations(
        automationsList
          .map((a) => ({
            id: a.id,
            name: a.name,
          }))
          .sort((a, b) => a.name.localeCompare(b.name))
      );
    } catch (error: unknown) {
      captureHookError(error, 'useAutomationLogs', 'loadAutomations');
      toast({
        title: t('pages.automationLogs.errors.load'),
        variant: 'destructive',
      });
    }
  }, [user, toast, t, listAutomations]);

  useEffect(() => {
    if (user) {
      loadAutomations();
    }
  }, [user, loadAutomations]);

  // Filters are now applied server-side via useDispatchLogsConnect
  // No need for client-side filtering - logs already filtered by the backend
  const filteredLogs = logs;

  /** FSQ7/FSQ34: Wrap exportCSV in try/catch + validate Date parsing */
  const exportCSV = useCallback(() => {
    try {
      const headers = [
        t('pages.automationLogs.table.dateTime'),
        t('pages.automationLogs.table.automation'),
        t('pages.automationLogs.table.store'),
        t('pages.automationLogs.table.group'),
        t('pages.automationLogs.table.category'),
        t('pages.automationLogs.table.product'),
        t('pages.automationLogs.table.affiliateLink'),
        t('pages.automationLogs.table.status'),
        t('pages.automationLogs.table.error'),
      ];

      const rows = filteredLogs.map((log) => {
        const date = new Date(log.created_at);
        const formattedDate = Number.isNaN(date.getTime())
          ? log.created_at
          : format(date, 'dd/MM/yyyy HH:mm:ss');
        return [
          formattedDate,
          log.automation_name,
          STORES.find((s) => s.value === log.store)?.label || log.store,
          log.group_name || log.group_id,
          log.product_category || '-',
          log.product_url,
          log.affiliate_url || '-',
          log.status,
          log.error || '-',
        ];
      });

      const csv = [headers, ...rows].map((row) => row.join(',')).join('\n');
      const blob = new Blob([csv], { type: 'text/csv' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `logs-${format(new Date(), 'yyyy-MM-dd')}.csv`;
      a.click();
      URL.revokeObjectURL(url);

      toast({ title: t('pages.automationLogs.export.success') });
    } catch (error: unknown) {
      captureHookError(error, 'useAutomationLogs', 'exportCSV');
      toast({
        title: t('pages.automationLogs.errors.export'),
        variant: 'destructive',
      });
    }
  }, [filteredLogs, toast, t]);

  const getProductHref = useCallback((log: DispatchLog): string => {
    return log.affiliate_url || log.product_page_url || log.product_url;
  }, []);

  const getGroupName = useCallback(
    (groupId: string, groupName?: string): string => {
      if (groupName) return groupName;
      const group = groups.find((g) => g.wa_group_id === groupId);
      return group?.name || groupId || t('pages.automationLogs.groupRemoved');
    },
    [groups, t]
  );

  const getStatusBadgeVariant = useCallback(
    (status: DispatchStatus): 'default' | 'secondary' | 'destructive' | 'outline' => {
      switch (status) {
        case 'sent':
          return 'default';
        case 'queued':
        case 'skipped':
          return 'secondary';
        case 'error':
          return 'destructive';
        default:
          return 'outline';
      }
    },
    []
  );

  return {
    logs,
    automations,
    groups,
    loading: isLoadingLogs || isListingAutomations || groupsLoading,
    queueStatus,
    filterAutomation,
    filterStore,
    filterStatus,
    filterGroup,
    searchTerm,
    filteredLogs,
    stores: STORES,
    totalCount,
    hasMore,
    setFilterAutomation,
    setFilterStore,
    setFilterStatus,
    setFilterGroup,
    setSearchTerm,
    exportCSV,
    getProductHref,
    getGroupName,
    isInstanceMissingError,
    getStatusBadgeVariant,
    refetchLogs,
  };
}
