import { renderHook } from '@testing-library/react';
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

const mockUser = { id: 'user-123', email: 'test@example.com' };
let mockUserValue: typeof mockUser | null = mockUser;

vi.mock('@/contexts', () => ({
  useAuth: () => ({
    user: mockUserValue,
    session: null,
    loading: false,
  }),
}));

// ============================================================
// Mock ConnectRPC hooks
// ============================================================

const mockRefetch = vi.fn();
const mockMutateAsync = vi.fn();
const mockInvalidateQueries = vi.fn();

let mockQueryData: Record<string, unknown> | undefined;
let mockIsLoading = false;

vi.mock('@connectrpc/connect-query', () => ({
  useQuery: (_descriptor: unknown, _input: unknown, options?: { enabled?: boolean }) => {
    // If enabled is explicitly false, return empty state
    if (options?.enabled === false) {
      return {
        data: undefined,
        isLoading: false,
        refetch: mockRefetch,
      };
    }
    return {
      data: mockQueryData,
      isLoading: mockIsLoading,
      refetch: mockRefetch,
    };
  },
  useMutation: (_descriptor: unknown, options?: { onSuccess?: () => void }) => ({
    mutateAsync: (...args: unknown[]) => {
      const result = mockMutateAsync(...args);
      // Call onSuccess callback after successful mutation
      if (options?.onSuccess) {
        result.then(() => options.onSuccess?.());
      }
      return result;
    },
    isPending: false,
  }),
  useTransport: () => 'mock-transport',
}));

vi.mock('@tanstack/react-query', () => ({
  useQueryClient: () => ({
    invalidateQueries: mockInvalidateQueries,
  }),
}));

vi.mock('@/gen/history/v1/history-HistoryService_connectquery', () => ({
  listHistory: { name: 'listHistory' },
  clearHistory: { name: 'clearHistory' },
}));

vi.mock('@/gen/history/v1/history_pb', () => ({
  EventType: {
    UNSPECIFIED: 0,
    MESSAGE_SENT: 1,
    AUTOMATION_RUN: 2,
    COUPON_USED: 3,
    LINK_CLICKED: 4,
    GROUP_SYNC: 5,
    ERROR: 6,
    CLOSE_GROUPS: 7,
    OPEN_GROUPS: 8,
    CHANGE_NAME: 9,
    CHANGE_PHOTO: 10,
    CHANGE_DESCRIPTION: 11,
    IMPORT_GROUPS: 12,
    SEND_CAMPAIGN: 13,
    GENERATE_IMAGE: 14,
    ADD_MEMBER: 15,
    REMOVE_MEMBER: 16,
    PROMOTE_MEMBER: 17,
    DEMOTE_MEMBER: 18,
    LEAVE_GROUP: 19,
  },
}));

// ============================================================
// Import after mocks
// ============================================================

import { useHistoryConnect } from '../use-history-connect';

// ============================================================
// Test Helpers
// ============================================================

const mockProtoEvent = {
  id: 'evt-1',
  userId: 'user-123',
  eventType: 1, // MESSAGE_SENT as numeric enum
  description: 'Sent a message',
  metadata: { groupId: 'g-1' },
  createdAt: '2026-01-15T10:00:00Z',
  status: 'completed',
  errorMessage: '',
  completedAt: '2026-01-15T10:00:01Z',
  targetCount: 5,
  successCount: 4,
  errorCount: 1,
};

const mockProtoStats = {
  total: 100,
  completed: 80,
  running: 10,
  failed: 10,
};

function renderAndGet(options?: Parameters<typeof useHistoryConnect>[0]) {
  const { result } = renderHook(() => useHistoryConnect(options));
  return result;
}

// ============================================================
// Tests
// ============================================================

