import type { Group } from '@/types/group';
import { act, renderHook, waitFor } from '@testing-library/react';
import type { ChangeEvent } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useGroupsAudio } from '../use-groups-audio';

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

const mockSendGroupAudio = vi.fn();

vi.mock('@/hooks/use-groups-connect', () => ({
  useGroupsConnect: () => ({
    sendGroupAudio: mockSendGroupAudio,
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

function stubFileReader(base64Result: string) {
  class MockFileReader {
    result: string | null = null;
    onload: ((e: ProgressEvent<FileReader>) => void) | null = null;
    onerror: ((e: ProgressEvent<FileReader>) => void) | null = null;
    readAsDataURL() {
      setTimeout(() => {
        this.result = base64Result;
        if (this.onload) {
          this.onload({} as ProgressEvent<FileReader>);
        }
      }, 0);
    }
  }
  vi.stubGlobal('FileReader', MockFileReader);
}

// ============================================================
// Test Setup
// ============================================================

describe('useGroupsAudio', () => {
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

  function renderAudio(overrides: Partial<Parameters<typeof useGroupsAudio>[0]> = {}) {
    return renderHook(() =>
      useGroupsAudio({
        selectedGroups: [],
        groups: mockGroups,
        clearSelection: mockClearSelection,
        ...overrides,
      })
    );
  }

  beforeEach(() => {
    vi.clearAllMocks();
    mockSendGroupAudio.mockResolvedValue({ message_id: 'msg-1', action_id: 'action-1' });
  });

  afterEach(() => {
    vi.resetAllMocks();
    vi.unstubAllGlobals();
  });

  // ============================================================
  // Initial State Tests
  // ============================================================

  describe('initial state', () => {
    it('should have correct initial values', () => {
      const { result } = renderAudio();

      expect(result.current.audioFile).toBeNull();
      expect(result.current.audioPtt).toBe(true);
      expect(result.current.audioCaption).toBe('');
      expect(result.current.sendingAudio).toBe(false);
      expect(result.current.sendAudioDialogOpen).toBe(false);
    });

    it('should expose all required functions', () => {
      const { result } = renderAudio();

      expect(typeof result.current.setAudioPtt).toBe('function');
      expect(typeof result.current.setAudioCaption).toBe('function');
      expect(typeof result.current.setSendAudioDialogOpen).toBe('function');
      expect(typeof result.current.handleAudioUpload).toBe('function');
      expect(typeof result.current.removeAudioFile).toBe('function');
      expect(typeof result.current.handleSendAudio).toBe('function');
    });
  });

  // ============================================================
  // Audio File Upload Tests
  // ============================================================

  describe('handleAudioUpload', () => {
    it('should accept valid audio file under 4MB', () => {
      const { result } = renderAudio();

      const validFile = createMockFile('audio.ogg', 'audio/ogg', 1024 * 1024);
      const event = createMockChangeEvent(validFile);

      act(() => {
        result.current.handleAudioUpload(event);
      });

      expect(result.current.audioFile).toBe(validFile);
      expect(mockToast).not.toHaveBeenCalled();
    });

    it('should reject audio file larger than 4MB', () => {
      const { result } = renderAudio();

      const largeFile = createMockFile('large-audio.ogg', 'audio/ogg', 5 * 1024 * 1024);
      const event = createMockChangeEvent(largeFile);

      act(() => {
        result.current.handleAudioUpload(event);
      });

      expect(result.current.audioFile).toBeNull();
      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.audioTooLarge',
        description: 'pages.groups.errors.audioTooLargeMessage',
      });
    });

    it('should accept audio file exactly at 4MB limit', () => {
      const { result } = renderAudio();

      const exactLimitFile = createMockFile('audio.ogg', 'audio/ogg', 4 * 1024 * 1024);
      const event = createMockChangeEvent(exactLimitFile);

      act(() => {
        result.current.handleAudioUpload(event);
      });

      expect(result.current.audioFile).toBe(exactLimitFile);
    });

    it('should reject file at 4MB + 1 byte', () => {
      const { result } = renderAudio();

      const overLimitFile = createMockFile('audio.ogg', 'audio/ogg', 4 * 1024 * 1024 + 1);
      const event = createMockChangeEvent(overLimitFile);

      act(() => {
        result.current.handleAudioUpload(event);
      });

      expect(result.current.audioFile).toBeNull();
      expect(mockToast).toHaveBeenCalled();
    });

    it('should handle null file in event (early return)', () => {
      const { result } = renderAudio();

      const validFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(validFile));
      });

      expect(result.current.audioFile).not.toBeNull();

      const event = createMockChangeEvent(null);
      act(() => {
        result.current.handleAudioUpload(event);
      });

      // File should remain unchanged when no new file provided (early return)
      expect(result.current.audioFile).toBe(validFile);
    });

    it('should clear input value when file is too large', () => {
      const { result } = renderAudio();

      const event = {
        target: {
          files: [createMockFile('large.ogg', 'audio/ogg', 5 * 1024 * 1024)],
          value: 'large.ogg',
        },
      } as unknown as ChangeEvent<HTMLInputElement>;

      act(() => {
        result.current.handleAudioUpload(event);
      });

      // The hook sets e.target.value = '' for rejected files
      expect((event.target as HTMLInputElement).value).toBe('');
    });
  });

  // ============================================================
  // Remove Audio File Tests
  // ============================================================

  describe('removeAudioFile', () => {
    it('should remove the selected audio file', () => {
      const { result } = renderAudio();

      const validFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(validFile));
      });

      expect(result.current.audioFile).not.toBeNull();

      act(() => {
        result.current.removeAudioFile();
      });

      expect(result.current.audioFile).toBeNull();
    });

    it('should handle removing when no file is set', () => {
      const { result } = renderAudio();

      expect(result.current.audioFile).toBeNull();

      act(() => {
        result.current.removeAudioFile();
      });

      expect(result.current.audioFile).toBeNull();
    });
  });

  // ============================================================
  // State Setters Tests
  // ============================================================

  describe('state setters', () => {
    it('should update audioPtt state', () => {
      const { result } = renderAudio();

      expect(result.current.audioPtt).toBe(true);

      act(() => {
        result.current.setAudioPtt(false);
      });

      expect(result.current.audioPtt).toBe(false);
    });

    it('should update audioCaption state', () => {
      const { result } = renderAudio();

      act(() => {
        result.current.setAudioCaption('Test caption');
      });

      expect(result.current.audioCaption).toBe('Test caption');
    });

    it('should update sendAudioDialogOpen state', () => {
      const { result } = renderAudio();

      act(() => {
        result.current.setSendAudioDialogOpen(true);
      });

      expect(result.current.sendAudioDialogOpen).toBe(true);
    });
  });

  // ============================================================
  // handleSendAudio Validation Tests
  // ============================================================

  describe('handleSendAudio - validation', () => {
    it('should show error when no groups selected', async () => {
      const { result } = renderAudio({ selectedGroups: [] });

      await act(async () => {
        await result.current.handleSendAudio();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.noGroupSelected',
        description: 'pages.groups.errors.noGroupSelectedMessage',
      });
      expect(mockSendGroupAudio).not.toHaveBeenCalled();
    });

    it('should show error when more than 10 groups selected', async () => {
      const manyGroups = Array.from({ length: 11 }, (_, i) =>
        createMockGroup({ id: `group-${i}` })
      );
      const selectedIds = manyGroups.map((g) => g.id);

      const { result } = renderAudio({
        selectedGroups: selectedIds,
        groups: manyGroups,
      });

      await act(async () => {
        await result.current.handleSendAudio();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.groupLimit',
        description: 'pages.groups.errors.audioGroupLimit',
      });
    });

    it('should allow exactly 10 groups', async () => {
      const tenGroups = Array.from({ length: 10 }, (_, i) =>
        createMockGroup({ id: `group-${i}`, wa_group_id: `${i}@g.us` })
      );
      const selectedIds = tenGroups.map((g) => g.id);

      const { result } = renderAudio({
        selectedGroups: selectedIds,
        groups: tenGroups,
      });

      // Should pass the 10-group check but fail at the "no audio" check
      await act(async () => {
        await result.current.handleSendAudio();
      });

      // Should NOT have the groupLimit error
      expect(mockToast).not.toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.errors.groupLimit',
        })
      );
    });

    it('should show error when no audio file selected', async () => {
      const { result } = renderAudio({ selectedGroups: ['group-1'] });

      await act(async () => {
        await result.current.handleSendAudio();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.noAudioSelected',
        description: 'pages.groups.errors.selectAudioFirst',
      });
    });

    it('should require OGG format for PTT audio', async () => {
      const { result } = renderAudio({ selectedGroups: ['group-1'] });

      const mp3File = createMockFile('audio.mp3', 'audio/mpeg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(mp3File));
        result.current.setAudioPtt(true);
      });

      await act(async () => {
        await result.current.handleSendAudio();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.invalidPttFormat',
        description: 'pages.groups.errors.pttFormatMessage',
      });
    });

    it('should allow non-OGG format when PTT is disabled', async () => {
      const { result } = renderAudio({ selectedGroups: ['group-1'] });

      const mp3File = createMockFile('audio.mp3', 'audio/mpeg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(mp3File));
        result.current.setAudioPtt(false);
      });

      stubFileReader('data:audio/mpeg;base64,test');

      await act(async () => {
        await result.current.handleSendAudio();
      });

      expect(mockToast).not.toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.errors.invalidPttFormat',
        })
      );
    });

    it('should show toast when instance is not connected', async () => {
      const originalStatus = mockLatestInstance.status;
      Object.assign(mockLatestInstance, { status: 'disconnected' });

      const { result } = renderAudio({ selectedGroups: ['group-1'] });

      const oggFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(oggFile));
      });

      await act(async () => {
        await result.current.handleSendAudio();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.noInstance',
        description: 'pages.groups.errors.connectWhatsAppFirst',
      });

      Object.assign(mockLatestInstance, { status: originalStatus });
    });

    it('should validate in order: groups, limit, audio, ptt, instance', async () => {
      const { result } = renderAudio({ selectedGroups: [] });

      await act(async () => {
        await result.current.handleSendAudio();
      });

      // First validation: no groups
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.errors.noGroupSelected',
        })
      );
    });
  });

  // ============================================================
  // handleSendAudio Success Tests
  // ============================================================

  describe('handleSendAudio - success', () => {
    it('should send audio successfully and show success toast', async () => {
      const { result } = renderAudio({ selectedGroups: ['group-1'] });

      const audioFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(audioFile));
        result.current.setAudioCaption('Test caption');
        result.current.setSendAudioDialogOpen(true);
      });

      stubFileReader('data:audio/ogg;base64,test');

      await act(async () => {
        await result.current.handleSendAudio();
      });

      await waitFor(() => {
        expect(mockSendGroupAudio).toHaveBeenCalledWith({
          instance_name: 'test-instance',
          group_id: '123@g.us',
          audio: 'data:audio/ogg;base64,test',
          mimetype: 'audio/ogg',
          ptt: true,
          caption: 'Test caption',
        });
      });

      // Verify cleanup
      expect(result.current.audioFile).toBeNull();
      expect(result.current.audioCaption).toBe('');
      expect(result.current.sendAudioDialogOpen).toBe(false);
      expect(mockClearSelection).toHaveBeenCalled();
    });

    it('should not send caption when it is empty or whitespace', async () => {
      const { result } = renderAudio({ selectedGroups: ['group-1'] });

      const audioFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(audioFile));
        result.current.setAudioCaption('   ');
      });

      stubFileReader('data:audio/ogg;base64,test');

      await act(async () => {
        await result.current.handleSendAudio();
      });

      await waitFor(() => {
        expect(mockSendGroupAudio).toHaveBeenCalledWith(
          expect.objectContaining({
            caption: undefined,
          })
        );
      });
    });

    it('should handle multiple groups with rate limiting', async () => {
      vi.useFakeTimers();

      const { result } = renderAudio({ selectedGroups: ['group-1', 'group-2'] });

      const audioFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(audioFile));
      });

      // Use immediate FileReader for fake timers
      class MockFileReader {
        result: string | null = null;
        onload: ((e: ProgressEvent<FileReader>) => void) | null = null;
        onerror: ((e: ProgressEvent<FileReader>) => void) | null = null;
        readAsDataURL() {
          this.result = 'data:audio/ogg;base64,test';
          Promise.resolve().then(() => {
            if (this.onload) {
              this.onload({} as ProgressEvent<FileReader>);
            }
          });
        }
      }
      vi.stubGlobal('FileReader', MockFileReader);

      const sendPromise = result.current.handleSendAudio();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockSendGroupAudio).toHaveBeenCalledTimes(2);

      vi.useRealTimers();
    });

    it('should return early when selectedGroupsData is empty', async () => {
      // selectedGroups has IDs that don't match any group in the groups array
      const { result } = renderAudio({
        selectedGroups: ['nonexistent-id'],
        groups: mockGroups,
      });

      const audioFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(audioFile));
      });

      stubFileReader('data:audio/ogg;base64,test');

      await act(async () => {
        await result.current.handleSendAudio();
      });

      expect(mockSendGroupAudio).not.toHaveBeenCalled();
    });
  });

  // ============================================================
  // Error Handling Tests
  // ============================================================

  describe('handleSendAudio - error handling', () => {
    it('should handle send failure and count errors', async () => {
      mockSendGroupAudio.mockRejectedValue(new Error('Network error'));

      const { result } = renderAudio({ selectedGroups: ['group-1'] });

      const audioFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(audioFile));
      });

      stubFileReader('data:audio/ogg;base64,test');

      await act(async () => {
        await result.current.handleSendAudio();
      });

      await waitFor(() => {
        expect(mockToast).toHaveBeenCalledWith(
          expect.objectContaining({
            title: 'pages.groups.success.audioSent',
          })
        );
      });

      expect(mockCaptureHookError).toHaveBeenCalledWith(
        expect.any(Error),
        'useGroupsAudio',
        'sendAudioToGroup'
      );
    });

    it('should always reset sendingAudio via finally block', async () => {
      mockSendGroupAudio.mockRejectedValue(new Error('Network error'));

      const { result } = renderAudio({ selectedGroups: ['group-1'] });

      const audioFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(audioFile));
      });

      stubFileReader('data:audio/ogg;base64,test');

      await act(async () => {
        await result.current.handleSendAudio();
      });

      expect(result.current.sendingAudio).toBe(false);
    });

    it('should handle partial failures across groups', async () => {
      vi.useFakeTimers();

      mockSendGroupAudio
        .mockResolvedValueOnce({ message_id: 'msg-1' })
        .mockRejectedValueOnce(new Error('fail'));

      const { result } = renderAudio({ selectedGroups: ['group-1', 'group-2'] });

      const audioFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(audioFile));
      });

      class MockFileReader {
        result: string | null = null;
        onload: ((e: ProgressEvent<FileReader>) => void) | null = null;
        onerror: ((e: ProgressEvent<FileReader>) => void) | null = null;
        readAsDataURL() {
          this.result = 'data:audio/ogg;base64,test';
          Promise.resolve().then(() => {
            if (this.onload) {
              this.onload({} as ProgressEvent<FileReader>);
            }
          });
        }
      }
      vi.stubGlobal('FileReader', MockFileReader);

      const sendPromise = result.current.handleSendAudio();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      await act(async () => {
        await sendPromise;
      });

      // Toast shows combined success and error counts
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.success.audioSent',
          description: expect.stringContaining('1'),
        })
      );

      vi.useRealTimers();
    });
  });

  // ============================================================
  // PTT Format Validation Tests
  // ============================================================

  describe('PTT format validation', () => {
    it('should accept .ogg extension for PTT', async () => {
      const { result } = renderAudio({ selectedGroups: ['group-1'] });

      const oggFile = createMockFile('voice.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(oggFile));
        result.current.setAudioPtt(true);
      });

      stubFileReader('data:audio/ogg;base64,test');

      await act(async () => {
        await result.current.handleSendAudio();
      });

      expect(mockToast).not.toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.errors.invalidPttFormat',
        })
      );
    });

    it('should accept audio/ogg mime type for PTT even with different extension', async () => {
      const { result } = renderAudio({ selectedGroups: ['group-1'] });

      const oggFile = createMockFile('voice.opus', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(oggFile));
        result.current.setAudioPtt(true);
      });

      stubFileReader('data:audio/ogg;base64,test');

      await act(async () => {
        await result.current.handleSendAudio();
      });

      expect(mockToast).not.toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.errors.invalidPttFormat',
        })
      );
    });

    it('should accept .OGG extension (case insensitive)', async () => {
      const { result } = renderAudio({ selectedGroups: ['group-1'] });

      const oggFile = createMockFile('voice.OGG', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(oggFile));
        result.current.setAudioPtt(true);
      });

      stubFileReader('data:audio/ogg;base64,test');

      await act(async () => {
        await result.current.handleSendAudio();
      });

      expect(mockToast).not.toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.errors.invalidPttFormat',
        })
      );
    });

    it('should reject .wav for PTT', async () => {
      const { result } = renderAudio({ selectedGroups: ['group-1'] });

      const wavFile = createMockFile('voice.wav', 'audio/wav', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(wavFile));
        result.current.setAudioPtt(true);
      });

      await act(async () => {
        await result.current.handleSendAudio();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.errors.invalidPttFormat',
        })
      );
    });
  });
});
