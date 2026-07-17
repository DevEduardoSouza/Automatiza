import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules (inline values to avoid TDZ issues with vi.mock hoisting)
// ============================================================

// Stable reference to avoid infinite useEffect re-renders
const stableUser = { id: 'user-1', email: 'admin@test.com' };
vi.mock('@/contexts', () => ({
  useAuth: () => ({ user: stableUser }),
}));

const stableT = (key: string) => key;
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: stableT }),
}));

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

vi.mock('@/lib/connect-transport', () => ({
  getErrorInfo: (e: unknown) => ({
    userMessage: e instanceof Error ? e.message : 'error',
    code: (e as { code?: string })?.code ?? 'INTERNAL',
  }),
}));

vi.mock('@bufbuild/protobuf', () => ({
  create: (_schema: unknown, data: unknown) => data,
}));

vi.mock('@/gen/admin/v1/admin_pb', () => ({
  AdminService: 'AdminService',
  GrantAdminRoleRequestSchema: 'GrantAdminRoleRequestSchema',
  ListAdminUsersRequestSchema: 'ListAdminUsersRequestSchema',
  RevokeAdminRoleRequestSchema: 'RevokeAdminRoleRequestSchema',
}));

vi.mock('@/gen/profiles/v1/profiles_pb', () => ({
  ProfilesService: 'ProfilesService',
  SearchProfilesRequestSchema: 'SearchProfilesRequestSchema',
}));

const mockListAdminUsers = vi.fn();
const mockGrantAdminRole = vi.fn();
const mockRevokeAdminRole = vi.fn();
const mockSearchProfiles = vi.fn();

vi.mock('@connectrpc/connect', () => ({
  createClient: (service: unknown) => {
    if (service === 'ProfilesService') {
      return { searchProfiles: mockSearchProfiles };
    }
    return {
      listAdminUsers: mockListAdminUsers,
      grantAdminRole: mockGrantAdminRole,
      revokeAdminRole: mockRevokeAdminRole,
    };
  },
}));

vi.mock('@connectrpc/connect-query', () => ({
  useTransport: () => 'mock-transport',
}));

// ============================================================
// Import after mocks
// ============================================================

import { toast } from 'sonner';
import { useTeamTab } from '../use-team-tab';

// ============================================================
// Test Helpers
// ============================================================

const mockAdminsResponse = {
  admins: [
    {
      userId: 'admin-1',
      role: 'admin',
      grantedAt: '2026-01-01T00:00:00Z',
      email: 'admin1@test.com',
    },
    {
      userId: 'admin-2',
      role: 'super_admin',
      grantedAt: '2026-01-15T00:00:00Z',
      email: 'admin2@test.com',
    },
  ],
};

/**
 * Renders the hook and waits for the initial useEffect fetch to complete.
 */
async function renderAndWait() {
  const hook = renderHook(() => useTeamTab());

  await waitFor(() => {
    expect(hook.result.current.loading).toBe(false);
  });

  return hook;
}

// ============================================================
// Tests
// ============================================================

