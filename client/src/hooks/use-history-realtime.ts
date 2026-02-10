import { useCallback, useEffect, useRef } from 'react';

/**
 * Hook that listens for real-time history change events via SSE
 * and triggers a callback when history entries are created or updated.
 *
 * The SSE hook (useSessionStatusSSE) dispatches CustomEvents on window
 * when history.change events arrive. This hook listens for those events
 * and triggers a debounced refetch of the history list.
 */
export function useHistoryRealtime(refetchHistory: () => void): void {
  const refetchTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const debouncedRefetch = useCallback(() => {
    if (refetchTimeoutRef.current) {
      clearTimeout(refetchTimeoutRef.current);
    }
    refetchTimeoutRef.current = setTimeout(() => {
      refetchHistory();
      refetchTimeoutRef.current = null;
    }, 500);
  }, [refetchHistory]);

  useEffect(() => {
    function handleHistoryEvent(event: Event) {
      const customEvent = event as CustomEvent;
      if (customEvent.detail?.type === 'history.change') {
        debouncedRefetch();
      }
    }

    window.addEventListener('sse:history-event', handleHistoryEvent);

    return () => {
      window.removeEventListener('sse:history-event', handleHistoryEvent);
      if (refetchTimeoutRef.current) {
        clearTimeout(refetchTimeoutRef.current);
      }
    };
  }, [debouncedRefetch]);
}
