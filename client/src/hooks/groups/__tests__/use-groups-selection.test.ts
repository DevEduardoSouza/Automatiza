import type { Group } from '@/types/group';
import { act, renderHook } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { useGroupsSelection } from '../use-groups-selection';

// ============================================================
// Test Data Factories
// ============================================================

function createMockGroup(overrides: Partial<Group> = {}): Group {
  return {
    id: `group-${Math.random().toString(36).slice(2, 9)}`,
    name: 'Test Group',
    members_count: 100,
    member_limit: 256,
    status: 'open',
    wa_group_id: '120363123456789@g.us',
    invite_code: null,
    is_admin: true,
    is_favorite: false,
    ...overrides,
  };
}

// ============================================================
// Tests
// ============================================================

describe('useGroupsSelection', () => {
  // ============================================================
  // Initial State Tests
  // ============================================================

  describe('initial state', () => {
    it('should start with empty selection', () => {
      const { result } = renderHook(() => useGroupsSelection());

      expect(result.current.selectedGroups).toEqual([]);
    });

    it('should provide all required functions', () => {
      const { result } = renderHook(() => useGroupsSelection());

      expect(typeof result.current.toggleGroup).toBe('function');
      expect(typeof result.current.toggleAll).toBe('function');
      expect(typeof result.current.clearSelection).toBe('function');
    });
  });

  // ============================================================
  // toggleGroup Tests
  // ============================================================

  describe('toggleGroup', () => {
    it('should add group to selection when not selected', () => {
      const { result } = renderHook(() => useGroupsSelection());

      act(() => {
        result.current.toggleGroup('group-1');
      });

      expect(result.current.selectedGroups).toEqual(['group-1']);
    });

    it('should remove group from selection when already selected', () => {
      const { result } = renderHook(() => useGroupsSelection());

      // First add the group
      act(() => {
        result.current.toggleGroup('group-1');
      });

      expect(result.current.selectedGroups).toContain('group-1');

      // Then toggle it off
      act(() => {
        result.current.toggleGroup('group-1');
      });

      expect(result.current.selectedGroups).not.toContain('group-1');
    });

    it('should handle multiple groups independently', () => {
      const { result } = renderHook(() => useGroupsSelection());

      act(() => {
        result.current.toggleGroup('group-1');
        result.current.toggleGroup('group-2');
        result.current.toggleGroup('group-3');
      });

      expect(result.current.selectedGroups).toHaveLength(3);
      expect(result.current.selectedGroups).toContain('group-1');
      expect(result.current.selectedGroups).toContain('group-2');
      expect(result.current.selectedGroups).toContain('group-3');

      // Remove one
      act(() => {
        result.current.toggleGroup('group-2');
      });

      expect(result.current.selectedGroups).toHaveLength(2);
      expect(result.current.selectedGroups).not.toContain('group-2');
    });

    it('should preserve order when adding groups', () => {
      const { result } = renderHook(() => useGroupsSelection());

      act(() => {
        result.current.toggleGroup('group-a');
      });
      act(() => {
        result.current.toggleGroup('group-b');
      });
      act(() => {
        result.current.toggleGroup('group-c');
      });

      expect(result.current.selectedGroups).toEqual(['group-a', 'group-b', 'group-c']);
    });
  });

  // ============================================================
  // toggleAll Tests
  // ============================================================

  describe('toggleAll', () => {
    it('should select all groups when none are selected', () => {
      const { result } = renderHook(() => useGroupsSelection());
      const groups = [
        createMockGroup({ id: 'group-1' }),
        createMockGroup({ id: 'group-2' }),
        createMockGroup({ id: 'group-3' }),
      ];

      act(() => {
        result.current.toggleAll(groups);
      });

      expect(result.current.selectedGroups).toHaveLength(3);
      expect(result.current.selectedGroups).toContain('group-1');
      expect(result.current.selectedGroups).toContain('group-2');
      expect(result.current.selectedGroups).toContain('group-3');
    });

    it('should deselect all groups when all are selected', () => {
      const { result } = renderHook(() => useGroupsSelection());
      const groups = [
        createMockGroup({ id: 'group-1' }),
        createMockGroup({ id: 'group-2' }),
        createMockGroup({ id: 'group-3' }),
      ];

      // First select all
      act(() => {
        result.current.toggleAll(groups);
      });

      expect(result.current.selectedGroups).toHaveLength(3);

      // Then toggle all (should deselect)
      act(() => {
        result.current.toggleAll(groups);
      });

      expect(result.current.selectedGroups).toHaveLength(0);
    });

    it('should select all when partially selected', () => {
      const { result } = renderHook(() => useGroupsSelection());
      const groups = [
        createMockGroup({ id: 'group-1' }),
        createMockGroup({ id: 'group-2' }),
        createMockGroup({ id: 'group-3' }),
      ];

      // Select only one
      act(() => {
        result.current.toggleGroup('group-1');
      });

      expect(result.current.selectedGroups).toHaveLength(1);

      // Toggle all (should select all since not all selected)
      act(() => {
        result.current.toggleAll(groups);
      });

      expect(result.current.selectedGroups).toHaveLength(3);
    });

    it('should handle empty groups array', () => {
      const { result } = renderHook(() => useGroupsSelection());

      act(() => {
        result.current.toggleAll([]);
      });

      expect(result.current.selectedGroups).toHaveLength(0);
    });

    it('should handle single group', () => {
      const { result } = renderHook(() => useGroupsSelection());
      const groups = [createMockGroup({ id: 'single-group' })];

      act(() => {
        result.current.toggleAll(groups);
      });

      expect(result.current.selectedGroups).toEqual(['single-group']);

      act(() => {
        result.current.toggleAll(groups);
      });

      expect(result.current.selectedGroups).toHaveLength(0);
    });
  });

  // ============================================================
  // clearSelection Tests
  // ============================================================

  describe('clearSelection', () => {
    it('should clear all selected groups', () => {
      const { result } = renderHook(() => useGroupsSelection());

      // Add some groups
      act(() => {
        result.current.toggleGroup('group-1');
        result.current.toggleGroup('group-2');
        result.current.toggleGroup('group-3');
      });

      expect(result.current.selectedGroups).toHaveLength(3);

      // Clear selection
      act(() => {
        result.current.clearSelection();
      });

      expect(result.current.selectedGroups).toHaveLength(0);
    });

    it('should handle clearing empty selection', () => {
      const { result } = renderHook(() => useGroupsSelection());

      expect(result.current.selectedGroups).toHaveLength(0);

      act(() => {
        result.current.clearSelection();
      });

      expect(result.current.selectedGroups).toHaveLength(0);
    });

    it('should allow reselection after clearing', () => {
      const { result } = renderHook(() => useGroupsSelection());

      // Add groups
      act(() => {
        result.current.toggleGroup('group-1');
        result.current.toggleGroup('group-2');
      });

      // Clear
      act(() => {
        result.current.clearSelection();
      });

      // Add again
      act(() => {
        result.current.toggleGroup('group-3');
      });

      expect(result.current.selectedGroups).toEqual(['group-3']);
    });
  });

  // ============================================================
  // Edge Cases
  // ============================================================

  describe('edge cases', () => {
    it('should handle rapid toggles correctly', () => {
      const { result } = renderHook(() => useGroupsSelection());

      act(() => {
        result.current.toggleGroup('group-1');
        result.current.toggleGroup('group-1');
        result.current.toggleGroup('group-1');
      });

      // Should be selected (odd number of toggles)
      expect(result.current.selectedGroups).toEqual(['group-1']);
    });

    it('should handle special characters in group IDs', () => {
      const { result } = renderHook(() => useGroupsSelection());
      const specialId = '120363123456789@g.us';

      act(() => {
        result.current.toggleGroup(specialId);
      });

      expect(result.current.selectedGroups).toContain(specialId);
    });

    it('should handle very long group IDs', () => {
      const { result } = renderHook(() => useGroupsSelection());
      const longId = 'a'.repeat(1000);

      act(() => {
        result.current.toggleGroup(longId);
      });

      expect(result.current.selectedGroups).toContain(longId);
    });

    it('should maintain selection stability across rerenders', () => {
      const { result, rerender } = renderHook(() => useGroupsSelection());

      act(() => {
        result.current.toggleGroup('group-1');
        result.current.toggleGroup('group-2');
      });

      const selectionBeforeRerender = [...result.current.selectedGroups];

      rerender();

      expect(result.current.selectedGroups).toEqual(selectionBeforeRerender);
    });
  });
});
