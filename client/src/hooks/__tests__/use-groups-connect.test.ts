import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useGroupsConnect } from '../use-groups-connect';

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

vi.mock('@/contexts', () => ({
  useAuth: () => ({
    user: { id: 'user-123', email: 'test@example.com' },
    session: null,
    loading: false,
  }),
}));

// ============================================================
// Mock Transport and RPC Client
// ============================================================

const mockClientMethods: Record<string, ReturnType<typeof vi.fn>> = {
  listGroups: vi.fn(),
  getGroup: vi.fn(),
  getGroupsByIds: vi.fn(),
  createGroup: vi.fn(),
  updateGroup: vi.fn(),
  deleteGroup: vi.fn(),
  bulkUpdateGroups: vi.fn(),
  toggleGroupFavorite: vi.fn(),
  syncGroups: vi.fn(),
  getSyncStatus: vi.fn(),
  sendGroupMessage: vi.fn(),
  sendGroupAudio: vi.fn(),
  sendGroupImage: vi.fn(),
  sendGroupPoll: vi.fn(),
  updateGroupSubject: vi.fn(),
  updateGroupDescription: vi.fn(),
  updateGroupPicture: vi.fn(),
  updateGroupSettings: vi.fn(),
  getGroupInviteCode: vi.fn(),
  getGroupParticipants: vi.fn(),
  addGroupMember: vi.fn(),
  removeGroupMember: vi.fn(),
  leaveGroup: vi.fn(),
  promoteMember: vi.fn(),
  demoteMember: vi.fn(),
};

vi.mock('@connectrpc/connect', () => ({
  createClient: () => mockClientMethods,
}));

vi.mock('@connectrpc/connect-query', () => ({
  useTransport: () => ({}),
}));

// ============================================================
// Mock Proto Modules (generated code)
// ============================================================

vi.mock('@/gen/groups/v1/groups_pb', () => ({
  GroupStatus: { UNSPECIFIED: 0, OPEN: 1, CLOSED: 2, FULL: 3 },
  GroupsService: { typeName: 'groups.v1.GroupsService' },
  ListGroupsRequestSchema: {},
  GetGroupRequestSchema: {},
  GetGroupsByIdsRequestSchema: {},
  CreateGroupRequestSchema: {},
  UpdateGroupRequestSchema: {},
  DeleteGroupRequestSchema: {},
  BulkUpdateGroupsRequestSchema: {},
  ToggleGroupFavoriteRequestSchema: {},
  SyncGroupsRequestSchema: {},
  GetSyncStatusRequestSchema: {},
  SendGroupMessageRequestSchema: {},
  SendGroupAudioRequestSchema: {},
  SendGroupImageRequestSchema: {},
  SendGroupPollRequestSchema: {},
  UpdateGroupSubjectRequestSchema: {},
  UpdateGroupDescriptionRequestSchema: {},
  UpdateGroupPictureRequestSchema: {},
  UpdateGroupSettingsRequestSchema: {},
  GetGroupInviteCodeRequestSchema: {},
  GetGroupParticipantsRequestSchema: {},
  AddGroupMemberRequestSchema: {},
  RemoveGroupMemberRequestSchema: {},
  LeaveGroupRequestSchema: {},
  PromoteMemberRequestSchema: {},
  DemoteMemberRequestSchema: {},
}));

vi.mock('@bufbuild/protobuf', () => ({
  create: (_schema: unknown, data: unknown) => data,
}));

// ============================================================
// Mock Error Info
// ============================================================

vi.mock('@/lib/connect-transport', () => ({
  getErrorInfo: (error: unknown) => ({
    code: 'UNKNOWN',
    message: error instanceof Error ? error.message : 'Unknown error',
    userMessage: error instanceof Error ? error.message : 'Unknown error',
    action: 'toast',
    retryable: false,
  }),
}));

// ============================================================
// GroupStatus enum reference (for test assertions only)
// ============================================================

const GroupStatusEnum = {
  UNSPECIFIED: 0,
  OPEN: 1,
  CLOSED: 2,
  FULL: 3,
} as const;

// ============================================================
// Mock React Query
// ============================================================

const mockQueryClient = {
  invalidateQueries: vi.fn(),
};

interface QueryOptions {
  queryKey: string[];
  queryFn: () => Promise<unknown>;
  enabled?: boolean;
  staleTime?: number;
  refetchInterval?: number;
}

interface MutationOptions<TData = unknown, TError = unknown, TVariables = unknown> {
  mutationFn: (variables: TVariables) => Promise<TData>;
  onSuccess?: (data: TData) => void;
  onError?: (error: TError) => void;
}

const mockUseQuery = vi.fn();
const mockUseMutation = vi.fn();

vi.mock('@tanstack/react-query', () => ({
  useQuery: (options: QueryOptions) => mockUseQuery(options),
  useMutation: (options: MutationOptions) => mockUseMutation(options),
  useQueryClient: () => mockQueryClient,
}));

// ============================================================
// Test Data Factories
// ============================================================

