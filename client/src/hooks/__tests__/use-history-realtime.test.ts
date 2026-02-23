import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useHistoryRealtime } from '../use-history-realtime';

// ============================================================
// Tests
// ============================================================

describe('useHistoryRealtime', () => {
  const mockRefetch = vi.fn();

  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  // ── Listener registration ─────────────────────────────────

  it('should register event listener for sse:history-event on mount', () => {
    const addSpy = vi.spyOn(window, 'addEventListener');

    renderHook(() => useHistoryRealtime(mockRefetch));

    expect(addSpy).toHaveBeenCalledWith('sse:history-event', expect.any(Function));
  });

  it('should remove event listener on unmount', () => {
    const removeSpy = vi.spyOn(window, 'removeEventListener');

    const { unmount } = renderHook(() => useHistoryRealtime(mockRefetch));

    unmount();

    expect(removeSpy).toHaveBeenCalledWith('sse:history-event', expect.any(Function));
  });

  // ── Event handling ────────────────────────────────────────

  it('should call refetchHistory after 500ms when event has type history.change', () => {
    renderHook(() => useHistoryRealtime(mockRefetch));

    act(() => {
      window.dispatchEvent(
        new CustomEvent('sse:history-event', {
          detail: { type: 'history.change' },
        })
      );
    });

    // Before timeout, refetch should not have been called
    expect(mockRefetch).not.toHaveBeenCalled();

    // Advance timers by 500ms
    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(mockRefetch).toHaveBeenCalledTimes(1);
  });

  it('should ignore events with a different type', () => {
    renderHook(() => useHistoryRealtime(mockRefetch));

    act(() => {
      window.dispatchEvent(
        new CustomEvent('sse:history-event', {
          detail: { type: 'session.status' },
        })
      );
    });

    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(mockRefetch).not.toHaveBeenCalled();
  });

  // ── Debounce ──────────────────────────────────────────────

  it('should debounce multiple rapid events into a single refetch', () => {
    renderHook(() => useHistoryRealtime(mockRefetch));

    // Fire 5 events in quick succession
    for (let i = 0; i < 5; i++) {
      act(() => {
        window.dispatchEvent(
          new CustomEvent('sse:history-event', {
            detail: { type: 'history.change' },
          })
        );
      });

      // Advance 100ms between each event (less than 500ms debounce)
      act(() => {
        vi.advanceTimersByTime(100);
      });
    }

    // At this point, 500ms have passed since the first event
    // but only 100ms since the last event. Refetch should not yet fire
    // because each event resets the 500ms timer.
    expect(mockRefetch).not.toHaveBeenCalled();

    // Advance remaining 400ms to trigger the debounced call
    act(() => {
      vi.advanceTimersByTime(400);
    });

    expect(mockRefetch).toHaveBeenCalledTimes(1);
  });

  // ── Cleanup ───────────────────────────────────────────────

  it('should clear pending timeout on unmount', () => {
    const { unmount } = renderHook(() => useHistoryRealtime(mockRefetch));

    // Dispatch event to start the debounce timer
    act(() => {
      window.dispatchEvent(
        new CustomEvent('sse:history-event', {
          detail: { type: 'history.change' },
        })
      );
    });

    // Unmount before the 500ms timeout fires
    unmount();

    // Advance past the debounce window
    act(() => {
      vi.advanceTimersByTime(1000);
    });

    // Refetch should NOT have been called because timeout was cleared
    expect(mockRefetch).not.toHaveBeenCalled();
  });
});