describe('useTeamTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListAdminUsers.mockResolvedValue(mockAdminsResponse);
    mockGrantAdminRole.mockResolvedValue({});
    mockRevokeAdminRole.mockResolvedValue({});
    mockSearchProfiles.mockResolvedValue({
      profiles: [{ id: 'found-user', email: 'new@test.com' }],
    });
    vi.spyOn(window, 'confirm').mockReturnValue(true);
  });

  describe('initial state', () => {
    it('should start with empty teamMembers and loading true', () => {
      mockListAdminUsers.mockImplementation(() => new Promise(() => {}));

      const { result } = renderHook(() => useTeamTab());

      expect(result.current.teamMembers).toEqual([]);
      expect(result.current.loading).toBe(true);
    });

    it('should have isDialogOpen false and newAdminEmail empty', async () => {
      const { result, unmount } = await renderAndWait();

      expect(result.current.isDialogOpen).toBe(false);
      expect(result.current.newAdminEmail).toBe('');
      unmount();
    });
  });

  describe('fetchTeamMembers', () => {
    it('should map response admins to TeamMember objects', async () => {
      const { result, unmount } = await renderAndWait();

      expect(result.current.loading).toBe(false);
      expect(result.current.teamMembers).toHaveLength(2);
      expect(result.current.teamMembers[0]).toEqual({
        id: 'admin-1',
        user_id: 'admin-1',
        role: 'admin',
        created_at: '2026-01-01T00:00:00Z',
        email: 'admin1@test.com',
      });
      unmount();
    });

    it('should toast error on fetch failure', async () => {
      mockListAdminUsers.mockRejectedValue(new Error('Network error'));

      const { unmount } = renderHook(() => useTeamTab());

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith('Network error');
      });

      unmount();
    });
  });

  describe('handleAddAdmin', () => {
    it('should toast error when email is empty', async () => {
      const { result, unmount } = await renderAndWait();

      await act(async () => {
        await result.current.handleAddAdmin();
      });

      expect(toast.error).toHaveBeenCalledWith('admin.team.errors.emailRequired');
      expect(mockSearchProfiles).not.toHaveBeenCalled();
      unmount();
    });

    it('should search, grant role, and toast success', async () => {
      const { result, unmount } = await renderAndWait();

      act(() => result.current.setNewAdminEmail('new@test.com'));

      await act(async () => {
        await result.current.handleAddAdmin();
      });

      // Wait for the fire-and-forget fetchTeamMembers refetch to complete
      await waitFor(() => {
        expect(mockListAdminUsers).toHaveBeenCalledTimes(2);
      });

      expect(mockSearchProfiles).toHaveBeenCalledWith(
        expect.objectContaining({ query: 'new@test.com', limit: 1 })
      );
      expect(mockGrantAdminRole).toHaveBeenCalledWith(
        expect.objectContaining({ userId: 'found-user' })
      );
      expect(toast.success).toHaveBeenCalledWith('admin.team.success.added');
      unmount();
    });

    it('should toast error when user not found', async () => {
      mockSearchProfiles.mockResolvedValue({ profiles: [] });

      const { result, unmount } = await renderAndWait();

      act(() => result.current.setNewAdminEmail('nobody@test.com'));

      await act(async () => {
        await result.current.handleAddAdmin();
      });

      expect(toast.error).toHaveBeenCalledWith('admin.team.errors.userNotFound');
      unmount();
    });

    it('should toast alreadyAdmin error on ALREADY_EXISTS code', async () => {
      const alreadyExistsError = Object.assign(new Error('Already exists'), {
        code: 'ALREADY_EXISTS',
      });
      mockGrantAdminRole.mockRejectedValue(alreadyExistsError);

      const { result, unmount } = await renderAndWait();

      act(() => result.current.setNewAdminEmail('existing@test.com'));

      await act(async () => {
        await result.current.handleAddAdmin();
      });

      expect(toast.error).toHaveBeenCalledWith('admin.team.errors.alreadyMember');
      unmount();
    });
  });

  describe('handleRemoveAdmin', () => {
    it('should revoke role and toast success when confirmed', async () => {
      const { result, unmount } = await renderAndWait();

      await act(async () => {
        await result.current.handleRemoveAdmin('admin-1', 'admin1@test.com', 'admin');
      });

      // Wait for the fire-and-forget fetchTeamMembers refetch to complete
      await waitFor(() => {
        expect(mockListAdminUsers).toHaveBeenCalledTimes(2);
      });

      expect(mockRevokeAdminRole).toHaveBeenCalledWith(
        expect.objectContaining({ userId: 'admin-1' })
      );
      expect(toast.success).toHaveBeenCalledWith('admin.team.success.removed');
      unmount();
    });

    it('should not revoke when user cancels confirm', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(false);

      const { result, unmount } = await renderAndWait();

      await act(async () => {
        await result.current.handleRemoveAdmin('admin-1', 'admin1@test.com', 'admin');
      });

      expect(mockRevokeAdminRole).not.toHaveBeenCalled();
      unmount();
    });

    it('should toast error on revoke failure', async () => {
      mockRevokeAdminRole.mockRejectedValue(new Error('Revoke failed'));

      const { result, unmount } = await renderAndWait();

      await act(async () => {
        await result.current.handleRemoveAdmin('admin-1', 'admin1@test.com', 'admin');
      });

      expect(toast.error).toHaveBeenCalledWith('Revoke failed');
      unmount();
    });
  });
});
