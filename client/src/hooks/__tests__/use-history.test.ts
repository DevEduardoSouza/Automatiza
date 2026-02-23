import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules (inline values to avoid TDZ issues with vi.mock hoisting)
// ============================================================

vi.mock('@/lib/sentry', () => ({
  captureHookError: vi.fn(),
  safeLocalStorage: {
    getItem: vi.fn().mockReturnValue(null),
    setItem: vi.fn(),
    removeItem: vi.fn(),
  },
}));

const mockToast = vi.fn();
vi.mock('@/hooks/use-toast', () => ({
  useToast: () => ({
    toast: mockToast,
    toasts: [],
    dismiss: vi.fn(),
  }),
}));

const mockT = (key: string) => key;
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: mockT,
    i18n: { language: 'pt-BR' },
  }),
}));

const mockInvalidateQueries = vi.fn();
vi.mock('@tanstack/react-query', () => ({
  useQueryClient: () => ({
    invalidateQueries: mockInvalidateQueries,
  }),
}));

// ============================================================
// Mock ConnectRPC history hooks
// ============================================================

const mockRefetch = vi.fn();
const mockClearHistoryFn = vi.fn();

const mockConnectEvents = [
  {
    id: 'evt-1',
    user_id: 'user-123',
    event_type: 'close_groups',
    description: 'Closed 5 groups',
    metadata: {},
    created_at: '2026-01-15T10:00:00Z',
    status: 'completed',
    error_message: 'some error',
    completed_at: '2026-01-15T10:01:00Z',
    target_count: 5,
    success_count: 4,
    error_count: 1,
  },
  {
    id: 'evt-2',
    user_id: 'user-123',
    event_type: 'error',
    description: 'Something failed',
    metadata: {},
    created_at: '2026-01-15T11:00:00Z',
    status: 'failed',
    error_message: undefined,
    completed_at: undefined,
    target_count: undefined,
    success_count: undefined,
    error_count: undefined,
  },
];

const mockConnectStats = {
  total: 50,
  completed: 40,
  running: 5,
  failed: 5,
};

let mockConnectReturn: Record<string, unknown>;

vi.mock('@/hooks/use-history-connect', () => ({
  useHistoryConnect: () => mockConnectReturn,
}));

vi.mock('@/hooks/use-history-realtime', () => ({
  useHistoryRealtime: vi.fn(),
}));

// ============================================================
// Import after mocks
// ============================================================

import { captureHookError } from '@/lib/sentry';
import { useHistory } from '../use-history';

// ============================================================
// Test Helpers
// ============================================================

function renderAndGet() {
  const { result } = renderHook(() => useHistory());
  return result;
}

// ============================================================
// Tests
// ============================================================

