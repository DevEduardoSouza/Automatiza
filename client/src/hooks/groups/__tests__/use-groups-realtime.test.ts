import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useGroupsRealtime } from '../use-groups-realtime';

// ============================================================
// Mock Modules
// ============================================================

const mockRefetch = vi.fn();

vi.mock('@/hooks/use-groups-connect', () => ({
  useGroupsConnect: () => ({
    refetch: mockRefetch,
  }),
}));

// ============================================================
// Helpers
// ============================================================

function dispatchGroupEvent(type: string, extraDetail: Record<string, unknown> = {}) {
  window.dispatchEvent(
    new CustomEvent('sse:group-event', {
      detail: { type, ...extraDetail },
    })
  );
}

// ============================================================
// Test Setup
// ============================================================

describe('useGroupsRealtime', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.resetAllMocks();
  });

  // ============================================================
  // Event Listener Registration
  // ============================================================

  describe('event listener registration', () => {
    it('should add event listener for sse:group-event on mount', () => {
      const addSpy = vi.spyOn(window, 'addEventListener');

      renderHook(() => useGroupsRealtime());

      expect(addSpy).toHaveBeenCalledWith('sse:group-event', expect.any(Function));

      addSpy.mockRestore();
    });

    it('should remove event listener on unmount', () => {
      const removeSpy = vi.spyOn(window, 'removeEventListener');

      const { unmount } = renderHook(() => useGroupsRealtime());
      unmount();

      expect(removeSpy).toHaveBeenCalledWith('sse:group-event', expect.any(Function));

      removeSpy.mockRestore();
    });
  });

  // ============================================================
  // SSE Event Handling - Valid Events
  // ============================================================

  describe('SSE event handling - valid events', () => {
    it('should call refetch after group.created event + 500ms', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('group.created');
      });

      expect(mockRefetch).not.toHaveBeenCalled();

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });

    it('should call refetch after group.updated event + 500ms', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('group.updated');
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });

    it('should call refetch after group.deleted event + 500ms', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('group.deleted');
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });

    it('should call refetch after group.member.changed event + 500ms', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('group.member.changed');
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });

    it('should call refetch after group.sync event + 500ms', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('group.sync');
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });
  });

  // ============================================================
  // SSE Event Handling - Invalid Events
  // ============================================================

  describe('SSE event handling - invalid events', () => {
    it('should NOT trigger refetch for unknown event type', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('message.received');
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).not.toHaveBeenCalled();
    });

    it('should NOT trigger refetch when event detail has no type', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        window.dispatchEvent(
          new CustomEvent('sse:group-event', {
            detail: {},
          })
        );
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).not.toHaveBeenCalled();
    });

    it('should NOT trigger refetch when event detail is null', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        window.dispatchEvent(
          new CustomEvent('sse:group-event', {
            detail: null,
          })
        );
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).not.toHaveBeenCalled();
    });

    it('should NOT trigger refetch when event detail is undefined', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        window.dispatchEvent(
          new CustomEvent('sse:group-event', {
            detail: undefined,
          })
        );
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).not.toHaveBeenCalled();
    });

    it('should NOT trigger refetch for unrelated window events', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        window.dispatchEvent(
          new CustomEvent('sse:other-event', {
            detail: { type: 'group.created' },
          })
        );
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).not.toHaveBeenCalled();
    });
  });

  // ============================================================
  // Debouncing Tests
  // ============================================================

  describe('debouncing', () => {
    it('should call refetch exactly 1 time when 5 events arrive in 100ms', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('group.created');
        vi.advanceTimersByTime(20);
        dispatchGroupEvent('group.updated');
        vi.advanceTimersByTime(20);
        dispatchGroupEvent('group.deleted');
        vi.advanceTimersByTime(20);
        dispatchGroupEvent('group.member.changed');
        vi.advanceTimersByTime(20);
        dispatchGroupEvent('group.sync');
      });

      expect(mockRefetch).not.toHaveBeenCalled();

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });

    it('should NOT call refetch before debounce period expires', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('group.created');
      });

      act(() => {
        vi.advanceTimersByTime(499);
      });

      expect(mockRefetch).not.toHaveBeenCalled();

      act(() => {
        vi.advanceTimersByTime(1);
      });

      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });

    it('should reset timer: event, wait 300ms, new event, wait 500ms -> refetch 1 time', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('group.created');
      });

      act(() => {
        vi.advanceTimersByTime(300);
      });

      expect(mockRefetch).not.toHaveBeenCalled();

      act(() => {
        dispatchGroupEvent('group.updated');
      });

      act(() => {
        vi.advanceTimersByTime(499);
      });

      expect(mockRefetch).not.toHaveBeenCalled();

      act(() => {
        vi.advanceTimersByTime(1);
      });

      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });

    it('should allow separate refetches for events spaced far apart', () => {
      renderHook(() => useGroupsRealtime());

      // First event
      act(() => {
        dispatchGroupEvent('group.created');
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).toHaveBeenCalledTimes(1);

      // Second event after debounce period
      act(() => {
        dispatchGroupEvent('group.updated');
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).toHaveBeenCalledTimes(2);
    });

    it('should handle very high frequency events (100 events)', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        for (let i = 0; i < 100; i++) {
          dispatchGroupEvent('group.updated');
        }
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });

    it('should handle mixed valid and invalid events', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('invalid.event');
        dispatchGroupEvent('group.created');
        dispatchGroupEvent('another.invalid');
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      // Only the valid event triggers refetch
      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });
  });

  // ============================================================
  // Cleanup Tests
  // ============================================================

  describe('cleanup', () => {
    it('should NOT call refetch when unmount happens before timeout', () => {
      const { unmount } = renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('group.created');
      });

      unmount();

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).not.toHaveBeenCalled();
    });

    it('should have no timer leaks on quick mount/unmount', () => {
      const { unmount: unmount1 } = renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('group.created');
      });

      unmount1();

      const { unmount: unmount2 } = renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('group.updated');
      });

      unmount2();

      act(() => {
        vi.advanceTimersByTime(1000);
      });

      expect(mockRefetch).not.toHaveBeenCalled();
    });

    it('should not respond to events dispatched after unmount', () => {
      const { unmount } = renderHook(() => useGroupsRealtime());

      unmount();

      act(() => {
        dispatchGroupEvent('group.created');
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).not.toHaveBeenCalled();
    });

    it('should handle multiple mount/unmount cycles correctly', () => {
      // First mount
      const { unmount: unmount1 } = renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('group.created');
      });

      unmount1();

      // Second mount
      const { unmount: unmount2 } = renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('group.updated');
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      // Only the second instance's event should trigger refetch
      expect(mockRefetch).toHaveBeenCalledTimes(1);

      unmount2();
    });
  });

  // ============================================================
  // Edge Cases
  // ============================================================

  describe('edge cases', () => {
    it('should handle event with extra detail fields', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('group.created', { groupId: '123', extra: 'data' });
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });

    it('should return void (no return value)', () => {
      const { result } = renderHook(() => useGroupsRealtime());

      expect(result.current).toBeUndefined();
    });

    it('should handle exactly 500ms debounce delay (boundary)', () => {
      renderHook(() => useGroupsRealtime());

      act(() => {
        dispatchGroupEvent('group.created');
      });

      act(() => {
        vi.advanceTimersByTime(499);
      });

      expect(mockRefetch).not.toHaveBeenCalled();

      act(() => {
        vi.advanceTimersByTime(1);
      });

      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });
  });
});
