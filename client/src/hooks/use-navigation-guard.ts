import { useEffect } from 'react';

/**
 * Blocks browser close/reload via `beforeunload` while a condition is true.
 * Replaces `useBlocker` which requires a data router (createBrowserRouter).
 *
 * @param when - Whether navigation should be blocked right now.
 */
export function useNavigationGuard(when: boolean): void {
  useEffect(() => {
    if (!when) return;

    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };

    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [when]);
}
