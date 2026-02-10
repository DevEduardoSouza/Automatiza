import { useCallback, useEffect, useRef } from 'react';

/**
 * Hook that listens for real-time job status change events via SSE
 * and triggers a callback when job statuses change.
 *
 * The SSE hook (useSessionStatusSSE) dispatches CustomEvents on window
 * when job.status events arrive. This hook listens for those events
 * and triggers a debounced refetch of the jobs list.
 */
export function useJobsRealtime(refetchJobs: () => void): void {
  const refetchTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const debouncedRefetch = useCallback(() => {
    if (refetchTimeoutRef.current) {
      clearTimeout(refetchTimeoutRef.current);
    }
    refetchTimeoutRef.current = setTimeout(() => {
      refetchJobs();
      refetchTimeoutRef.current = null;
    }, 500);
  }, [refetchJobs]);

  useEffect(() => {
    function handleJobEvent(event: Event) {
      const customEvent = event as CustomEvent;
      if (customEvent.detail?.type === 'job.status') {
        debouncedRefetch();
      }
    }

    window.addEventListener('sse:job-event', handleJobEvent);

    return () => {
      window.removeEventListener('sse:job-event', handleJobEvent);
      if (refetchTimeoutRef.current) {
        clearTimeout(refetchTimeoutRef.current);
      }
    };
  }, [debouncedRefetch]);
}
