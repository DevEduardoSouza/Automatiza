import type { Group } from '@/types/group';
import { act, renderHook, waitFor } from '@testing-library/react';
import type { ChangeEvent } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useGroupsMessaging } from '../use-groups-messaging';

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
const mockSendGroupMessage = vi.fn();
const mockSendGroupImage = vi.fn();
const mockGetGroupParticipants = vi.fn();

vi.mock('@/hooks/use-groups-connect', () => ({
  useGroupsConnect: () => ({
    sendGroupMessage: mockSendGroupMessage,
    sendGroupImage: mockSendGroupImage,
    getGroupParticipants: mockGetGroupParticipants,
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
const mockCheckInstanceStatus = vi.fn();

vi.mock('@/hooks/use-instances-connect', () => ({
  useInstancesConnect: () => ({
    latestInstance: mockLatestInstance,
    checkInstanceStatus: mockCheckInstanceStatus,
  }),
}));

vi.mock('@sentry/react', () => ({
  addBreadcrumb: vi.fn(),
  captureMessage: vi.fn(),
  captureException: vi.fn(),
}));

vi.mock('@/lib/sentry', () => ({
  captureHookError: vi.fn(),
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

describe('useGroupsMessaging', () => {
  const mockClearSelection = vi.fn();
  const mockGroups: Group[] = [
    createMockGroup({ id: 'group-1', wa_group_id: '123@g.us', name: 'Group 1' }),
    createMockGroup({ id: 'group-2', wa_group_id: '456@g.us', name: 'Group 2' }),
    createMockGroup({
      id: 'group-3',
      wa_group_id: '789@g.us',
      name: 'Group 3',
      status: 'closed',
      is_admin: false,
    }),
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    // Restore mockLatestInstance to connected state (may have been mutated by "no instance" tests)
    Object.assign(mockLatestInstance, {
      id: 'instance-1',
      user_id: 'user-1',
      instance_id: 'test-instance',
      status: 'connected',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    });
    mockSendGroupMessage.mockResolvedValue({ message_id: 'msg-1', action_id: 'action-1' });
    mockSendGroupImage.mockResolvedValue({ message_id: 'msg-1', action_id: 'action-1' });
    mockCheckInstanceStatus.mockResolvedValue({ success: true, state: 'open' });
    mockGetGroupParticipants.mockResolvedValue([
      { id: 'participant-1@s.whatsapp.net', phone_number: '5511111111111', is_admin: false, is_super_admin: false },
      { id: 'participant-2@s.whatsapp.net', phone_number: '5522222222222', is_admin: false, is_super_admin: false },
    ]);
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
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      expect(result.current.message).toBe('');
      expect(result.current.mediaFile).toBeNull();
      expect(result.current.mentionOpen).toBe(false);
      expect(result.current.sendMessageDialogOpen).toBe(false);
    });
  });

  // ============================================================
  // State Setters Tests
  // ============================================================

  describe('state setters', () => {
    it('should update message state', () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Hello, world!');
      });

      expect(result.current.message).toBe('Hello, world!');
    });

    it('should update mentionOpen state', () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMentionOpen(true);
      });

      expect(result.current.mentionOpen).toBe(true);
    });

    it('should update sendMessageDialogOpen state', () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setSendMessageDialogOpen(true);
      });

      expect(result.current.sendMessageDialogOpen).toBe(true);
    });
  });

  // ============================================================
  // insertMention Tests
  // ============================================================

  describe('insertMention', () => {
    it('should append mention to message', () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Hello ');
        result.current.insertMention('@todos');
      });

      expect(result.current.message).toBe('Hello @todos');
    });

    it('should close mention dropdown after inserting', () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMentionOpen(true);
        result.current.insertMention('@todos');
      });

      expect(result.current.mentionOpen).toBe(false);
    });

    it('should show success toast after inserting mention', () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.insertMention('@todos');
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.success.mentionAdded',
        description: expect.stringContaining('@todos'),
      });
    });
  });

  // ============================================================
  // Media Upload Tests
  // ============================================================

  describe('handleMediaUpload', () => {
    it('should accept valid image file', () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const imageFile = createMockFile('photo.jpg', 'image/jpeg', 1024);
      const event = createMockChangeEvent(imageFile);

      act(() => {
        result.current.handleMediaUpload(event);
      });

      expect(result.current.mediaFile).toBe(imageFile);
    });

    it('should accept valid video file', () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const videoFile = createMockFile('video.mp4', 'video/mp4', 1024);
      const event = createMockChangeEvent(videoFile);

      act(() => {
        result.current.handleMediaUpload(event);
      });

      expect(result.current.mediaFile).toBe(videoFile);
    });

    it('should reject non-image/video files', () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const pdfFile = createMockFile('document.pdf', 'application/pdf', 1024);
      const event = createMockChangeEvent(pdfFile);

      act(() => {
        result.current.handleMediaUpload(event);
      });

      expect(result.current.mediaFile).toBeNull();
      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.invalidFile',
        description: 'pages.groups.errors.onlyImagesAndVideos',
      });
    });

    it('should handle null file', () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const event = createMockChangeEvent(null);

      act(() => {
        result.current.handleMediaUpload(event);
      });

      expect(result.current.mediaFile).toBeNull();
    });
  });

  // ============================================================
  // removeMediaFile Tests
  // ============================================================

  describe('removeMediaFile', () => {
    it('should remove the selected media file', () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      // First add a file
      const imageFile = createMockFile('photo.jpg', 'image/jpeg', 1024);
      act(() => {
        result.current.handleMediaUpload(createMockChangeEvent(imageFile));
      });

      expect(result.current.mediaFile).not.toBeNull();

      // Remove it
      act(() => {
        result.current.removeMediaFile();
      });

      expect(result.current.mediaFile).toBeNull();
    });
  });

  // ============================================================
  // handleSendMessage Validation Tests
  // ============================================================

  describe('handleSendMessage - validation', () => {
    it('should show error when no groups selected', async () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Test message');
      });

      await act(async () => {
        await result.current.handleSendMessage();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.noGroupSelected',
        description: 'pages.groups.errors.noGroupSelectedMessage',
      });
      expect(mockSendGroupMessage).not.toHaveBeenCalled();
    });

    it('should show error when message is empty', async () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      await act(async () => {
        await result.current.handleSendMessage();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.emptyMessage',
        description: 'pages.groups.errors.emptyMessageDesc',
      });
    });

    it('should show error when message is only whitespace', async () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('   ');
      });

      await act(async () => {
        await result.current.handleSendMessage();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.emptyMessage',
        description: 'pages.groups.errors.emptyMessageDesc',
      });
    });
  });

  // ============================================================
  // Instance Connection Tests
  // ============================================================

  describe('handleSendMessage - instance connection', () => {
    it('should check connection status before sending', async () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Test message');
      });

      const sendPromise = result.current.handleSendMessage();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockCheckInstanceStatus).toHaveBeenCalledWith('test-instance');

      vi.useRealTimers();
    });

    it('should show error when WhatsApp is disconnected', async () => {
      mockCheckInstanceStatus.mockResolvedValue({ success: true, state: 'close' });

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Test message');
      });

      await act(async () => {
        await result.current.handleSendMessage();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.whatsappDisconnected',
        description: 'pages.groups.errors.reconnectWhatsApp',
      });
    });

    it('should show error when status check fails', async () => {
      mockCheckInstanceStatus.mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Test message');
      });

      await act(async () => {
        await result.current.handleSendMessage();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.verifyConnection',
        description: 'pages.groups.errors.verifyConnectionMessage',
      });
    });
  });

  // ============================================================
  // Send Text Message Tests
  // ============================================================

  describe('handleSendMessage - text message', () => {
    it('should send text message successfully', async () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Hello, world!');
        result.current.setSendMessageDialogOpen(true);
      });

      const sendPromise = result.current.handleSendMessage();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockSendGroupMessage).toHaveBeenCalledWith({
        instance_name: 'test-instance',
        group_id: '123@g.us',
        message: 'Hello, world!',
        mentions: undefined,
      });

      // Verify cleanup
      expect(result.current.message).toBe('');
      expect(result.current.sendMessageDialogOpen).toBe(false);
      expect(mockClearSelection).toHaveBeenCalled();

      vi.useRealTimers();
    });

    it('should send message to multiple groups with rate limiting', async () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1', 'group-2'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Hello, everyone!');
      });

      const sendPromise = result.current.handleSendMessage();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(5000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockSendGroupMessage).toHaveBeenCalledTimes(2);

      vi.useRealTimers();
    });
  });

  // ============================================================
  // Send Image Message Tests
  // ============================================================

  describe('handleSendMessage - image message', () => {
    it('should send image with caption', async () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const imageFile = createMockFile('photo.jpg', 'image/jpeg', 1024);

      act(() => {
        result.current.setMessage('Check out this photo!');
        result.current.handleMediaUpload(createMockChangeEvent(imageFile));
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
        await result.current.handleSendMessage();
      });

      await waitFor(() => {
        expect(mockSendGroupImage).toHaveBeenCalledWith({
          instance_name: 'test-instance',
          group_id: '123@g.us',
          image: 'data:image/jpeg;base64,test',
          caption: 'Check out this photo!',
          mimetype: 'image/jpeg',
        });
      });

      // Verify cleanup
      expect(result.current.mediaFile).toBeNull();

      vi.unstubAllGlobals();
    });
  });

  // ============================================================
  // Mention All (@todos) Tests
  // ============================================================

  describe('handleSendMessage - mentions', () => {
    it('should fetch participants when message contains @todos', async () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Hello @todos!');
      });

      const sendPromise = result.current.handleSendMessage();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockGetGroupParticipants).toHaveBeenCalledWith({
        instance_name: 'test-instance',
        group_id: '123@g.us',
      });

      expect(mockSendGroupMessage).toHaveBeenCalledWith(
        expect.objectContaining({
          mentions: ['participant-1@s.whatsapp.net', 'participant-2@s.whatsapp.net'],
        })
      );

      vi.useRealTimers();
    });

    it('should handle empty participant list for @todos', async () => {
      mockGetGroupParticipants.mockResolvedValue([]);

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Hello @todos!');
      });

      await act(async () => {
        await result.current.handleSendMessage();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          description: 'pages.groups.errors.emptyGroupMention',
        })
      );
    });

    it('should handle failed participant fetch for @todos', async () => {
      mockGetGroupParticipants.mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Hello @todos!');
      });

      await act(async () => {
        await result.current.handleSendMessage();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          description: 'pages.groups.errors.fetchParticipantsFailed',
        })
      );
    });
  });

  // ============================================================
  // Error Handling Tests
  // ============================================================

  describe('handleSendMessage - error handling', () => {
    it('should handle send failure and count errors', async () => {
      vi.useFakeTimers();

      mockSendGroupMessage.mockRejectedValue(new Error('Send failed'));

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Test');
      });

      const sendPromise = result.current.handleSendMessage();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      // Should show success toast with error count
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.success.messagesSent',
        })
      );

      vi.useRealTimers();
    });

    it('should handle partial failures in multiple groups', async () => {
      vi.useFakeTimers();

      // First succeeds, second fails
      mockSendGroupMessage
        .mockResolvedValueOnce({ message_id: 'msg-1', action_id: 'action-1' })
        .mockRejectedValueOnce(new Error('Network error'));

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1', 'group-2'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Test');
      });

      const sendPromise = result.current.handleSendMessage();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(5000);
      });

      await act(async () => {
        await sendPromise;
      });

      // Should show results with both success and error counts
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          description: expect.stringContaining('1'),
        })
      );

      vi.useRealTimers();
    });
  });

  // ============================================================
  // Edge Cases
  // ============================================================

  describe('edge cases', () => {
    it('should handle very long messages', async () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const longMessage = 'A'.repeat(4096); // Max message length

      act(() => {
        result.current.setMessage(longMessage);
      });

      const sendPromise = result.current.handleSendMessage();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockSendGroupMessage).toHaveBeenCalledWith(
        expect.objectContaining({
          message: longMessage,
        })
      );

      vi.useRealTimers();
    });

    it('should handle special characters in message', async () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const specialMessage = 'Hello "world" with <tags> & symbols!';

      act(() => {
        result.current.setMessage(specialMessage);
      });

      const sendPromise = result.current.handleSendMessage();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockSendGroupMessage).toHaveBeenCalledWith(
        expect.objectContaining({
          message: specialMessage,
        })
      );

      vi.useRealTimers();
    });

    it('should handle unicode and emojis in message', async () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const unicodeMessage = 'Test test';

      act(() => {
        result.current.setMessage(unicodeMessage);
      });

      const sendPromise = result.current.handleSendMessage();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockSendGroupMessage).toHaveBeenCalledWith(
        expect.objectContaining({
          message: unicodeMessage,
        })
      );

      vi.useRealTimers();
    });
  });

  // ============================================================
  // File Size Validation Tests
  // ============================================================

  describe('handleMediaUpload - file size validation', () => {
    it('should reject files larger than 7MB', () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const largeFile = createMockFile('huge.jpg', 'image/jpeg', 8 * 1024 * 1024);
      const event = createMockChangeEvent(largeFile);

      act(() => {
        result.current.handleMediaUpload(event);
      });

      expect(result.current.mediaFile).toBeNull();
      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.fileTooLarge',
        description: 'pages.groups.errors.fileTooLargeMessage',
      });
    });

    it('should accept files exactly at 7MB', () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const file = createMockFile('exact.jpg', 'image/jpeg', 7 * 1024 * 1024);
      const event = createMockChangeEvent(file);

      act(() => {
        result.current.handleMediaUpload(event);
      });

      expect(result.current.mediaFile).toBe(file);
    });
  });

  // ============================================================
  // Individual @number Mentions Tests
  // ============================================================

  describe('handleSendMessage - individual @number mentions', () => {
    it('should detect and send individual phone mentions', async () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Hello @5511999999999 check this!');
      });

      const sendPromise = result.current.handleSendMessage();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockSendGroupMessage).toHaveBeenCalledWith(
        expect.objectContaining({
          mentions: ['5511999999999@s.whatsapp.net'],
        })
      );

      vi.useRealTimers();
    });

    it('should detect multiple individual mentions', async () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Hey @5511999999999 and @5511888888888');
      });

      const sendPromise = result.current.handleSendMessage();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockSendGroupMessage).toHaveBeenCalledWith(
        expect.objectContaining({
          mentions: ['5511999999999@s.whatsapp.net', '5511888888888@s.whatsapp.net'],
        })
      );

      vi.useRealTimers();
    });

    it('should NOT match short numbers (less than 8 digits)', async () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Hey @1234567');
      });

      const sendPromise = result.current.handleSendMessage();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockSendGroupMessage).toHaveBeenCalledWith(
        expect.objectContaining({
          mentions: undefined,
        })
      );

      vi.useRealTimers();
    });
  });

  // ============================================================
  // No Instance Connected Tests
  // ============================================================

  describe('handleSendMessage - no instance', () => {
    it('should show error when latestInstance is null', async () => {
      // Override mock with null instance
      const originalMock = mockLatestInstance;
      Object.assign(mockLatestInstance, {
        id: '',
        user_id: '',
        instance_id: '',
        status: 'disconnected',
      });

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setMessage('Test');
      });

      await act(async () => {
        await result.current.handleSendMessage();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.noInstance',
        description: 'pages.groups.errors.connectWhatsAppFirst',
      });

      // Restore
      Object.assign(mockLatestInstance, originalMock);
    });
  });

  // ============================================================
  // File Read Error Tests
  // ============================================================

  describe('handleSendMessage - file read error', () => {
    it('should show error and NOT send when file read fails', async () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const imageFile = createMockFile('broken.jpg', 'image/jpeg', 1024);

      act(() => {
        result.current.setMessage('Caption');
        result.current.handleMediaUpload(createMockChangeEvent(imageFile));
      });

      // Mock FileReader that fails
      class FailingFileReader {
        result: string | null = null;
        onload: ((e: ProgressEvent<FileReader>) => void) | null = null;
        onerror: ((e: ProgressEvent<FileReader>) => void) | null = null;
        readAsDataURL() {
          setTimeout(() => {
            if (this.onerror) {
              this.onerror(new ProgressEvent('error') as ProgressEvent<FileReader>);
            }
          }, 0);
        }
      }
      vi.stubGlobal('FileReader', FailingFileReader);

      await act(async () => {
        await result.current.handleSendMessage();
      });

      // Should NOT have called sendGroupImage
      expect(mockSendGroupImage).not.toHaveBeenCalled();
      expect(mockSendGroupMessage).not.toHaveBeenCalled();

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.groups.errors.processImage',
        })
      );

      vi.unstubAllGlobals();
    });
  });

  // ============================================================
  // Mention Participants Loading Tests
  // ============================================================

  describe('mention participants', () => {
    it('should load participants when mention opens', async () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      await act(async () => {
        result.current.setMentionOpen(true);
      });

      expect(result.current.mentionOpen).toBe(true);
      expect(mockGetGroupParticipants).toHaveBeenCalledWith({
        instance_name: 'test-instance',
        group_id: '123@g.us',
      });
    });

    it('should NOT load participants when no groups selected', async () => {
      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      await act(async () => {
        result.current.setMentionOpen(true);
      });

      expect(mockGetGroupParticipants).not.toHaveBeenCalled();
    });

    it('should deduplicate participants by phone_number across groups', async () => {
      // Both groups return same participant
      mockGetGroupParticipants.mockResolvedValue([
        {
          id: 'same@s.whatsapp.net',
          phone_number: '5511999999999',
          is_admin: false,
          is_super_admin: false,
        },
      ]);

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: ['group-1', 'group-2'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      await act(async () => {
        result.current.setMentionOpen(true);
      });

      // Should have only one entry despite two groups
      expect(result.current.mentionParticipants).toHaveLength(1);
    });

    it('should limit participant fetch to 5 groups', async () => {
      const manyGroups: Group[] = Array.from({ length: 10 }, (_, i) =>
        createMockGroup({
          id: `group-${i}`,
          wa_group_id: `${i}@g.us`,
          name: `Group ${i}`,
        })
      );

      mockGetGroupParticipants.mockResolvedValue([]);

      const { result } = renderHook(() =>
        useGroupsMessaging({
          selectedGroups: manyGroups.map((g) => g.id),
          groups: manyGroups,
          clearSelection: mockClearSelection,
        })
      );

      await act(async () => {
        result.current.setMentionOpen(true);
      });

      // Should only fetch for first 5 groups
      expect(mockGetGroupParticipants).toHaveBeenCalledTimes(5);
    });
  });
});