function createMockProtoGroup(overrides: Record<string, unknown> = {}) {
  return {
    id: 'group-1',
    userId: 'user-123',
    instanceId: 'instance-1',
    waGroupId: '120363123456789@g.us',
    name: 'Test Group',
    description: 'Test description',
    membersCount: 100,
    memberLimit: 256,
    status: GroupStatusEnum.OPEN,
    inviteCode: 'ABC123',
    isAdmin: true,
    isFavorite: false,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

// ============================================================
// Helper: setup default mockUseMutation
// ============================================================

/**
 * Creates a default useMutation mock that executes mutationFn
 * and calls onSuccess on resolve or onError on reject.
 */
function setupDefaultMutationMock() {
  mockUseMutation.mockImplementation((options: MutationOptions) => ({
    mutateAsync: async (variables: unknown) => {
      try {
        const result = await options.mutationFn(variables);
        if (options.onSuccess) {
          options.onSuccess(result);
        }
        return result;
      } catch (err) {
        if (options.onError) {
          options.onError(err);
        }
        throw err;
      }
    },
    isPending: false,
  }));
}

// ============================================================
// Test Setup
// ============================================================

describe('useGroupsConnect', () => {
  beforeEach(() => {
    vi.clearAllMocks();

    // Default: list groups query returns empty
    mockUseQuery.mockImplementation(() => ({
      data: [],
      isLoading: false,
      refetch: vi.fn().mockResolvedValue(undefined),
    }));

    // Default: mutations execute their mutationFn and call onSuccess/onError
    setupDefaultMutationMock();

    // Default client method responses
    mockClientMethods.listGroups.mockResolvedValue({
      groups: [createMockProtoGroup()],
    });
    mockClientMethods.getGroup.mockResolvedValue({
      group: createMockProtoGroup(),
    });
    mockClientMethods.getGroupsByIds.mockResolvedValue({
      groups: [createMockProtoGroup(), createMockProtoGroup({ id: 'group-2', name: 'Group 2' })],
    });
    mockClientMethods.createGroup.mockResolvedValue({
      group: createMockProtoGroup({ id: 'new-group' }),
    });
    mockClientMethods.updateGroup.mockResolvedValue({
      group: createMockProtoGroup({ name: 'Updated Group' }),
    });
    mockClientMethods.deleteGroup.mockResolvedValue({});
    mockClientMethods.bulkUpdateGroups.mockResolvedValue({ updatedCount: 3 });
    mockClientMethods.toggleGroupFavorite.mockResolvedValue({
      group: createMockProtoGroup({ isFavorite: true }),
    });
    mockClientMethods.syncGroups.mockResolvedValue({
      success: true,
      jobId: 'job-123',
      message: 'Sync started',
    });
    mockClientMethods.getSyncStatus.mockResolvedValue({
      status: 'completed',
      progress: 100,
      error: '',
      groupsSynced: 10,
      totalGroups: 10,
    });
    mockClientMethods.sendGroupMessage.mockResolvedValue({
      messageId: 'msg-1',
      actionId: 'action-1',
    });
    mockClientMethods.sendGroupAudio.mockResolvedValue({
      messageId: 'msg-2',
      actionId: 'action-2',
    });
    mockClientMethods.sendGroupImage.mockResolvedValue({
      messageId: 'msg-3',
      actionId: 'action-3',
    });
    mockClientMethods.sendGroupPoll.mockResolvedValue({
      messageId: 'msg-4',
      actionId: 'action-4',
    });
    mockClientMethods.updateGroupSubject.mockResolvedValue({ success: true });
    mockClientMethods.updateGroupDescription.mockResolvedValue({ success: true });
    mockClientMethods.updateGroupPicture.mockResolvedValue({ success: true });
    mockClientMethods.updateGroupSettings.mockResolvedValue({ success: true });
    mockClientMethods.getGroupInviteCode.mockResolvedValue({
      inviteCode: 'INVITE123',
      inviteUrl: 'https://chat.whatsapp.com/INVITE123',
    });
    mockClientMethods.getGroupParticipants.mockResolvedValue({
      participants: [
        { id: 'p1@s.whatsapp.net', isAdmin: true, isSuperAdmin: false },
        { id: 'p2@s.whatsapp.net', isAdmin: false, isSuperAdmin: false },
      ],
    });
    mockClientMethods.addGroupMember.mockResolvedValue({
      addedCount: 2,
      failedNumbers: [],
    });
    mockClientMethods.removeGroupMember.mockResolvedValue({
      removedCount: 1,
      failedNumbers: ['5511999999999'],
    });
    mockClientMethods.leaveGroup.mockResolvedValue({ success: true });
    mockClientMethods.promoteMember.mockResolvedValue({ success: true });
    mockClientMethods.demoteMember.mockResolvedValue({ success: true });
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  // ============================================================
  // 1. Initial State / Query Tests
  // ============================================================

  describe('initial state and list groups query', () => {
    it('should return empty groups array by default', () => {
      const { result } = renderHook(() => useGroupsConnect());

      expect(result.current.groups).toEqual([]);
    });

    it('should return loading=false when query is not loading', () => {
      const { result } = renderHook(() => useGroupsConnect());

      expect(result.current.loading).toBe(false);
    });

    it('should return loading=true when query is loading', () => {
      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: true,
        refetch: vi.fn(),
      }));

      const { result } = renderHook(() => useGroupsConnect());

      expect(result.current.loading).toBe(true);
    });

    it('should call useQuery with correct query key containing user id', () => {
      renderHook(() => useGroupsConnect());

      expect(mockUseQuery).toHaveBeenCalledWith(
        expect.objectContaining({
          queryKey: ['groups', 'list', 'user-123'],
          enabled: true,
          staleTime: 30000,
          refetchInterval: 60000,
        })
      );
    });

    it('should map proto groups to local groups from query data', () => {
      mockUseQuery.mockImplementation(() => ({
        data: [
          {
            id: 'group-1',
            user_id: 'user-123',
            instance_id: 'instance-1',
            wa_group_id: '120363123456789@g.us',
            name: 'Test Group',
            description: 'Test description',
            members_count: 100,
            member_limit: 256,
            status: 'open',
            invite_code: 'ABC123',
            is_admin: true,
            is_favorite: false,
            created_at: '2026-01-01T00:00:00Z',
            updated_at: '2026-01-01T00:00:00Z',
          },
        ],
        isLoading: false,
        refetch: vi.fn(),
      }));

      const { result } = renderHook(() => useGroupsConnect());

      expect(result.current.groups).toHaveLength(1);
      expect(result.current.groups[0].id).toBe('group-1');
      expect(result.current.groups[0].name).toBe('Test Group');
    });

    it('should provide a refetch function', async () => {
      const mockRefetch = vi.fn().mockResolvedValue(undefined);
      mockUseQuery.mockImplementation(() => ({
        data: [],
        isLoading: false,
        refetch: mockRefetch,
      }));

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.refetch();
      });

      expect(mockRefetch).toHaveBeenCalled();
    });

    it('should set up correct number of mutations (23 total)', () => {
      renderHook(() => useGroupsConnect());

      // 20 original + 5 member management = 25
      expect(mockUseMutation).toHaveBeenCalledTimes(25);
    });
  });

  // ============================================================
  // 2. Converter Functions Tests
  // ============================================================

  describe('converter functions (toProtoStatus, fromProtoStatus, fromProtoGroup)', () => {
    it('should convert proto groups to local groups via listGroups mutation', async () => {
      const protoGroup = createMockProtoGroup({
        status: GroupStatusEnum.CLOSED,
        isFavorite: true,
      });

      mockClientMethods.listGroups.mockResolvedValue({ groups: [protoGroup] });

      const { result } = renderHook(() => useGroupsConnect());

      const groups = await act(async () => {
        return result.current.listGroups(true);
      });

      expect(groups).toHaveLength(1);
      expect(groups[0].status).toBe('closed');
      expect(groups[0].is_favorite).toBe(true);
      expect(groups[0].wa_group_id).toBe('120363123456789@g.us');
      expect(groups[0].user_id).toBe('user-123');
    });

    it('should convert open status correctly', async () => {
      mockClientMethods.listGroups.mockResolvedValue({
        groups: [createMockProtoGroup({ status: GroupStatusEnum.OPEN })],
      });

      const { result } = renderHook(() => useGroupsConnect());
      const groups = await act(async () => result.current.listGroups());

      expect(groups[0].status).toBe('open');
    });

    it('should convert closed status correctly', async () => {
      mockClientMethods.listGroups.mockResolvedValue({
        groups: [createMockProtoGroup({ status: GroupStatusEnum.CLOSED })],
      });

      const { result } = renderHook(() => useGroupsConnect());
      const groups = await act(async () => result.current.listGroups());

      expect(groups[0].status).toBe('closed');
    });

    it('should convert full status correctly', async () => {
      mockClientMethods.listGroups.mockResolvedValue({
        groups: [createMockProtoGroup({ status: GroupStatusEnum.FULL })],
      });

      const { result } = renderHook(() => useGroupsConnect());
      const groups = await act(async () => result.current.listGroups());

      expect(groups[0].status).toBe('full');
    });

    it('should default UNSPECIFIED status to open', async () => {
      mockClientMethods.listGroups.mockResolvedValue({
        groups: [createMockProtoGroup({ status: GroupStatusEnum.UNSPECIFIED })],
      });

      const { result } = renderHook(() => useGroupsConnect());
      const groups = await act(async () => result.current.listGroups());

      expect(groups[0].status).toBe('open');
    });

    it('should convert all proto group fields to local group fields', async () => {
      const protoGroup = createMockProtoGroup({
        id: 'grp-abc',
        userId: 'usr-xyz',
        instanceId: 'inst-001',
        waGroupId: '999@g.us',
        name: 'Proto Group',
        description: 'Proto desc',
        membersCount: 42,
        memberLimit: 100,
        status: GroupStatusEnum.FULL,
        inviteCode: 'XYZ789',
        isAdmin: false,
        isFavorite: true,
        createdAt: '2026-06-15T12:00:00Z',
        updatedAt: '2026-06-16T12:00:00Z',
      });

      mockClientMethods.getGroup.mockResolvedValue({ group: protoGroup });

      const { result } = renderHook(() => useGroupsConnect());
      const group = await act(async () => result.current.getGroup('grp-abc'));

      expect(group).toEqual({
        id: 'grp-abc',
        user_id: 'usr-xyz',
        instance_id: 'inst-001',
        wa_group_id: '999@g.us',
        name: 'Proto Group',
        description: 'Proto desc',
        members_count: 42,
        member_limit: 100,
        status: 'full',
        invite_code: 'XYZ789',
        is_admin: false,
        is_favorite: true,
        created_at: '2026-06-15T12:00:00Z',
        updated_at: '2026-06-16T12:00:00Z',
      });
    });
  });

  // ============================================================
  // 3. ListGroups Mutation Tests
  // ============================================================

  describe('listGroups', () => {
    it('should call client.listGroups and return mapped groups', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const groups = await act(async () => result.current.listGroups());

      expect(mockClientMethods.listGroups).toHaveBeenCalled();
      expect(groups).toHaveLength(1);
      expect(groups[0].id).toBe('group-1');
    });

    it('should pass adminOnly parameter', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.listGroups(true);
      });

      expect(mockClientMethods.listGroups).toHaveBeenCalledWith(
        expect.objectContaining({ adminOnly: true })
      );
    });

    it('should default adminOnly to false', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.listGroups();
      });

      expect(mockClientMethods.listGroups).toHaveBeenCalledWith(
        expect.objectContaining({ adminOnly: false })
      );
    });

    it('should show toast on error', async () => {
      mockClientMethods.listGroups.mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.listGroups();
        } catch {
          // Expected
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
          description: 'Network error',
        })
      );
    });
  });

  // ============================================================
  // 4. GetGroup Tests
  // ============================================================

  describe('getGroup', () => {
    it('should call client.getGroup and return mapped group', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const group = await act(async () => result.current.getGroup('group-1'));

      expect(mockClientMethods.getGroup).toHaveBeenCalledWith(
        expect.objectContaining({ id: 'group-1' })
      );
      expect(group.id).toBe('group-1');
      expect(group.name).toBe('Test Group');
    });

    it('should throw when group is not found (null response)', async () => {
      mockClientMethods.getGroup.mockResolvedValue({ group: null });

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.getGroup('nonexistent');
          expect.fail('Should have thrown');
        } catch (err) {
          expect((err as Error).message).toBe('pages.groups.errors.notFound');
        }
      });
    });

    it('should show error toast on failure', async () => {
      mockClientMethods.getGroup.mockRejectedValue(new Error('Not found'));

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.getGroup('group-1');
        } catch {
          // Expected
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
        })
      );
    });
  });

  // ============================================================
  // 5. GetGroupsByIds Tests
  // ============================================================

  describe('getGroupsByIds', () => {
    it('should call client.getGroupsByIds and return mapped groups', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const groups = await act(async () => result.current.getGroupsByIds(['group-1', 'group-2']));

      expect(mockClientMethods.getGroupsByIds).toHaveBeenCalledWith(
        expect.objectContaining({ ids: ['group-1', 'group-2'] })
      );
      expect(groups).toHaveLength(2);
    });

    it('should return empty array for empty ids', async () => {
      mockClientMethods.getGroupsByIds.mockResolvedValue({ groups: [] });

      const { result } = renderHook(() => useGroupsConnect());
      const groups = await act(async () => result.current.getGroupsByIds([]));

      expect(groups).toHaveLength(0);
    });

    it('should show error toast on failure', async () => {
      mockClientMethods.getGroupsByIds.mockRejectedValue(new Error('Batch fetch failed'));

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.getGroupsByIds(['g1']);
        } catch {
          // Expected
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
        })
      );
    });
  });

  // ============================================================
  // 6. CreateGroup Tests
  // ============================================================

  describe('createGroup', () => {
    it('should call client.createGroup with correct parameters', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const newGroup = await act(async () =>
        result.current.createGroup({
          wa_group_id: '999@g.us',
          name: 'New Group',
          description: 'New description',
          instance_id: 'inst-1',
          members_count: 50,
          member_limit: 200,
          status: 'open',
          invite_code: 'NEW123',
          is_admin: true,
        })
      );

      expect(mockClientMethods.createGroup).toHaveBeenCalledWith(
        expect.objectContaining({
          waGroupId: '999@g.us',
          name: 'New Group',
          description: 'New description',
          instanceId: 'inst-1',
          membersCount: 50,
          memberLimit: 200,
          status: GroupStatusEnum.OPEN,
          inviteCode: 'NEW123',
          isAdmin: true,
        })
      );
      expect(newGroup.id).toBe('new-group');
    });

    it('should convert local status to proto status', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.createGroup({
          wa_group_id: '999@g.us',
          name: 'Closed Group',
          status: 'closed',
        });
      });

      expect(mockClientMethods.createGroup).toHaveBeenCalledWith(
        expect.objectContaining({ status: GroupStatusEnum.CLOSED })
      );
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.createGroup({ wa_group_id: '999@g.us', name: 'New Group' });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should throw when response has no group', async () => {
      mockClientMethods.createGroup.mockResolvedValue({ group: null });

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.createGroup({ wa_group_id: '999@g.us', name: 'Fail Group' });
          expect.fail('Should have thrown');
        } catch (err) {
          expect((err as Error).message).toBe('pages.groups.errors.createFailed');
        }
      });
    });

    it('should show error toast on failure', async () => {
      mockClientMethods.createGroup.mockRejectedValue(new Error('Create failed'));

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.createGroup({ wa_group_id: '999@g.us', name: 'New Group' });
        } catch {
          // Expected
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
        })
      );
    });

    it('should handle createGroup with minimal parameters', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.createGroup({ wa_group_id: '999@g.us', name: 'Minimal Group' });
      });

      expect(mockClientMethods.createGroup).toHaveBeenCalledWith(
        expect.objectContaining({
          waGroupId: '999@g.us',
          name: 'Minimal Group',
          description: undefined,
          instanceId: undefined,
          membersCount: undefined,
          memberLimit: undefined,
          status: undefined,
          inviteCode: undefined,
          isAdmin: undefined,
        })
      );
    });
  });

  // ============================================================
  // 7. UpdateGroup Tests
  // ============================================================

  describe('updateGroup', () => {
    it('should call client.updateGroup with correct parameters', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const updated = await act(async () =>
        result.current.updateGroup({
          id: 'group-1',
          name: 'Updated Name',
          description: 'Updated desc',
          members_count: 120,
          member_limit: 300,
          status: 'full',
          invite_code: 'UPD456',
          is_admin: false,
          is_favorite: true,
        })
      );

      expect(mockClientMethods.updateGroup).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'group-1',
          name: 'Updated Name',
          description: 'Updated desc',
          membersCount: 120,
          memberLimit: 300,
          status: GroupStatusEnum.FULL,
          inviteCode: 'UPD456',
          isAdmin: false,
          isFavorite: true,
        })
      );
      expect(updated.name).toBe('Updated Group');
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.updateGroup({ id: 'group-1', name: 'New' });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should throw when response has no group', async () => {
      mockClientMethods.updateGroup.mockResolvedValue({ group: null });

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.updateGroup({ id: 'group-1' });
          expect.fail('Should have thrown');
        } catch (err) {
          expect((err as Error).message).toBe('pages.groups.errors.updateFailed');
        }
      });
    });

    it('should handle updateGroup with only id', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.updateGroup({ id: 'group-1' });
      });

      expect(mockClientMethods.updateGroup).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'group-1',
          name: undefined,
          description: undefined,
          status: undefined,
        })
      );
    });
  });

  // ============================================================
  // 8. DeleteGroup Tests
  // ============================================================

  describe('deleteGroup', () => {
    it('should call client.deleteGroup with correct id', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.deleteGroup('group-1');
      });

      expect(mockClientMethods.deleteGroup).toHaveBeenCalledWith(
        expect.objectContaining({ id: 'group-1' })
      );
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.deleteGroup('group-1');
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should show error toast on failure', async () => {
      mockClientMethods.deleteGroup.mockRejectedValue(new Error('Delete failed'));

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.deleteGroup('group-1');
        } catch {
          // Expected
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
          description: 'Delete failed',
        })
      );
    });
  });

  // ============================================================
  // 9. BulkUpdateGroups Tests
  // ============================================================

  describe('bulkUpdateGroups', () => {
    it('should call client.bulkUpdateGroups and return updated count', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const count = await act(async () =>
        result.current.bulkUpdateGroups({
          ids: ['g1', 'g2', 'g3'],
          status: 'closed',
          name: 'Bulk Name',
          is_favorite: true,
        })
      );

      expect(mockClientMethods.bulkUpdateGroups).toHaveBeenCalledWith(
        expect.objectContaining({
          ids: ['g1', 'g2', 'g3'],
          status: GroupStatusEnum.CLOSED,
          name: 'Bulk Name',
          isFavorite: true,
        })
      );
      expect(count).toBe(3);
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.bulkUpdateGroups({ ids: ['g1'], status: 'open' });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should handle bulk update without optional parameters', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.bulkUpdateGroups({ ids: ['g1'] });
      });

      expect(mockClientMethods.bulkUpdateGroups).toHaveBeenCalledWith(
        expect.objectContaining({
          ids: ['g1'],
          status: undefined,
          name: undefined,
          isFavorite: undefined,
        })
      );
    });

    it('should handle bulkUpdateGroups with full status conversion', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.bulkUpdateGroups({ ids: ['g1', 'g2'], status: 'full' });
      });

      expect(mockClientMethods.bulkUpdateGroups).toHaveBeenCalledWith(
        expect.objectContaining({ status: GroupStatusEnum.FULL })
      );
    });
  });

  // ============================================================
  // 10. ToggleGroupFavorite Tests
  // ============================================================

  describe('toggleGroupFavorite', () => {
    it('should call client.toggleGroupFavorite with correct parameters', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const group = await act(async () => result.current.toggleGroupFavorite('group-1', true));

      expect(mockClientMethods.toggleGroupFavorite).toHaveBeenCalledWith(
        expect.objectContaining({ id: 'group-1', isFavorite: true })
      );
      expect(group.is_favorite).toBe(true);
    });

    it('should toggle favorite to false', async () => {
      mockClientMethods.toggleGroupFavorite.mockResolvedValue({
        group: createMockProtoGroup({ isFavorite: false }),
      });

      const { result } = renderHook(() => useGroupsConnect());

      const group = await act(async () => result.current.toggleGroupFavorite('group-1', false));

      expect(group.is_favorite).toBe(false);
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.toggleGroupFavorite('group-1', true);
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should throw when response has no group', async () => {
      mockClientMethods.toggleGroupFavorite.mockResolvedValue({ group: null });

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.toggleGroupFavorite('group-1', true);
          expect.fail('Should have thrown');
        } catch (err) {
          expect((err as Error).message).toBe('pages.groups.errors.updateFailed');
        }
      });
    });
  });

  // ============================================================
  // 11. SyncGroups Tests
  // ============================================================

  describe('syncGroups', () => {
    it('should call client.syncGroups and return result', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const syncResult = await act(async () => result.current.syncGroups('my-instance'));

      expect(mockClientMethods.syncGroups).toHaveBeenCalledWith(
        expect.objectContaining({ instanceName: 'my-instance' })
      );
      expect(syncResult).toEqual({
        success: true,
        job_id: 'job-123',
        message: 'Sync started',
      });
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.syncGroups('my-instance');
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should show error toast on failure', async () => {
      mockClientMethods.syncGroups.mockRejectedValue(new Error('Sync failed'));

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.syncGroups('my-instance');
        } catch {
          // Expected
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
        })
      );
    });
  });

  // ============================================================
  // 12. GetSyncStatus Tests
  // ============================================================

  describe('getSyncStatus', () => {
    it('should call client.getSyncStatus and return mapped result', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const statusResult = await act(async () => result.current.getSyncStatus('job-123'));

      expect(mockClientMethods.getSyncStatus).toHaveBeenCalledWith(
        expect.objectContaining({ jobId: 'job-123' })
      );
      expect(statusResult).toEqual({
        status: 'completed',
        progress: 100,
        error: '',
        groups_synced: 10,
        total_groups: 10,
      });
    });

    it('should handle in-progress sync status', async () => {
      mockClientMethods.getSyncStatus.mockResolvedValue({
        status: 'in_progress',
        progress: 45,
        error: '',
        groupsSynced: 5,
        totalGroups: 11,
      });

      const { result } = renderHook(() => useGroupsConnect());

      const statusResult = await act(async () => result.current.getSyncStatus('job-456'));

      expect(statusResult).toEqual({
        status: 'in_progress',
        progress: 45,
        error: '',
        groups_synced: 5,
        total_groups: 11,
      });
    });

    it('should handle failed sync status with error', async () => {
      mockClientMethods.getSyncStatus.mockResolvedValue({
        status: 'failed',
        progress: 0,
        error: 'Connection timeout',
        groupsSynced: 0,
        totalGroups: 10,
      });

      const { result } = renderHook(() => useGroupsConnect());

      const statusResult = await act(async () => result.current.getSyncStatus('job-789'));

      expect(statusResult.error).toBe('Connection timeout');
      expect(statusResult.status).toBe('failed');
    });
  });

  // ============================================================
  // 13. SendGroupMessage Tests
  // ============================================================

  describe('sendGroupMessage', () => {
    it('should call client.sendGroupMessage with correct parameters', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const msgResult = await act(async () =>
        result.current.sendGroupMessage({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          message: 'Hello world',
          mentions: ['user1@s.whatsapp.net'],
          images: ['image1.jpg'],
          reply_to: 'msg-0',
        })
      );

      expect(mockClientMethods.sendGroupMessage).toHaveBeenCalledWith(
        expect.objectContaining({
          instanceName: 'my-instance',
          groupId: '123@g.us',
          message: 'Hello world',
          mentions: ['user1@s.whatsapp.net'],
          images: ['image1.jpg'],
          replyTo: 'msg-0',
        })
      );
      expect(msgResult).toEqual({ message_id: 'msg-1', action_id: 'action-1' });
    });

    it('should default mentions and images to empty arrays', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.sendGroupMessage({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          message: 'Hello',
        });
      });

      expect(mockClientMethods.sendGroupMessage).toHaveBeenCalledWith(
        expect.objectContaining({ mentions: [], images: [], replyTo: undefined })
      );
    });

    it('should show error toast on failure', async () => {
      mockClientMethods.sendGroupMessage.mockRejectedValue(new Error('Send failed'));

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.sendGroupMessage({
            instance_name: 'inst',
            group_id: 'g1',
            message: 'test',
          });
        } catch {
          // Expected
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive', title: 'common.error' })
      );
    });
  });

  // ============================================================
  // 14. SendGroupAudio Tests
  // ============================================================

  describe('sendGroupAudio', () => {
    it('should call client.sendGroupAudio with correct parameters', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const audioResult = await act(async () =>
        result.current.sendGroupAudio({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          audio: 'base64audiodata',
          mimetype: 'audio/ogg',
          ptt: true,
          caption: 'Voice message',
        })
      );

      expect(mockClientMethods.sendGroupAudio).toHaveBeenCalledWith(
        expect.objectContaining({
          instanceName: 'my-instance',
          groupId: '123@g.us',
          audio: 'base64audiodata',
          mimetype: 'audio/ogg',
          ptt: true,
          caption: 'Voice message',
        })
      );
      expect(audioResult).toEqual({ message_id: 'msg-2', action_id: 'action-2' });
    });

    it('should handle audio without optional fields', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.sendGroupAudio({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          audio: 'base64audiodata',
          ptt: false,
        });
      });

      expect(mockClientMethods.sendGroupAudio).toHaveBeenCalledWith(
        expect.objectContaining({ ptt: false, mimetype: undefined, caption: undefined })
      );
    });
  });

  // ============================================================
  // 15. SendGroupImage Tests
  // ============================================================

  describe('sendGroupImage', () => {
    it('should call client.sendGroupImage with correct parameters', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const imgResult = await act(async () =>
        result.current.sendGroupImage({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          image: 'base64imagedata',
          mimetype: 'image/jpeg',
          caption: 'Photo caption',
          reply_to: 'msg-prev',
        })
      );

      expect(mockClientMethods.sendGroupImage).toHaveBeenCalledWith(
        expect.objectContaining({
          instanceName: 'my-instance',
          groupId: '123@g.us',
          image: 'base64imagedata',
          mimetype: 'image/jpeg',
          caption: 'Photo caption',
          replyTo: 'msg-prev',
        })
      );
      expect(imgResult).toEqual({ message_id: 'msg-3', action_id: 'action-3' });
    });

    it('should handle image without optional fields', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.sendGroupImage({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          image: 'base64imagedata',
        });
      });

      expect(mockClientMethods.sendGroupImage).toHaveBeenCalledWith(
        expect.objectContaining({ mimetype: undefined, caption: undefined, replyTo: undefined })
      );
    });
  });

  // ============================================================
  // 16. SendGroupPoll Tests
  // ============================================================

  describe('sendGroupPoll', () => {
    it('should call client.sendGroupPoll with correct parameters', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const pollResult = await act(async () =>
        result.current.sendGroupPoll({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          name: 'Favorite color?',
          options: ['Red', 'Blue', 'Green'],
          multiple_answers: true,
        })
      );

      expect(mockClientMethods.sendGroupPoll).toHaveBeenCalledWith(
        expect.objectContaining({
          instanceName: 'my-instance',
          groupId: '123@g.us',
          name: 'Favorite color?',
          options: ['Red', 'Blue', 'Green'],
          multipleAnswers: true,
        })
      );
      expect(pollResult).toEqual({ message_id: 'msg-4', action_id: 'action-4' });
    });

    it('should default multipleAnswers to false', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.sendGroupPoll({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          name: 'Poll',
          options: ['Yes', 'No'],
        });
      });

      expect(mockClientMethods.sendGroupPoll).toHaveBeenCalledWith(
        expect.objectContaining({ multipleAnswers: false })
      );
    });
  });

  // ============================================================
  // 17. UpdateGroupSubject Tests
  // ============================================================

  describe('updateGroupSubject', () => {
    it('should call client.updateGroupSubject and return success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const success = await act(async () =>
        result.current.updateGroupSubject({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          subject: 'New Subject',
        })
      );

      expect(mockClientMethods.updateGroupSubject).toHaveBeenCalledWith(
        expect.objectContaining({
          instanceName: 'my-instance',
          groupId: '123@g.us',
          subject: 'New Subject',
        })
      );
      expect(success).toBe(true);
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.updateGroupSubject({
          instance_name: 'inst',
          group_id: 'gid',
          subject: 'Sub',
        });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });
  });

  // ============================================================
  // 18. UpdateGroupDescription Tests
  // ============================================================

  describe('updateGroupDescription', () => {
    it('should call client.updateGroupDescription and return success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const success = await act(async () =>
        result.current.updateGroupDescription({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          description: 'New Description',
        })
      );

      expect(mockClientMethods.updateGroupDescription).toHaveBeenCalledWith(
        expect.objectContaining({
          instanceName: 'my-instance',
          groupId: '123@g.us',
          description: 'New Description',
        })
      );
      expect(success).toBe(true);
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.updateGroupDescription({
          instance_name: 'inst',
          group_id: 'gid',
          description: 'Desc',
        });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });
  });

  // ============================================================
  // 19. UpdateGroupPicture Tests
  // ============================================================

  describe('updateGroupPicture', () => {
    it('should call client.updateGroupPicture with base64 data', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const success = await act(async () =>
        result.current.updateGroupPicture({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          base64_data: 'base64imgdata',
          mimetype: 'image/png',
        })
      );

      expect(mockClientMethods.updateGroupPicture).toHaveBeenCalledWith(
        expect.objectContaining({
          instanceName: 'my-instance',
          groupId: '123@g.us',
          picture: { case: 'base64Data', value: 'base64imgdata' },
          mimetype: 'image/png',
        })
      );
      expect(success).toBe(true);
    });

    it('should call client.updateGroupPicture with url', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.updateGroupPicture({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          url: 'https://example.com/photo.jpg',
        });
      });

      expect(mockClientMethods.updateGroupPicture).toHaveBeenCalledWith(
        expect.objectContaining({
          picture: { case: 'url', value: 'https://example.com/photo.jpg' },
        })
      );
    });

    it('should handle picture without url or base64', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.updateGroupPicture({
          instance_name: 'my-instance',
          group_id: '123@g.us',
        });
      });

      expect(mockClientMethods.updateGroupPicture).toHaveBeenCalledWith(
        expect.objectContaining({
          picture: { case: undefined, value: undefined },
        })
      );
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.updateGroupPicture({
          instance_name: 'inst',
          group_id: 'gid',
          url: 'https://pic.jpg',
        });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });
  });

  // ============================================================
  // 20. UpdateGroupSettings Tests
  // ============================================================

  describe('updateGroupSettings', () => {
    it('should call client.updateGroupSettings with correct parameters', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const success = await act(async () =>
        result.current.updateGroupSettings({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          messages_admin_only: true,
        })
      );

      expect(mockClientMethods.updateGroupSettings).toHaveBeenCalledWith(
        expect.objectContaining({
          instanceName: 'my-instance',
          groupId: '123@g.us',
          messagesAdminOnly: true,
        })
      );
      expect(success).toBe(true);
    });

    it('should handle messages_admin_only set to false', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.updateGroupSettings({
          instance_name: 'inst',
          group_id: 'gid',
          messages_admin_only: false,
        });
      });

      expect(mockClientMethods.updateGroupSettings).toHaveBeenCalledWith(
        expect.objectContaining({ messagesAdminOnly: false })
      );
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.updateGroupSettings({
          instance_name: 'inst',
          group_id: 'gid',
          messages_admin_only: true,
        });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });
  });

  // ============================================================
  // 21. GetGroupInviteCode Tests
  // ============================================================

  describe('getGroupInviteCode', () => {
    it('should call client.getGroupInviteCode and return result', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const inviteResult = await act(async () =>
        result.current.getGroupInviteCode('my-instance', '123@g.us')
      );

      expect(mockClientMethods.getGroupInviteCode).toHaveBeenCalledWith(
        expect.objectContaining({ instanceName: 'my-instance', groupId: '123@g.us' })
      );
      expect(inviteResult).toEqual({
        invite_code: 'INVITE123',
        invite_url: 'https://chat.whatsapp.com/INVITE123',
      });
    });

    it('should show error toast on failure', async () => {
      mockClientMethods.getGroupInviteCode.mockRejectedValue(new Error('Forbidden'));

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.getGroupInviteCode('inst', 'gid');
        } catch {
          // Expected
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
          description: 'Forbidden',
        })
      );
    });
  });

  // ============================================================
  // 22. GetGroupParticipants Tests
  // ============================================================

  describe('getGroupParticipants', () => {
    it('should call client.getGroupParticipants and return mapped participants', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const participants = await act(async () =>
        result.current.getGroupParticipants({
          instance_name: 'my-instance',
          group_id: '123@g.us',
        })
      );

      expect(mockClientMethods.getGroupParticipants).toHaveBeenCalledWith(
        expect.objectContaining({ instanceName: 'my-instance', groupId: '123@g.us' })
      );
      expect(participants).toEqual([
        { id: 'p1@s.whatsapp.net', is_admin: true, is_super_admin: false },
        { id: 'p2@s.whatsapp.net', is_admin: false, is_super_admin: false },
      ]);
    });

    it('should handle empty participants list', async () => {
      mockClientMethods.getGroupParticipants.mockResolvedValue({ participants: [] });

      const { result } = renderHook(() => useGroupsConnect());

      const participants = await act(async () =>
        result.current.getGroupParticipants({
          instance_name: 'my-instance',
          group_id: '123@g.us',
        })
      );

      expect(participants).toEqual([]);
    });

    it('should show error toast on failure', async () => {
      mockClientMethods.getGroupParticipants.mockRejectedValue(new Error('Permission denied'));

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.getGroupParticipants({
            instance_name: 'inst',
            group_id: 'gid',
          });
        } catch {
          // Expected
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
          description: 'Permission denied',
        })
      );
    });
  });

  // ============================================================
  // 23. AddGroupMember Tests
  // ============================================================

  describe('addGroupMember', () => {
    it('should call client.addGroupMember with correct parameters', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const addResult = await act(async () =>
        result.current.addGroupMember({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          phone_numbers: ['5511999999999', '5511888888888'],
        })
      );

      expect(mockClientMethods.addGroupMember).toHaveBeenCalledWith(
        expect.objectContaining({
          instanceName: 'my-instance',
          groupId: '123@g.us',
          phoneNumbers: ['5511999999999', '5511888888888'],
        })
      );
      expect(addResult).toEqual({ added_count: 2, failed_numbers: [] });
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.addGroupMember({
          instance_name: 'inst',
          group_id: 'gid',
          phone_numbers: ['5511999999999'],
        });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should show error toast on failure', async () => {
      mockClientMethods.addGroupMember.mockRejectedValue(new Error('Add member failed'));

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.addGroupMember({
            instance_name: 'inst',
            group_id: 'gid',
            phone_numbers: ['5511999999999'],
          });
        } catch {
          // Expected
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
          description: 'Add member failed',
        })
      );
    });
  });

  // ============================================================
  // 24. RemoveGroupMember Tests
  // ============================================================

  describe('removeGroupMember', () => {
    it('should call client.removeGroupMember with correct parameters', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const removeResult = await act(async () =>
        result.current.removeGroupMember({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          phone_numbers: ['5511999999999'],
        })
      );

      expect(mockClientMethods.removeGroupMember).toHaveBeenCalledWith(
        expect.objectContaining({
          instanceName: 'my-instance',
          groupId: '123@g.us',
          phoneNumbers: ['5511999999999'],
        })
      );
      expect(removeResult).toEqual({
        removed_count: 1,
        failed_numbers: ['5511999999999'],
      });
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.removeGroupMember({
          instance_name: 'inst',
          group_id: 'gid',
          phone_numbers: ['5511999999999'],
        });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });
  });

  // ============================================================
  // 25. LeaveGroup Tests
  // ============================================================

  describe('leaveGroup', () => {
    it('should call client.leaveGroup and return success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const success = await act(async () =>
        result.current.leaveGroup({
          instance_name: 'my-instance',
          group_id: '123@g.us',
        })
      );

      expect(mockClientMethods.leaveGroup).toHaveBeenCalledWith(
        expect.objectContaining({
          instanceName: 'my-instance',
          groupId: '123@g.us',
        })
      );
      expect(success).toBe(true);
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.leaveGroup({
          instance_name: 'inst',
          group_id: 'gid',
        });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should show error toast on failure', async () => {
      mockClientMethods.leaveGroup.mockRejectedValue(new Error('Leave failed'));

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.leaveGroup({
            instance_name: 'inst',
            group_id: 'gid',
          });
        } catch {
          // Expected
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
          description: 'Leave failed',
        })
      );
    });
  });

  // ============================================================
  // 26. PromoteMember Tests
  // ============================================================

  describe('promoteMember', () => {
    it('should call client.promoteMember with correct parameters', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const success = await act(async () =>
        result.current.promoteMember({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          phone_number: '5511999999999',
        })
      );

      expect(mockClientMethods.promoteMember).toHaveBeenCalledWith(
        expect.objectContaining({
          instanceName: 'my-instance',
          groupId: '123@g.us',
          phoneNumber: '5511999999999',
        })
      );
      expect(success).toBe(true);
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.promoteMember({
          instance_name: 'inst',
          group_id: 'gid',
          phone_number: '5511999999999',
        });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should show error toast on failure', async () => {
      mockClientMethods.promoteMember.mockRejectedValue(new Error('Promote failed'));

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.promoteMember({
            instance_name: 'inst',
            group_id: 'gid',
            phone_number: '5511999999999',
          });
        } catch {
          // Expected
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
          description: 'Promote failed',
        })
      );
    });
  });

  // ============================================================
  // 27. DemoteMember Tests
  // ============================================================

  describe('demoteMember', () => {
    it('should call client.demoteMember with correct parameters', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      const success = await act(async () =>
        result.current.demoteMember({
          instance_name: 'my-instance',
          group_id: '123@g.us',
          phone_number: '5511999999999',
        })
      );

      expect(mockClientMethods.demoteMember).toHaveBeenCalledWith(
        expect.objectContaining({
          instanceName: 'my-instance',
          groupId: '123@g.us',
          phoneNumber: '5511999999999',
        })
      );
      expect(success).toBe(true);
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.demoteMember({
          instance_name: 'inst',
          group_id: 'gid',
          phone_number: '5511999999999',
        });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should show error toast on failure', async () => {
      mockClientMethods.demoteMember.mockRejectedValue(new Error('Demote failed'));

      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        try {
          await result.current.demoteMember({
            instance_name: 'inst',
            group_id: 'gid',
            phone_number: '5511999999999',
          });
        } catch {
          // Expected
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
          description: 'Demote failed',
        })
      );
    });
  });

  // ============================================================
  // 28. Loading States Tests
  // ============================================================

  describe('loading states (isPending flags)', () => {
    it('should expose all CRUD loading states as false by default', () => {
      const { result } = renderHook(() => useGroupsConnect());

      expect(result.current.isListingGroups).toBe(false);
      expect(result.current.isGettingGroup).toBe(false);
      expect(result.current.isCreatingGroup).toBe(false);
      expect(result.current.isUpdatingGroup).toBe(false);
      expect(result.current.isTogglingFavorite).toBe(false);
      expect(result.current.isSyncingGroups).toBe(false);
      expect(result.current.isGettingSyncStatus).toBe(false);
      expect(result.current.isDeletingGroup).toBe(false);
      expect(result.current.isBulkUpdating).toBe(false);
    });

    it('should expose all messaging loading states as false by default', () => {
      const { result } = renderHook(() => useGroupsConnect());

      expect(result.current.isSendingMessage).toBe(false);
      expect(result.current.isSendingAudio).toBe(false);
      expect(result.current.isSendingImage).toBe(false);
      expect(result.current.isSendingPoll).toBe(false);
    });

    it('should expose all settings loading states as false by default', () => {
      const { result } = renderHook(() => useGroupsConnect());

      expect(result.current.isUpdatingSubject).toBe(false);
      expect(result.current.isUpdatingDescription).toBe(false);
      expect(result.current.isUpdatingPicture).toBe(false);
      expect(result.current.isUpdatingSettings).toBe(false);
    });

    it('should expose utility and member management loading states as false by default', () => {
      const { result } = renderHook(() => useGroupsConnect());

      expect(result.current.isGettingInviteCode).toBe(false);
      expect(result.current.isAddingMember).toBe(false);
      expect(result.current.isRemovingMember).toBe(false);
      expect(result.current.isLeavingGroup).toBe(false);
      expect(result.current.isPromotingMember).toBe(false);
      expect(result.current.isDemotingMember).toBe(false);
    });

    it('should reflect isPending=true for specific mutation (listGroups)', () => {
      let callIdx = 0;
      mockUseMutation.mockImplementation((options: MutationOptions) => {
        const idx = callIdx++;
        return {
          mutateAsync: async (variables: unknown) => options.mutationFn(variables),
          isPending: idx === 0, // listGroupsMutation is first
        };
      });

      const { result } = renderHook(() => useGroupsConnect());

      expect(result.current.isListingGroups).toBe(true);
      expect(result.current.isGettingGroup).toBe(false);
    });

    it('should reflect isPending=true for createGroup mutation', () => {
      let callIdx = 0;
      mockUseMutation.mockImplementation((options: MutationOptions) => {
        const idx = callIdx++;
        return {
          mutateAsync: async (variables: unknown) => options.mutationFn(variables),
          isPending: idx === 3, // createGroupMutation is 4th (index 3)
        };
      });

      const { result } = renderHook(() => useGroupsConnect());

      expect(result.current.isCreatingGroup).toBe(true);
      expect(result.current.isListingGroups).toBe(false);
    });

    it('should reflect isPending=true for sendMessage mutation', () => {
      let callIdx = 0;
      mockUseMutation.mockImplementation((options: MutationOptions) => {
        const idx = callIdx++;
        return {
          mutateAsync: async (variables: unknown) => options.mutationFn(variables),
          // sendGroupMessage is the 11th mutation (index 10)
          isPending: idx === 10,
        };
      });

      const { result } = renderHook(() => useGroupsConnect());

      expect(result.current.isSendingMessage).toBe(true);
      expect(result.current.isSendingAudio).toBe(false);
    });

    it('should reflect isPending=true for addMember mutation', () => {
      let callIdx = 0;
      mockUseMutation.mockImplementation((options: MutationOptions) => {
        const idx = callIdx++;
        return {
          mutateAsync: async (variables: unknown) => options.mutationFn(variables),
          // addGroupMember is the 21st mutation (index 20)
          isPending: idx === 20,
        };
      });

      const { result } = renderHook(() => useGroupsConnect());

      expect(result.current.isAddingMember).toBe(true);
      expect(result.current.isRemovingMember).toBe(false);
    });
  });

  // ============================================================
  // 29. Cache Invalidation Patterns Tests
  // ============================================================

  describe('cache invalidation patterns', () => {
    it('should invalidate groups cache after createGroup', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.createGroup({ wa_group_id: 'g@g.us', name: 'New' });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should invalidate groups cache after updateGroup', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.updateGroup({ id: 'group-1', name: 'Updated' });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should invalidate groups cache after deleteGroup', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.deleteGroup('group-1');
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should invalidate groups cache after bulkUpdateGroups', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.bulkUpdateGroups({ ids: ['g1'] });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should invalidate groups cache after toggleGroupFavorite', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.toggleGroupFavorite('group-1', true);
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should invalidate groups cache after syncGroups', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.syncGroups('inst');
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should invalidate groups cache after settings mutations', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.updateGroupSubject({
          instance_name: 'inst',
          group_id: 'gid',
          subject: 'Sub',
        });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should invalidate groups cache after member management mutations', async () => {
      const { result } = renderHook(() => useGroupsConnect());

      await act(async () => {
        await result.current.addGroupMember({
          instance_name: 'inst',
          group_id: 'gid',
          phone_numbers: ['5511999999999'],
        });
      });

      expect(mockQueryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['groups'],
      });
    });

    it('should NOT invalidate groups cache for messaging mutations', async () => {
      mockQueryClient.invalidateQueries.mockClear();

      // We need to track if invalidateQueries is called only by onSuccess handlers
      // Messaging mutations have no onSuccess handler with cache invalidation
      // But our default mock calls onSuccess for all mutations.
      // The sendGroupMessage mutation has no onSuccess, only onError
      // So we need a smarter mock for this test.

      mockUseMutation.mockImplementation((options: MutationOptions) => ({
        mutateAsync: async (variables: unknown) => {
          const result = await options.mutationFn(variables);
          // Only call onSuccess if it exists
          if (options.onSuccess) {
            options.onSuccess(result);
          }
          return result;
        },
        isPending: false,
      }));

      const { result } = renderHook(() => useGroupsConnect());

      mockQueryClient.invalidateQueries.mockClear();

      await act(async () => {
        await result.current.sendGroupMessage({
          instance_name: 'inst',
          group_id: 'gid',
          message: 'hello',
        });
      });

      // sendGroupMessage has NO onSuccess, so no cache invalidation
      expect(mockQueryClient.invalidateQueries).not.toHaveBeenCalled();
    });

    it('should NOT invalidate groups cache for getSyncStatus', async () => {
      mockUseMutation.mockImplementation((options: MutationOptions) => ({
        mutateAsync: async (variables: unknown) => {
          const result = await options.mutationFn(variables);
          if (options.onSuccess) {
            options.onSuccess(result);
          }
          return result;
        },
        isPending: false,
      }));

      const { result } = renderHook(() => useGroupsConnect());

      mockQueryClient.invalidateQueries.mockClear();

      await act(async () => {
        await result.current.getSyncStatus('job-1');
      });

      expect(mockQueryClient.invalidateQueries).not.toHaveBeenCalled();
    });

    it('should NOT invalidate groups cache for getGroupInviteCode', async () => {
      mockUseMutation.mockImplementation((options: MutationOptions) => ({
        mutateAsync: async (variables: unknown) => {
          const result = await options.mutationFn(variables);
          if (options.onSuccess) {
            options.onSuccess(result);
          }
          return result;
        },
        isPending: false,
      }));

      const { result } = renderHook(() => useGroupsConnect());

      mockQueryClient.invalidateQueries.mockClear();

      await act(async () => {
        await result.current.getGroupInviteCode('inst', 'gid');
      });

      expect(mockQueryClient.invalidateQueries).not.toHaveBeenCalled();
    });
  });

  // ============================================================
  // 30. Edge Cases & Return Shape Tests
  // ============================================================

  describe('edge cases', () => {
    it('should handle null groups response gracefully', () => {
      mockUseQuery.mockImplementation(() => ({
        data: null,
        isLoading: false,
        refetch: vi.fn(),
      }));

      const { result } = renderHook(() => useGroupsConnect());

      expect(result.current.groups).toEqual([]);
    });

    it('should handle undefined groups response gracefully', () => {
      mockUseQuery.mockImplementation(() => ({
        data: undefined,
        isLoading: false,
        refetch: vi.fn(),
      }));

      const { result } = renderHook(() => useGroupsConnect());

      expect(result.current.groups).toEqual([]);
    });

    it('should return all expected methods and properties', () => {
      const { result } = renderHook(() => useGroupsConnect());

      // CRUD functions
      expect(typeof result.current.listGroups).toBe('function');
      expect(typeof result.current.getGroup).toBe('function');
      expect(typeof result.current.getGroupsByIds).toBe('function');
      expect(typeof result.current.createGroup).toBe('function');
      expect(typeof result.current.updateGroup).toBe('function');
      expect(typeof result.current.deleteGroup).toBe('function');
      expect(typeof result.current.bulkUpdateGroups).toBe('function');
      expect(typeof result.current.toggleGroupFavorite).toBe('function');
      expect(typeof result.current.syncGroups).toBe('function');
      expect(typeof result.current.getSyncStatus).toBe('function');
      expect(typeof result.current.refetch).toBe('function');

      // Messaging functions
      expect(typeof result.current.sendGroupMessage).toBe('function');
      expect(typeof result.current.sendGroupAudio).toBe('function');
      expect(typeof result.current.sendGroupImage).toBe('function');
      expect(typeof result.current.sendGroupPoll).toBe('function');

      // Settings functions
      expect(typeof result.current.updateGroupSubject).toBe('function');
      expect(typeof result.current.updateGroupDescription).toBe('function');
      expect(typeof result.current.updateGroupPicture).toBe('function');
      expect(typeof result.current.updateGroupSettings).toBe('function');

      // Utilities functions
      expect(typeof result.current.getGroupInviteCode).toBe('function');
      expect(typeof result.current.getGroupParticipants).toBe('function');

      // Member management functions
      expect(typeof result.current.addGroupMember).toBe('function');
      expect(typeof result.current.removeGroupMember).toBe('function');
      expect(typeof result.current.leaveGroup).toBe('function');
      expect(typeof result.current.promoteMember).toBe('function');
      expect(typeof result.current.demoteMember).toBe('function');

      // Data
      expect(Array.isArray(result.current.groups)).toBe(true);
      expect(typeof result.current.loading).toBe('boolean');

      // All loading states
      expect(typeof result.current.isListingGroups).toBe('boolean');
      expect(typeof result.current.isGettingGroup).toBe('boolean');
      expect(typeof result.current.isCreatingGroup).toBe('boolean');
      expect(typeof result.current.isUpdatingGroup).toBe('boolean');
      expect(typeof result.current.isTogglingFavorite).toBe('boolean');
      expect(typeof result.current.isSyncingGroups).toBe('boolean');
      expect(typeof result.current.isGettingSyncStatus).toBe('boolean');
      expect(typeof result.current.isDeletingGroup).toBe('boolean');
      expect(typeof result.current.isBulkUpdating).toBe('boolean');
      expect(typeof result.current.isSendingMessage).toBe('boolean');
      expect(typeof result.current.isSendingAudio).toBe('boolean');
      expect(typeof result.current.isSendingImage).toBe('boolean');
      expect(typeof result.current.isSendingPoll).toBe('boolean');
      expect(typeof result.current.isUpdatingSubject).toBe('boolean');
      expect(typeof result.current.isUpdatingDescription).toBe('boolean');
      expect(typeof result.current.isUpdatingPicture).toBe('boolean');
      expect(typeof result.current.isUpdatingSettings).toBe('boolean');
      expect(typeof result.current.isGettingInviteCode).toBe('boolean');
      expect(typeof result.current.isAddingMember).toBe('boolean');
      expect(typeof result.current.isRemovingMember).toBe('boolean');
      expect(typeof result.current.isLeavingGroup).toBe('boolean');
      expect(typeof result.current.isPromotingMember).toBe('boolean');
      expect(typeof result.current.isDemotingMember).toBe('boolean');
    });
  });
});
