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

// Mock useGroupsConnect
const mockSendGroupAudio = vi.fn();

vi.mock('@/hooks/use-groups-connect', () => ({
  useGroupsConnect: () => ({
    sendGroupAudio: mockSendGroupAudio,
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

  beforeEach(() => {
    vi.clearAllMocks();
    mockSendGroupAudio.mockResolvedValue({ message_id: 'msg-1', action_id: 'action-1' });
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
        useGroupsAudio({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      expect(result.current.audioFile).toBeNull();
      expect(result.current.audioPtt).toBe(true);
      expect(result.current.audioCaption).toBe('');
      expect(result.current.sendingAudio).toBe(false);
      expect(result.current.sendAudioDialogOpen).toBe(false);
    });
  });

  // ============================================================
  // Audio File Upload Tests
  // ============================================================

  describe('handleAudioUpload', () => {
    it('should accept valid audio file under 4MB', () => {
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const validFile = createMockFile('audio.ogg', 'audio/ogg', 1024 * 1024); // 1MB
      const event = createMockChangeEvent(validFile);

      act(() => {
        result.current.handleAudioUpload(event);
      });

      expect(result.current.audioFile).toBe(validFile);
      expect(mockToast).not.toHaveBeenCalled();
    });

    it('should reject audio file larger than 4MB', () => {
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const largeFile = createMockFile('large-audio.ogg', 'audio/ogg', 5 * 1024 * 1024); // 5MB
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
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const exactLimitFile = createMockFile('audio.ogg', 'audio/ogg', 4 * 1024 * 1024); // Exactly 4MB
      const event = createMockChangeEvent(exactLimitFile);

      act(() => {
        result.current.handleAudioUpload(event);
      });

      expect(result.current.audioFile).toBe(exactLimitFile);
    });

    it('should handle null file in event', () => {
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      // First set a file
      const validFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(validFile));
      });

      expect(result.current.audioFile).not.toBeNull();

      // Now clear it by passing null
      const event = createMockChangeEvent(null);
      act(() => {
        result.current.handleAudioUpload(event);
      });

      // File should remain unchanged when no new file provided
      expect(result.current.audioFile).toBe(validFile);
    });
  });

  // ============================================================
  // Remove Audio File Tests
  // ============================================================

  describe('removeAudioFile', () => {
    it('should remove the selected audio file', () => {
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      // First add a file
      const validFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(validFile));
      });

      expect(result.current.audioFile).not.toBeNull();

      // Remove it
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
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      expect(result.current.audioPtt).toBe(true);

      act(() => {
        result.current.setAudioPtt(false);
      });

      expect(result.current.audioPtt).toBe(false);
    });

    it('should update audioCaption state', () => {
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      act(() => {
        result.current.setAudioCaption('Test caption');
      });

      expect(result.current.audioCaption).toBe('Test caption');
    });

    it('should update sendAudioDialogOpen state', () => {
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

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
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: [],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

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

      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: selectedIds,
          groups: manyGroups,
          clearSelection: mockClearSelection,
        })
      );

      await act(async () => {
        await result.current.handleSendAudio();
      });

      expect(mockToast).toHaveBeenCalledWith({
        variant: 'destructive',
        title: 'pages.groups.errors.groupLimit',
        description: 'pages.groups.errors.audioGroupLimit',
      });
    });

    it('should show error when no audio file selected', async () => {
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

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
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      // Set PTT to true and use non-OGG file
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
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const mp3File = createMockFile('audio.mp3', 'audio/mpeg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(mp3File));
        result.current.setAudioPtt(false);
      });

      // Mock FileReader class properly
      class MockFileReader {
        result: string | null = null;
        onload: ((e: ProgressEvent<FileReader>) => void) | null = null;
        onerror: ((e: ProgressEvent<FileReader>) => void) | null = null;
        readAsDataURL() {
          setTimeout(() => {
            this.result = 'data:audio/mpeg;base64,test';
            if (this.onload) {
              this.onload({} as ProgressEvent<FileReader>);
            }
          }, 0);
        }
      }
      vi.stubGlobal('FileReader', MockFileReader);

      await act(async () => {
        await result.current.handleSendAudio();
      });

      // Should not show format error
      expect(mockToast).not.toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.errors.invalidPttFormat',
        })
      );

      vi.unstubAllGlobals();
    });
  });

  // ============================================================
  // handleSendAudio Success Tests
  // ============================================================

  describe('handleSendAudio - success', () => {
    it('should send audio successfully and show success toast', async () => {
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const audioFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(audioFile));
        result.current.setAudioCaption('Test caption');
        result.current.setSendAudioDialogOpen(true);
      });

      // Mock FileReader class properly
      class MockFileReader {
        result: string | null = null;
        onload: ((e: ProgressEvent<FileReader>) => void) | null = null;
        onerror: ((e: ProgressEvent<FileReader>) => void) | null = null;
        readAsDataURL() {
          setTimeout(() => {
            this.result = 'data:audio/ogg;base64,test';
            if (this.onload) {
              this.onload({} as ProgressEvent<FileReader>);
            }
          }, 0);
        }
      }
      vi.stubGlobal('FileReader', MockFileReader);

      await act(async () => {
        await result.current.handleSendAudio();
      });

      // Verify the audio was sent
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

      vi.unstubAllGlobals();
    });

    it('should handle multiple groups with rate limiting', async () => {
      vi.useFakeTimers();

      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: ['group-1', 'group-2'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const audioFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(audioFile));
      });

      // Mock FileReader class properly
      class MockFileReader {
        result: string | null = null;
        onload: ((e: ProgressEvent<FileReader>) => void) | null = null;
        onerror: ((e: ProgressEvent<FileReader>) => void) | null = null;
        readAsDataURL() {
          this.result = 'data:audio/ogg;base64,test';
          // Immediately call onload for fake timers
          Promise.resolve().then(() => {
            if (this.onload) {
              this.onload({} as ProgressEvent<FileReader>);
            }
          });
        }
      }
      vi.stubGlobal('FileReader', MockFileReader);

      const sendPromise = result.current.handleSendAudio();

      // Advance timers to handle rate limiting delays
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000);
      });

      await act(async () => {
        await sendPromise;
      });

      // Should have been called for both groups
      expect(mockSendGroupAudio).toHaveBeenCalledTimes(2);

      vi.unstubAllGlobals();
      vi.useRealTimers();
    });
  });

  // ============================================================
  // Error Handling Tests
  // ============================================================

  describe('handleSendAudio - error handling', () => {
    it('should handle send failure and count errors', async () => {
      mockSendGroupAudio.mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const audioFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(audioFile));
      });

      // Mock FileReader class properly
      class MockFileReader {
        result: string | null = null;
        onload: ((e: ProgressEvent<FileReader>) => void) | null = null;
        onerror: ((e: ProgressEvent<FileReader>) => void) | null = null;
        readAsDataURL() {
          setTimeout(() => {
            this.result = 'data:audio/ogg;base64,test';
            if (this.onload) {
              this.onload({} as ProgressEvent<FileReader>);
            }
          }, 0);
        }
      }
      vi.stubGlobal('FileReader', MockFileReader);

      await act(async () => {
        await result.current.handleSendAudio();
      });

      // Should show success toast with error count
      await waitFor(() => {
        expect(mockToast).toHaveBeenCalledWith(
          expect.objectContaining({
            title: 'pages.groups.success.audioSent',
          })
        );
      });

      vi.unstubAllGlobals();
    });

    it('should handle missing instance', async () => {
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const audioFile = createMockFile('audio.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(audioFile));
      });

      // Mock FileReader class properly
      class MockFileReader {
        result: string | null = null;
        onload: ((e: ProgressEvent<FileReader>) => void) | null = null;
        onerror: ((e: ProgressEvent<FileReader>) => void) | null = null;
        readAsDataURL() {
          setTimeout(() => {
            this.result = 'data:audio/ogg;base64,test';
            if (this.onload) {
              this.onload({} as ProgressEvent<FileReader>);
            }
          }, 0);
        }
      }
      vi.stubGlobal('FileReader', MockFileReader);

      await act(async () => {
        await result.current.handleSendAudio();
      });

      // Verify sendingAudio returns to false after completion
      expect(result.current.sendingAudio).toBe(false);

      vi.unstubAllGlobals();
    });
  });

  // ============================================================
  // PTT Format Validation Tests
  // ============================================================

  describe('PTT format validation', () => {
    it('should accept .ogg extension for PTT', async () => {
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      const oggFile = createMockFile('voice.ogg', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(oggFile));
        result.current.setAudioPtt(true);
      });

      // Mock FileReader class properly
      class MockFileReader {
        result: string | null = null;
        onload: ((e: ProgressEvent<FileReader>) => void) | null = null;
        onerror: ((e: ProgressEvent<FileReader>) => void) | null = null;
        readAsDataURL() {
          setTimeout(() => {
            this.result = 'data:audio/ogg;base64,test';
            if (this.onload) {
              this.onload({} as ProgressEvent<FileReader>);
            }
          }, 0);
        }
      }
      vi.stubGlobal('FileReader', MockFileReader);

      await act(async () => {
        await result.current.handleSendAudio();
      });

      // Should not show format error
      expect(mockToast).not.toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.errors.invalidPttFormat',
        })
      );

      vi.unstubAllGlobals();
    });

    it('should accept audio/ogg mime type for PTT even with different extension', async () => {
      const { result } = renderHook(() =>
        useGroupsAudio({
          selectedGroups: ['group-1'],
          groups: mockGroups,
          clearSelection: mockClearSelection,
        })
      );

      // File with different extension but correct mime type
      const oggFile = createMockFile('voice.opus', 'audio/ogg', 1024);
      act(() => {
        result.current.handleAudioUpload(createMockChangeEvent(oggFile));
        result.current.setAudioPtt(true);
      });

      // Mock FileReader class properly
      class MockFileReader {
        result: string | null = null;
        onload: ((e: ProgressEvent<FileReader>) => void) | null = null;
        onerror: ((e: ProgressEvent<FileReader>) => void) | null = null;
        readAsDataURL() {
          setTimeout(() => {
            this.result = 'data:audio/ogg;base64,test';
            if (this.onload) {
              this.onload({} as ProgressEvent<FileReader>);
            }
          }, 0);
        }
      }
      vi.stubGlobal('FileReader', MockFileReader);

      await act(async () => {
        await result.current.handleSendAudio();
      });

      // Should not show format error
      expect(mockToast).not.toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.errors.invalidPttFormat',
        })
      );

      vi.unstubAllGlobals();
    });
  });
});
