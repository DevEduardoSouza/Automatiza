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

  function renderBulkActions(overrides: Partial<Parameters<typeof useGroupsBulkActions>[0]> = {}) {
    return renderHook(() =>
      useGroupsBulkActions({
        selectedGroups: [],
        groups: mockGroups,
        instanceName: mockInstanceName,
        loadGroups: mockLoadGroups,
        clearSelection: mockClearSelection,
        setLoading: mockSetLoading,
        ...overrides,
      })
    );
  }

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
      const { result } = renderBulkActions();

      expect(result.current.groupName).toBe('');
      expect(result.current.groupPhoto).toBeNull();
      expect(result.current.description).toBe('');
      expect(result.current.autoNumberGroups).toBe(false);
      expect(result.current.changeNameDialogOpen).toBe(false);
      expect(result.current.changePhotoDialogOpen).toBe(false);
      expect(result.current.changeDescriptionDialogOpen).toBe(false);
    });

    it('should expose all setters and handlers', () => {
      const { result } = renderBulkActions();

      expect(typeof result.current.setGroupName).toBe('function');
      expect(typeof result.current.setDescription).toBe('function');
      expect(typeof result.current.setAutoNumberGroups).toBe('function');
      expect(typeof result.current.setChangeNameDialogOpen).toBe('function');
      expect(typeof result.current.setChangePhotoDialogOpen).toBe('function');
      expect(typeof result.current.setChangeDescriptionDialogOpen).toBe('function');
      expect(typeof result.current.handlePhotoUpload).toBe('function');
      expect(typeof result.current.handleBulkAction).toBe('function');
    });
  });

  // ============================================================
  // State Setters Tests
  // ============================================================

  describe('state setters', () => {
    it('should update groupName state', () => {
      const { result } = renderBulkActions();

      act(() => {
        result.current.setGroupName('New Group Name');
      });

      expect(result.current.groupName).toBe('New Group Name');
    });

    it('should update description state', () => {
      const { result } = renderBulkActions();

      act(() => {
        result.current.setDescription('New Description');
      });

      expect(result.current.description).toBe('New Description');
    });

    it('should update autoNumberGroups state', () => {
      const { result } = renderBulkActions();

      act(() => {
        result.current.setAutoNumberGroups(true);
      });

      expect(result.current.autoNumberGroups).toBe(true);
    });

    it('should update dialog states', () => {
      const { result } = renderBulkActions();

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
      const { result } = renderBulkActions();

      const imageFile = createMockFile('photo.jpg', 'image/jpeg', 1024);
      const event = createMockChangeEvent(imageFile);

      act(() => {
        result.current.handlePhotoUpload(event);
      });

      expect(result.current.groupPhoto).toBe(imageFile);
    });

    it('should reject non-image files', () => {
      const { result } = renderBulkActions();

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
      const { result } = renderBulkActions();

      const pngFile = createMockFile('photo.png', 'image/png', 1024);
      const event = createMockChangeEvent(pngFile);

      act(() => {
        result.current.handlePhotoUpload(event);
      });

      expect(result.current.groupPhoto).toBe(pngFile);
    });

    it('should accept WebP files', () => {
      const { result } = renderBulkActions();

      const webpFile = createMockFile('photo.webp', 'image/webp', 1024);
      const event = createMockChangeEvent(webpFile);

      act(() => {
        result.current.handlePhotoUpload(event);
      });

      expect(result.current.groupPhoto).toBe(webpFile);
    });

    it('should not update state when no file is selected (empty files list)', () => {
      const { result } = renderBulkActions();

      const event = {
        target: {
          files: [],
          value: '',
        },
      } as unknown as ChangeEvent<HTMLInputElement>;

      act(() => {
        result.current.handlePhotoUpload(event);
      });

      // File is undefined since files?.[0] yields undefined, which is falsy
      expect(result.current.groupPhoto).toBeNull();
    });
  });

  // ============================================================
  // handleBulkAction Validation Tests
  // ============================================================

  describe('handleBulkAction - validation', () => {
    it('should show error when no groups selected', async () => {
      const { result } = renderBulkActions({ selectedGroups: [] });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.noGroupSelected',
        description: 'pages.groups.errors.selectGroupFirst',
      });
      expect(mockSetLoading).not.toHaveBeenCalled();
    });

    it('should not call any API when no groups selected', async () => {
      const { result } = renderBulkActions({ selectedGroups: [] });

      await act(async () => {
        await result.current.handleBulkAction('Abrir grupos');
      });

      expect(mockUpdateGroupSettings).not.toHaveBeenCalled();
      expect(mockUpdateGroupSubject).not.toHaveBeenCalled();
      expect(mockUpdateGroupPicture).not.toHaveBeenCalled();
      expect(mockUpdateGroupDescription).not.toHaveBeenCalled();
    });
  });

  // ============================================================
  // Close Groups Action Tests
  // ============================================================

  describe('handleBulkAction - close groups', () => {
    it('should close groups successfully', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

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
      const { result } = renderBulkActions({ selectedGroups: ['group-1', 'group-2'] });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockUpdateGroupSettings).toHaveBeenCalledTimes(2);
      expect(mockUpdateGroup).toHaveBeenCalledTimes(2);
    });

    it('should handle close group returning false (failure)', async () => {
      mockUpdateGroupSettings.mockResolvedValue(false);

      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      // updateGroup should NOT be called because updateGroupSettings returned false
      expect(mockUpdateGroup).not.toHaveBeenCalledWith(
        expect.objectContaining({ status: 'closed' })
      );

      // Error should be captured
      expect(mockCaptureHookError).toHaveBeenCalledWith(
        expect.any(Error),
        'useGroupsBulkActions',
        'executeBulkAction'
      );
    });
  });

  // ============================================================
  // Open Groups Action Tests
  // ============================================================

  describe('handleBulkAction - open groups', () => {
    it('should open groups successfully', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

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

    it('should handle open group returning false', async () => {
      mockUpdateGroupSettings.mockResolvedValue(false);

      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      await act(async () => {
        await result.current.handleBulkAction('Abrir grupos');
      });

      expect(mockCaptureHookError).toHaveBeenCalled();
    });
  });

  // ============================================================
  // Change Name Action Tests
  // ============================================================

  describe('handleBulkAction - change name', () => {
    it('should change group name successfully', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

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
      const { result } = renderBulkActions({ selectedGroups: ['group-1', 'group-2'] });

      act(() => {
        result.current.setGroupName('Base Name');
        result.current.setAutoNumberGroups(true);
      });

      await act(async () => {
        await result.current.handleBulkAction('Alterar nome');
      });

      expect(mockUpdateGroupSubject).toHaveBeenCalledWith(
        expect.objectContaining({
          subject: '#1 Base Name',
        })
      );

      expect(mockUpdateGroupSubject).toHaveBeenCalledWith(
        expect.objectContaining({
          subject: '#2 Base Name',
        })
      );
    });

    it('should use default name with auto-numbering when groupName is empty', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setGroupName('');
        result.current.setAutoNumberGroups(true);
      });

      await act(async () => {
        await result.current.handleBulkAction('Alterar nome');
      });

      // Should use the default name from t('pages.groups.defaults.newGroupName')
      expect(mockUpdateGroupSubject).toHaveBeenCalledWith(
        expect.objectContaining({
          subject: '#1 pages.groups.defaults.newGroupName',
        })
      );
    });

    it('should error when name is empty and auto-number is disabled', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setGroupName('');
        result.current.setAutoNumberGroups(false);
      });

      await act(async () => {
        await result.current.handleBulkAction('Alterar nome');
      });

      // Should not call updateGroupSubject because the inner throw is caught
      expect(mockUpdateGroupSubject).not.toHaveBeenCalled();
      expect(mockCaptureHookError).toHaveBeenCalled();
    });

    it('should error when name is whitespace-only and auto-number is disabled', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setGroupName('   ');
        result.current.setAutoNumberGroups(false);
      });

      await act(async () => {
        await result.current.handleBulkAction('Alterar nome');
      });

      expect(mockUpdateGroupSubject).not.toHaveBeenCalled();
    });

    it('should handle updateGroupSubject returning false', async () => {
      mockUpdateGroupSubject.mockResolvedValue(false);

      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setGroupName('Name');
      });

      await act(async () => {
        await result.current.handleBulkAction('Alterar nome');
      });

      expect(mockCaptureHookError).toHaveBeenCalled();
    });
  });

  // ============================================================
  // Change Photo Action Tests
  // ============================================================

  describe('handleBulkAction - change photo', () => {
    it('should change group photo successfully', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      const imageFile = createMockFile('photo.jpg', 'image/jpeg', 1024);
      act(() => {
        result.current.handlePhotoUpload(createMockChangeEvent(imageFile));
      });

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
          base64_data: 'test',
          mimetype: 'image/jpeg',
        });
      });

      expect(result.current.groupPhoto).toBeNull();

      vi.unstubAllGlobals();
    });

    it('should error when no photo selected', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      await act(async () => {
        await result.current.handleBulkAction('Alterar foto');
      });

      expect(mockUpdateGroupPicture).not.toHaveBeenCalled();
      expect(mockCaptureHookError).toHaveBeenCalled();
    });

    it('should handle updateGroupPicture returning false', async () => {
      mockUpdateGroupPicture.mockResolvedValue(false);

      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      const imageFile = createMockFile('photo.jpg', 'image/jpeg', 1024);
      act(() => {
        result.current.handlePhotoUpload(createMockChangeEvent(imageFile));
      });

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

      expect(mockCaptureHookError).toHaveBeenCalled();
      vi.unstubAllGlobals();
    });
  });

  // ============================================================
  // Change Description Action Tests
  // ============================================================

  describe('handleBulkAction - change description', () => {
    it('should change group description successfully', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setDescription('New Description');
      });

      await act(async () => {
        await result.current.handleBulkAction('Alterar descri\u00e7\u00e3o');
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

      expect(result.current.description).toBe('');
    });

    it('should error when description is empty', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setDescription('');
      });

      await act(async () => {
        await result.current.handleBulkAction('Alterar descri\u00e7\u00e3o');
      });

      expect(mockUpdateGroupDescription).not.toHaveBeenCalled();
    });

    it('should error when description is whitespace only', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setDescription('   ');
      });

      await act(async () => {
        await result.current.handleBulkAction('Alterar descri\u00e7\u00e3o');
      });

      expect(mockUpdateGroupDescription).not.toHaveBeenCalled();
    });

    it('should handle updateGroupDescription returning false', async () => {
      mockUpdateGroupDescription.mockResolvedValue(false);

      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setDescription('Valid Description');
      });

      await act(async () => {
        await result.current.handleBulkAction('Alterar descri\u00e7\u00e3o');
      });

      expect(mockCaptureHookError).toHaveBeenCalled();
    });
  });

  // ============================================================
  // Unknown Action Tests
  // ============================================================

  describe('handleBulkAction - unknown action', () => {
    it('should handle unknown action without crashing', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      await act(async () => {
        await result.current.handleBulkAction('Unknown Action');
      });

      expect(mockSetLoading).toHaveBeenCalledWith(false);
      expect(mockCaptureHookError).toHaveBeenCalled();
    });
  });

  // ============================================================
  // Error Handling Tests
  // ============================================================

  describe('handleBulkAction - error handling', () => {
    it('should handle API exceptions gracefully', async () => {
      mockUpdateGroupSettings.mockRejectedValue(new Error('Network error'));

      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockCaptureHookError).toHaveBeenCalledWith(
        expect.any(Error),
        'useGroupsBulkActions',
        'executeBulkAction'
      );
      expect(mockSetLoading).toHaveBeenCalledWith(false);
    });

    it('should handle partial failures with correct toast', async () => {
      mockUpdateGroupSettings.mockResolvedValueOnce(true).mockResolvedValueOnce(false);

      const { result } = renderBulkActions({ selectedGroups: ['group-1', 'group-2'] });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.success.partialComplete',
        })
      );
    });

    it('should show all-error toast when all groups fail', async () => {
      mockUpdateGroupSettings.mockResolvedValue(false);

      const { result } = renderBulkActions({ selectedGroups: ['group-1', 'group-2'] });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'common.error',
          variant: 'destructive',
        })
      );
    });

    it('should show success toast when all groups succeed', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1', 'group-2'] });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.success.actionComplete',
        })
      );
    });

    it('should handle loadGroups failure in outer catch', async () => {
      mockLoadGroups.mockRejectedValue(new Error('Load failed'));

      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockCaptureHookError).toHaveBeenCalledWith(
        expect.any(Error),
        'useGroupsBulkActions',
        'executeAction'
      );

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
        })
      );
    });

    it('should handle non-Error exceptions in outer catch', async () => {
      mockLoadGroups.mockRejectedValue('string error');

      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          description: 'pages.groups.errors.bulkAction',
        })
      );
    });

    it('should handle non-Error exceptions in inner catch', async () => {
      mockUpdateGroupSettings.mockRejectedValue('string error');

      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockCaptureHookError).toHaveBeenCalledWith(
        'string error',
        'useGroupsBulkActions',
        'executeBulkAction'
      );
    });
  });

  // ============================================================
  // Loading State Tests
  // ============================================================

  describe('loading state', () => {
    it('should set loading true at start and false at end', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockSetLoading).toHaveBeenCalledWith(true);
      expect(mockSetLoading).toHaveBeenCalledWith(false);

      const calls = mockSetLoading.mock.calls;
      expect(calls[0][0]).toBe(true);
      expect(calls[calls.length - 1][0]).toBe(false);
    });

    it('should set loading false even when outer catch fires', async () => {
      mockLoadGroups.mockRejectedValue(new Error('fail'));

      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      const lastCall = mockSetLoading.mock.calls[mockSetLoading.mock.calls.length - 1];
      expect(lastCall[0]).toBe(false);
    });
  });

  // ============================================================
  // Cleanup Tests
  // ============================================================

  describe('cleanup after action', () => {
    it('should reset all state after successful action', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setGroupName('Test Name');
        result.current.setDescription('Test Description');
        result.current.setAutoNumberGroups(true);
        result.current.setChangeNameDialogOpen(true);
        result.current.setChangePhotoDialogOpen(true);
        result.current.setChangeDescriptionDialogOpen(true);
      });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(result.current.groupName).toBe('');
      expect(result.current.groupPhoto).toBeNull();
      expect(result.current.description).toBe('');
      expect(result.current.autoNumberGroups).toBe(false);
      expect(result.current.changeNameDialogOpen).toBe(false);
      expect(result.current.changePhotoDialogOpen).toBe(false);
      expect(result.current.changeDescriptionDialogOpen).toBe(false);
    });

    it('should call loadGroups after action', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockLoadGroups).toHaveBeenCalled();
    });

    it('should call clearSelection after action', async () => {
      const { result } = renderBulkActions({ selectedGroups: ['group-1'] });

      await act(async () => {
        await result.current.handleBulkAction('Fechar grupos');
      });

      expect(mockClearSelection).toHaveBeenCalled();
    });
  });
});
