import { useAuth } from '@/contexts';
import { useAutomationsConnect } from '@/hooks/use-automations-connect';
import { useProfilesConnect } from '@/hooks/use-profiles-connect';
import { useCallback, useEffect, useMemo, useState } from 'react';

/**
 * Return type for the useAutomationLimits hook.
 * Provides automation limit information for UI display and action control.
 */
export interface UseAutomationLimitsReturn {
  /** Current number of automations the user has */
  currentCount: number;
  /** Maximum number of automations allowed for the user's plan */
  maxCount: number;
  /** Whether the user can create a new automation */
  canCreate: boolean;
  /** Number of remaining automations the user can create */
  remaining: number;
  /** Whether the limit has been reached (currentCount >= maxCount) */
  limitReached: boolean;
  /** Whether the user is close to the limit (currentCount >= maxCount - 1) */
  nearLimit: boolean;
  /** Whether limit data is still loading */
  isLoading: boolean;
  /** Refetch the current automation count */
  refetch: () => Promise<void>;
}

/** Default max automations if profile data is unavailable */
const DEFAULT_MAX_AUTOMATIONS = 10;

/**
 * Hook for automation limit information and validation.
 * Provides current count, max count, and derived state for UI display.
 *
 * @example
 * ```tsx
 * function CreateAutomationButton() {
 *   const { canCreate, currentCount, maxCount, limitReached } = useAutomationLimits();
 *
 *   return (
 *     <div>
 *       <Badge variant={limitReached ? 'destructive' : 'default'}>
 *         {currentCount}/{maxCount}
 *       </Badge>
 *       <Button disabled={!canCreate}>Create</Button>
 *     </div>
 *   );
 * }
 * ```
 */
export function useAutomationLimits(): UseAutomationLimitsReturn {
  const { user } = useAuth();
  const profilesConnect = useProfilesConnect();
  const automationsConnect = useAutomationsConnect();

  const [currentCount, setCurrentCount] = useState(0);
  const [isLoading, setIsLoading] = useState(true);

  // Get max automations from user profile/limits
  const maxCount = useMemo(() => {
    const v = profilesConnect.limits?.max_automations;
    if (typeof v === 'number' && Number.isFinite(v) && v > 0) {
      return Math.floor(v);
    }
    return DEFAULT_MAX_AUTOMATIONS;
  }, [profilesConnect.limits?.max_automations]);

  // Fetch automation count
  const fetchCount = useCallback(async () => {
    if (!user?.id) {
      setCurrentCount(0);
      setIsLoading(false);
      return;
    }

    setIsLoading(true);
    try {
      const automations = await automationsConnect.listAutomations();
      setCurrentCount(automations.length);
    } catch {
      // Keep current count on error, don't block UI
    } finally {
      setIsLoading(false);
    }
  }, [user?.id, automationsConnect.listAutomations]);

  // Fetch on mount and when user changes
  useEffect(() => {
    fetchCount();
  }, [fetchCount]);

  // Derived state
  const limitReached = currentCount >= maxCount;
  const nearLimit = maxCount > 0 && currentCount >= maxCount - 1;
  const canCreate = !limitReached;
  const remaining = Math.max(0, maxCount - currentCount);

  return {
    currentCount,
    maxCount,
    canCreate,
    remaining,
    limitReached,
    nearLimit,
    isLoading: isLoading || profilesConnect.loading,
    refetch: fetchCount,
  };
}
