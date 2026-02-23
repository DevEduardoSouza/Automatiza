import type { Group } from '@/types/group';
import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useGroupsPoll } from '../use-groups-poll';

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

const mockSendGroupPoll = vi.fn();

vi.mock('@/hooks/use-groups-connect', () => ({
  useGroupsConnect: () => ({
    sendGroupPoll: mockSendGroupPoll,
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

describe('useGroupsPoll', () => {
  const mockClearSelection = vi.fn();
  const mockGroups: Group[] = [
    createMockGroup({ id: 'group-1', wa_group_id: '123@g.us', name: 'Group 1' }),
    createMockGroup({ id: 'group-2', wa_group_id: '456@g.us', name: 'Group 2' }),
    createMockGroup({ id: 'group-3', wa_group_id: '789@g.us', name: 'Group 3' }),
  ];

  function renderPoll(overrides: Partial<Parameters<typeof useGroupsPoll>[0]> = {}) {
    return renderHook(() =>
      useGroupsPoll({
        selectedGroups: [],
        groups: mockGroups,
        clearSelection: mockClearSelection,
        ...overrides,
      })
    );
  }

  beforeEach(() => {
    vi.clearAllMocks();
    mockSendGroupPoll.mockResolvedValue({ message_id: 'msg-1', action_id: 'action-1' });
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  // ============================================================
  // Initial State Tests
  // ============================================================

  describe('initial state', () => {
    it('should have correct initial values', () => {
      const { result } = renderPoll();

      expect(result.current.pollDialogOpen).toBe(false);
      expect(result.current.pollQuestion).toBe('');
      expect(result.current.pollOptions).toEqual(['', '']);
      expect(result.current.sendingPoll).toBe(false);
    });

    it('should start with minimum 2 poll options', () => {
      const { result } = renderPoll();

      expect(result.current.pollOptions).toHaveLength(2);
    });

    it('should expose all required functions', () => {
      const { result } = renderPoll();

      expect(typeof result.current.setPollDialogOpen).toBe('function');
      expect(typeof result.current.setPollQuestion).toBe('function');
      expect(typeof result.current.setPollOptions).toBe('function');
      expect(typeof result.current.addPollOption).toBe('function');
      expect(typeof result.current.removePollOption).toBe('function');
      expect(typeof result.current.updatePollOption).toBe('function');
      expect(typeof result.current.handleSendPoll).toBe('function');
    });
  });

  // ============================================================
  // Poll Options Management Tests
  // ============================================================

  describe('addPollOption', () => {
    it('should add a new empty poll option', () => {
      const { result } = renderPoll();

      expect(result.current.pollOptions).toHaveLength(2);

      act(() => {
        result.current.addPollOption();
      });

      expect(result.current.pollOptions).toHaveLength(3);
      expect(result.current.pollOptions[2]).toBe('');
    });

    it('should enforce maximum of 12 poll options', () => {
      const { result } = renderPoll();

      // Add 10 more options (2 initial + 10 = 12)
      act(() => {
        for (let i = 0; i < 10; i++) {
          result.current.addPollOption();
        }
      });

      expect(result.current.pollOptions).toHaveLength(12);

      // Try to add one more
      act(() => {
        result.current.addPollOption();
      });

      expect(result.current.pollOptions).toHaveLength(12);
      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.errors.limitReached',
        description: 'pages.groups.errors.maxPollOptions',
        variant: 'destructive',
      });
    });

    it('should allow adding up to exactly 12 options', () => {
      const { result } = renderPoll();

      act(() => {
        for (let i = 0; i < 9; i++) {
          result.current.addPollOption();
        }
      });

      expect(result.current.pollOptions).toHaveLength(11);
      mockToast.mockClear();

      act(() => {
        result.current.addPollOption();
      });

      expect(result.current.pollOptions).toHaveLength(12);
      expect(mockToast).not.toHaveBeenCalled();
    });
  });

  describe('removePollOption', () => {
    it('should remove a poll option at the specified index', () => {
      const { result } = renderPoll();

      act(() => {
        result.current.addPollOption();
        result.current.updatePollOption(0, 'Option A');
        result.current.updatePollOption(1, 'Option B');
        result.current.updatePollOption(2, 'Option C');
      });

      expect(result.current.pollOptions).toHaveLength(3);

      act(() => {
        result.current.removePollOption(1);
      });

      expect(result.current.pollOptions).toHaveLength(2);
      expect(result.current.pollOptions).toEqual(['Option A', 'Option C']);
    });

    it('should enforce minimum of 2 poll options', () => {
      const { result } = renderPoll();

      expect(result.current.pollOptions).toHaveLength(2);

      act(() => {
        result.current.removePollOption(0);
      });

      expect(result.current.pollOptions).toHaveLength(2);
    });

    it('should allow removal when more than 2 options exist', () => {
      const { result } = renderPoll();

      act(() => {
        result.current.addPollOption();
      });

      expect(result.current.pollOptions).toHaveLength(3);

      act(() => {
        result.current.removePollOption(2);
      });

      expect(result.current.pollOptions).toHaveLength(2);
    });

    it('should remove first option correctly', () => {
      const { result } = renderPoll();

      act(() => {
        result.current.addPollOption();
        result.current.updatePollOption(0, 'First');
        result.current.updatePollOption(1, 'Second');
        result.current.updatePollOption(2, 'Third');
      });

      act(() => {
        result.current.removePollOption(0);
      });

      expect(result.current.pollOptions).toEqual(['Second', 'Third']);
    });

    it('should remove last option correctly', () => {
      const { result } = renderPoll();

      act(() => {
        result.current.addPollOption();
        result.current.updatePollOption(0, 'First');
        result.current.updatePollOption(1, 'Second');
        result.current.updatePollOption(2, 'Third');
      });

      act(() => {
        result.current.removePollOption(2);
      });

      expect(result.current.pollOptions).toEqual(['First', 'Second']);
    });
  });

  describe('updatePollOption', () => {
    it('should update a poll option at the specified index', () => {
      const { result } = renderPoll();

      act(() => {
        result.current.updatePollOption(0, 'First option');
      });

      expect(result.current.pollOptions[0]).toBe('First option');
      expect(result.current.pollOptions[1]).toBe('');
    });

    it('should update multiple options independently', () => {
      const { result } = renderPoll();

      act(() => {
        result.current.updatePollOption(0, 'Option 1');
        result.current.updatePollOption(1, 'Option 2');
      });

      expect(result.current.pollOptions).toEqual(['Option 1', 'Option 2']);
    });

    it('should overwrite previous value', () => {
      const { result } = renderPoll();

      act(() => {
        result.current.updatePollOption(0, 'Initial');
      });

      act(() => {
        result.current.updatePollOption(0, 'Updated');
      });

      expect(result.current.pollOptions[0]).toBe('Updated');
    });
  });

  // ============================================================
  // State Setters Tests
  // ============================================================

  describe('state setters', () => {
    it('should update pollDialogOpen state', () => {
      const { result } = renderPoll();

      act(() => {
        result.current.setPollDialogOpen(true);
      });

      expect(result.current.pollDialogOpen).toBe(true);
    });

    it('should update pollQuestion state', () => {
      const { result } = renderPoll();

      act(() => {
        result.current.setPollQuestion('What is your favorite color?');
      });

      expect(result.current.pollQuestion).toBe('What is your favorite color?');
    });

    it('should update pollOptions state directly', () => {
      const { result } = renderPoll();

      act(() => {
        result.current.setPollOptions(['Red', 'Blue', 'Green']);
      });

      expect(result.current.pollOptions).toEqual(['Red', 'Blue', 'Green']);
    });
  });

  // ============================================================
  // handleSendPoll Validation Tests
  // ============================================================

  describe('handleSendPoll - validation', () => {
    it('should show error when poll question is empty', async () => {
      const { result } = renderPoll({ selectedGroups: ['group-1'] });

      await act(async () => {
        await result.current.handleSendPoll();
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.errors.questionRequired',
        description: 'pages.groups.errors.questionRequiredMessage',
        variant: 'destructive',
      });
      expect(mockSendGroupPoll).not.toHaveBeenCalled();
    });

    it('should show error when poll question is only whitespace', async () => {
      const { result } = renderPoll({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setPollQuestion('   ');
      });

      await act(async () => {
        await result.current.handleSendPoll();
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.errors.questionRequired',
        description: 'pages.groups.errors.questionRequiredMessage',
        variant: 'destructive',
      });
    });

    it('should show error when fewer than 2 valid options', async () => {
      const { result } = renderPoll({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setPollQuestion('Test question');
        result.current.updatePollOption(0, 'Only one option');
        // Second option is empty string
      });

      await act(async () => {
        await result.current.handleSendPoll();
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.errors.insufficientOptions',
        description: 'pages.groups.errors.insufficientOptionsMessage',
        variant: 'destructive',
      });
    });

    it('should show error when all options are empty or whitespace', async () => {
      const { result } = renderPoll({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setPollQuestion('Test question');
        result.current.updatePollOption(0, '   ');
        result.current.updatePollOption(1, '');
      });

      await act(async () => {
        await result.current.handleSendPoll();
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.errors.insufficientOptions',
        description: 'pages.groups.errors.insufficientOptionsMessage',
        variant: 'destructive',
      });
    });

    it('should show error when no groups selected', async () => {
      const { result } = renderPoll({ selectedGroups: [] });

      act(() => {
        result.current.setPollQuestion('Test question');
        result.current.updatePollOption(0, 'Option 1');
        result.current.updatePollOption(1, 'Option 2');
      });

      await act(async () => {
        await result.current.handleSendPoll();
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.errors.selectGroups',
        description: 'pages.groups.errors.selectGroupsPollMessage',
        variant: 'destructive',
      });
    });

    it('should validate in order: question, options, groups', async () => {
      const { result } = renderPoll({ selectedGroups: [] });

      // Empty question - should be caught first
      await act(async () => {
        await result.current.handleSendPoll();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.errors.questionRequired',
        })
      );
    });
  });

  // ============================================================
  // handleSendPoll Success Tests
  // ============================================================

  describe('handleSendPoll - success', () => {
    it('should send poll successfully to single group', async () => {
      vi.useFakeTimers();

      const { result } = renderPoll({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setPollQuestion('What is your favorite color?');
        result.current.updatePollOption(0, 'Red');
        result.current.updatePollOption(1, 'Blue');
        result.current.setPollDialogOpen(true);
      });

      const sendPromise = result.current.handleSendPoll();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockSendGroupPoll).toHaveBeenCalledWith({
        instance_name: 'test-instance',
        group_id: '123@g.us',
        name: 'What is your favorite color?',
        options: ['Red', 'Blue'],
      });

      // Verify state cleanup
      expect(result.current.pollDialogOpen).toBe(false);
      expect(result.current.pollQuestion).toBe('');
      expect(result.current.pollOptions).toEqual(['', '']);
      expect(mockClearSelection).toHaveBeenCalled();

      vi.useRealTimers();
    });

    it('should send poll to multiple groups with rate limiting', async () => {
      vi.useFakeTimers();

      const { result } = renderPoll({ selectedGroups: ['group-1', 'group-2', 'group-3'] });

      act(() => {
        result.current.setPollQuestion('Test poll?');
        result.current.updatePollOption(0, 'Yes');
        result.current.updatePollOption(1, 'No');
      });

      const sendPromise = result.current.handleSendPoll();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(5000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockSendGroupPoll).toHaveBeenCalledTimes(3);

      vi.useRealTimers();
    });

    it('should only send non-empty options', async () => {
      vi.useFakeTimers();

      const { result } = renderPoll({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setPollQuestion('Test?');
        result.current.addPollOption();
        result.current.addPollOption();
        result.current.updatePollOption(0, 'Option A');
        result.current.updatePollOption(1, '');
        result.current.updatePollOption(2, 'Option C');
        result.current.updatePollOption(3, '   ');
      });

      const sendPromise = result.current.handleSendPoll();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockSendGroupPoll).toHaveBeenCalledWith(
        expect.objectContaining({
          options: ['Option A', 'Option C'],
        })
      );

      vi.useRealTimers();
    });

    it('should show success toast with correct count', async () => {
      vi.useFakeTimers();

      const { result } = renderPoll({ selectedGroups: ['group-1', 'group-2'] });

      act(() => {
        result.current.setPollQuestion('Test?');
        result.current.updatePollOption(0, 'A');
        result.current.updatePollOption(1, 'B');
      });

      const sendPromise = result.current.handleSendPoll();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(5000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.groups.success.pollSent',
        description: expect.stringContaining('2'),
      });

      vi.useRealTimers();
    });

    it('should reset sendingPoll to false after success', async () => {
      vi.useFakeTimers();

      const { result } = renderPoll({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setPollQuestion('Test?');
        result.current.updatePollOption(0, 'A');
        result.current.updatePollOption(1, 'B');
      });

      const sendPromise = result.current.handleSendPoll();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(result.current.sendingPoll).toBe(false);

      vi.useRealTimers();
    });
  });

  // ============================================================
  // Error Handling Tests
  // ============================================================

  describe('handleSendPoll - error handling', () => {
    it('should handle disconnected instance (null)', async () => {
      // Override the latestInstance mock temporarily
      const originalStatus = mockLatestInstance.status;
      Object.assign(mockLatestInstance, { status: 'disconnected' });

      const { result } = renderPoll({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setPollQuestion('Test?');
        result.current.updatePollOption(0, 'A');
        result.current.updatePollOption(1, 'B');
      });

      await act(async () => {
        await result.current.handleSendPoll();
      });

      expect(mockCaptureHookError).toHaveBeenCalledWith(
        expect.any(Error),
        'useGroupsPoll',
        'handleSendPoll'
      );

      expect(result.current.sendingPoll).toBe(false);

      // Restore
      Object.assign(mockLatestInstance, { status: originalStatus });
    });

    it('should handle partial failures', async () => {
      vi.useFakeTimers();

      mockSendGroupPoll
        .mockResolvedValueOnce({ message_id: 'msg-1', action_id: 'action-1' })
        .mockRejectedValueOnce(new Error('Network error'));

      const { result } = renderPoll({ selectedGroups: ['group-1', 'group-2'] });

      act(() => {
        result.current.setPollQuestion('Test?');
        result.current.updatePollOption(0, 'A');
        result.current.updatePollOption(1, 'B');
      });

      const sendPromise = result.current.handleSendPoll();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(5000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.groups.success.pollPartiallySent',
        })
      );

      expect(mockCaptureHookError).toHaveBeenCalledWith(
        expect.any(Error),
        'useGroupsPoll',
        'sendPollToGroup'
      );

      vi.useRealTimers();
    });

    it('should handle complete failure (all groups error)', async () => {
      vi.useFakeTimers();

      mockSendGroupPoll.mockRejectedValue(new Error('Network error'));

      const { result } = renderPoll({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setPollQuestion('Test?');
        result.current.updatePollOption(0, 'A');
        result.current.updatePollOption(1, 'B');
      });

      const sendPromise = result.current.handleSendPoll();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
        })
      );

      vi.useRealTimers();
    });

    it('should reset sendingPoll state after error', async () => {
      vi.useFakeTimers();

      mockSendGroupPoll.mockRejectedValue(new Error('Network error'));

      const { result } = renderPoll({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setPollQuestion('Test?');
        result.current.updatePollOption(0, 'A');
        result.current.updatePollOption(1, 'B');
      });

      const sendPromise = result.current.handleSendPoll();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(result.current.sendingPoll).toBe(false);

      vi.useRealTimers();
    });

    it('should capture non-Error exceptions from sendGroupPoll', async () => {
      vi.useFakeTimers();

      mockSendGroupPoll.mockRejectedValue('string error');

      const { result } = renderPoll({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setPollQuestion('Test?');
        result.current.updatePollOption(0, 'A');
        result.current.updatePollOption(1, 'B');
      });

      const sendPromise = result.current.handleSendPoll();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockCaptureHookError).toHaveBeenCalledWith(
        'string error',
        'useGroupsPoll',
        'sendPollToGroup'
      );

      vi.useRealTimers();
    });
  });

  // ============================================================
  // Edge Cases
  // ============================================================

  describe('edge cases', () => {
    it('should handle very long poll question', async () => {
      vi.useFakeTimers();

      const { result } = renderPoll({ selectedGroups: ['group-1'] });

      const longQuestion = 'A'.repeat(256);

      act(() => {
        result.current.setPollQuestion(longQuestion);
        result.current.updatePollOption(0, 'A');
        result.current.updatePollOption(1, 'B');
      });

      const sendPromise = result.current.handleSendPoll();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockSendGroupPoll).toHaveBeenCalledWith(
        expect.objectContaining({
          name: longQuestion,
        })
      );

      vi.useRealTimers();
    });

    it('should handle special characters in options', async () => {
      vi.useFakeTimers();

      const { result } = renderPoll({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setPollQuestion('Test?');
        result.current.updatePollOption(0, 'Option with emoji 123');
        result.current.updatePollOption(1, 'Option with "quotes"');
      });

      const sendPromise = result.current.handleSendPoll();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockSendGroupPoll).toHaveBeenCalledWith(
        expect.objectContaining({
          options: ['Option with emoji 123', 'Option with "quotes"'],
        })
      );

      vi.useRealTimers();
    });

    it('should handle exactly 12 options', async () => {
      vi.useFakeTimers();

      const { result } = renderPoll({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setPollQuestion('Test?');
        for (let i = 0; i < 10; i++) {
          result.current.addPollOption();
        }
        for (let i = 0; i < 12; i++) {
          result.current.updatePollOption(i, `Option ${i + 1}`);
        }
      });

      expect(result.current.pollOptions).toHaveLength(12);

      const sendPromise = result.current.handleSendPoll();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(mockSendGroupPoll).toHaveBeenCalledWith(
        expect.objectContaining({
          options: expect.arrayContaining(['Option 1', 'Option 2', 'Option 12']),
        })
      );

      vi.useRealTimers();
    });

    it('should reset state after successful send even with many options', async () => {
      vi.useFakeTimers();

      const { result } = renderPoll({ selectedGroups: ['group-1'] });

      act(() => {
        result.current.setPollQuestion('Test?');
        result.current.addPollOption();
        result.current.updatePollOption(0, 'A');
        result.current.updatePollOption(1, 'B');
        result.current.updatePollOption(2, 'C');
        result.current.setPollDialogOpen(true);
      });

      const sendPromise = result.current.handleSendPoll();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000);
      });

      await act(async () => {
        await sendPromise;
      });

      expect(result.current.pollDialogOpen).toBe(false);
      expect(result.current.pollQuestion).toBe('');
      expect(result.current.pollOptions).toEqual(['', '']);

      vi.useRealTimers();
    });
  });
});
