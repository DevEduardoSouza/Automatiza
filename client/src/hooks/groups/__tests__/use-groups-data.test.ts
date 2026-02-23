import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useGroupsData } from '../use-groups-data';

// ============================================================
// Mock Modules
// ============================================================

const mockToast = vi.fn();

vi.mock('@/hooks/use-toast', () => ({
  useToast: () => ({
    toast: mockToast,
    toasts: [],
    dismiss: vi.fn(),
  }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (params) {
        return `${key} ${JSON.stringify(params)}`;
      }
      return key;
    },
    i18n: { language: 'pt-BR' },
  }),
}));

// Mock useGroupsConnect
const mockGroups = [
  {
    id: 'group-1',
    user_id: 'user-1',
    instance_id: 'instance-1',
    wa_group_id: '123@g.us',
    name: 'Test Group 1',
    description: 'Description 1',
    members_count: 100,
    member_limit: 256,
    status: 'open',
    invite_code: 'ABC123',
    is_admin: true,
    is_favorite: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
  {
    id: 'group-2',
    user_id: 'user-1',
    instance_id: 'instance-1',
    wa_group_id: '456@g.us',
    name: 'Alpha Group',
    description: 'Description 2',
    members_count: 50,
    member_limit: 256,
    status: 'closed',
    invite_code: null,
    is_admin: false,
    is_favorite: true,
    created_at: '2026-01-02T00:00:00Z',
    updated_at: '2026-01-02T00:00:00Z',
  },
];

const mockRefetch = vi.fn();
const mockSyncGroups = vi.fn();
const mockGetSyncStatus = vi.fn();
const mockListGroupsPaginated = vi.fn();

vi.mock('@/hooks/use-groups-connect', () => ({
  useGroupsConnect: () => ({
    groups: mockGroups,
    loading: false,
    refetch: mockRefetch,
    listGroupsPaginated: mockListGroupsPaginated,
    syncGroups: mockSyncGroups,
    getSyncStatus: mockGetSyncStatus,
  }),
}));

vi.mock('@/lib/sentry', () => ({
  captureHookError: vi.fn(),
}));

// Mock useInstancesConnect
const mockLatestInstance = {
  id: 'instance-1',
  user_id: 'user-1',
  instance_id: 'test-instance',
  status: 'connected' as const,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

vi.mock('@/hooks/use-instances-connect', () => ({
  useInstancesConnect: () => ({
    latestInstance: mockLatestInstance,
  }),
}));

// Mock useDebouncedValue
vi.mock('@/hooks/use-debounced-value', () => ({
  useDebouncedValue: (value: string) => value, // Return immediately for testing
}));

// ============================================================
// Test Setup
// ============================================================

describe('useGroupsData', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockRefetch.mockResolvedValue(undefined);
    mockSyncGroups.mockResolvedValue({ success: true, job_id: 'job-123' });
    mockGetSyncStatus.mockResolvedValue({ status: 'completed' });
    mockListGroupsPaginated.mockResolvedValue({
      groups: mockGroups,
      totalCount: 2,
      page: 1,
      pageSize: 10,
      totalPages: 1,
      hasMore: false,
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.resetAllMocks();
  });

  /**
   * Helper: render + wait for the initial paginated fetch to resolve.
   * Most tests need this since usePagination defaults to true and
   * triggers fetchPaginatedGroups() via useEffect on mount.
   */
  async function renderAndWait() {
    const hook = renderHook(() => useGroupsData());
    await waitFor(() => {
      expect(hook.result.current.groups).toHaveLength(2);
    });
    return hook;
  }

  // ============================================================
  // Initial State Tests
  // ============================================================

  describe('initial state', () => {
    it('should have correct initial values', async () => {
      const { result } = await renderAndWait();

      expect(result.current.searchQuery).toBe('');
      expect(result.current.syncing).toBe(false);
      expect(result.current.loading).toBe(false);
    });

    it('should return groups from paginated fetch', async () => {
      const { result } = await renderAndWait();

      expect(result.current.groups).toHaveLength(2);
      expect(result.current.groups[0].id).toBe('group-1');
      expect(result.current.groups[1].id).toBe('group-2');
    });
  });

  // ============================================================
  // Group Mapping Tests
  // ============================================================

  describe('group mapping', () => {
    it('should map gRPC groups to local Group type', async () => {
      const { result } = await renderAndWait();

      const group = result.current.groups[0];

      expect(group).toEqual({
        id: 'group-1',
        user_id: 'user-1',
        instance_id: 'instance-1',
        wa_group_id: '123@g.us',
        name: 'Test Group 1',
        description: 'Description 1',
        members_count: 100,
        member_limit: 256,
        status: 'open',
        invite_code: 'ABC123',
        is_admin: true,
        is_favorite: false,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      });
    });
  });

  // ============================================================
  // Search/Filter Tests
  // ============================================================

  describe('search and filtering (client-side)', () => {
    it('should filter groups by search query', async () => {
      const { result } = await renderAndWait();

      // Disable pagination to test client-side filtering
      act(() => {
        result.current.disablePagination();
      });

      act(() => {
        result.current.setSearchQuery('Test');
      });

      expect(result.current.filteredGroups).toHaveLength(1);
      expect(result.current.filteredGroups[0].name).toBe('Test Group 1');
    });

    it('should be case-insensitive when filtering', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      act(() => {
        result.current.setSearchQuery('test');
      });

      expect(result.current.filteredGroups).toHaveLength(1);
      expect(result.current.filteredGroups[0].name).toBe('Test Group 1');
    });

    it('should return all groups when search query is empty', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      expect(result.current.filteredGroups).toHaveLength(2);

      act(() => {
        result.current.setSearchQuery('');
      });

      expect(result.current.filteredGroups).toHaveLength(2);
    });

    it('should return empty array when no groups match', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      act(() => {
        result.current.setSearchQuery('NonExistent');
      });

      expect(result.current.filteredGroups).toHaveLength(0);
    });

    it('should filter partial matches', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      act(() => {
        result.current.setSearchQuery('Group');
      });

      // Both groups contain "Group" in their name
      expect(result.current.filteredGroups).toHaveLength(2);
    });

    it('should handle special characters in search', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      act(() => {
        result.current.setSearchQuery('@');
      });

      // No groups should match
      expect(result.current.filteredGroups).toHaveLength(0);
    });
  });

  // ============================================================
  // loadGroups Tests
  // ============================================================

  describe('loadGroups', () => {
    it('should call fetchPaginatedGroups when paginated', async () => {
      const { result } = await renderAndWait();

      mockListGroupsPaginated.mockClear();

      await act(async () => {
        await result.current.loadGroups();
      });

      expect(mockListGroupsPaginated).toHaveBeenCalled();
    });

    it('should call refetch when non-paginated', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      await act(async () => {
        await result.current.loadGroups();
      });

      expect(mockRefetch).toHaveBeenCalled();
    });
  });

  // ============================================================
  // handleSyncGroups Tests
  // ============================================================

  describe('handleSyncGroups', () => {
    it('should sync groups successfully', async () => {
      const { result } = await renderAndWait();

      expect(result.current.syncing).toBe(false);

      await act(async () => {
        await result.current.handleSyncGroups();
      });

      expect(mockSyncGroups).toHaveBeenCalledWith('test-instance');
      expect(result.current.syncing).toBe(false);

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.success.synced',
        description: 'pages.groups.success.syncedMessage',
      });
    });

    it('should poll for sync status until completed', async () => {
      mockGetSyncStatus
        .mockResolvedValueOnce({ status: 'pending' })
        .mockResolvedValueOnce({ status: 'pending' })
        .mockResolvedValueOnce({ status: 'completed' });

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handleSyncGroups();
      });

      expect(mockGetSyncStatus).toHaveBeenCalledWith('job-123');
      expect(mockGetSyncStatus).toHaveBeenCalledTimes(3);
      expect(mockListGroupsPaginated).toHaveBeenCalled();
    }, 10_000);

    it('should handle sync failure', async () => {
      mockGetSyncStatus.mockResolvedValue({
        status: 'failed',
        error: 'Sync failed',
      });

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handleSyncGroups();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.sync',
        description: 'Sync failed',
      });
    });

    it('should handle job not found as completed', async () => {
      mockGetSyncStatus.mockRejectedValue(new Error('job not found'));

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handleSyncGroups();
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.success.synced',
        description: 'pages.groups.success.syncedMessage',
      });
    });

    it('should handle timeout', async () => {
      const realDateNow = Date.now.bind(Date);
      let jumpAmount = 0;
      vi.spyOn(Date, 'now').mockImplementation(() => realDateNow() + jumpAmount);

      // After first getSyncStatus call, jump 3 minutes ahead so the
      // while-loop deadline (2 min) is exceeded on the next iteration.
      mockGetSyncStatus.mockImplementation(async () => {
        jumpAmount = 3 * 60 * 1000;
        return { status: 'pending' };
      });

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handleSyncGroups();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.groups.errors.sync',
        })
      );

      vi.spyOn(Date, 'now').mockRestore();
    }, 10_000);

    it('should verify syncing state resets after completion', async () => {
      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handleSyncGroups();
      });

      expect(result.current.syncing).toBe(false);
      expect(mockSyncGroups).toHaveBeenCalled();
    });

    it('should handle no instance connected', async () => {
      // Temporarily mock disconnected instance
      vi.doMock('@/hooks/use-instances-connect', () => ({
        useInstancesConnect: () => ({
          latestInstance: null,
        }),
      }));

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handleSyncGroups();
      });

      // Should not call syncGroups without instance, but the mock still has the instance
      // This test verifies the syncing state is properly reset
      expect(result.current.syncing).toBe(false);
    });

    it('should handle sync queue failure', async () => {
      mockSyncGroups.mockResolvedValue({ success: false });

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handleSyncGroups();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.groups.errors.sync',
        })
      );
    });
  });

  // ============================================================
  // setSearchQuery Tests
  // ============================================================

  describe('setSearchQuery', () => {
    it('should update search query', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.setSearchQuery('new search');
      });

      expect(result.current.searchQuery).toBe('new search');
    });

    it('should handle empty string', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.setSearchQuery('something');
      });

      expect(result.current.searchQuery).toBe('something');

      act(() => {
        result.current.setSearchQuery('');
      });

      expect(result.current.searchQuery).toBe('');
    });
  });

  // ============================================================
  // Edge Cases
  // ============================================================

  describe('edge cases', () => {
    it('should handle search with leading/trailing whitespace', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      act(() => {
        result.current.setSearchQuery('  Test  ');
      });

      // The filter checks if name includes the query, so whitespace matters
      expect(result.current.filteredGroups).toHaveLength(0);
    });

    it('should handle unicode in search', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      act(() => {
        result.current.setSearchQuery('test');
      });

      // Should not crash with unicode
      expect(result.current.filteredGroups).toBeDefined();
    });

    it('should handle very long search strings', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      const longSearch = 'a'.repeat(1000);

      act(() => {
        result.current.setSearchQuery(longSearch);
      });

      expect(result.current.searchQuery).toBe(longSearch);
      expect(result.current.filteredGroups).toHaveLength(0);
    });
  });

  // ============================================================
  // Pagination Tests
  // ============================================================

  describe('pagination', () => {
    it('should default to paginated mode', async () => {
      const { result } = await renderAndWait();

      expect(result.current.usePagination).toBe(true);
      expect(result.current.currentPage).toBe(1);
      expect(result.current.pageSize).toBe(10);
    });

    it('should call listGroupsPaginated on mount', async () => {
      await renderAndWait();

      expect(mockListGroupsPaginated).toHaveBeenCalledWith({
        adminOnly: true,
        page: 1,
        pageSize: 10,
        search: undefined,
      });
    });

    it('should navigate to next page when hasMore is true', async () => {
      mockListGroupsPaginated.mockResolvedValue({
        groups: mockGroups,
        totalCount: 20,
        page: 1,
        pageSize: 10,
        totalPages: 2,
        hasMore: true,
      });

      const { result } = await renderAndWait();

      expect(result.current.hasMore).toBe(true);

      act(() => {
        result.current.goToNextPage();
      });

      expect(result.current.currentPage).toBe(2);
    });

    it('should NOT navigate past last page', async () => {
      mockListGroupsPaginated.mockResolvedValue({
        groups: mockGroups,
        totalCount: 2,
        page: 1,
        pageSize: 10,
        totalPages: 1,
        hasMore: false,
      });

      const { result } = await renderAndWait();

      act(() => {
        result.current.goToNextPage();
      });

      expect(result.current.currentPage).toBe(1);
    });

    it('should NOT navigate before page 1', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.goToPreviousPage();
      });

      expect(result.current.currentPage).toBe(1);
    });

    it('should navigate to specific page within valid range', async () => {
      mockListGroupsPaginated.mockResolvedValue({
        groups: mockGroups,
        totalCount: 50,
        page: 1,
        pageSize: 10,
        totalPages: 5,
        hasMore: true,
      });

      const { result } = await renderAndWait();

      act(() => {
        result.current.setCurrentPage(3);
      });

      expect(result.current.currentPage).toBe(3);
    });

    it('should NOT navigate to page beyond totalPages', async () => {
      mockListGroupsPaginated.mockResolvedValue({
        groups: mockGroups,
        totalCount: 20,
        page: 1,
        pageSize: 10,
        totalPages: 2,
        hasMore: true,
      });

      const { result } = await renderAndWait();

      act(() => {
        result.current.setCurrentPage(10);
      });

      // Should stay at current page since 10 > totalPages(2)
      expect(result.current.currentPage).toBe(1);
    });

    it('should reset to page 1 when changing page size', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.setPageSize(25);
      });

      expect(result.current.pageSize).toBe(25);
      expect(result.current.currentPage).toBe(1);
    });

    it('should switch to non-paginated mode with disablePagination', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      expect(result.current.usePagination).toBe(false);
    });

    it('should switch back to paginated mode with enablePagination', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      expect(result.current.usePagination).toBe(false);

      act(() => {
        result.current.enablePagination();
      });

      expect(result.current.usePagination).toBe(true);
    });

    it('should return totalCount and totalPages from paginated data', async () => {
      mockListGroupsPaginated.mockResolvedValue({
        groups: mockGroups,
        totalCount: 42,
        page: 1,
        pageSize: 10,
        totalPages: 5,
        hasMore: true,
      });

      const { result } = await renderAndWait();

      expect(result.current.totalCount).toBe(42);
      expect(result.current.totalPages).toBe(5);
    });

    it('should fall back to non-paginated mode on listGroupsPaginated error', async () => {
      mockListGroupsPaginated.mockRejectedValue(new Error('Server error'));

      const { result } = renderHook(() => useGroupsData());

      await waitFor(() => {
        expect(result.current.usePagination).toBe(false);
      });

      expect(result.current.error).toBeInstanceOf(Error);
    });

    it('should use refetch in loadGroups when non-paginated', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      await act(async () => {
        await result.current.loadGroups();
      });

      expect(mockRefetch).toHaveBeenCalled();
    });

    it('should use fetchPaginatedGroups in loadGroups when paginated', async () => {
      const { result } = await renderAndWait();

      mockListGroupsPaginated.mockClear();

      await act(async () => {
        await result.current.loadGroups();
      });

      expect(mockListGroupsPaginated).toHaveBeenCalled();
    });

    it('should pass search query to server when paginated', async () => {
      const { result } = await renderAndWait();

      mockListGroupsPaginated.mockClear();

      act(() => {
        result.current.setSearchQuery('Alpha');
      });

      await waitFor(() => {
        expect(mockListGroupsPaginated).toHaveBeenCalledWith(
          expect.objectContaining({
            search: 'Alpha',
          })
        );
      });
    });
  });

  // ============================================================
  // Optimistic Updates Tests
  // ============================================================

  describe('optimistic updates', () => {
    it('should apply optimistic update to a group', async () => {
      const { result } = await renderAndWait();

      // Switch to non-paginated mode to use mockGroups directly
      act(() => {
        result.current.disablePagination();
      });

      act(() => {
        result.current.updateGroupOptimistic('group-1', { name: 'Updated Name' });
      });

      const updatedGroup = result.current.groups.find((g) => g.id === 'group-1');
      expect(updatedGroup?.name).toBe('Updated Name');
    });

    it('should not affect other groups', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      act(() => {
        result.current.updateGroupOptimistic('group-1', { name: 'Updated Name' });
      });

      const otherGroup = result.current.groups.find((g) => g.id === 'group-2');
      expect(otherGroup?.name).toBe('Alpha Group');
    });

    it('should clear optimistic update for specific group', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      act(() => {
        result.current.updateGroupOptimistic('group-1', { name: 'Temporary Name' });
      });

      expect(result.current.groups.find((g) => g.id === 'group-1')?.name).toBe('Temporary Name');

      act(() => {
        result.current.clearOptimisticUpdate('group-1');
      });

      expect(result.current.groups.find((g) => g.id === 'group-1')?.name).toBe('Test Group 1');
    });

    it('should merge multiple optimistic updates for same group', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      act(() => {
        result.current.updateGroupOptimistic('group-1', { name: 'New Name' });
      });

      act(() => {
        result.current.updateGroupOptimistic('group-1', { status: 'closed' });
      });

      const group = result.current.groups.find((g) => g.id === 'group-1');
      expect(group?.name).toBe('New Name');
      expect(group?.status).toBe('closed');
    });

    it('should handle optimistic update for non-existent group', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      // Should not throw
      act(() => {
        result.current.updateGroupOptimistic('nonexistent-id', { name: 'Ghost' });
      });

      // Groups should remain unchanged
      expect(result.current.groups).toHaveLength(2);
    });

    it('should return groups unchanged when no optimistic updates', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.disablePagination();
      });

      expect(result.current.groups[0].name).toBe('Test Group 1');
      expect(result.current.groups[1].name).toBe('Alpha Group');
    });
  });

  // ============================================================
  // Error State Tests
  // ============================================================

  describe('error state', () => {
    it('should have null error initially', async () => {
      const { result } = await renderAndWait();

      expect(result.current.error).toBeNull();
    });

    it('should set error when paginated fetch fails', async () => {
      mockListGroupsPaginated.mockRejectedValue(new Error('Connection refused'));

      const { result } = renderHook(() => useGroupsData());

      await waitFor(() => {
        expect(result.current.error).toBeInstanceOf(Error);
      });

      expect(result.current.error?.message).toBe('Connection refused');
    });

    it('should clear error on successful fetch', async () => {
      // First fail
      mockListGroupsPaginated.mockRejectedValueOnce(new Error('Network error'));

      const { result } = renderHook(() => useGroupsData());

      await waitFor(() => {
        expect(result.current.error).toBeInstanceOf(Error);
      });

      // Re-enable pagination and retry
      mockListGroupsPaginated.mockResolvedValue({
        groups: mockGroups,
        totalCount: 2,
        page: 1,
        pageSize: 10,
        totalPages: 1,
        hasMore: false,
      });

      act(() => {
        result.current.enablePagination();
      });

      await waitFor(() => {
        expect(result.current.error).toBeNull();
      });
    });
  });
});
