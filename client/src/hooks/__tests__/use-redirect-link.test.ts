import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useRedirectLink } from '../use-redirect-link';

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
    t: (key: string) => key,
    i18n: { language: 'pt-BR' },
  }),
}));

// ============================================================
// Mock: Auth
// ============================================================

const mockUser = { id: 'user-123', email: 'test@example.com' };

vi.mock('@/contexts', () => ({
  useAuth: () => ({
    user: mockUser,
    session: null,
    loading: false,
  }),
}));

// ============================================================
// Mock: ConnectRPC Query (string sentinels)
// ============================================================

vi.mock('@/gen/redirect/v1/redirect-RedirectLinkService_connectquery', () => ({
  listRedirectLinks: 'listRedirectLinks',
}));

vi.mock('@/gen/redirect/v1/redirect_pb', () => ({
  DistributionStrategy: {
    CLICK_LIMIT: 1,
    MEMBER_LIMIT: 2,
    ROUND_ROBIN: 3,
  },
  RedirectLinkService: 'RedirectLinkService',
  CheckSlugAvailabilityRequestSchema: 'CheckSlugAvailabilityRequestSchema',
  CreateRedirectLinkRequestSchema: 'CreateRedirectLinkRequestSchema',
  DeleteRedirectLinkRequestSchema: 'DeleteRedirectLinkRequestSchema',
  RedirectGroupSchema: 'RedirectGroupSchema',
  UpdateRedirectLinkRequestSchema: 'UpdateRedirectLinkRequestSchema',
}));

// ============================================================
// Mock: @bufbuild/protobuf
// ============================================================

vi.mock('@bufbuild/protobuf', () => ({
  create: (_schema: unknown, data: unknown) => data,
}));

// ============================================================
// Mock: @connectrpc/connect
// ============================================================

const mockCheckSlugAvailability = vi.fn();
const mockCreateRedirectLink = vi.fn();
const mockUpdateRedirectLink = vi.fn();
const mockDeleteRedirectLink = vi.fn();

vi.mock('@connectrpc/connect', () => ({
  createClient: () => ({
    checkSlugAvailability: mockCheckSlugAvailability,
    createRedirectLink: mockCreateRedirectLink,
    updateRedirectLink: mockUpdateRedirectLink,
    deleteRedirectLink: mockDeleteRedirectLink,
  }),
}));

// ============================================================
// Mock: @connectrpc/connect-query
// ============================================================

type UseQueryCall = {
  descriptor: string;
  input: Record<string, unknown>;
  options: { enabled: boolean };
};

const useQueryCalls: UseQueryCall[] = [];
const useQueryReturnMap = new Map<string, { data: unknown; isLoading: boolean }>();

function createQueryReturn(data: unknown = undefined, isLoading = false) {
  return { data, isLoading };
}

vi.mock('@connectrpc/connect-query', () => ({
  useQuery: (descriptor: string, input: unknown, options: unknown) => {
    useQueryCalls.push({
      descriptor,
      input: input as Record<string, unknown>,
      options: options as UseQueryCall['options'],
    });
    return useQueryReturnMap.get(descriptor) ?? createQueryReturn();
  },
  useTransport: () => 'mock-transport',
  createConnectQueryKey: () => ['mock-query-key'],
}));

// ============================================================
// Mock: @tanstack/react-query
// ============================================================

const mockMutate = vi.fn();
const mockInvalidateQueries = vi.fn();

vi.mock('@tanstack/react-query', () => ({
  useMutation: (opts: {
    mutationFn: (...args: unknown[]) => Promise<unknown>;
    onSuccess?: () => void;
    onError?: (err: unknown) => void;
  }) => ({
    mutate: (data: unknown, overrides?: { onSuccess?: () => void }) => {
      mockMutate(data);
      opts
        .mutationFn(data)
        .then(() => {
          opts.onSuccess?.();
          overrides?.onSuccess?.();
        })
        .catch((err: unknown) => {
          opts.onError?.(err);
        });
    },
    isPending: false,
  }),
  useQueryClient: () => ({
    invalidateQueries: mockInvalidateQueries,
  }),
}));

// ============================================================
// Mock: useGroupsConnect
// ============================================================

