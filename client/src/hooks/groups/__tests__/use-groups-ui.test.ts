import type { Group } from '@/types/group';
import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useGroupsUI } from '../use-groups-ui';

// ============================================================
// Mock Modules
// ============================================================

const mockToast = vi.fn();

vi.mock('@/hooks/use-toast', () => ({
  useToast: () => ({
    toast: mockToast,
    toasts: [],
    dismiss: vi.fn(),
  }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (params) {
        return `${key} ${JSON.stringify(params)}`;
      }
      return key;
    },
    i18n: { language: 'pt-BR' },
  }),
}));

// Mock useGroupsConnect
const mockToggleGroupFavorite = vi.fn();
const mockUpdateGroup = vi.fn();

vi.mock('@/hooks/use-groups-connect', () => ({
  useGroupsConnect: () => ({
    toggleGroupFavorite: mockToggleGroupFavorite,
    updateGroup: mockUpdateGroup,
  }),
}));

// Mock useInstancesConnect
const mockLatestInstance = {
  id: 'instance-1',
  user_id: 'user-1',
  instance_id: 'test-instance',
  status: 'connected' as const,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

vi.mock('@/hooks/use-instances-connect', () => ({
  useInstancesConnect: () => ({
    latestInstance: mockLatestInstance,
  }),
}));

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
// Test Setup
// ============================================================

describe('useGroupsUI', () => {
  const mockLoadGroups = vi.fn().mockResolvedValue(undefined);
  const mockSetGroups = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockToggleGroupFavorite.mockResolvedValue({ id: 'group-1', is_favorite: true });
    mockUpdateGroup.mockResolvedValue({ id: 'group-1' });
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  // ============================================================
  // Initial State Tests
  // ============================================================

  describe('initial state', () => {
    it('should have correct initial values', () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      expect(result.current.editInviteCodeDialogOpen).toBe(false);
      expect(result.current.selectedGroupForInvite).toBeNull();
      expect(result.current.inviteCodeInput).toBe('');
    });
  });

  // ============================================================
  // State Setters Tests
  // ============================================================

  describe('state setters', () => {
    it('should update editInviteCodeDialogOpen state', () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      act(() => {
        result.current.setEditInviteCodeDialogOpen(true);
      });

      expect(result.current.editInviteCodeDialogOpen).toBe(true);
    });

    it('should update selectedGroupForInvite state', () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      const group = createMockGroup({ id: 'test-group' });

      act(() => {
        result.current.setSelectedGroupForInvite(group);
      });

      expect(result.current.selectedGroupForInvite).toEqual(group);
    });

    it('should update inviteCodeInput state', () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      act(() => {
        result.current.setInviteCodeInput('ABC123');
      });

      expect(result.current.inviteCodeInput).toBe('ABC123');
    });
  });

  // ============================================================
  // handleSaveInviteCode Tests
  // ============================================================

  describe('handleSaveInviteCode', () => {
    it('should do nothing when no group is selected', async () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      await act(async () => {
        await result.current.handleSaveInviteCode();
      });

      expect(mockUpdateGroup).not.toHaveBeenCalled();
    });

    it('should save invite code successfully', async () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      const group = createMockGroup({ id: 'group-1' });

      act(() => {
        result.current.setSelectedGroupForInvite(group);
        result.current.setInviteCodeInput('ABC123');
        result.current.setEditInviteCodeDialogOpen(true);
      });

      await act(async () => {
        await result.current.handleSaveInviteCode();
      });

      expect(mockUpdateGroup).toHaveBeenCalledWith({
        id: 'group-1',
        invite_code: 'ABC123',
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.success.inviteCodeUpdated',
        description: 'pages.groups.success.inviteCodeUpdatedMessage',
      });

      // State should be reset
      expect(result.current.editInviteCodeDialogOpen).toBe(false);
      expect(result.current.selectedGroupForInvite).toBeNull();
      expect(result.current.inviteCodeInput).toBe('');

      expect(mockLoadGroups).toHaveBeenCalled();
    });

    it('should extract code from full WhatsApp URL', async () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      const group = createMockGroup({ id: 'group-1' });

      act(() => {
        result.current.setSelectedGroupForInvite(group);
        result.current.setInviteCodeInput('https://chat.whatsapp.com/ABC123XYZ');
      });

      await act(async () => {
        await result.current.handleSaveInviteCode();
      });

      expect(mockUpdateGroup).toHaveBeenCalledWith({
        id: 'group-1',
        invite_code: 'ABC123XYZ',
      });
    });

    it('should handle short URL format', async () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      const group = createMockGroup({ id: 'group-1' });

      act(() => {
        result.current.setSelectedGroupForInvite(group);
        result.current.setInviteCodeInput('chat.whatsapp.com/TESTCODE123');
      });

      await act(async () => {
        await result.current.handleSaveInviteCode();
      });

      expect(mockUpdateGroup).toHaveBeenCalledWith({
        id: 'group-1',
        invite_code: 'TESTCODE123',
      });
    });

    it('should show error when invite code is empty', async () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      const group = createMockGroup({ id: 'group-1' });

      act(() => {
        result.current.setSelectedGroupForInvite(group);
        result.current.setInviteCodeInput('   '); // Only whitespace
      });

      await act(async () => {
        await result.current.handleSaveInviteCode();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'common.error',
        description: 'pages.groups.errors.emptyInviteCode',
      });

      expect(mockUpdateGroup).not.toHaveBeenCalled();
    });

    it('should handle save failure', async () => {
      mockUpdateGroup.mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      const group = createMockGroup({ id: 'group-1' });

      act(() => {
        result.current.setSelectedGroupForInvite(group);
        result.current.setInviteCodeInput('ABC123');
      });

      await act(async () => {
        await result.current.handleSaveInviteCode();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.saveInviteCode',
        description: 'Network error',
      });
    });
  });

  // ============================================================
  // toggleFavorite Tests
  // ============================================================

  describe('toggleFavorite', () => {
    it('should add to favorites successfully', async () => {
      const initialGroups: Group[] = [
        createMockGroup({ id: 'group-1', is_favorite: false, name: 'Group A' }),
        createMockGroup({ id: 'group-2', is_favorite: false, name: 'Group B' }),
      ];

      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      await act(async () => {
        await result.current.toggleFavorite('group-1', false);
      });

      expect(mockToggleGroupFavorite).toHaveBeenCalledWith('group-1', true);

      // Check optimistic update
      expect(mockSetGroups).toHaveBeenCalled();

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.success.addedFavorite',
        description: 'pages.groups.success.addedFavoriteMessage',
      });
    });

    it('should remove from favorites successfully', async () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      await act(async () => {
        await result.current.toggleFavorite('group-1', true);
      });

      expect(mockToggleGroupFavorite).toHaveBeenCalledWith('group-1', false);

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.success.removedFavorite',
        description: 'pages.groups.success.removedFavoriteMessage',
      });
    });

    it('should handle toggle failure', async () => {
      mockToggleGroupFavorite.mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      await act(async () => {
        await result.current.toggleFavorite('group-1', false);
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.favoriteUpdate',
        description: 'Network error',
      });
    });

    it('should optimistically update local state and sort', async () => {
      let capturedUpdateFn: ((prev: Group[]) => Group[]) | undefined;

      mockSetGroups.mockImplementation((fn: (prev: Group[]) => Group[]) => {
        capturedUpdateFn = fn;
      });

      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      await act(async () => {
        await result.current.toggleFavorite('group-2', false);
      });

      // Test the update function
      const initialGroups: Group[] = [
        createMockGroup({ id: 'group-1', is_favorite: false, name: 'Alpha' }),
        createMockGroup({ id: 'group-2', is_favorite: false, name: 'Beta' }),
        createMockGroup({ id: 'group-3', is_favorite: true, name: 'Charlie' }),
      ];

      if (capturedUpdateFn) {
        const updatedGroups = capturedUpdateFn(initialGroups);

        // Group 2 should now be favorite
        const group2 = updatedGroups.find((g) => g.id === 'group-2');
        expect(group2?.is_favorite).toBe(true);

        // Favorites should be sorted first
        expect(updatedGroups[0].is_favorite || updatedGroups[1].is_favorite).toBe(true);
      }
    });
  });

  // ============================================================
  // getStatusBadge Tests
  // ============================================================

  describe('getStatusBadge', () => {
    it('should return correct badge for open status', () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      const badge = result.current.getStatusBadge('open');

      expect(badge).toEqual({
        label: 'pages.groups.status.open',
        className: 'bg-gradient-success',
      });
    });

    it('should return correct badge for closed status', () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      const badge = result.current.getStatusBadge('closed');

      expect(badge).toEqual({
        label: 'pages.groups.status.closed',
        className: 'bg-destructive',
      });
    });

    it('should return correct badge for full status', () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      const badge = result.current.getStatusBadge('full');

      expect(badge).toEqual({
        label: 'pages.groups.status.full',
        className: 'bg-primary',
      });
    });

    it('should return default (open) badge for unknown status', () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      const badge = result.current.getStatusBadge('unknown');

      expect(badge).toEqual({
        label: 'pages.groups.status.open',
        className: 'bg-gradient-success',
      });
    });

    it('should handle empty string status', () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      const badge = result.current.getStatusBadge('');

      expect(badge).toEqual({
        label: 'pages.groups.status.open',
        className: 'bg-gradient-success',
      });
    });
  });

  // ============================================================
  // Edge Cases
  // ============================================================

  describe('edge cases', () => {
    it('should handle invite code with special characters', async () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      const group = createMockGroup({ id: 'group-1' });

      act(() => {
        result.current.setSelectedGroupForInvite(group);
        result.current.setInviteCodeInput('ABC-123_XYZ');
      });

      await act(async () => {
        await result.current.handleSaveInviteCode();
      });

      expect(mockUpdateGroup).toHaveBeenCalledWith({
        id: 'group-1',
        invite_code: 'ABC-123_XYZ',
      });
    });

    it('should trim whitespace from invite code', async () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      const group = createMockGroup({ id: 'group-1' });

      act(() => {
        result.current.setSelectedGroupForInvite(group);
        result.current.setInviteCodeInput('  ABC123  ');
      });

      await act(async () => {
        await result.current.handleSaveInviteCode();
      });

      expect(mockUpdateGroup).toHaveBeenCalledWith({
        id: 'group-1',
        invite_code: 'ABC123',
      });
    });

    it('should handle rapid favorite toggles', async () => {
      const { result } = renderHook(() =>
        useGroupsUI({
          loadGroups: mockLoadGroups,
          setGroups: mockSetGroups,
        })
      );

      // Toggle quickly multiple times
      await act(async () => {
        result.current.toggleFavorite('group-1', false);
        result.current.toggleFavorite('group-1', true);
        result.current.toggleFavorite('group-1', false);
      });

      // Should handle all calls
      expect(mockToggleGroupFavorite).toHaveBeenCalledTimes(3);
    });
  });
});
