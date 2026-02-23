import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules (BEFORE imports)
// ============================================================

vi.mock('@/gen/automations/v1/automations-AutomationsService_connectquery', () => ({
  listDispatchLogs: {},
}));

/**
 * Mutable mock data controlling what useDispatchLogsConnect returns.
 * Tests mutate these fields before rendering to simulate different server states.
 */
const mockQueryData = {
  logs: [] as Array<{
    id: string;
    automationId: string;
    groupId: string;
    groupName: string;
    canonicalKey: string;
    offerTitle: string;
    status: string;
  }>,
  totalCount: 0,
  hasMore: false,
};

vi.mock('@connectrpc/connect-query', () => ({
  useQuery: vi.fn(() => ({
    data: mockQueryData,
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
  })),
}));

// ============================================================
// Import after mocks
// ============================================================

import { useQuery } from '@connectrpc/connect-query';
import { useDispatchLogsPaginated } from '../use-dispatch-logs-connect';

// ============================================================
// Tests: useDispatchLogsPaginated
// ============================================================

describe('useDispatchLogsPaginated', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockQueryData.logs = [];
    mockQueryData.totalCount = 0;
    mockQueryData.hasMore = false;
  });

  // ── Initial state ───────────────────────────────────────────

  describe('initial state', () => {
    it('should start at page 0', () => {
      const { result } = renderHook(() => useDispatchLogsPaginated('auto-123'));

      expect(result.current.page).toBe(0);
    });

    it('should default pageSize to 20', () => {
      const { result } = renderHook(() => useDispatchLogsPaginated('auto-123'));

      expect(result.current.pageSize).toBe(20);
    });

    it('should calculate totalPages correctly (45 items / 20 pageSize = 3 pages)', () => {
      mockQueryData.totalCount = 45;

      const { result } = renderHook(() => useDispatchLogsPaginated('auto-123', 20));

      expect(result.current.totalPages).toBe(3);
    });

    it('should disable query when automationId is empty string', () => {
      renderHook(() => useDispatchLogsPaginated(''));

      // useDispatchLogsConnect converts empty automationId to undefined (automationId || undefined)
      // and sets enabled: false via !!automationId
      expect(useQuery).toHaveBeenCalledWith(
        expect.anything(),
        expect.objectContaining({ automationId: undefined }),
        expect.objectContaining({ enabled: false })
      );
    });
  });

  // ── nextPage ────────────────────────────────────────────────

  describe('nextPage', () => {
    it('should increment page when hasMore is true', () => {
      mockQueryData.hasMore = true;
      mockQueryData.totalCount = 60;

      const { result } = renderHook(() => useDispatchLogsPaginated('auto-123', 20));

      expect(result.current.page).toBe(0);

      act(() => {
        result.current.nextPage();
      });

      expect(result.current.page).toBe(1);
    });

    it('should not increment page when hasMore is false', () => {
      mockQueryData.hasMore = false;
      mockQueryData.totalCount = 10;

      const { result } = renderHook(() => useDispatchLogsPaginated('auto-123', 20));

      act(() => {
        result.current.nextPage();
      });

      expect(result.current.page).toBe(0);
    });

    it('should pass correct offset to underlying query (page * pageSize)', () => {
      mockQueryData.hasMore = true;
      mockQueryData.totalCount = 100;

      const { result } = renderHook(() => useDispatchLogsPaginated('auto-123', 20));

      act(() => {
        result.current.nextPage();
      });

      // After nextPage, page=1, offset should be 1*20=20
      const lastCall = vi.mocked(useQuery).mock.calls.at(-1);
      expect(lastCall?.[1]).toEqual(
        expect.objectContaining({ offset: 20, limit: 20 })
      );
    });
  });

  // ── prevPage ────────────────────────────────────────────────

  describe('prevPage', () => {
    it('should decrement page when page > 0', () => {
      mockQueryData.hasMore = true;
      mockQueryData.totalCount = 100;

      const { result } = renderHook(() => useDispatchLogsPaginated('auto-123', 20));

      // Go to page 1 first
      act(() => {
        result.current.nextPage();
      });
      expect(result.current.page).toBe(1);

      // Go back to page 0
      act(() => {
        result.current.prevPage();
      });
      expect(result.current.page).toBe(0);
    });

    it('should not decrement page when already at page 0', () => {
      const { result } = renderHook(() => useDispatchLogsPaginated('auto-123', 20));

      expect(result.current.page).toBe(0);

      act(() => {
        result.current.prevPage();
      });

      expect(result.current.page).toBe(0);
    });
  });

  // ── goToPage ────────────────────────────────────────────────

  describe('goToPage', () => {
    it('should set a valid page number', () => {
      mockQueryData.totalCount = 100;

      const { result } = renderHook(() => useDispatchLogsPaginated('auto-123', 20));

      // totalPages = ceil(100/20) = 5, so page 3 is valid (0-indexed: 0,1,2,3,4)
      act(() => {
        result.current.goToPage(3);
      });

      expect(result.current.page).toBe(3);
    });

    it('should reject negative page numbers', () => {
      mockQueryData.totalCount = 100;

      const { result } = renderHook(() => useDispatchLogsPaginated('auto-123', 20));

      act(() => {
        result.current.goToPage(-1);
      });

      expect(result.current.page).toBe(0);
    });

    it('should reject page number >= totalPages', () => {
      mockQueryData.totalCount = 100;

      const { result } = renderHook(() => useDispatchLogsPaginated('auto-123', 20));

      // totalPages = 5, so page 5 is out of bounds
      act(() => {
        result.current.goToPage(5);
      });

      expect(result.current.page).toBe(0);
    });
  });

  // ── totalPages calculation ──────────────────────────────────

  describe('totalPages', () => {
    it('should return 0 when totalCount is 0', () => {
      mockQueryData.totalCount = 0;

      const { result } = renderHook(() => useDispatchLogsPaginated('auto-123', 20));

      expect(result.current.totalPages).toBe(0);
    });

    it('should apply ceil rounding (21/20 = 2, 40/20 = 2, 41/20 = 3)', () => {
      // 21 items / 20 per page = 2 pages (ceil)
      mockQueryData.totalCount = 21;
      const { result: r1 } = renderHook(() => useDispatchLogsPaginated('auto-123', 20));
      expect(r1.current.totalPages).toBe(2);

      // 40 items / 20 per page = 2 pages (exact)
      mockQueryData.totalCount = 40;
      const { result: r2 } = renderHook(() => useDispatchLogsPaginated('auto-123', 20));
      expect(r2.current.totalPages).toBe(2);

      // 41 items / 20 per page = 3 pages (ceil)
      mockQueryData.totalCount = 41;
      const { result: r3 } = renderHook(() => useDispatchLogsPaginated('auto-123', 20));
      expect(r3.current.totalPages).toBe(3);
    });
  });
});
