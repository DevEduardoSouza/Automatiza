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

const mockToggleGroupFavorite = vi.fn();
const mockUpdateGroup = vi.fn();

vi.mock('@/hooks/use-groups-connect', () => ({
  useGroupsConnect: () => ({
    toggleGroupFavorite: mockToggleGroupFavorite,
    updateGroup: mockUpdateGroup,
  }),
}));

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

const mockCaptureHookError = vi.fn();

vi.mock('@/lib/sentry', () => ({
  captureHookError: (...args: unknown[]) => mockCaptureHookError(...args),
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
  const mockUpdateGroupOptimistic = vi.fn();

  function renderUI(overrides: Partial<Parameters<typeof useGroupsUI>[0]> = {}) {
    return renderHook(() =>
      useGroupsUI({
        loadGroups: mockLoadGroups,
        updateGroupOptimistic: mockUpdateGroupOptimistic,
        ...overrides,
      })
    );
  }

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
      const { result } = renderUI();

      expect(result.current.editInviteCodeDialogOpen).toBe(false);
      expect(result.current.selectedGroupForInvite).toBeNull();
      expect(result.current.inviteCodeInput).toBe('');
    });

    it('should expose all required functions', () => {
      const { result } = renderUI();

      expect(typeof result.current.setEditInviteCodeDialogOpen).toBe('function');
      expect(typeof result.current.setSelectedGroupForInvite).toBe('function');
      expect(typeof result.current.setInviteCodeInput).toBe('function');
      expect(typeof result.current.handleSaveInviteCode).toBe('function');
      expect(typeof result.current.toggleFavorite).toBe('function');
      expect(typeof result.current.getStatusBadge).toBe('function');
    });
  });

  // ============================================================
  // State Setters Tests
  // ============================================================

  describe('state setters', () => {
    it('should update editInviteCodeDialogOpen state', () => {
      const { result } = renderUI();

      act(() => {
        result.current.setEditInviteCodeDialogOpen(true);
      });

      expect(result.current.editInviteCodeDialogOpen).toBe(true);

      act(() => {
        result.current.setEditInviteCodeDialogOpen(false);
      });

      expect(result.current.editInviteCodeDialogOpen).toBe(false);
    });

    it('should update selectedGroupForInvite state', () => {
      const { result } = renderUI();

      const group = createMockGroup({ id: 'test-group' });

      act(() => {
        result.current.setSelectedGroupForInvite(group);
      });

      expect(result.current.selectedGroupForInvite).toEqual(group);

      act(() => {
        result.current.setSelectedGroupForInvite(null);
      });

      expect(result.current.selectedGroupForInvite).toBeNull();
    });

    it('should update inviteCodeInput state', () => {
      const { result } = renderUI();

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
    it('should do nothing when no group is selected (early return)', async () => {
      const { result } = renderUI();

      await act(async () => {
        await result.current.handleSaveInviteCode();
      });

      expect(mockUpdateGroup).not.toHaveBeenCalled();
      expect(mockToast).not.toHaveBeenCalled();
    });

    it('should save invite code successfully', async () => {
      const { result } = renderUI();

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

    it('should extract code from full WhatsApp URL (https)', async () => {
      const { result } = renderUI();

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

    it('should handle short URL format (without https)', async () => {
      const { result } = renderUI();

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

    it('should show error when invite code is empty after trimming', async () => {
      const { result } = renderUI();

      const group = createMockGroup({ id: 'group-1' });

      act(() => {
        result.current.setSelectedGroupForInvite(group);
        result.current.setInviteCodeInput('   ');
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

    it('should show error when invite code is empty string', async () => {
      const { result } = renderUI();

      const group = createMockGroup({ id: 'group-1' });

      act(() => {
        result.current.setSelectedGroupForInvite(group);
        result.current.setInviteCodeInput('');
      });

      await act(async () => {
        await result.current.handleSaveInviteCode();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          description: 'pages.groups.errors.emptyInviteCode',
        })
      );
    });

    it('should handle save failure with Error instance', async () => {
      mockUpdateGroup.mockRejectedValue(new Error('Network error'));

      const { result } = renderUI();

      const group = createMockGroup({ id: 'group-1' });

      act(() => {
        result.current.setSelectedGroupForInvite(group);
        result.current.setInviteCodeInput('ABC123');
      });

      await act(async () => {
        await result.current.handleSaveInviteCode();
      });

      expect(mockCaptureHookError).toHaveBeenCalledWith(
        expect.any(Error),
        'useGroupsUI',
        'handleSaveInviteCode'
      );

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.saveInviteCode',
        description: 'Network error',
      });
    });

    it('should handle save failure with non-Error exception', async () => {
      mockUpdateGroup.mockRejectedValue('string error');

      const { result } = renderUI();

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
        description: 'pages.groups.errors.saveInviteCode',
      });
    });

    it('should trim whitespace from invite code', async () => {
      const { result } = renderUI();

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

    it('should handle invite code with special characters', async () => {
      const { result } = renderUI();

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
  });

  // ============================================================
  // toggleFavorite Tests
  // ============================================================

  describe('toggleFavorite', () => {
    it('should add to favorites successfully', async () => {
      const { result } = renderUI();

      await act(async () => {
        await result.current.toggleFavorite('group-1', false);
      });

      // Optimistic update first
      expect(mockUpdateGroupOptimistic).toHaveBeenCalledWith('group-1', { is_favorite: true });

      // Then RPC call
      expect(mockToggleGroupFavorite).toHaveBeenCalledWith('group-1', true);

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.success.addedFavorite',
        description: 'pages.groups.success.addedFavoriteMessage',
      });
    });

    it('should remove from favorites successfully', async () => {
      const { result } = renderUI();

      await act(async () => {
        await result.current.toggleFavorite('group-1', true);
      });

      expect(mockUpdateGroupOptimistic).toHaveBeenCalledWith('group-1', { is_favorite: false });
      expect(mockToggleGroupFavorite).toHaveBeenCalledWith('group-1', false);

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.success.removedFavorite',
        description: 'pages.groups.success.removedFavoriteMessage',
      });
    });

    it('should revert optimistic update on error', async () => {
      mockToggleGroupFavorite.mockRejectedValue(new Error('Network error'));

      const { result } = renderUI();

      await act(async () => {
        await result.current.toggleFavorite('group-1', false);
      });

      // First call: optimistic update (set to true)
      expect(mockUpdateGroupOptimistic).toHaveBeenNthCalledWith(1, 'group-1', {
        is_favorite: true,
      });

      // Second call: revert (set back to false)
      expect(mockUpdateGroupOptimistic).toHaveBeenNthCalledWith(2, 'group-1', {
        is_favorite: false,
      });

      expect(mockCaptureHookError).toHaveBeenCalledWith(
        expect.any(Error),
        'useGroupsUI',
        'toggleFavorite'
      );

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.favoriteUpdate',
        description: 'Network error',
      });
    });

    it('should revert optimistic update on error when removing favorite', async () => {
      mockToggleGroupFavorite.mockRejectedValue(new Error('fail'));

      const { result } = renderUI();

      await act(async () => {
        await result.current.toggleFavorite('group-1', true);
      });

      // First call: optimistic update (set to false)
      expect(mockUpdateGroupOptimistic).toHaveBeenNthCalledWith(1, 'group-1', {
        is_favorite: false,
      });

      // Second call: revert (set back to true)
      expect(mockUpdateGroupOptimistic).toHaveBeenNthCalledWith(2, 'group-1', {
        is_favorite: true,
      });
    });

    it('should handle non-Error exceptions', async () => {
      mockToggleGroupFavorite.mockRejectedValue('string error');

      const { result } = renderUI();

      await act(async () => {
        await result.current.toggleFavorite('group-1', false);
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.favoriteUpdate',
        description: 'pages.groups.errors.favoriteUpdate',
      });
    });

    it('should handle rapid favorite toggles', async () => {
      const { result } = renderUI();

      await act(async () => {
        result.current.toggleFavorite('group-1', false);
        result.current.toggleFavorite('group-1', true);
        result.current.toggleFavorite('group-1', false);
      });

      expect(mockToggleGroupFavorite).toHaveBeenCalledTimes(3);
    });
  });

  // ============================================================
  // getStatusBadge Tests
  // ============================================================

  describe('getStatusBadge', () => {
    it('should return correct badge for open status', () => {
      const { result } = renderUI();

      const badge = result.current.getStatusBadge('open');

      expect(badge).toEqual({
        label: 'pages.groups.status.open',
        className: 'bg-gradient-success',
      });
    });

    it('should return correct badge for closed status', () => {
      const { result } = renderUI();

      const badge = result.current.getStatusBadge('closed');

      expect(badge).toEqual({
        label: 'pages.groups.status.closed',
        className: 'bg-destructive',
      });
    });

    it('should return correct badge for full status', () => {
      const { result } = renderUI();

      const badge = result.current.getStatusBadge('full');

      expect(badge).toEqual({
        label: 'pages.groups.status.full',
        className: 'bg-primary',
      });
    });

    it('should return default (open) badge for unknown status', () => {
      const { result } = renderUI();

      const badge = result.current.getStatusBadge('unknown');

      expect(badge).toEqual({
        label: 'pages.groups.status.open',
        className: 'bg-gradient-success',
      });
    });

    it('should return default (open) badge for empty string status', () => {
      const { result } = renderUI();

      const badge = result.current.getStatusBadge('');

      expect(badge).toEqual({
        label: 'pages.groups.status.open',
        className: 'bg-gradient-success',
      });
    });

    it('should be stable across rerenders (memoized)', () => {
      const { result, rerender } = renderUI();

      const badge1 = result.current.getStatusBadge('open');
      rerender();
      const badge2 = result.current.getStatusBadge('open');

      expect(badge1).toEqual(badge2);
    });
  });

  // ============================================================
  // Edge Cases
  // ============================================================

  describe('edge cases', () => {
    it('should handle URL with query parameters', async () => {
      const { result } = renderUI();

      const group = createMockGroup({ id: 'group-1' });

      act(() => {
        result.current.setSelectedGroupForInvite(group);
        result.current.setInviteCodeInput('https://chat.whatsapp.com/ABC123?ref=share');
      });

      await act(async () => {
        await result.current.handleSaveInviteCode();
      });

      // Regex extracts only alphanumeric chars after the domain
      expect(mockUpdateGroup).toHaveBeenCalledWith({
        id: 'group-1',
        invite_code: 'ABC123',
      });
    });

    it('should handle setting group for invite and then clearing it', async () => {
      const { result } = renderUI();

      const group = createMockGroup({ id: 'group-1' });

      act(() => {
        result.current.setSelectedGroupForInvite(group);
      });

      expect(result.current.selectedGroupForInvite).not.toBeNull();

      act(() => {
        result.current.setSelectedGroupForInvite(null);
      });

      await act(async () => {
        await result.current.handleSaveInviteCode();
      });

      // Should early return without calling anything
      expect(mockUpdateGroup).not.toHaveBeenCalled();
    });
  });
});
