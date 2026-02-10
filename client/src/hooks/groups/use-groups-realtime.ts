import { useGroupsConnect } from '@/hooks/use-groups-connect';
import { useCallback, useEffect, useRef } from 'react';

/**
 * Group-related SSE event types from the backend.
 */
const GROUP_SSE_EVENTS = [
  'group.created',
  'group.updated',
  'group.deleted',
  'group.member.changed',
  'group.sync',
] as const;

/**
 * Hook that listens for real-time group change events via SSE
 * and automatically invalidates the groups query cache.
 *
 * The SSE hook (useSessionStatusSSE) dispatches CustomEvents on window
 * when group-related events arrive. This hook listens for those events
 * and triggers a debounced refetch of the groups list.
 */
export function useGroupsRealtime(): void {
  const { refetch } = useGroupsConnect();
  const refetchTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  /**
   * Debounced refetch to avoid rapid consecutive refetches
   * when multiple events arrive in quick succession.
   */
  const debouncedRefetch = useCallback(() => {
    if (refetchTimeoutRef.current) {
      clearTimeout(refetchTimeoutRef.current);
    }
    refetchTimeoutRef.current = setTimeout(() => {
      refetch();
      refetchTimeoutRef.current = null;
    }, 500);
  }, [refetch]);

  useEffect(() => {
    function handleGroupEvent(event: Event) {
      const customEvent = event as CustomEvent;
      if (customEvent.detail?.type && GROUP_SSE_EVENTS.includes(customEvent.detail.type)) {
        debouncedRefetch();
      }
    }

    window.addEventListener('sse:group-event', handleGroupEvent);

    return () => {
      window.removeEventListener('sse:group-event', handleGroupEvent);
      if (refetchTimeoutRef.current) {
        clearTimeout(refetchTimeoutRef.current);
      }
    };
  }, [debouncedRefetch]);
}