const mockGroups = [
  {
    id: 'g-1',
    name: 'Grupo A',
    wa_group_id: '111@g.us',
    invite_code: 'INV1',
    members_count: 100,
    member_limit: 200,
    is_favorite: true,
  },
  {
    id: 'g-2',
    name: 'Grupo B',
    wa_group_id: '222@g.us',
    invite_code: 'INV2',
    members_count: 50,
    member_limit: 200,
    is_favorite: false,
  },
  {
    id: 'g-3',
    name: 'Grupo C',
    wa_group_id: '333@g.us',
    invite_code: 'INV3',
    members_count: 150,
    member_limit: 200,
    is_favorite: false,
  },
];

vi.mock('@/hooks/use-groups-connect', () => ({
  useGroupsConnect: () => ({
    groups: mockGroups,
    loading: false,
    refetch: vi.fn(),
    listGroups: vi.fn(),
    listGroupsPaginated: vi.fn(),
    getGroup: vi.fn(),
    getGroupsByIds: vi.fn(),
    createGroup: vi.fn(),
    updateGroup: vi.fn(),
    toggleGroupFavorite: vi.fn(),
    syncGroups: vi.fn(),
    getSyncStatus: vi.fn(),
    deleteGroup: vi.fn(),
    bulkUpdateGroups: vi.fn(),
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
    isListingGroups: false,
    isGettingGroup: false,
    isCreatingGroup: false,
    isUpdatingGroup: false,
    isTogglingFavorite: false,
    isSyncingGroups: false,
    isGettingSyncStatus: false,
    isDeletingGroup: false,
    isBulkUpdating: false,
    isSendingMessage: false,
    isSendingAudio: false,
    isSendingImage: false,
    isSendingPoll: false,
    isUpdatingSubject: false,
    isUpdatingDescription: false,
    isUpdatingPicture: false,
    isUpdatingSettings: false,
    isGettingInviteCode: false,
    isGettingParticipants: false,
    isAddingMember: false,
    isRemovingMember: false,
    isLeavingGroup: false,
    isPromotingMember: false,
    isDemotingMember: false,
  }),
}));

// ============================================================
// Mock: useDebouncedValue (passthrough)
// ============================================================

vi.mock('@/hooks/use-debounced-value', () => ({
  useDebouncedValue: (value: string) => value,
}));

// ============================================================
// Mock: Sentry
// ============================================================

const mockCaptureHookError = vi.fn();
const mockSafeClipboard = vi.fn();

vi.mock('@/lib/sentry', () => ({
  captureHookError: (...args: unknown[]) => mockCaptureHookError(...args),
  safeClipboard: (...args: unknown[]) => mockSafeClipboard(...args),
}));

// ============================================================
// Mock: @dnd-kit/sortable
// ============================================================

vi.mock('@dnd-kit/sortable', () => ({
  arrayMove: (arr: unknown[], from: number, to: number) => {
    const result = [...arr];
    const [removed] = result.splice(from, 1);
    result.splice(to, 0, removed);
    return result;
  },
}));

// ============================================================
// Helpers
// ============================================================

function renderRedirectLink() {
  return renderHook(() => useRedirectLink());
}

// ============================================================
// Tests
// ============================================================