describe('useHistory', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockInvalidateQueries.mockResolvedValue(undefined);
    mockClearHistoryFn.mockResolvedValue(15);

    mockConnectReturn = {
      events: mockConnectEvents,
      hasMore: true,
      loading: false,
      stats: mockConnectStats,
      clearHistory: mockClearHistoryFn,
      clearingHistory: false,
      refetch: mockRefetch,
      totalCount: 50,
      page: 1,
      pageSize: 20,
      totalPages: 3,
    };
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── Data mapping ───────────────────────────────────────────

  describe('data mapping', () => {
    it('should map connect events to ActionHistory format', () => {
      const result = renderAndGet();

      expect(result.current.history).toHaveLength(2);
      expect(result.current.history[0]).toEqual({
        id: 'evt-1',
        action_type: 'close_groups',
        description: 'Closed 5 groups',
        status: 'completed',
        target_count: 5,
        success_count: 4,
        error_count: 1,
        error_message: 'some error',
        created_at: '2026-01-15T10:00:00Z',
        completed_at: '2026-01-15T10:01:00Z',
      });
    });

    it('should default status to completed when missing', () => {
      mockConnectReturn = {
        ...mockConnectReturn,
        events: [
          {
            id: 'evt-3',
            user_id: 'user-123',
            event_type: 'message_sent',
            description: 'Test',
            metadata: {},
            created_at: '2026-01-15T10:00:00Z',
            status: '',
          },
        ],
      };

      const result = renderAndGet();

      expect(result.current.history[0].status).toBe('completed');
    });

    it('should default numeric fields to 0 when undefined', () => {
      const result = renderAndGet();

      // evt-2 has undefined target_count, success_count, error_count
      expect(result.current.history[1].target_count).toBe(0);
      expect(result.current.history[1].success_count).toBe(0);
      expect(result.current.history[1].error_count).toBe(0);
    });

    it('should default error_message to null when undefined', () => {
      const result = renderAndGet();

      expect(result.current.history[1].error_message).toBeNull();
    });

    it('should default completed_at to null when undefined', () => {
      const result = renderAndGet();

      expect(result.current.history[1].completed_at).toBeNull();
    });

    it('should pass loading from connect hook', () => {
      mockConnectReturn = { ...mockConnectReturn, loading: true };

      const result = renderAndGet();

      expect(result.current.loading).toBe(true);
    });

    it('should pass stats from connect hook', () => {
      const result = renderAndGet();

      expect(result.current.stats).toEqual({
        total: 50,
        completed: 40,
        running: 5,
        failed: 5,
      });
    });
  });

  // ── Filters ───────────────────────────────────────────────

  describe('filters', () => {
    it('should have initial filters with null eventType and null status', () => {
      const result = renderAndGet();

      expect(result.current.filters).toEqual({
        eventType: null,
        status: null,
      });
    });

    it('should update filters with setFilters', () => {
      const result = renderAndGet();

      act(() => {
        result.current.setFilters({ eventType: 'error', status: 'failed' });
      });

      expect(result.current.filters).toEqual({
        eventType: 'error',
        status: 'failed',
      });
    });

    it('should reset currentPage to 1 when setFilters is called', () => {
      const result = renderAndGet();

      // Move to page 3 first
      act(() => {
        result.current.setCurrentPage(3);
      });
      expect(result.current.currentPage).toBe(3);

      // Setting filters should reset to page 1
      act(() => {
        result.current.setFilters({ eventType: 'error', status: null });
      });
      expect(result.current.currentPage).toBe(1);
    });

    it('should reset filters with clearFilters', () => {
      const result = renderAndGet();

      act(() => {
        result.current.setFilters({ eventType: 'error', status: 'failed' });
      });

      act(() => {
        result.current.clearFilters();
      });

      expect(result.current.filters).toEqual({
        eventType: null,
        status: null,
      });
    });

    it('should reset currentPage to 1 when clearFilters is called', () => {
      const result = renderAndGet();

      act(() => {
        result.current.setCurrentPage(5);
      });

      act(() => {
        result.current.clearFilters();
      });

      expect(result.current.currentPage).toBe(1);
    });
  });

  // ── Pagination ────────────────────────────────────────────

  describe('pagination', () => {
    it('should have initial page=1 and pageSize=20', () => {
      const result = renderAndGet();

      expect(result.current.currentPage).toBe(1);
      expect(result.current.pageSize).toBe(20);
    });

    it('should update currentPage with setCurrentPage', () => {
      const result = renderAndGet();

      act(() => {
        result.current.setCurrentPage(3);
      });

      expect(result.current.currentPage).toBe(3);
    });

    it('should update pageSize with setPageSize', () => {
      const result = renderAndGet();

      act(() => {
        result.current.setPageSize(50);
      });

      expect(result.current.pageSize).toBe(50);
    });

    it('should return totalCount from connect hook', () => {
      const result = renderAndGet();

      expect(result.current.totalCount).toBe(50);
    });

    it('should return totalPages from connect hook', () => {
      const result = renderAndGet();

      expect(result.current.totalPages).toBe(3);
    });

    it('should return hasMore from connect hook', () => {
      const result = renderAndGet();

      expect(result.current.hasMore).toBe(true);
    });
  });

  // ── getActionTypeLabel ────────────────────────────────────

  describe('getActionTypeLabel', () => {
    it('should return i18n key for close_groups', () => {
      const result = renderAndGet();

      expect(result.current.getActionTypeLabel('close_groups')).toBe(
        'pages.history.actions.closeGroups'
      );
    });

    it('should return i18n key for open_groups', () => {
      const result = renderAndGet();

      expect(result.current.getActionTypeLabel('open_groups')).toBe(
        'pages.history.actions.openGroups'
      );
    });

    it('should return i18n key for send_message', () => {
      const result = renderAndGet();

      expect(result.current.getActionTypeLabel('send_message')).toBe(
        'pages.history.actions.sendMessage'
      );
    });

    it('should return i18n key for message_sent (alias for sendMessage)', () => {
      const result = renderAndGet();

      expect(result.current.getActionTypeLabel('message_sent')).toBe(
        'pages.history.actions.sendMessage'
      );
    });

    it('should return i18n key for automation_run', () => {
      const result = renderAndGet();

      expect(result.current.getActionTypeLabel('automation_run')).toBe(
        'pages.history.actions.automationRun'
      );
    });

    it('should return i18n key for error', () => {
      const result = renderAndGet();

      expect(result.current.getActionTypeLabel('error')).toBe(
        'pages.history.actions.error'
      );
    });

    it('should return the type itself when not found in labels map', () => {
      const result = renderAndGet();

      expect(result.current.getActionTypeLabel('unknown_action')).toBe('unknown_action');
    });
  });

  // ── loadHistory ───────────────────────────────────────────

  describe('loadHistory', () => {
    it('should invalidate connect-query cache', async () => {
      const result = renderAndGet();

      await act(async () => {
        await result.current.loadHistory();
      });

      expect(mockInvalidateQueries).toHaveBeenCalledWith({
        queryKey: ['connectRpc', 'history.v1.HistoryService'],
      });
    });

    it('should show error toast on failure', async () => {
      mockInvalidateQueries.mockRejectedValueOnce(new Error('Network error'));

      const result = renderAndGet();

      await act(async () => {
        await result.current.loadHistory();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.history.errors.load',
        })
      );
    });

    it('should capture error in Sentry on failure', async () => {
      const error = new Error('Network error');
      mockInvalidateQueries.mockRejectedValueOnce(error);

      const result = renderAndGet();

      await act(async () => {
        await result.current.loadHistory();
      });

      expect(captureHookError).toHaveBeenCalledWith(error, 'useHistory', 'loadHistory');
    });
  });

  // ── clearHistory ──────────────────────────────────────────

  describe('clearHistory', () => {
    it('should call clearHistoryFn from connect hook', async () => {
      const result = renderAndGet();

      await act(async () => {
        await result.current.clearHistory();
      });

      expect(mockClearHistoryFn).toHaveBeenCalled();
    });

    it('should show success toast with deleted count', async () => {
      const result = renderAndGet();

      await act(async () => {
        await result.current.clearHistory();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.history.clearSuccess',
        })
      );
    });

    it('should return deletedCount on success', async () => {
      const result = renderAndGet();

      let deletedCount = 0;
      await act(async () => {
        deletedCount = await result.current.clearHistory();
      });

      expect(deletedCount).toBe(15);
    });

    it('should show error toast on failure', async () => {
      mockClearHistoryFn.mockRejectedValueOnce(new Error('Clear failed'));

      const result = renderAndGet();

      await act(async () => {
        await result.current.clearHistory();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.history.errors.clear',
        })
      );
    });

    it('should return 0 on error', async () => {
      mockClearHistoryFn.mockRejectedValueOnce(new Error('Clear failed'));

      const result = renderAndGet();

      let deletedCount = -1;
      await act(async () => {
        deletedCount = await result.current.clearHistory();
      });

      expect(deletedCount).toBe(0);
    });

    it('should capture error in Sentry on failure', async () => {
      const error = new Error('Clear failed');
      mockClearHistoryFn.mockRejectedValueOnce(error);

      const result = renderAndGet();

      await act(async () => {
        await result.current.clearHistory();
      });

      expect(captureHookError).toHaveBeenCalledWith(error, 'useHistory', 'clearHistory');
    });

    it('should pass clearingHistory from connect hook', () => {
      mockConnectReturn = { ...mockConnectReturn, clearingHistory: true };

      const result = renderAndGet();

      expect(result.current.clearingHistory).toBe(true);
    });
  });
});
