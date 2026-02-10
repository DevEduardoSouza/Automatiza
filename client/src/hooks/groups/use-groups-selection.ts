import type { Group } from '@/types/group';
import { useCallback, useState } from 'react';
import type { UseGroupsSelectionReturn } from './types';

/**
 * Hook for managing group selection state
 */
export function useGroupsSelection(): UseGroupsSelectionReturn {
  const [selectedGroups, setSelectedGroups] = useState<string[]>([]);

  const toggleGroup = useCallback((id: string) => {
    setSelectedGroups((prev) =>
      prev.includes(id) ? prev.filter((gid) => gid !== id) : [...prev, id]
    );
  }, []);

  const toggleAll = useCallback((groups: Group[]) => {
    setSelectedGroups((prev) => {
      if (prev.length === groups.length) {
        return [];
      }
      return groups.map((g) => g.id);
    });
  }, []);

  const clearSelection = useCallback(() => {
    setSelectedGroups([]);
  }, []);

  return {
    selectedGroups,
    toggleGroup,
    toggleAll,
    clearSelection,
  };
}