describe('useRedirectLink', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useQueryCalls.length = 0;
    useQueryReturnMap.clear();
    mockCheckSlugAvailability.mockResolvedValue({ available: true });
    mockCreateRedirectLink.mockResolvedValue({ link: { id: 'new-link' } });
    mockUpdateRedirectLink.mockResolvedValue({ link: { id: 'updated' } });
    mockDeleteRedirectLink.mockResolvedValue({});
    mockSafeClipboard.mockResolvedValue(true);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── Initial State ──────────────────────────────────────────────
  describe('initial state', () => {
    it('should generate a slug with 8 alphanumeric characters', () => {
      const { result } = renderRedirectLink();
      expect(result.current.slug).toMatch(/^[a-z0-9]{8}$/);
    });

    it('should have empty linkName', () => {
      const { result } = renderRedirectLink();
      expect(result.current.linkName).toBe('');
    });

    it('should default distributionStrategy to click_limit', () => {
      const { result } = renderRedirectLink();
      expect(result.current.distributionStrategy).toBe('click_limit');
    });

    it('should have empty groupPriorities', () => {
      const { result } = renderRedirectLink();
      expect(result.current.groupPriorities).toEqual([]);
    });

    it('should have dialogOpen = false', () => {
      const { result } = renderRedirectLink();
      expect(result.current.dialogOpen).toBe(false);
    });

    it('should have editingLink = null', () => {
      const { result } = renderRedirectLink();
      expect(result.current.editingLink).toBeNull();
    });

    it('should map allGroups from useGroupsConnect', () => {
      const { result } = renderRedirectLink();
      expect(result.current.allGroups).toHaveLength(3);
      expect(result.current.allGroups[0].id).toBe('g-1');
      expect(result.current.allGroups[0].name).toBe('Grupo A');
    });
  });

  // ── Group Selection ────────────────────────────────────────────
  describe('group selection', () => {
    it('should toggle group selection - add', () => {
      const { result } = renderRedirectLink();
      act(() => result.current.toggleGroupSelection('g-1'));
      expect(result.current.selectedGroupIds).toContain('g-1');
    });

    it('should toggle group selection - remove', () => {
      const { result } = renderRedirectLink();
      act(() => result.current.toggleGroupSelection('g-1'));
      act(() => result.current.toggleGroupSelection('g-1'));
      expect(result.current.selectedGroupIds).not.toContain('g-1');
    });

    it('should select all groups', () => {
      const { result } = renderRedirectLink();
      act(() => result.current.selectAllGroups());
      expect(result.current.selectedGroupIds).toEqual(['g-1', 'g-2', 'g-3']);
    });

    it('should deselect all groups', () => {
      const { result } = renderRedirectLink();
      act(() => result.current.selectAllGroups());
      act(() => result.current.deselectAllGroups());
      expect(result.current.selectedGroupIds).toEqual([]);
    });

    it('should apply group selection with sequential priorities', () => {
      const { result } = renderRedirectLink();
      act(() => result.current.toggleGroupSelection('g-1'));
      act(() => result.current.toggleGroupSelection('g-3'));
      act(() => result.current.applyGroupSelection());

      expect(result.current.groupPriorities).toHaveLength(2);
      expect(result.current.groupPriorities[0].priority).toBe(1);
      expect(result.current.groupPriorities[1].priority).toBe(2);
      expect(result.current.dialogOpen).toBe(false);
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.redirectLink.success.groupsUpdated',
        })
      );
    });
  });

  // ── Move Group ─────────────────────────────────────────────────
  describe('moveGroup', () => {
    function setupWithGroups() {
      const hook = renderRedirectLink();
      act(() => hook.result.current.selectAllGroups());
      act(() => hook.result.current.applyGroupSelection());
      mockToast.mockClear();
      return hook;
    }

    it('should move group down', () => {
      const { result } = setupWithGroups();
      const firstGroupId = result.current.groupPriorities[0].id;

      act(() => result.current.moveGroup(0, 'down'));

      expect(result.current.groupPriorities[1].id).toBe(firstGroupId);
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.redirectLink.success.orderUpdated',
        })
      );
    });

    it('should move group up', () => {
      const { result } = setupWithGroups();
      const lastGroupId = result.current.groupPriorities[2].id;

      act(() => result.current.moveGroup(2, 'up'));

      expect(result.current.groupPriorities[1].id).toBe(lastGroupId);
    });

    it('should not move first group up (boundary)', () => {
      const { result } = setupWithGroups();
      const firstGroupId = result.current.groupPriorities[0].id;

      act(() => result.current.moveGroup(0, 'up'));

      expect(result.current.groupPriorities[0].id).toBe(firstGroupId);
    });

    it('should not move last group down (boundary)', () => {
      const { result } = setupWithGroups();
      const lastGroupId = result.current.groupPriorities[2].id;

      act(() => result.current.moveGroup(2, 'down'));

      expect(result.current.groupPriorities[2].id).toBe(lastGroupId);
    });

    it('should update priority numbers after move', () => {
      const { result } = setupWithGroups();
      act(() => result.current.moveGroup(0, 'down'));

      result.current.groupPriorities.forEach((g, i) => {
        expect(g.priority).toBe(i + 1);
      });
    });
  });

  // ── Handle Drag End ────────────────────────────────────────────
  describe('handleDragEnd', () => {
    function setupWithGroups() {
      const hook = renderRedirectLink();
      act(() => hook.result.current.selectAllGroups());
      act(() => hook.result.current.applyGroupSelection());
      mockToast.mockClear();
      return hook;
    }

    it('should reorder when active.id !== over.id', () => {
      const { result } = setupWithGroups();
      const firstId = result.current.groupPriorities[0].id;
      const secondId = result.current.groupPriorities[1].id;

      act(() => {
        result.current.handleDragEnd({
          active: { id: firstId },
          over: { id: secondId },
        } as unknown as import('@dnd-kit/core').DragEndEvent);
      });

      expect(result.current.groupPriorities[0].id).toBe(secondId);
      expect(result.current.groupPriorities[1].id).toBe(firstId);
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.redirectLink.success.orderUpdated',
        })
      );
    });

    it('should noop when over is null', () => {
      const { result } = setupWithGroups();
      const before = [...result.current.groupPriorities];

      act(() => {
        result.current.handleDragEnd({
          active: { id: 'g-1' },
          over: null,
        } as unknown as import('@dnd-kit/core').DragEndEvent);
      });

      expect(result.current.groupPriorities.map((g) => g.id)).toEqual(before.map((g) => g.id));
    });

    it('should noop when active.id === over.id', () => {
      const { result } = setupWithGroups();
      const before = [...result.current.groupPriorities];

      act(() => {
        result.current.handleDragEnd({
          active: { id: 'g-1' },
          over: { id: 'g-1' },
        } as unknown as import('@dnd-kit/core').DragEndEvent);
      });

      expect(result.current.groupPriorities.map((g) => g.id)).toEqual(before.map((g) => g.id));
    });
  });

  // ── Validation - Create ────────────────────────────────────────
  describe('validation - create', () => {
    it('should show toast when linkName is empty', async () => {
      const { result } = renderRedirectLink();

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.redirectLink.errors.nameRequired',
        })
      );
    });

    it('should show toast when slug is empty', async () => {
      const { result } = renderRedirectLink();
      act(() => result.current.setLinkName('Test Link'));
      act(() => result.current.setSlug(''));

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.redirectLink.errors.slugRequired',
        })
      );
    });

    it('should show toast when groupPriorities is empty', async () => {
      const { result } = renderRedirectLink();
      act(() => result.current.setLinkName('Test Link'));

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.redirectLink.errors.groupsRequired',
        })
      );
    });

    it('should show toast when slug has invalid format', async () => {
      const { result } = renderRedirectLink();
      act(() => result.current.setLinkName('Test Link'));
      act(() => result.current.setSlug('INVALID_SLUG!'));
      act(() => result.current.selectAllGroups());
      act(() => result.current.applyGroupSelection());

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.redirectLink.errors.invalidSlug',
        })
      );
    });

    it('should show toast when click_limit strategy has invalid limits', async () => {
      const { result } = renderRedirectLink();
      act(() => result.current.setLinkName('Test Link'));
      act(() => result.current.setDistributionStrategy('click_limit'));
      act(() => result.current.selectAllGroups());
      act(() => result.current.applyGroupSelection());
      // Groups have no click_limit set by default in the priorities

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.redirectLink.errors.clickLimitsRequired',
        })
      );
    });

    it('should NOT validate click limits when strategy is member_limit', async () => {
      const { result } = renderRedirectLink();
      act(() => result.current.setLinkName('Test Link'));
      act(() => result.current.setDistributionStrategy('member_limit'));
      act(() => result.current.selectAllGroups());
      act(() => result.current.applyGroupSelection());

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      // Should NOT have clickLimitsRequired toast
      const clickLimitCalls = mockToast.mock.calls.filter(
        (call) => call[0]?.title === 'pages.redirectLink.errors.clickLimitsRequired'
      );
      expect(clickLimitCalls).toHaveLength(0);
    });
  });

  // ── Slug Availability ──────────────────────────────────────────
  describe('slug availability', () => {
    function setupValidForm() {
      const hook = renderRedirectLink();
      act(() => hook.result.current.setLinkName('Test Link'));
      act(() => hook.result.current.setDistributionStrategy('member_limit'));
      act(() => hook.result.current.selectAllGroups());
      act(() => hook.result.current.applyGroupSelection());
      mockToast.mockClear();
      return hook;
    }

    it('should show toast when slug already exists', async () => {
      mockCheckSlugAvailability.mockResolvedValue({ available: false });
      const { result } = setupValidForm();

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.redirectLink.errors.slugExists',
        })
      );
    });

    it('should show toast when checkSlug API fails', async () => {
      mockCheckSlugAvailability.mockRejectedValue(new Error('Network error'));
      const { result } = setupValidForm();

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.redirectLink.errors.checkSlug',
        })
      );
      expect(mockCaptureHookError).toHaveBeenCalledWith(
        expect.any(Error),
        'useRedirectLink',
        'checkSlugAvailability'
      );
    });

    it('should proceed to createMutation when slug is available', async () => {
      mockCheckSlugAvailability.mockResolvedValue({ available: true });
      const { result } = setupValidForm();

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      expect(mockMutate).toHaveBeenCalled();
    });
  });

  // ── Create Mutation ────────────────────────────────────────────
  describe('createMutation', () => {
    function setupValidForm() {
      const hook = renderRedirectLink();
      act(() => hook.result.current.setLinkName('Test Link'));
      act(() => hook.result.current.setDistributionStrategy('member_limit'));
      act(() => hook.result.current.selectAllGroups());
      act(() => hook.result.current.applyGroupSelection());
      mockToast.mockClear();
      mockMutate.mockClear();
      return hook;
    }

    it('should call mutate with converted proto params', async () => {
      const { result } = setupValidForm();

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      expect(mockMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Test Link',
          distributionStrategy: 2, // MEMBER_LIMIT
        })
      );
    });

    it('should show success toast and invalidate queries on success', async () => {
      const { result } = setupValidForm();

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.redirectLink.success.created',
        })
      );
    });

    it('should show error toast + captureHookError on create failure', async () => {
      mockCreateRedirectLink.mockRejectedValue(new Error('Create failed'));
      const { result } = setupValidForm();

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      // Wait for async mutation flow
      await act(async () => {
        await new Promise((r) => setTimeout(r, 10));
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.redirectLink.errors.create',
        })
      );
      expect(mockCaptureHookError).toHaveBeenCalledWith(
        expect.any(Error),
        'useRedirectLink',
        'createMutation'
      );
    });
  });

  // ── Update Mode ────────────────────────────────────────────────
  describe('update mode', () => {
    function setupEditMode() {
      const hook = renderRedirectLink();
      const mockLink = {
        id: 'link-1',
        user_id: 'user-123',
        name: 'Existing Link',
        slug: 'existing-slug',
        distribution_strategy: 'member_limit' as const,
        group_priorities: [
          {
            id: 'g-1',
            name: 'Grupo A',
            wa_group_id: '111@g.us',
            invite_code: 'INV1',
            members_count: 100,
            member_limit: 200,
            priority: 1,
          },
        ],
        is_active: true,
        total_clicks: 42,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-15T00:00:00Z',
      };

      act(() => hook.result.current.startEditLink(mockLink));
      mockToast.mockClear();
      mockMutate.mockClear();
      return hook;
    }

    it('should populate form when entering edit mode', () => {
      const hook = renderRedirectLink();
      const mockLink = {
        id: 'link-1',
        user_id: 'user-123',
        name: 'Existing Link',
        slug: 'existing-slug',
        distribution_strategy: 'member_limit' as const,
        group_priorities: [
          {
            id: 'g-1',
            name: 'Grupo A',
            wa_group_id: '111@g.us',
            invite_code: 'INV1',
            members_count: 100,
            member_limit: 200,
            priority: 1,
          },
        ],
        is_active: true,
        total_clicks: 42,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-15T00:00:00Z',
      };

      act(() => hook.result.current.startEditLink(mockLink));

      expect(hook.result.current.linkName).toBe('Existing Link');
      expect(hook.result.current.slug).toBe('existing-slug');
      expect(hook.result.current.distributionStrategy).toBe('member_limit');
      expect(hook.result.current.editingLink).not.toBeNull();
      expect(hook.result.current.groupPriorities).toHaveLength(1);
    });

    it('should merge is_favorite from allGroups when editing', () => {
      const hook = renderRedirectLink();
      const mockLink = {
        id: 'link-1',
        user_id: 'user-123',
        name: 'Existing Link',
        slug: 'existing-slug',
        distribution_strategy: 'click_limit' as const,
        group_priorities: [
          {
            id: 'g-1',
            name: 'Grupo A',
            wa_group_id: '111@g.us',
            invite_code: 'INV1',
            members_count: 100,
            member_limit: 200,
            priority: 1,
            is_favorite: false,
          },
        ],
        is_active: true,
        total_clicks: 42,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-15T00:00:00Z',
      };

      act(() => hook.result.current.startEditLink(mockLink));

      // g-1 has is_favorite=true in allGroups (mockGroups)
      expect(hook.result.current.groupPriorities[0].is_favorite).toBe(true);
    });

    it('should show editMode toast when entering edit', () => {
      const hook = renderRedirectLink();
      const mockLink = {
        id: 'link-1',
        user_id: 'user-123',
        name: 'Existing Link',
        slug: 'existing-slug',
        distribution_strategy: 'click_limit' as const,
        group_priorities: [],
        is_active: true,
        total_clicks: 0,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-15T00:00:00Z',
      };

      act(() => hook.result.current.startEditLink(mockLink));

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.redirectLink.success.editMode',
        })
      );
    });

    it('should call updateMutation when editingLink is set', async () => {
      const { result } = setupEditMode();

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      expect(mockMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'link-1',
          name: 'Existing Link',
        })
      );
    });

    it('should NOT check slug availability on update', async () => {
      const { result } = setupEditMode();

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      expect(mockCheckSlugAvailability).not.toHaveBeenCalled();
    });

    it('should validate name on update', async () => {
      const { result } = setupEditMode();
      act(() => result.current.setLinkName(''));

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.redirectLink.errors.nameRequired',
        })
      );
    });

    it('should validate groups on update', async () => {
      const { result } = setupEditMode();
      act(() => result.current.setGroupPriorities([]));

      await act(async () => {
        await result.current.handleCreateOrUpdate();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.redirectLink.errors.groupsRequired',
        })
      );
    });
  });

  // ── Delete Link ────────────────────────────────────────────────
  describe('deleteLink', () => {
    it('should call deleteMutation with ID', async () => {
      const { result } = renderRedirectLink();

      await act(async () => {
        await result.current.deleteLink('link-123');
      });

      expect(mockMutate).toHaveBeenCalledWith('link-123');
    });

    it('should show success toast on delete', async () => {
      const { result } = renderRedirectLink();

      await act(async () => {
        await result.current.deleteLink('link-123');
      });

      // Wait for async mutation flow
      await act(async () => {
        await new Promise((r) => setTimeout(r, 10));
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.redirectLink.success.deleted',
        })
      );
    });

    it('should show error toast on delete failure', async () => {
      mockDeleteRedirectLink.mockRejectedValue(new Error('Delete failed'));
      const { result } = renderRedirectLink();

      await act(async () => {
        await result.current.deleteLink('link-123');
      });

      // Wait for async mutation flow
      await act(async () => {
        await new Promise((r) => setTimeout(r, 10));
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.redirectLink.errors.delete',
        })
      );
    });
  });

  // ── Cancel Edit ────────────────────────────────────────────────
  describe('cancelEdit', () => {
    it('should reset form and show toast', () => {
      const hook = renderRedirectLink();
      const mockLink = {
        id: 'link-1',
        user_id: 'user-123',
        name: 'Existing Link',
        slug: 'existing-slug',
        distribution_strategy: 'click_limit' as const,
        group_priorities: [],
        is_active: true,
        total_clicks: 0,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-15T00:00:00Z',
      };

      act(() => hook.result.current.startEditLink(mockLink));
      mockToast.mockClear();

      act(() => hook.result.current.cancelEdit());

      expect(hook.result.current.editingLink).toBeNull();
      expect(hook.result.current.linkName).toBe('');
      expect(hook.result.current.groupPriorities).toEqual([]);
      expect(hook.result.current.slug).toMatch(/^[a-z0-9]{8}$/);
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.redirectLink.success.editCancelled',
        })
      );
    });
  });

  // ── Copy To Clipboard ─────────────────────────────────────────
  describe('copyToClipboard', () => {
    it('should call safeClipboard with correct URL', async () => {
      const { result } = renderRedirectLink();

      await act(async () => {
        await result.current.copyToClipboard('test-slug');
      });

      expect(mockSafeClipboard).toHaveBeenCalledWith(expect.stringContaining('/r/test-slug'));
    });

    it('should show success toast when clipboard succeeds', async () => {
      mockSafeClipboard.mockResolvedValue(true);
      const { result } = renderRedirectLink();

      await act(async () => {
        await result.current.copyToClipboard('test-slug');
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.redirectLink.success.copied',
        })
      );
    });

    it('should show error toast when clipboard fails', async () => {
      mockSafeClipboard.mockResolvedValue(false);
      const { result } = renderRedirectLink();

      await act(async () => {
        await result.current.copyToClipboard('test-slug');
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
        })
      );
    });
  });

  // ── Loading ────────────────────────────────────────────────────
  describe('loading', () => {
    it('should be false when all queries/mutations are idle', () => {
      const { result } = renderRedirectLink();
      expect(result.current.loading).toBe(false);
    });
  });

  // ── Saved Links ────────────────────────────────────────────────
  describe('savedLinks', () => {
    it('should map proto links via fromProtoSavedLink', () => {
      useQueryReturnMap.set(
        'listRedirectLinks',
        createQueryReturn({
          links: [
            {
              id: 'link-1',
              userId: 'user-123',
              name: 'Test Link',
              slug: 'test',
              distributionStrategy: 1, // CLICK_LIMIT
              groupPriorities: [
                {
                  id: 'g-1',
                  name: 'Grupo A',
                  waGroupId: '111@g.us',
                  inviteCode: 'INV1',
                  membersCount: 100,
                  memberLimit: 200,
                  priority: 1,
                  clickLimit: 50,
                  isFavorite: true,
                },
              ],
              isActive: true,
              totalClicks: 42,
              createdAt: '2026-01-01T00:00:00Z',
              updatedAt: '2026-01-15T00:00:00Z',
            },
          ],
        })
      );

      const { result } = renderRedirectLink();

      expect(result.current.savedLinks).toHaveLength(1);
      expect(result.current.savedLinks[0]).toEqual(
        expect.objectContaining({
          id: 'link-1',
          name: 'Test Link',
          slug: 'test',
          distribution_strategy: 'click_limit',
          is_active: true,
          total_clicks: 42,
        })
      );
      expect(result.current.savedLinks[0].group_priorities[0]).toEqual(
        expect.objectContaining({
          id: 'g-1',
          name: 'Grupo A',
          wa_group_id: '111@g.us',
          members_count: 100,
          member_limit: 200,
        })
      );
    });

    it('should return empty array when response is empty', () => {
      const { result } = renderRedirectLink();
      expect(result.current.savedLinks).toEqual([]);
    });
  });

  // ── Set State Functions ────────────────────────────────────────
  describe('state setters', () => {
    it('should update slug via setSlug', () => {
      const { result } = renderRedirectLink();
      act(() => result.current.setSlug('new-slug'));
      expect(result.current.slug).toBe('new-slug');
    });

    it('should update linkName via setLinkName', () => {
      const { result } = renderRedirectLink();
      act(() => result.current.setLinkName('New Link'));
      expect(result.current.linkName).toBe('New Link');
    });

    it('should update searchTerm via setSearchTerm', () => {
      const { result } = renderRedirectLink();
      act(() => result.current.setSearchTerm('search'));
      expect(result.current.searchTerm).toBe('search');
    });

    it('should update dialogOpen via setDialogOpen', () => {
      const { result } = renderRedirectLink();
      act(() => result.current.setDialogOpen(true));
      expect(result.current.dialogOpen).toBe(true);
    });

    it('should update distributionStrategy via setDistributionStrategy', () => {
      const { result } = renderRedirectLink();
      act(() => result.current.setDistributionStrategy('round_robin'));
      expect(result.current.distributionStrategy).toBe('round_robin');
    });
  });
});
