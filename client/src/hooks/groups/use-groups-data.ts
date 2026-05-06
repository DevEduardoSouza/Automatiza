import { useDebouncedValue } from '@/hooks/use-debounced-value';
import { type PaginatedGroupsResult, useGroupsConnect } from '@/hooks/use-groups-connect';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import type { Group } from '@/types/group';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { UseGroupsDataReturn } from './types';

const DEFAULT_PAGE_SIZE = 10;

/**
 * Hook for managing groups data: loading, filtering, syncing with server-side pagination.
 * Uses ConnectRPC GroupsService for all operations.
 */
export function useGroupsData(): UseGroupsDataReturn {
  const { t } = useTranslation();
  const { toast } = useToast();

  // Use ConnectRPC hooks for data and instances
  const {
    groups: grpcGroups,
    loading: initialLoading,
    refetch,
    listGroupsPaginated,
    syncGroups: syncGroupsRpc,
    getSyncStatus,
  } = useGroupsConnect();
  const { instances } = useInstancesConnect();

  const [searchQuery, setSearchQuery] = useState('');
  const [syncing, setSyncing] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  // Pagination state
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
  const [paginatedData, setPaginatedData] = useState<PaginatedGroupsResult | null>(null);
  const [isPaginatedLoading, setIsPaginatedLoading] = useState(false);
  const [usePagination, setUsePagination] = useState(true);

  // Optimistic updates - stores partial group data keyed by group ID
  const [optimisticUpdates, setOptimisticUpdates] = useState<Map<string, Partial<Group>>>(
    new Map()
  );

  const debouncedSearchQuery = useDebouncedValue(searchQuery, 300);

  // Fetch paginated data when pagination params or search changes
  const fetchPaginatedGroups = useCallback(async () => {
    setIsPaginatedLoading(true);
    setError(null);
    try {
      const result = await listGroupsPaginated({
        adminOnly: true,
        page: currentPage,
        pageSize,
        search: debouncedSearchQuery || undefined,
      });
      setPaginatedData(result);
    } catch (err) {
      captureHookError(err, 'useGroupsData', 'loadPaginatedGroups');
      setError(err instanceof Error ? err : new Error(t('pages.groups.errors.loadFailed')));
      // On error, fall back to non-paginated mode
      setUsePagination(false);
    } finally {
      setIsPaginatedLoading(false);
    }
  }, [listGroupsPaginated, currentPage, pageSize, debouncedSearchQuery, t]);

  // Reset to page 1 when search changes
  // biome-ignore lint/correctness/useExhaustiveDependencies: debouncedSearchQuery triggers reset intentionally
  useEffect(() => {
    if (usePagination) {
      setCurrentPage(1);
    }
  }, [debouncedSearchQuery, usePagination]);

  // Fetch paginated data when pagination is enabled
  useEffect(() => {
    if (usePagination) {
      fetchPaginatedGroups();
    }
  }, [usePagination, fetchPaginatedGroups]);

  // Map gRPC groups to local Group type (for non-paginated mode)
  const nonPaginatedGroups: Group[] = useMemo(
    () =>
      grpcGroups.map((g) => ({
        id: g.id,
        user_id: g.user_id,
        instance_id: g.instance_id,
        wa_group_id: g.wa_group_id,
        name: g.name,
        description: g.description,
        members_count: g.members_count,
        member_limit: g.member_limit,
        status: g.status,
        invite_code: g.invite_code,
        is_admin: g.is_admin,
        is_favorite: g.is_favorite,
        created_at: g.created_at,
        updated_at: g.updated_at,
      })),
    [grpcGroups]
  );

  // Map paginated groups to local Group type
  const paginatedGroups: Group[] = useMemo(
    () =>
      paginatedData?.groups.map((g) => ({
        id: g.id,
        user_id: g.user_id,
        instance_id: g.instance_id,
        wa_group_id: g.wa_group_id,
        name: g.name,
        description: g.description,
        members_count: g.members_count,
        member_limit: g.member_limit,
        status: g.status,
        invite_code: g.invite_code,
        is_admin: g.is_admin,
        is_favorite: g.is_favorite,
        created_at: g.created_at,
        updated_at: g.updated_at,
      })) ?? [],
    [paginatedData]
  );

  // Apply optimistic updates to groups
  const applyOptimisticUpdates = useCallback(
    (groupList: Group[]): Group[] => {
      if (optimisticUpdates.size === 0) return groupList;
      return groupList.map((group) => {
        const update = optimisticUpdates.get(group.id);
        if (update) {
          return { ...group, ...update };
        }
        return group;
      });
    },
    [optimisticUpdates]
  );

  // Use paginated or non-paginated groups based on mode, with optimistic updates applied
  const groups = useMemo(() => {
    const baseGroups = usePagination ? paginatedGroups : nonPaginatedGroups;
    return applyOptimisticUpdates(baseGroups);
  }, [usePagination, paginatedGroups, nonPaginatedGroups, applyOptimisticUpdates]);

  // Client-side filtering for non-paginated mode
  const filteredGroups = useMemo(() => {
    if (usePagination) {
      // Server-side filtering, return as-is
      return groups;
    }
    // Client-side filtering
    return groups.filter((group) =>
      group.name.toLowerCase().includes(debouncedSearchQuery.toLowerCase())
    );
  }, [usePagination, groups, debouncedSearchQuery]);

  // Function to update a group optimistically (instant UI update)
  const updateGroupOptimistic = useCallback((groupId: string, updates: Partial<Group>) => {
    setOptimisticUpdates((prev) => {
      const newMap = new Map(prev);
      const existing = newMap.get(groupId) || {};
      newMap.set(groupId, { ...existing, ...updates });
      return newMap;
    });
  }, []);

  // Clear optimistic updates (call after server confirms or on error)
  const clearOptimisticUpdate = useCallback((groupId: string) => {
    setOptimisticUpdates((prev) => {
      const newMap = new Map(prev);
      newMap.delete(groupId);
      return newMap;
    });
  }, []);

  const loading = usePagination ? isPaginatedLoading : initialLoading;

  const loadGroups = useCallback(async () => {
    if (usePagination) {
      await fetchPaginatedGroups();
    } else {
      await refetch();
    }
  }, [usePagination, fetchPaginatedGroups, refetch]);

  // Enable pagination mode
  const enablePagination = useCallback(() => {
    setUsePagination(true);
  }, []);

  // Disable pagination mode (go back to client-side filtering)
  const disablePagination = useCallback(() => {
    setUsePagination(false);
    setPaginatedData(null);
  }, []);

  // Page navigation functions
  const goToNextPage = useCallback(() => {
    if (paginatedData?.hasMore) {
      setCurrentPage((prev) => prev + 1);
    }
  }, [paginatedData?.hasMore]);

  const goToPreviousPage = useCallback(() => {
    if (currentPage > 1) {
      setCurrentPage((prev) => prev - 1);
    }
  }, [currentPage]);

  const goToPage = useCallback(
    (page: number) => {
      const maxPage = paginatedData?.totalPages ?? 1;
      if (page >= 1 && page <= maxPage) {
        setCurrentPage(page);
      }
    },
    [paginatedData?.totalPages]
  );

  const changePageSize = useCallback((newSize: number) => {
    setPageSize(newSize);
    setCurrentPage(1); // Reset to first page when changing page size
  }, []);

  const handleSyncGroups = useCallback(async () => {
    if (syncing) return;

    setSyncing(true);

    try {
      // Sync from EVERY connected chip, not just the primary. Each chip
      // owns a different set of groups (each WhatsApp account has its own
      // membership) so syncing only one would leave the other chips' groups
      // stale or absent. Jobs are queued and polled in parallel so the wait
      // time is bounded by the slowest single chip, not the sum.
      const connectedInstances = instances.filter((i) => i.status === 'connected');
      if (connectedInstances.length === 0) {
        throw new Error(t('pages.groups.errors.noInstance'));
      }

      type Job = { instanceName: string; jobId: string };
      const queueResults = await Promise.allSettled(
        connectedInstances.map((inst) => syncGroupsRpc(inst.instance_id))
      );

      const jobs: Job[] = [];
      const queueErrors: string[] = [];
      queueResults.forEach((result, idx) => {
        const inst = connectedInstances[idx];
        if (result.status === 'fulfilled' && result.value?.success && result.value.job_id) {
          jobs.push({ instanceName: inst.instance_id, jobId: String(result.value.job_id) });
        } else {
          queueErrors.push(inst.friendly_name?.trim() || inst.instance_id);
        }
      });

      if (jobs.length === 0) {
        throw new Error(t('pages.groups.errors.syncQueue'));
      }

      // Poll all jobs concurrently. Done when the pending set is empty or
      // the per-batch deadline elapses.
      const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));
      const deadline = Date.now() + 2 * 60 * 1000;
      const pending = new Set(jobs.map((j) => j.jobId));
      const failures: string[] = [];

      while (pending.size > 0 && Date.now() < deadline) {
        const checks = await Promise.allSettled(
          [...pending].map(async (jobId) => {
            try {
              const status = await getSyncStatus(jobId);
              return { jobId, state: status?.status, error: status?.error };
            } catch (e: unknown) {
              const msg = e instanceof Error ? e.message : '';
              // "job not found" usually means the job already completed and
              // got cleaned up — treat as success rather than throwing.
              if (
                msg.toLowerCase().includes('job not found') ||
                msg.toLowerCase().includes('not_found')
              ) {
                return { jobId, state: 'completed' as const, error: undefined };
              }
              throw e;
            }
          })
        );

        for (const c of checks) {
          if (c.status !== 'fulfilled') continue;
          const { jobId, state, error } = c.value;
          if (state === 'completed') {
            pending.delete(jobId);
          } else if (state === 'failed') {
            const job = jobs.find((j) => j.jobId === jobId);
            const inst = job
              ? connectedInstances.find((i) => i.instance_id === job.instanceName)
              : undefined;
            const label = inst?.friendly_name?.trim() || job?.instanceName || 'chip';
            failures.push(error ? `${label}: ${error}` : label);
            pending.delete(jobId);
          }
        }

        if (pending.size > 0) await sleep(1000);
      }

      await loadGroups();

      const totalProblems = pending.size + failures.length + queueErrors.length;
      if (totalProblems > 0 && totalProblems === connectedInstances.length) {
        // Every chip failed — surface as a hard error.
        throw new Error(
          pending.size > 0
            ? t('pages.groups.errors.syncTimeout')
            : t('pages.groups.errors.syncFailed')
        );
      }

      if (totalProblems > 0) {
        toast({
          variant: 'destructive',
          title: t('pages.groups.errors.syncPartial', {
            failed: totalProblems,
            total: connectedInstances.length,
            defaultValue: 'Sincronizado parcialmente: {{failed}} de {{total}} números falharam',
          }),
        });
      } else {
        toast({
          title: t('pages.groups.success.synced'),
          description:
            connectedInstances.length === 1
              ? t('pages.groups.success.syncedMessage')
              : t('pages.groups.success.syncedAllInstances', {
                  count: connectedInstances.length,
                  defaultValue: '{{count}} números sincronizados',
                }),
        });
      }
    } catch (error: unknown) {
      captureHookError(error, 'useGroupsData', 'handleSync');
      const errorMessage = error instanceof Error ? error.message : t('pages.groups.errors.sync');
      toast({
        variant: 'destructive',
        title: t('pages.groups.errors.sync'),
        description: errorMessage,
      });
    } finally {
      setSyncing(false);
    }
  }, [syncing, toast, t, loadGroups, syncGroupsRpc, getSyncStatus, instances]);

  return {
    groups,
    loading,
    error,
    searchQuery,
    filteredGroups,
    syncing,
    setSearchQuery,
    loadGroups,
    handleSyncGroups,
    // Pagination state
    usePagination,
    currentPage,
    pageSize,
    totalCount: paginatedData?.totalCount ?? nonPaginatedGroups.length,
    totalPages: paginatedData?.totalPages ?? 1,
    hasMore: paginatedData?.hasMore ?? false,
    // Pagination actions
    enablePagination,
    disablePagination,
    setCurrentPage: goToPage,
    setPageSize: changePageSize,
    goToNextPage,
    goToPreviousPage,
    // Optimistic updates
    updateGroupOptimistic,
    clearOptimisticUpdate,
  };
}
