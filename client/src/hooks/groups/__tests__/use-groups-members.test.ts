import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useGroupsMembers } from '../use-groups-members';

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
const mockGetGroupParticipants = vi.fn();
const mockAddGroupMember = vi.fn();
const mockRemoveGroupMember = vi.fn();
const mockLeaveGroup = vi.fn();
const mockPromoteMember = vi.fn();
const mockDemoteMember = vi.fn();

vi.mock('@/hooks/use-groups-connect', () => ({
  useGroupsConnect: () => ({
    getGroupParticipants: mockGetGroupParticipants,
    addGroupMember: mockAddGroupMember,
    removeGroupMember: mockRemoveGroupMember,
    leaveGroup: mockLeaveGroup,
    promoteMember: mockPromoteMember,
    demoteMember: mockDemoteMember,
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

// Mock Sentry
const mockCaptureHookError = vi.fn();

vi.mock('@/lib/sentry', () => ({
  captureHookError: (...args: unknown[]) => mockCaptureHookError(...args),
}));

// ============================================================
// Test Data
// ============================================================

const mockParticipants = [
  {
    id: '5511999990001@s.whatsapp.net',
    is_admin: false,
    is_super_admin: false,
    phone_number: '5511999990001',
  },
  {
    id: '5511999990002@s.whatsapp.net',
    is_admin: true,
    is_super_admin: false,
    phone_number: '5511999990002',
  },
  {
    id: '5511999990003@s.whatsapp.net',
    is_admin: false,
    is_super_admin: true,
    phone_number: '5511999990003',
  },
];

const mockGroup = { wa_group_id: '120363123456789@g.us', name: 'Test Group' };

// ============================================================
// Test Setup
// ============================================================

describe('useGroupsMembers', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetGroupParticipants.mockResolvedValue(mockParticipants);
    mockAddGroupMember.mockResolvedValue({ added_count: 1, failed_numbers: [] });
    mockRemoveGroupMember.mockResolvedValue({ removed_count: 1, failed_numbers: [] });
    mockLeaveGroup.mockResolvedValue(true);
    mockPromoteMember.mockResolvedValue(true);
    mockDemoteMember.mockResolvedValue(true);
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  // ============================================================
  // 1. Initial State Tests
  // ============================================================

  describe('initial state', () => {
    it('should have correct initial values', () => {
      const { result } = renderHook(() => useGroupsMembers());

      expect(result.current.participants).toEqual([]);
      expect(result.current.loadingParticipants).toBe(false);
      expect(result.current.membersDialogOpen).toBe(false);
      expect(result.current.selectedGroupForMembers).toBeNull();
      expect(result.current.phoneNumberInput).toBe('');
      expect(result.current.searchQuery).toBe('');
      expect(result.current.filteredParticipants).toEqual([]);
      expect(result.current.isAddingMember).toBe(false);
      expect(result.current.isRemovingMember).toBe(false);
      expect(result.current.isLeavingGroup).toBe(false);
      expect(result.current.isPromotingMember).toBe(false);
      expect(result.current.isDemotingMember).toBe(false);
    });
  });

  // ============================================================
  // 2. openMembersDialog Tests
  // ============================================================

  describe('openMembersDialog', () => {
    it('should set selectedGroupForMembers, open dialog, and reset phone/search inputs', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      // Set some values first to verify they get reset
      act(() => {
        result.current.setPhoneNumberInput('5511999990001');
        result.current.setSearchQuery('search');
      });

      expect(result.current.phoneNumberInput).toBe('5511999990001');
      expect(result.current.searchQuery).toBe('search');

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      expect(result.current.membersDialogOpen).toBe(true);
      expect(result.current.selectedGroupForMembers).toEqual(mockGroup);
      expect(result.current.phoneNumberInput).toBe('');
      expect(result.current.searchQuery).toBe('');
    });

    it('should call getGroupParticipants with correct instance_name and group_id', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      expect(mockGetGroupParticipants).toHaveBeenCalledWith({
        instance_name: 'test-instance',
        group_id: '120363123456789@g.us',
      });
    });

    it('should set participants from result', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toEqual(mockParticipants);
      });
    });

    it('should transition loadingParticipants from false to true to false', async () => {
      let resolveParticipants: (value: typeof mockParticipants) => void;
      mockGetGroupParticipants.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolveParticipants = resolve;
          })
      );

      const { result } = renderHook(() => useGroupsMembers());

      expect(result.current.loadingParticipants).toBe(false);

      let openPromise: Promise<void>;
      act(() => {
        openPromise = (async () => {
          result.current.openMembersDialog(mockGroup);
        })();
      });

      await waitFor(() => {
        expect(result.current.loadingParticipants).toBe(true);
      });

      await act(async () => {
        resolveParticipants!(mockParticipants);
        await openPromise!;
      });

      await waitFor(() => {
        expect(result.current.loadingParticipants).toBe(false);
      });
    });

    it('should clear participants on error and call captureHookError', async () => {
      const error = new Error('Network error');
      mockGetGroupParticipants.mockRejectedValue(error);

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toEqual([]);
        expect(result.current.loadingParticipants).toBe(false);
      });

      expect(mockCaptureHookError).toHaveBeenCalledWith(
        error,
        'useGroupsMembers',
        'loadParticipants'
      );
    });

    it('should NOT load participants when instanceName is empty', async () => {
      const originalInstance = mockLatestInstance.instance_id;
      Object.defineProperty(mockLatestInstance, 'instance_id', {
        value: '',
        writable: true,
        configurable: true,
      });

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      expect(mockGetGroupParticipants).not.toHaveBeenCalled();

      Object.defineProperty(mockLatestInstance, 'instance_id', {
        value: originalInstance,
        writable: true,
        configurable: true,
      });
    });
  });

  // ============================================================
  // 3. closeMembersDialog Tests
  // ============================================================

  describe('closeMembersDialog', () => {
    it('should close dialog, clear selectedGroup, participants, phoneNumberInput, and searchQuery', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      // Open dialog first to populate state
      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.membersDialogOpen).toBe(true);
        expect(result.current.participants).toHaveLength(3);
      });

      // Set additional state
      act(() => {
        result.current.setPhoneNumberInput('5511888880001');
        result.current.setSearchQuery('test');
      });

      // Close dialog
      act(() => {
        result.current.closeMembersDialog();
      });

      expect(result.current.membersDialogOpen).toBe(false);
      expect(result.current.selectedGroupForMembers).toBeNull();
      expect(result.current.participants).toEqual([]);
      expect(result.current.phoneNumberInput).toBe('');
      expect(result.current.searchQuery).toBe('');
    });
  });

  // ============================================================
  // 4. filteredParticipants Tests
  // ============================================================

  describe('filteredParticipants', () => {
    it('should return all participants when searchQuery is empty', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      expect(result.current.filteredParticipants).toHaveLength(3);
    });

    it('should filter by participant id (case-insensitive)', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      act(() => {
        result.current.setSearchQuery('0001');
      });

      expect(result.current.filteredParticipants).toHaveLength(1);
      expect(result.current.filteredParticipants[0].id).toBe('5511999990001@s.whatsapp.net');

      // Verify case-insensitivity on id field
      act(() => {
        result.current.setSearchQuery('S.WHATSAPP');
      });

      // All participants have @s.whatsapp.net in their id
      expect(result.current.filteredParticipants).toHaveLength(3);
    });

    it('should filter by phone_number (case-insensitive)', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      act(() => {
        result.current.setSearchQuery('5511999990002');
      });

      expect(result.current.filteredParticipants).toHaveLength(1);
      expect(result.current.filteredParticipants[0].phone_number).toBe('5511999990002');
    });

    it('should return empty array when no match', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      act(() => {
        result.current.setSearchQuery('nonexistent');
      });

      expect(result.current.filteredParticipants).toHaveLength(0);
    });

    it('should handle participants with undefined phone_number', async () => {
      const participantsWithUndefinedPhone = [
        {
          id: '5511999990001@s.whatsapp.net',
          is_admin: false,
          is_super_admin: false,
          phone_number: '5511999990001',
        },
        {
          id: '5511999990004@s.whatsapp.net',
          is_admin: false,
          is_super_admin: false,
          phone_number: undefined as unknown as string,
        },
      ];
      mockGetGroupParticipants.mockResolvedValue(participantsWithUndefinedPhone);

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(2);
      });

      // Search by id should still work for participant with undefined phone_number
      act(() => {
        result.current.setSearchQuery('0004');
      });

      expect(result.current.filteredParticipants).toHaveLength(1);
      expect(result.current.filteredParticipants[0].id).toBe('5511999990004@s.whatsapp.net');

      // Search by phone should not crash on undefined phone_number
      act(() => {
        result.current.setSearchQuery('5511999990001');
      });

      expect(result.current.filteredParticipants).toHaveLength(1);
      expect(result.current.filteredParticipants[0].id).toBe('5511999990001@s.whatsapp.net');
    });
  });

  // ============================================================
  // 5. handleAddMember Tests
  // ============================================================

  describe('handleAddMember', () => {
    it('should add member successfully (toast success, clear phone input, reload participants)', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      mockGetGroupParticipants.mockClear();

      act(() => {
        result.current.setPhoneNumberInput('5511888880001');
      });

      await act(async () => {
        await result.current.handleAddMember();
      });

      expect(mockAddGroupMember).toHaveBeenCalledWith({
        instance_name: 'test-instance',
        group_id: '120363123456789@g.us',
        phone_numbers: ['5511888880001'],
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.members.memberAdded',
      });

      expect(result.current.phoneNumberInput).toBe('');
      expect(mockGetGroupParticipants).toHaveBeenCalled();
    });

    it('should show error toast when failed_numbers > 0 and added_count is 0', async () => {
      mockAddGroupMember.mockResolvedValue({
        added_count: 0,
        failed_numbers: ['5511888880002'],
      });

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      act(() => {
        result.current.setPhoneNumberInput('5511888880002');
      });

      await act(async () => {
        await result.current.handleAddMember();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'common.error',
        description: expect.stringContaining('pages.groups.members.memberAddedWithErrors'),
      });

      // Should NOT have shown success toast since added_count is 0
      expect(mockToast).not.toHaveBeenCalledWith({
        title: 'pages.groups.members.memberAdded',
      });
    });

    it('should show both success and error toast when added_count > 0 AND failed_numbers > 0', async () => {
      mockAddGroupMember.mockResolvedValue({
        added_count: 1,
        failed_numbers: ['5511888880002'],
      });

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      act(() => {
        result.current.setPhoneNumberInput('5511888880001');
      });

      await act(async () => {
        await result.current.handleAddMember();
      });

      // Should show success toast
      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.members.memberAdded',
      });

      // Should also show destructive toast for failures
      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'common.error',
        description: expect.stringContaining('pages.groups.members.memberAddedWithErrors'),
      });

      expect(mockToast).toHaveBeenCalledTimes(2);
    });

    it('should return early when no selectedGroup', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      act(() => {
        result.current.setPhoneNumberInput('5511888880001');
      });

      await act(async () => {
        await result.current.handleAddMember();
      });

      expect(mockAddGroupMember).not.toHaveBeenCalled();
    });

    it('should return early when phoneNumberInput is empty', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await act(async () => {
        await result.current.handleAddMember();
      });

      expect(mockAddGroupMember).not.toHaveBeenCalled();
    });

    it('should return early when phoneNumberInput is whitespace only', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      act(() => {
        result.current.setPhoneNumberInput('   ');
      });

      await act(async () => {
        await result.current.handleAddMember();
      });

      expect(mockAddGroupMember).not.toHaveBeenCalled();
    });

    it('should return early when instanceName is empty', async () => {
      const originalInstance = mockLatestInstance.instance_id;
      Object.defineProperty(mockLatestInstance, 'instance_id', {
        value: '',
        writable: true,
        configurable: true,
      });

      const { result } = renderHook(() => useGroupsMembers());

      act(() => {
        result.current.setPhoneNumberInput('5511888880001');
      });

      await act(async () => {
        await result.current.handleAddMember();
      });

      expect(mockAddGroupMember).not.toHaveBeenCalled();

      Object.defineProperty(mockLatestInstance, 'instance_id', {
        value: originalInstance,
        writable: true,
        configurable: true,
      });
    });

    it('should set isAddingMember during operation (true -> false in finally)', async () => {
      let resolveAdd: (value: { added_count: number; failed_numbers: string[] }) => void;
      mockAddGroupMember.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolveAdd = resolve;
          })
      );

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      act(() => {
        result.current.setPhoneNumberInput('5511888880001');
      });

      expect(result.current.isAddingMember).toBe(false);

      let addPromise: Promise<void>;
      act(() => {
        addPromise = result.current.handleAddMember();
      });

      await waitFor(() => {
        expect(result.current.isAddingMember).toBe(true);
      });

      await act(async () => {
        resolveAdd!({ added_count: 1, failed_numbers: [] });
        await addPromise!;
      });

      await waitFor(() => {
        expect(result.current.isAddingMember).toBe(false);
      });
    });

    it('should trim phone number before sending', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      act(() => {
        result.current.setPhoneNumberInput('  5511888880001  ');
      });

      await act(async () => {
        await result.current.handleAddMember();
      });

      expect(mockAddGroupMember).toHaveBeenCalledWith(
        expect.objectContaining({
          phone_numbers: ['5511888880001'],
        })
      );
    });
  });

  // ============================================================
  // 6. handleRemoveMember Tests
  // ============================================================

  describe('handleRemoveMember', () => {
    it('should remove member successfully (extracts phone from WhatsApp ID, toast success, reload participants)', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      mockGetGroupParticipants.mockClear();

      await act(async () => {
        await result.current.handleRemoveMember('5511999990001@s.whatsapp.net');
      });

      expect(mockRemoveGroupMember).toHaveBeenCalledWith({
        instance_name: 'test-instance',
        group_id: '120363123456789@g.us',
        phone_numbers: ['5511999990001'],
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.members.memberRemoved',
      });

      expect(mockGetGroupParticipants).toHaveBeenCalled();
    });

    it('should return early when no selectedGroup', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        await result.current.handleRemoveMember('5511999990001@s.whatsapp.net');
      });

      expect(mockRemoveGroupMember).not.toHaveBeenCalled();
    });

    it('should return early when instanceName is empty', async () => {
      const originalInstance = mockLatestInstance.instance_id;
      Object.defineProperty(mockLatestInstance, 'instance_id', {
        value: '',
        writable: true,
        configurable: true,
      });

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        await result.current.handleRemoveMember('5511999990001@s.whatsapp.net');
      });

      expect(mockRemoveGroupMember).not.toHaveBeenCalled();

      Object.defineProperty(mockLatestInstance, 'instance_id', {
        value: originalInstance,
        writable: true,
        configurable: true,
      });
    });

    it('should set isRemovingMember during operation', async () => {
      let resolveRemove: (value: { removed_count: number; failed_numbers: string[] }) => void;
      mockRemoveGroupMember.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolveRemove = resolve;
          })
      );

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      expect(result.current.isRemovingMember).toBe(false);

      let removePromise: Promise<void>;
      act(() => {
        removePromise = result.current.handleRemoveMember('5511999990001@s.whatsapp.net');
      });

      await waitFor(() => {
        expect(result.current.isRemovingMember).toBe(true);
      });

      await act(async () => {
        resolveRemove!({ removed_count: 1, failed_numbers: [] });
        await removePromise!;
      });

      await waitFor(() => {
        expect(result.current.isRemovingMember).toBe(false);
      });
    });

    it('should handle phone extraction correctly (strips @s.whatsapp.net)', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      await act(async () => {
        await result.current.handleRemoveMember('5521987654321@s.whatsapp.net');
      });

      expect(mockRemoveGroupMember).toHaveBeenCalledWith(
        expect.objectContaining({
          phone_numbers: ['5521987654321'],
        })
      );
    });

    it('should set isRemovingMember to false on error (finally block)', async () => {
      mockRemoveGroupMember.mockRejectedValue(new Error('Remove failed'));

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      await act(async () => {
        try {
          await result.current.handleRemoveMember('5511999990001@s.whatsapp.net');
        } catch {
          // Expected to throw
        }
      });

      expect(result.current.isRemovingMember).toBe(false);
    });
  });

  // ============================================================
  // 7. handleLeaveGroup Tests
  // ============================================================

  describe('handleLeaveGroup', () => {
    it('should leave group successfully (toast success, calls closeMembersDialog)', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.membersDialogOpen).toBe(true);
      });

      await act(async () => {
        await result.current.handleLeaveGroup();
      });

      expect(mockLeaveGroup).toHaveBeenCalledWith({
        instance_name: 'test-instance',
        group_id: '120363123456789@g.us',
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.members.leftGroup',
      });

      // closeMembersDialog should have been called, clearing all state
      expect(result.current.membersDialogOpen).toBe(false);
      expect(result.current.selectedGroupForMembers).toBeNull();
      expect(result.current.participants).toEqual([]);
    });

    it('should return early when no selectedGroup', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        await result.current.handleLeaveGroup();
      });

      expect(mockLeaveGroup).not.toHaveBeenCalled();
    });

    it('should return early when instanceName is empty', async () => {
      const originalInstance = mockLatestInstance.instance_id;
      Object.defineProperty(mockLatestInstance, 'instance_id', {
        value: '',
        writable: true,
        configurable: true,
      });

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        await result.current.handleLeaveGroup();
      });

      expect(mockLeaveGroup).not.toHaveBeenCalled();

      Object.defineProperty(mockLatestInstance, 'instance_id', {
        value: originalInstance,
        writable: true,
        configurable: true,
      });
    });

    it('should set isLeavingGroup during operation', async () => {
      let resolveLeave: (value: boolean) => void;
      mockLeaveGroup.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolveLeave = resolve;
          })
      );

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.membersDialogOpen).toBe(true);
      });

      expect(result.current.isLeavingGroup).toBe(false);

      let leavePromise: Promise<void>;
      act(() => {
        leavePromise = result.current.handleLeaveGroup();
      });

      await waitFor(() => {
        expect(result.current.isLeavingGroup).toBe(true);
      });

      await act(async () => {
        resolveLeave!(true);
        await leavePromise!;
      });

      await waitFor(() => {
        expect(result.current.isLeavingGroup).toBe(false);
      });
    });

    it('should set isLeavingGroup to false on error (finally block)', async () => {
      mockLeaveGroup.mockRejectedValue(new Error('Leave failed'));

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.membersDialogOpen).toBe(true);
      });

      await act(async () => {
        try {
          await result.current.handleLeaveGroup();
        } catch {
          // Expected to throw
        }
      });

      expect(result.current.isLeavingGroup).toBe(false);
    });
  });

  // ============================================================
  // 8. handlePromoteMember Tests
  // ============================================================

  describe('handlePromoteMember', () => {
    it('should promote member successfully (extracts phone, toast success, reload participants)', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      mockGetGroupParticipants.mockClear();

      await act(async () => {
        await result.current.handlePromoteMember('5511999990001@s.whatsapp.net');
      });

      expect(mockPromoteMember).toHaveBeenCalledWith({
        instance_name: 'test-instance',
        group_id: '120363123456789@g.us',
        phone_number: '5511999990001',
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.members.memberPromoted',
      });

      expect(mockGetGroupParticipants).toHaveBeenCalled();
    });

    it('should return early when no selectedGroup', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        await result.current.handlePromoteMember('5511999990001@s.whatsapp.net');
      });

      expect(mockPromoteMember).not.toHaveBeenCalled();
    });

    it('should return early when instanceName is empty', async () => {
      const originalInstance = mockLatestInstance.instance_id;
      Object.defineProperty(mockLatestInstance, 'instance_id', {
        value: '',
        writable: true,
        configurable: true,
      });

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        await result.current.handlePromoteMember('5511999990001@s.whatsapp.net');
      });

      expect(mockPromoteMember).not.toHaveBeenCalled();

      Object.defineProperty(mockLatestInstance, 'instance_id', {
        value: originalInstance,
        writable: true,
        configurable: true,
      });
    });

    it('should set isPromotingMember during operation', async () => {
      let resolvePromote: (value: boolean) => void;
      mockPromoteMember.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolvePromote = resolve;
          })
      );

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      expect(result.current.isPromotingMember).toBe(false);

      let promotePromise: Promise<void>;
      act(() => {
        promotePromise = result.current.handlePromoteMember('5511999990001@s.whatsapp.net');
      });

      await waitFor(() => {
        expect(result.current.isPromotingMember).toBe(true);
      });

      await act(async () => {
        resolvePromote!(true);
        await promotePromise!;
      });

      await waitFor(() => {
        expect(result.current.isPromotingMember).toBe(false);
      });
    });

    it('should set isPromotingMember to false on error (finally block)', async () => {
      mockPromoteMember.mockRejectedValue(new Error('Promote failed'));

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      await act(async () => {
        try {
          await result.current.handlePromoteMember('5511999990001@s.whatsapp.net');
        } catch {
          // Expected to throw
        }
      });

      expect(result.current.isPromotingMember).toBe(false);
    });
  });

  // ============================================================
  // 9. handleDemoteMember Tests
  // ============================================================

  describe('handleDemoteMember', () => {
    it('should demote member successfully (extracts phone, toast success, reload participants)', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      mockGetGroupParticipants.mockClear();

      await act(async () => {
        await result.current.handleDemoteMember('5511999990002@s.whatsapp.net');
      });

      expect(mockDemoteMember).toHaveBeenCalledWith({
        instance_name: 'test-instance',
        group_id: '120363123456789@g.us',
        phone_number: '5511999990002',
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.members.memberDemoted',
      });

      expect(mockGetGroupParticipants).toHaveBeenCalled();
    });

    it('should return early when no selectedGroup', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        await result.current.handleDemoteMember('5511999990002@s.whatsapp.net');
      });

      expect(mockDemoteMember).not.toHaveBeenCalled();
    });

    it('should return early when instanceName is empty', async () => {
      const originalInstance = mockLatestInstance.instance_id;
      Object.defineProperty(mockLatestInstance, 'instance_id', {
        value: '',
        writable: true,
        configurable: true,
      });

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        await result.current.handleDemoteMember('5511999990002@s.whatsapp.net');
      });

      expect(mockDemoteMember).not.toHaveBeenCalled();

      Object.defineProperty(mockLatestInstance, 'instance_id', {
        value: originalInstance,
        writable: true,
        configurable: true,
      });
    });

    it('should set isDemotingMember during operation', async () => {
      let resolveDemote: (value: boolean) => void;
      mockDemoteMember.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolveDemote = resolve;
          })
      );

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      expect(result.current.isDemotingMember).toBe(false);

      let demotePromise: Promise<void>;
      act(() => {
        demotePromise = result.current.handleDemoteMember('5511999990002@s.whatsapp.net');
      });

      await waitFor(() => {
        expect(result.current.isDemotingMember).toBe(true);
      });

      await act(async () => {
        resolveDemote!(true);
        await demotePromise!;
      });

      await waitFor(() => {
        expect(result.current.isDemotingMember).toBe(false);
      });
    });

    it('should set isDemotingMember to false on error (finally block)', async () => {
      mockDemoteMember.mockRejectedValue(new Error('Demote failed'));

      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      await act(async () => {
        try {
          await result.current.handleDemoteMember('5511999990002@s.whatsapp.net');
        } catch {
          // Expected to throw
        }
      });

      expect(result.current.isDemotingMember).toBe(false);
    });
  });

  // ============================================================
  // 10. Edge Cases
  // ============================================================

  describe('edge cases', () => {
    it('should handle extractPhoneFromWaId with standard WhatsApp ID format', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      await act(async () => {
        await result.current.handleRemoveMember('5511999990001@s.whatsapp.net');
      });

      expect(mockRemoveGroupMember).toHaveBeenCalledWith(
        expect.objectContaining({
          phone_numbers: ['5511999990001'],
        })
      );
    });

    it('should handle extractPhoneFromWaId when input has no @s.whatsapp.net suffix', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      // Pass a plain phone number (no WhatsApp suffix)
      await act(async () => {
        await result.current.handleRemoveMember('5511999990001');
      });

      expect(mockRemoveGroupMember).toHaveBeenCalledWith(
        expect.objectContaining({
          phone_numbers: ['5511999990001'],
        })
      );
    });

    it('should handle extractPhoneFromWaId with country code variations', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      // International phone number with WhatsApp suffix
      await act(async () => {
        await result.current.handlePromoteMember('14155552671@s.whatsapp.net');
      });

      expect(mockPromoteMember).toHaveBeenCalledWith(
        expect.objectContaining({
          phone_number: '14155552671',
        })
      );
    });

    it('should handle extractPhoneFromWaId with only @s.whatsapp.net (edge case empty phone)', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      await act(async () => {
        await result.current.handleDemoteMember('@s.whatsapp.net');
      });

      expect(mockDemoteMember).toHaveBeenCalledWith(
        expect.objectContaining({
          phone_number: '',
        })
      );
    });

    it('should handle concurrent operations gracefully', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      // Fire promote and demote concurrently - both should complete without crashing
      await act(async () => {
        const promotePromise = result.current.handlePromoteMember('5511999990001@s.whatsapp.net');
        const demotePromise = result.current.handleDemoteMember('5511999990002@s.whatsapp.net');
        await Promise.all([promotePromise, demotePromise]);
      });

      expect(mockPromoteMember).toHaveBeenCalledTimes(1);
      expect(mockDemoteMember).toHaveBeenCalledTimes(1);
      expect(result.current.isPromotingMember).toBe(false);
      expect(result.current.isDemotingMember).toBe(false);
    });

    it('should handle network errors during participant reload after add', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      // First call succeeds (initial load already done), the reload after add fails
      mockGetGroupParticipants.mockRejectedValueOnce(new Error('Reload failed'));

      act(() => {
        result.current.setPhoneNumberInput('5511888880001');
      });

      // The add itself succeeds but participant reload fails
      // The hook should still complete without crashing since loadParticipants catches errors
      await act(async () => {
        await result.current.handleAddMember();
      });

      expect(mockAddGroupMember).toHaveBeenCalled();
      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.members.memberAdded',
      });
      expect(result.current.isAddingMember).toBe(false);
    });

    it('should handle network errors during participant reload after remove', async () => {
      const { result } = renderHook(() => useGroupsMembers());

      await act(async () => {
        result.current.openMembersDialog(mockGroup);
      });

      await waitFor(() => {
        expect(result.current.participants).toHaveLength(3);
      });

      // The reload after remove fails
      mockGetGroupParticipants.mockRejectedValueOnce(new Error('Reload failed'));

      await act(async () => {
        await result.current.handleRemoveMember('5511999990001@s.whatsapp.net');
      });

      expect(mockRemoveGroupMember).toHaveBeenCalled();
      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.members.memberRemoved',
      });
      expect(result.current.isRemovingMember).toBe(false);
    });
  });
});
