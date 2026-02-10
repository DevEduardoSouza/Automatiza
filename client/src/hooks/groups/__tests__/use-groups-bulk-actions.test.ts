import type { Group } from '@/types/group';
import { act, renderHook, waitFor } from '@testing-library/react';
import type { ChangeEvent } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useGroupsBulkActions } from '../use-groups-bulk-actions';

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
const mockUpdateGroupSettings = vi.fn();
const mockUpdateGroupSubject = vi.fn();
const mockUpdateGroupPicture = vi.fn();
const mockUpdateGroupDescription = vi.fn();
const mockUpdateGroup = vi.fn();

vi.mock('@/hooks/use-groups-connect', () => ({
  useGroupsConnect: () => ({
    updateGroupSettings: mockUpdateGroupSettings,
    updateGroupSubject: mockUpdateGroupSubject,
    updateGroupPicture: mockUpdateGroupPicture,
    updateGroupDescription: mockUpdateGroupDescription,
    updateGroup: mockUpdateGroup,
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

function createMockFile(name: string, type: string, sizeInBytes: number): File {
  const content = new Array(sizeInBytes).fill('a').join('');
  return new File([content], name, { type });
}

function createMockChangeEvent(file: File | null): ChangeEvent<HTMLInputElement> {
  return {
    target: {
      files: file ? [file] : null,
      value: file ? file.name : '',
    },
  } as unknown as ChangeEvent<HTMLInputElement>;
}

// ============================================================
// Test Setup
// ============================================================

describe('useGroupsBulkActions', () => {
  const mockClearSelection = vi.fn();
  const mockLoadGroups = vi.fn().mockResolvedValue(undefined);
  const mockSetLoading = vi.fn();
  const mockInstanceName = 'test-instance';

  const mockGroups: Group[] = [
    createMockGroup({
      id: 'group-1',
      wa_group_id: '123@g.us',
      name: 'Group 1',
      is_admin: true,
    }),
    createMockGroup({
      id: 'group-2',
      wa_group_id: '456@g.us',
      name: 'Group 2',
      is_admin: true,
    }),
    createMockGroup({
      id: 'group-3',
      wa_group_id: '789@g.us',
      name: 'Group 3',
      is_admin: false,
    }),
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    mockUpdateGroupSettings.mockResolvedValue(true);
    mockUpdateGroupSubject.mockResolvedValue(true);
    mockUpdateGroupPicture.mockResolvedValue(true);
    mockUpdateGroupDescription.mockResolvedValue(true);
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
        useGroupsBulkActions({
          selectedGroups: [],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      expect(result.current.groupName).toBe('');
      expect(result.current.groupPhoto).toBeNull();
      expect(result.current.description).toBe('');
      expect(result.current.autoNumberGroups).toBe(false);
      expect(result.current.changeNameDialogOpen).toBe(false);
      expect(result.current.changePhotoDialogOpen).toBe(false);
      expect(result.current.changeDescriptionDialogOpen).toBe(false);
    });
  });

  // ============================================================
  // State Setters Tests
  // ============================================================

  describe('state setters', () => {
    it('should update groupName state', () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: [],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      act(() => {
        result.current.setGroupName('New Group Name');
      });

      expect(result.current.groupName).toBe('New Group Name');
    });

    it('should update description state', () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: [],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      act(() => {
        result.current.setDescription('New Description');
      });

      expect(result.current.description).toBe('New Description');
    });

    it('should update autoNumberGroups state', () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: [],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      act(() => {
        result.current.setAutoNumberGroups(true);
      });

      expect(result.current.autoNumberGroups).toBe(true);
    });

    it('should update dialog states', () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: [],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      act(() => {
        result.current.setChangeNameDialogOpen(true);
        result.current.setChangePhotoDialogOpen(true);
        result.current.setChangeDescriptionDialogOpen(true);
      });

      expect(result.current.changeNameDialogOpen).toBe(true);
      expect(result.current.changePhotoDialogOpen).toBe(true);
      expect(result.current.changeDescriptionDialogOpen).toBe(true);
    });
  });

  // ============================================================
  // Photo Upload Tests
  // ============================================================

  describe('handlePhotoUpload', () => {
    it('should accept valid image file', () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: [],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      const imageFile = createMockFile('photo.jpg', 'image/jpeg', 1024);
      const event = createMockChangeEvent(imageFile);

      act(() => {
        result.current.handlePhotoUpload(event);
      });

      expect(result.current.groupPhoto).toBe(imageFile);
    });

    it('should reject non-image files', () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: [],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      const pdfFile = createMockFile('doc.pdf', 'application/pdf', 1024);
      const event = createMockChangeEvent(pdfFile);

      act(() => {
        result.current.handlePhotoUpload(event);
      });

      expect(result.current.groupPhoto).toBeNull();
      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.invalidFile',
        description: 'pages.groups.errors.onlyImages',
      });
    });

    it('should accept PNG files', () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: [],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      const pngFile = createMockFile('photo.png', 'image/png', 1024);
      const event = createMockChangeEvent(pngFile);

      act(() => {
        result.current.handlePhotoUpload(event);
      });

      expect(result.current.groupPhoto).toBe(pngFile);
    });
  });

  // ============================================================
  // handleBulkAction Validation Tests
  // ============================================================

  describe('handleBulkAction - validation', () => {
    it('should show error when no groups selected', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: [],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.noGroupSelected',
        description: 'pages.groups.errors.selectGroupFirst',
      });
    });

    it('should show error when non-admin groups are selected', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-3'], // Not an admin
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.groups.errors.permissionDenied',
        })
      );
    });
  });

  // ============================================================
  // Close Groups Action Tests
  // ============================================================

  describe('handleBulkAction - close groups', () => {
    it('should close groups successfully', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockUpdateGroupSettings).toHaveBeenCalledWith({
        instance_name: mockInstanceName,
        group_id: '123@g.us',
        messages_admin_only: true,
      });

      expect(mockUpdateGroup).toHaveBeenCalledWith({
        id: 'group-1',
        status: 'closed',
      });

      expect(mockLoadGroups).toHaveBeenCalled();
      expect(mockClearSelection).toHaveBeenCalled();
    });

    it('should close multiple groups', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1', 'group-2'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockUpdateGroupSettings).toHaveBeenCalledTimes(2);
      expect(mockUpdateGroup).toHaveBeenCalledTimes(2);
    });
  });

  // ============================================================
  // Open Groups Action Tests
  // ============================================================

  describe('handleBulkAction - open groups', () => {
    it('should open groups successfully', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      await act(async () => {
        await result.current.handleBulkAction('Abrir grupos');
      });

      expect(mockUpdateGroupSettings).toHaveBeenCalledWith({
        instance_name: mockInstanceName,
        group_id: '123@g.us',
        messages_admin_only: false,
      });

      expect(mockUpdateGroup).toHaveBeenCalledWith({
        id: 'group-1',
        status: 'open',
      });
    });
  });

  // ============================================================
  // Change Name Action Tests
  // ============================================================

  describe('handleBulkAction - change name', () => {
    it('should change group name successfully', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      act(() => {
        result.current.setGroupName('New Name');
      });

      await act(async () => {
        await result.current.handleBulkAction('Alterar nome');
      });

      expect(mockUpdateGroupSubject).toHaveBeenCalledWith({
        instance_name: mockInstanceName,
        group_id: '123@g.us',
        subject: 'New Name',
      });

      expect(mockUpdateGroup).toHaveBeenCalledWith({
        id: 'group-1',
        name: 'New Name',
      });

      // State should be reset
      expect(result.current.groupName).toBe('');
    });

    it('should auto-number groups when enabled', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1', 'group-2'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      act(() => {
        result.current.setGroupName('Base Name');
        result.current.setAutoNumberGroups(true);
      });

      await act(async () => {
        await result.current.handleBulkAction('Alterar nome');
      });

      // First group should get #1
      expect(mockUpdateGroupSubject).toHaveBeenCalledWith(
        expect.objectContaining({
          subject: '#1 Base Name',
        })
      );

      // Second group should get #2
      expect(mockUpdateGroupSubject).toHaveBeenCalledWith(
        expect.objectContaining({
          subject: '#2 Base Name',
        })
      );
    });

    it('should show error when name is empty and auto-number is disabled', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      act(() => {
        result.current.setGroupName('');
        result.current.setAutoNumberGroups(false);
      });

      await act(async () => {
        await result.current.handleBulkAction('Alterar nome');
      });

      // Should not call updateGroupSubject
      expect(mockUpdateGroupSubject).not.toHaveBeenCalled();
    });
  });

  // ============================================================
  // Change Photo Action Tests
  // ============================================================

  describe('handleBulkAction - change photo', () => {
    it('should change group photo successfully', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      const imageFile = createMockFile('photo.jpg', 'image/jpeg', 1024);
      act(() => {
        result.current.handlePhotoUpload(createMockChangeEvent(imageFile));
      });

      // Mock FileReader class properly
      class MockFileReader {
        result: string | null = null;
        onload: ((e: ProgressEvent<FileReader>) => void) | null = null;
        onerror: ((e: ProgressEvent<FileReader>) => void) | null = null;
        readAsDataURL() {
          setTimeout(() => {
            this.result = 'data:image/jpeg;base64,test';
            if (this.onload) {
              this.onload({} as ProgressEvent<FileReader>);
            }
          }, 0);
        }
      }
      vi.stubGlobal('FileReader', MockFileReader);

      await act(async () => {
        await result.current.handleBulkAction('Alterar foto');
      });

      await waitFor(() => {
        expect(mockUpdateGroupPicture).toHaveBeenCalledWith({
          instance_name: mockInstanceName,
          group_id: '123@g.us',
          base64_data: 'test', // Base64 without data URI prefix
          mimetype: 'image/jpeg',
        });
      });

      // State should be reset
      expect(result.current.groupPhoto).toBeNull();

      vi.unstubAllGlobals();
    });

    it('should show error when no photo selected', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      await act(async () => {
        await result.current.handleBulkAction('Alterar foto');
      });

      // Should not call updateGroupPicture
      expect(mockUpdateGroupPicture).not.toHaveBeenCalled();
    });
  });

  // ============================================================
  // Change Description Action Tests
  // ============================================================

  describe('handleBulkAction - change description', () => {
    it('should change group description successfully', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      act(() => {
        result.current.setDescription('New Description');
      });

      await act(async () => {
        await result.current.handleBulkAction('Alterar descricao');
      });

      expect(mockUpdateGroupDescription).toHaveBeenCalledWith({
        instance_name: mockInstanceName,
        group_id: '123@g.us',
        description: 'New Description',
      });

      expect(mockUpdateGroup).toHaveBeenCalledWith({
        id: 'group-1',
        description: 'New Description',
      });

      // State should be reset
      expect(result.current.description).toBe('');
    });

    it('should show error when description is empty', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      act(() => {
        result.current.setDescription('   '); // Whitespace only
      });

      await act(async () => {
        await result.current.handleBulkAction('Alterar descricao');
      });

      // Should not call updateGroupDescription
      expect(mockUpdateGroupDescription).not.toHaveBeenCalled();
    });
  });

  // ============================================================
  // Error Handling Tests
  // ============================================================

  describe('handleBulkAction - error handling', () => {
    it('should handle API errors gracefully', async () => {
      mockUpdateGroupSettings.mockResolvedValue(false); // Simulate failure

      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      // Should handle the error
      expect(mockSetLoading).toHaveBeenCalledWith(false);
    });

    it('should handle partial failures', async () => {
      // First succeeds, second fails
      mockUpdateGroupSettings.mockResolvedValueOnce(true).mockResolvedValueOnce(false);

      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1', 'group-2'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      // Should show partial success toast
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.success.partialComplete',
        })
      );
    });

    it('should handle unknown action', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      await act(async () => {
        await result.current.handleBulkAction('Unknown Action');
      });

      // Should complete without crashing
      expect(mockSetLoading).toHaveBeenCalledWith(false);
    });
  });

  // ============================================================
  // Loading State Tests
  // ============================================================

  describe('loading state', () => {
    it('should set loading true at start and false at end', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockSetLoading).toHaveBeenCalledWith(true);
      expect(mockSetLoading).toHaveBeenCalledWith(false);

      // First call should be true, last should be false
      const calls = mockSetLoading.mock.calls;
      expect(calls[0][0]).toBe(true);
      expect(calls[calls.length - 1][0]).toBe(false);
    });
  });

  // ============================================================
  // Cleanup Tests
  // ============================================================

  describe('cleanup after action', () => {
    it('should reset all state after successful action', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      // Set some state
      act(() => {
        result.current.setGroupName('Test Name');
        result.current.setDescription('Test Description');
        result.current.setAutoNumberGroups(true);
        result.current.setChangeNameDialogOpen(true);
      });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      // All state should be reset
      expect(result.current.groupName).toBe('');
      expect(result.current.groupPhoto).toBeNull();
      expect(result.current.description).toBe('');
      expect(result.current.autoNumberGroups).toBe(false);
      expect(result.current.changeNameDialogOpen).toBe(false);
      expect(result.current.changePhotoDialogOpen).toBe(false);
      expect(result.current.changeDescriptionDialogOpen).toBe(false);
    });

    it('should call loadGroups after action', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockLoadGroups).toHaveBeenCalled();
    });

    it('should call clearSelection after action', async () => {
      const { result } = renderHook(() =>
        useGroupsBulkActions({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          instanceName: mockInstanceName,
          loadGroups: mockLoadGroups,
          clearSelection: mockClearSelection,
          setLoading: mockSetLoading,
        })
      );

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockClearSelection).toHaveBeenCalled();
    });
  });
});