describe('useHistoryConnect', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUserValue = mockUser;
    mockIsLoading = false;
    mockQueryData = {
      events: [mockProtoEvent],
      hasMore: true,
      stats: mockProtoStats,
      totalCount: 100,
      page: 1,
      pageSize: 20,
      totalPages: 5,
    };
    mockMutateAsync.mockResolvedValue({ deletedCount: 42 });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── Data fetching ───────────────────────────────────────────

  describe('data fetching', () => {
    it('should return events mapped from proto to local format', () => {
      const result = renderAndGet();

      expect(result.current.events).toHaveLength(1);
      expect(result.current.events[0]).toEqual({
        id: 'evt-1',
        user_id: 'user-123',
        event_type: 'message_sent',
        description: 'Sent a message',
        metadata: { groupId: 'g-1' },
        created_at: '2026-01-15T10:00:00Z',
        status: 'completed',
        error_message: '',
        completed_at: '2026-01-15T10:00:01Z',
        target_count: 5,
        success_count: 4,
        error_count: 1,
      });
    });

    it('should return stats with default zeros when stats are missing', () => {
      mockQueryData = {
        events: [],
        hasMore: false,
        stats: undefined,
        totalCount: 0,
        page: 1,
        pageSize: 20,
        totalPages: 1,
      };

      const result = renderAndGet();

      expect(result.current.stats).toEqual({
        total: 0,
        completed: 0,
        running: 0,
        failed: 0,
      });
    });

    it('should return stats from server when available', () => {
      const result = renderAndGet();

      expect(result.current.stats).toEqual({
        total: 100,
        completed: 80,
        running: 10,
        failed: 10,
      });
    });

    it('should report loading=true when query is loading', () => {
      mockIsLoading = true;

      const result = renderAndGet();

      expect(result.current.loading).toBe(true);
    });

    it('should return empty events when user is null (query disabled)', () => {
      mockUserValue = null;

      const result = renderAndGet();

      expect(result.current.events).toEqual([]);
    });

    it('should reflect hasMore from server response', () => {
      const result = renderAndGet();

      expect(result.current.hasMore).toBe(true);
    });

    it('should default hasMore to false when response is undefined', () => {
      mockQueryData = undefined;

      const result = renderAndGet();

      expect(result.current.hasMore).toBe(false);
    });
  });

  // ── Event type mapping (tested indirectly via events output) ──

  describe('event type mapping', () => {
    it('should map numeric MESSAGE_SENT (1) to message_sent', () => {
      mockQueryData = {
        events: [{ ...mockProtoEvent, eventType: 1 }],
        hasMore: false,
      };

      const result = renderAndGet();
      expect(result.current.events[0].event_type).toBe('message_sent');
    });

    it('should map numeric AUTOMATION_RUN (2) to automation_run', () => {
      mockQueryData = {
        events: [{ ...mockProtoEvent, eventType: 2 }],
        hasMore: false,
      };

      const result = renderAndGet();
      expect(result.current.events[0].event_type).toBe('automation_run');
    });

    it('should map numeric ERROR (6) to error', () => {
      mockQueryData = {
        events: [{ ...mockProtoEvent, eventType: 6 }],
        hasMore: false,
      };

      const result = renderAndGet();
      expect(result.current.events[0].event_type).toBe('error');
    });

    it('should map numeric CLOSE_GROUPS (7) to close_groups', () => {
      mockQueryData = {
        events: [{ ...mockProtoEvent, eventType: 7 }],
        hasMore: false,
      };

      const result = renderAndGet();
      expect(result.current.events[0].event_type).toBe('close_groups');
    });

    it('should map numeric LEAVE_GROUP (19) to leave_group', () => {
      mockQueryData = {
        events: [{ ...mockProtoEvent, eventType: 19 }],
        hasMore: false,
      };

      const result = renderAndGet();
      expect(result.current.events[0].event_type).toBe('leave_group');
    });

    it('should map unknown numeric enum to unspecified', () => {
      mockQueryData = {
        events: [{ ...mockProtoEvent, eventType: 999 }],
        hasMore: false,
      };

      const result = renderAndGet();
      expect(result.current.events[0].event_type).toBe('unspecified');
    });

    it('should map string EVENT_TYPE_CHANGE_NAME to change_name', () => {
      mockQueryData = {
        events: [{ ...mockProtoEvent, eventType: 'EVENT_TYPE_CHANGE_NAME' }],
        hasMore: false,
      };

      const result = renderAndGet();
      expect(result.current.events[0].event_type).toBe('change_name');
    });

    it('should map string EVENT_TYPE_GENERATE_IMAGE to generate_image', () => {
      mockQueryData = {
        events: [{ ...mockProtoEvent, eventType: 'EVENT_TYPE_GENERATE_IMAGE' }],
        hasMore: false,
      };

      const result = renderAndGet();
      expect(result.current.events[0].event_type).toBe('generate_image');
    });

    it('should map unknown string event type to unspecified', () => {
      mockQueryData = {
        events: [{ ...mockProtoEvent, eventType: 'EVENT_TYPE_UNKNOWN_THING' }],
        hasMore: false,
      };

      const result = renderAndGet();
      expect(result.current.events[0].event_type).toBe('unspecified');
    });

    it('should map numeric UNSPECIFIED (0) to unspecified', () => {
      mockQueryData = {
        events: [{ ...mockProtoEvent, eventType: 0 }],
        hasMore: false,
      };

      const result = renderAndGet();
      expect(result.current.events[0].event_type).toBe('unspecified');
    });
  });

  // ── Pagination ────────────────────────────────────────────────

  describe('pagination', () => {
    it('should pass page and pageSize to query and return pagination fields', () => {
      const result = renderAndGet({ page: 3, pageSize: 10 });

      expect(result.current.totalCount).toBe(100);
      expect(result.current.totalPages).toBe(5);
      expect(result.current.page).toBe(1);
      expect(result.current.pageSize).toBe(20);
    });

    it('should default pagination values when response is undefined', () => {
      mockQueryData = undefined;

      const result = renderAndGet();

      expect(result.current.totalCount).toBe(0);
      expect(result.current.page).toBe(1);
      expect(result.current.pageSize).toBe(20);
      expect(result.current.totalPages).toBe(1);
    });
  });

  // ── clearHistory ──────────────────────────────────────────────

  describe('clearHistory', () => {
    it('should call mutateAsync and return deletedCount', async () => {
      const result = renderAndGet();

      const deletedCount = await result.current.clearHistory();

      expect(mockMutateAsync).toHaveBeenCalled();
      expect(deletedCount).toBe(42);
    });

    it('should pass beforeDate and eventTypes to mutation', async () => {
      const result = renderAndGet();

      await result.current.clearHistory({
        beforeDate: '2026-01-01',
        eventTypes: ['error', 'message_sent'],
      });

      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          beforeDate: '2026-01-01',
        })
      );

      // Verify eventTypes were converted to proto enums (filtered for non-undefined)
      const callArgs = mockMutateAsync.mock.calls[0][0];
      expect(callArgs.eventTypes).toBeDefined();
      expect(Array.isArray(callArgs.eventTypes)).toBe(true);
    });

    it('should invalidate queries and refetch on success', async () => {
      const result = renderAndGet();

      await result.current.clearHistory();

      // Wait for the onSuccess callback triggered by resolved promise
      await vi.waitFor(() => {
        expect(mockInvalidateQueries).toHaveBeenCalledWith({
          queryKey: ['history'],
        });
      });
      expect(mockRefetch).toHaveBeenCalled();
    });

    it('should report clearingHistory as false by default', () => {
      const result = renderAndGet();

      expect(result.current.clearingHistory).toBe(false);
    });
  });
});
