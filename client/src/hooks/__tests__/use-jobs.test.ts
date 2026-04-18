import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useJobs } from '../use-jobs';

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
// Mock: useJobsConnect (mutable fns)
// ============================================================

let mockCreateJob = vi.fn().mockResolvedValue({
  id: 'new-1',
  action_type: 'send_message',
  scheduled_for: '2026-03-01T10:00:00Z',
  status: 'pending',
  payload: { groups: [] },
  created_at: '2026-02-22T00:00:00Z',
  updated_at: '2026-02-22T00:00:00Z',
});
let mockUpdateJob = vi.fn().mockResolvedValue({ id: 'edit-1' });
let mockDeleteJob = vi.fn().mockResolvedValue(undefined);
let mockRefetchJobs = vi.fn();

vi.mock('@/hooks/use-jobs-connect', () => ({
  useJobsConnect: () => ({
    jobs: [],
    loadingJobs: false,
    hasNextPage: false,
    totalCount: 0,
    loadNextPage: vi.fn(),
    createJob: mockCreateJob,
    updateJob: mockUpdateJob,
    deleteJob: mockDeleteJob,
    isCreatingJob: false,
    isUpdatingJob: false,
    isDeletingJob: false,
    refetchJobs: mockRefetchJobs,
  }),
}));

// ============================================================
// Mock: useJobsRealtime
// ============================================================

const mockUseJobsRealtime = vi.fn();

vi.mock('@/hooks/use-jobs-realtime', () => ({
  useJobsRealtime: (...args: unknown[]) => mockUseJobsRealtime(...args),
}));

// ============================================================
// Mock: useAiToolsConnect
// ============================================================

let mockEnhanceMessage = vi.fn().mockResolvedValue(['Enhanced message']);

vi.mock('@/hooks/use-ai-tools-connect', () => ({
  useAiToolsConnect: () => ({
    enhanceMessage: mockEnhanceMessage,
    isEnhancing: false,
  }),
}));

// ============================================================
// Mock: useGroupsConnect
// ============================================================

vi.mock('@/hooks/use-groups-connect', () => ({
  useGroupsConnect: () => ({
    groups: [
      { id: 'g1', name: 'Group 1', wa_group_id: '111@g.us', is_favorite: true },
      { id: 'g2', name: 'Group 2', wa_group_id: '222@g.us', is_favorite: false },
    ],
    loading: false,
  }),
}));

// ============================================================
// Mock: Auth, QueryClient, Sentry
// ============================================================

vi.mock('@/contexts', () => ({
  useAuth: () => ({ user: { id: 'user-1' } }),
}));

const mockInvalidateQueries = vi.fn();

vi.mock('@tanstack/react-query', () => ({
  useQueryClient: () => ({ invalidateQueries: mockInvalidateQueries }),
}));

vi.mock('@/lib/sentry', () => ({
  captureHookError: vi.fn(),
}));

// ============================================================
// Helpers
// ============================================================

function renderJobs() {
  return renderHook(() => useJobs());
}

/**
 * Sets up form state for scheduling a job.
 * Must be called inside act().
 */
function setFormState(
  result: ReturnType<typeof renderJobs>['result'],
  overrides: {
    actionType?: string;
    scheduledDate?: string;
    scheduledTime?: string;
    selectedGroups?: string[];
    payload?: string;
    pollQuestion?: string;
    pollOptions?: string[];
    autoNumberGroups?: boolean;
  }
) {
  if (overrides.actionType !== undefined) result.current.setActionType(overrides.actionType);
  if (overrides.scheduledDate !== undefined) result.current.setScheduledDate(overrides.scheduledDate);
  if (overrides.scheduledTime !== undefined) result.current.setScheduledTime(overrides.scheduledTime);
  if (overrides.selectedGroups !== undefined) result.current.setSelectedGroups(overrides.selectedGroups);
  if (overrides.payload !== undefined) result.current.setPayload(overrides.payload);
  if (overrides.pollQuestion !== undefined) result.current.setPollQuestion(overrides.pollQuestion);
  if (overrides.pollOptions !== undefined) result.current.setPollOptions(overrides.pollOptions);
  if (overrides.autoNumberGroups !== undefined) result.current.setAutoNumberGroups(overrides.autoNumberGroups);
}

const BASE_FORM = {
  scheduledDate: '2026-03-01',
  scheduledTime: '10:00',
  selectedGroups: ['g1'],
} as const;

// ============================================================
// Tests
// ============================================================

describe('useJobs', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCreateJob = vi.fn().mockResolvedValue({
      id: 'new-1',
      action_type: 'send_message',
      scheduled_for: '2026-03-01T10:00:00Z',
      status: 'pending',
      payload: { groups: [] },
      created_at: '2026-02-22T00:00:00Z',
      updated_at: '2026-02-22T00:00:00Z',
    });
    mockUpdateJob = vi.fn().mockResolvedValue({ id: 'edit-1' });
    mockDeleteJob = vi.fn().mockResolvedValue(undefined);
    mockRefetchJobs = vi.fn();
    mockEnhanceMessage = vi.fn().mockResolvedValue(['Enhanced message']);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── 1. Form State ────────────────────────────────────────────
  describe('Form State', () => {
    it('should have correct initial values', () => {
      const { result } = renderJobs();

      expect(result.current.actionType).toBe('');
      expect(result.current.scheduledDate).toBe('');
      expect(result.current.scheduledTime).toBe('');
      expect(result.current.payload).toBe('');
      expect(result.current.mediaFile).toBeNull();
      expect(result.current.selectedGroups).toEqual([]);
      expect(result.current.mentionOpen).toBe(false);
      expect(result.current.dialogOpen).toBe(false);
      expect(result.current.editingJobId).toBeNull();
      expect(result.current.autoNumberGroups).toBe(false);
      expect(result.current.searchTerm).toBe('');
      expect(result.current.pollQuestion).toBe('');
      expect(result.current.pollOptions).toEqual(['', '']);
    });

    it('should reset all form fields when resetForm is called', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, {
          actionType: 'send_message',
          scheduledDate: '2026-03-01',
          scheduledTime: '10:00',
          selectedGroups: ['g1'],
          payload: 'Hello',
        });
        result.current.setAutoNumberGroups(true);
        result.current.setSearchTerm('search');
        result.current.setPollQuestion('Q?');
        result.current.setPollOptions(['A', 'B', 'C']);
        result.current.setDialogOpen(true);
      });

      await act(async () => {
        result.current.resetForm();
      });

      expect(result.current.actionType).toBe('');
      expect(result.current.scheduledDate).toBe('');
      expect(result.current.scheduledTime).toBe('');
      expect(result.current.payload).toBe('');
      expect(result.current.mediaFile).toBeNull();
      expect(result.current.selectedGroups).toEqual([]);
      expect(result.current.autoNumberGroups).toBe(false);
      expect(result.current.searchTerm).toBe('');
      expect(result.current.pollQuestion).toBe('');
      expect(result.current.pollOptions).toEqual(['', '']);
      expect(result.current.dialogOpen).toBe(false);
      expect(result.current.editingJobId).toBeNull();
    });
  });

  // ── 2. handleScheduleJob - Validation ────────────────────────
  describe('handleScheduleJob - Validation', () => {
    it('should show toast when actionType is empty', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, { ...BASE_FORM, actionType: '' });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive', title: 'pages.jobs.errors.requiredFields' })
      );
      expect(mockCreateJob).not.toHaveBeenCalled();
    });

    it('should show toast when scheduledDate is empty', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, { actionType: 'send_message', scheduledDate: '', scheduledTime: '10:00', selectedGroups: ['g1'] });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive', title: 'pages.jobs.errors.requiredFields' })
      );
      expect(mockCreateJob).not.toHaveBeenCalled();
    });

    it('should show toast when scheduledTime is empty', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, { actionType: 'send_message', scheduledDate: '2026-03-01', scheduledTime: '', selectedGroups: ['g1'] });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive', title: 'pages.jobs.errors.requiredFields' })
      );
      expect(mockCreateJob).not.toHaveBeenCalled();
    });

    it('should show toast when no groups selected', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, { actionType: 'send_message', ...BASE_FORM, selectedGroups: [] });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive', title: 'pages.jobs.errors.noGroups' })
      );
      expect(mockCreateJob).not.toHaveBeenCalled();
    });

    it('should show toast when send_message has empty payload', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, { actionType: 'send_message', ...BASE_FORM, payload: '' });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive', title: 'pages.jobs.errors.emptyMessage' })
      );
      expect(mockCreateJob).not.toHaveBeenCalled();
    });

    it('should show toast when send_message has whitespace-only payload', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, { actionType: 'send_message', ...BASE_FORM, payload: '   ' });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive', title: 'pages.jobs.errors.emptyMessage' })
      );
      expect(mockCreateJob).not.toHaveBeenCalled();
    });

    it('should show toast when send_poll has empty question', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, {
          actionType: 'send_poll',
          ...BASE_FORM,
          pollQuestion: '',
          pollOptions: ['A', 'B'],
        });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive', title: 'pages.jobs.errors.emptyPollQuestion' })
      );
      expect(mockCreateJob).not.toHaveBeenCalled();
    });

    it('should show toast when send_poll has fewer than 2 valid options', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, {
          actionType: 'send_poll',
          ...BASE_FORM,
          pollQuestion: 'Which color?',
          pollOptions: ['Red', '', ''],
        });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive', title: 'pages.jobs.errors.insufficientOptions' })
      );
      expect(mockCreateJob).not.toHaveBeenCalled();
    });

  });

  // ── 3. handleScheduleJob - Success create ────────────────────
  describe('handleScheduleJob - Success create', () => {
    it('should create send_message job with correct payload', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, { actionType: 'send_message', ...BASE_FORM, payload: 'Hello world' });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockCreateJob).toHaveBeenCalledWith(
        expect.objectContaining({
          action_type: 'send_message',
          payload: expect.objectContaining({ groups: ['g1'], message: 'Hello world' }),
        })
      );
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ title: 'pages.jobs.success.scheduled' })
      );
      expect(result.current.dialogOpen).toBe(false);
    });

    it('should create send_poll job with filtered options', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, {
          actionType: 'send_poll',
          ...BASE_FORM,
          pollQuestion: 'Favorite?',
          pollOptions: ['Red', '', 'Blue'],
        });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockCreateJob).toHaveBeenCalledWith(
        expect.objectContaining({
          action_type: 'send_poll',
          payload: expect.objectContaining({
            groups: ['g1'],
            question: 'Favorite?',
            options: ['Red', 'Blue'],
          }),
        })
      );
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ title: 'pages.jobs.success.scheduled' })
      );
      expect(result.current.dialogOpen).toBe(false);
    });

    it('should create update_description job with correct payload', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, { actionType: 'update_description', ...BASE_FORM, payload: 'New description' });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockCreateJob).toHaveBeenCalledWith(
        expect.objectContaining({
          action_type: 'update_description',
          payload: expect.objectContaining({ groups: ['g1'], description: 'New description' }),
        })
      );
      expect(result.current.dialogOpen).toBe(false);
    });

    it('should create close_groups job with just groups', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, { actionType: 'close_groups', ...BASE_FORM });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockCreateJob).toHaveBeenCalledWith(
        expect.objectContaining({
          action_type: 'close_groups',
          payload: expect.objectContaining({ groups: ['g1'] }),
        })
      );
      expect(result.current.dialogOpen).toBe(false);
    });

    it('should create open_groups job with just groups', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, { actionType: 'open_groups', ...BASE_FORM });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockCreateJob).toHaveBeenCalledWith(
        expect.objectContaining({
          action_type: 'open_groups',
          payload: expect.objectContaining({ groups: ['g1'] }),
        })
      );
      expect(result.current.dialogOpen).toBe(false);
    });

    it('should create change_group_name job with name and autoNumber', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, {
          actionType: 'change_group_name',
          ...BASE_FORM,
          payload: 'New Name',
          autoNumberGroups: true,
        });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockCreateJob).toHaveBeenCalledWith(
        expect.objectContaining({
          action_type: 'change_group_name',
          payload: expect.objectContaining({ groups: ['g1'], name: 'New Name', autoNumber: true }),
        })
      );
      expect(result.current.dialogOpen).toBe(false);
    });

    it('should create change_group_photo job', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, { actionType: 'change_group_photo', ...BASE_FORM });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockCreateJob).toHaveBeenCalledWith(
        expect.objectContaining({
          action_type: 'change_group_photo',
          payload: expect.objectContaining({ groups: ['g1'] }),
        })
      );
      expect(result.current.dialogOpen).toBe(false);
    });

    it('should call refetchJobs after successful create', async () => {
      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, { actionType: 'close_groups', ...BASE_FORM });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockRefetchJobs).toHaveBeenCalled();
    });
  });

  // ── 4. handleScheduleJob - Edit mode ─────────────────────────
  describe('handleScheduleJob - Edit mode', () => {
    it('should call updateJob instead of createJob when editingJobId is set', async () => {
      const { result } = renderJobs();

      // Simulate editing by calling handleEditJob first
      const mockJob = {
        id: 'job-42',
        action_type: 'send_message' as const,
        scheduled_for: '2026-03-15T14:30:00Z',
        status: 'pending' as const,
        payload: { groups: ['g1'], message: 'Original message' },
        created_at: '2026-02-22T00:00:00Z',
        updated_at: '2026-02-22T00:00:00Z',
      };

      await act(async () => {
        result.current.handleEditJob(mockJob);
      });

      // Modify the message
      await act(async () => {
        result.current.setPayload('Updated message');
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockUpdateJob).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'job-42',
          action_type: 'send_message',
          payload: expect.objectContaining({ message: 'Updated message' }),
        })
      );
      expect(mockCreateJob).not.toHaveBeenCalled();
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ title: 'pages.jobs.success.updated' })
      );
      expect(result.current.dialogOpen).toBe(false);
    });
  });

  // ── 5. handleScheduleJob - Error ─────────────────────────────
  describe('handleScheduleJob - Error', () => {
    it('should show destructive toast and call captureHookError on createJob failure', async () => {
      const { captureHookError } = await import('@/lib/sentry');
      mockCreateJob.mockRejectedValue(new Error('Network error'));

      const { result } = renderJobs();

      await act(async () => {
        setFormState(result, { actionType: 'close_groups', ...BASE_FORM });
      });

      await act(async () => {
        await result.current.handleScheduleJob();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive', title: 'pages.jobs.errors.schedule' })
      );
      expect(captureHookError).toHaveBeenCalledWith(
        expect.any(Error),
        'useJobs',
        'handleScheduleJob'
      );
      // Dialog should NOT close on error (resetForm not called)
      // Note: dialogOpen was never opened explicitly here, so we check that resetForm was NOT called
      // by verifying the form state is preserved
      expect(result.current.actionType).toBe('close_groups');
    });
  });

  // ── 6. handleDeleteJob ───────────────────────────────────────
  describe('handleDeleteJob', () => {
    it('should show success toast and refetch on successful delete', async () => {
      const { result } = renderJobs();

      await act(async () => {
        await result.current.handleDeleteJob('job-99');
      });

      expect(mockDeleteJob).toHaveBeenCalledWith('job-99');
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ title: 'pages.jobs.success.deleted' })
      );
      expect(mockRefetchJobs).toHaveBeenCalled();
    });

    it('should show destructive toast and call captureHookError on delete failure', async () => {
      const { captureHookError } = await import('@/lib/sentry');
      mockDeleteJob.mockRejectedValue(new Error('Delete failed'));

      const { result } = renderJobs();

      await act(async () => {
        await result.current.handleDeleteJob('job-99');
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive', title: 'pages.jobs.errors.delete' })
      );
      expect(captureHookError).toHaveBeenCalledWith(
        expect.any(Error),
        'useJobs',
        'handleDeleteJob'
      );
    });
  });

  // ── 7. handleEnhanceMessage ──────────────────────────────────
  describe('handleEnhanceMessage', () => {
    it('should show toast when payload is empty', async () => {
      const { result } = renderJobs();

      await act(async () => {
        await result.current.handleEnhanceMessage();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive', title: 'common.error' })
      );
      expect(mockEnhanceMessage).not.toHaveBeenCalled();
    });

    it('should set payload from single enhanced message', async () => {
      mockEnhanceMessage.mockResolvedValue(['AI-enhanced text']);

      const { result } = renderJobs();

      await act(async () => {
        result.current.setPayload('Original text');
      });

      await act(async () => {
        await result.current.handleEnhanceMessage();
      });

      expect(result.current.payload).toBe('AI-enhanced text');
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ title: 'pages.jobs.success.enhanced' })
      );
    });

    it('should show count toast when multiple messages returned', async () => {
      mockEnhanceMessage.mockResolvedValue(['Msg 1', 'Msg 2', 'Msg 3']);

      const { result } = renderJobs();

      await act(async () => {
        result.current.setPayload('Original');
      });

      await act(async () => {
        await result.current.handleEnhanceMessage();
      });

      // First message is used as payload
      expect(result.current.payload).toBe('Msg 1');
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.jobs.success.enhanced',
          description: 'pages.jobs.success.enhancedMultipleMessage',
        })
      );
    });

    it('should show destructive toast when empty array returned', async () => {
      mockEnhanceMessage.mockResolvedValue([]);

      const { result } = renderJobs();

      await act(async () => {
        result.current.setPayload('Some text');
      });

      await act(async () => {
        await result.current.handleEnhanceMessage();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.jobs.errors.unexpectedResponse',
        })
      );
    });

    it('should show destructive toast and captureHookError on enhance failure', async () => {
      const { captureHookError } = await import('@/lib/sentry');
      mockEnhanceMessage.mockRejectedValue(new Error('API error'));

      const { result } = renderJobs();

      await act(async () => {
        result.current.setPayload('Text');
      });

      await act(async () => {
        await result.current.handleEnhanceMessage();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.jobs.errors.enhance',
        })
      );
      expect(captureHookError).toHaveBeenCalledWith(
        expect.any(Error),
        'useJobs',
        'handleEnhanceMessage'
      );
    });
  });

  // ── 8. handleEditJob ─────────────────────────────────────────
  describe('handleEditJob', () => {
    const makeJob = (overrides: Partial<import('../use-jobs-connect').LocalJob>): import('../use-jobs-connect').LocalJob => ({
      id: 'job-1',
      action_type: 'send_message',
      scheduled_for: '2026-04-10T15:30:00Z',
      status: 'pending',
      payload: { groups: ['g1'] },
      created_at: '2026-02-22T00:00:00Z',
      updated_at: '2026-02-22T00:00:00Z',
      ...overrides,
    });

    it('should populate form for send_message', async () => {
      const { result } = renderJobs();

      await act(async () => {
        result.current.handleEditJob(
          makeJob({ action_type: 'send_message', payload: { groups: ['g1', 'g2'], message: 'Hello' } })
        );
      });

      expect(result.current.editingJobId).toBe('job-1');
      expect(result.current.actionType).toBe('send_message');
      expect(result.current.selectedGroups).toEqual(['g1', 'g2']);
      expect(result.current.payload).toBe('Hello');
      expect(result.current.dialogOpen).toBe(true);
    });

    it('should populate form for send_poll', async () => {
      const { result } = renderJobs();

      await act(async () => {
        result.current.handleEditJob(
          makeJob({
            action_type: 'send_poll',
            payload: { groups: ['g1'], question: 'Which?', options: ['A', 'B', 'C'] },
          })
        );
      });

      expect(result.current.actionType).toBe('send_poll');
      expect(result.current.pollQuestion).toBe('Which?');
      expect(result.current.pollOptions).toEqual(['A', 'B', 'C']);
      expect(result.current.dialogOpen).toBe(true);
    });

    it('should populate form for update_description', async () => {
      const { result } = renderJobs();

      await act(async () => {
        result.current.handleEditJob(
          makeJob({ action_type: 'update_description', payload: { groups: ['g1'], description: 'New desc' } })
        );
      });

      expect(result.current.actionType).toBe('update_description');
      expect(result.current.payload).toBe('New desc');
      expect(result.current.dialogOpen).toBe(true);
    });

    it('should populate form for change_group_name with autoNumber', async () => {
      const { result } = renderJobs();

      await act(async () => {
        result.current.handleEditJob(
          makeJob({
            action_type: 'change_group_name',
            payload: { groups: ['g1'], name: 'Team', autoNumber: true },
          })
        );
      });

      expect(result.current.actionType).toBe('change_group_name');
      expect(result.current.payload).toBe('Team');
      expect(result.current.autoNumberGroups).toBe(true);
      expect(result.current.dialogOpen).toBe(true);
    });

    it('should populate form for change_group_photo', async () => {
      const { result } = renderJobs();

      await act(async () => {
        result.current.handleEditJob(
          makeJob({
            action_type: 'change_group_photo',
            payload: { groups: ['g1'], image: 'data:image/png;base64,...' },
          })
        );
      });

      expect(result.current.actionType).toBe('change_group_photo');
      expect(result.current.payload).toBe('data:image/png;base64,...');
      expect(result.current.dialogOpen).toBe(true);
    });

    it('should handle backward compat: send_message with type=poll becomes send_poll', async () => {
      const { result } = renderJobs();

      await act(async () => {
        result.current.handleEditJob(
          makeJob({
            action_type: 'send_message',
            payload: { groups: ['g1'], type: 'poll', question: 'Q?', options: ['X', 'Y'] },
          })
        );
      });

      expect(result.current.actionType).toBe('send_poll');
      expect(result.current.pollQuestion).toBe('Q?');
      expect(result.current.pollOptions).toEqual(['X', 'Y']);
    });

    it('should parse date/time from ISO string', async () => {
      const { result } = renderJobs();

      await act(async () => {
        result.current.handleEditJob(
          makeJob({ scheduled_for: '2026-04-10T15:30:00Z' })
        );
      });

      expect(result.current.scheduledDate).toBe('2026-04-10');
      // Time depends on local timezone, just verify it's set and has HH:MM format
      expect(result.current.scheduledTime).toMatch(/^\d{2}:\d{2}$/);
    });
  });

  // ── 9. toggleGroup ───────────────────────────────────────────
  describe('toggleGroup', () => {
    it('should add group when not selected', async () => {
      const { result } = renderJobs();

      await act(async () => {
        result.current.toggleGroup('g1');
      });

      expect(result.current.selectedGroups).toEqual(['g1']);
    });

    it('should remove group when already selected', async () => {
      const { result } = renderJobs();

      await act(async () => {
        result.current.setSelectedGroups(['g1', 'g2']);
      });

      await act(async () => {
        result.current.toggleGroup('g1');
      });

      expect(result.current.selectedGroups).toEqual(['g2']);
    });
  });

  // ── 10. insertMention ────────────────────────────────────────
  describe('insertMention', () => {
    it('should append mention to payload and close mentionOpen', async () => {
      const { result } = renderJobs();

      await act(async () => {
        result.current.setPayload('Hello ');
        result.current.setMentionOpen(true);
      });

      await act(async () => {
        result.current.insertMention('@everyone');
      });

      expect(result.current.payload).toBe('Hello @everyone');
      expect(result.current.mentionOpen).toBe(false);
    });
  });

  // ── 11. addPollOption ────────────────────────────────────────
  describe('addPollOption', () => {
    it('should add a new empty option when under limit', async () => {
      const { result } = renderJobs();

      await act(async () => {
        result.current.addPollOption();
      });

      expect(result.current.pollOptions).toEqual(['', '', '']);
    });

    it('should show destructive toast when at 12 options limit', async () => {
      const { result } = renderJobs();

      await act(async () => {
        result.current.setPollOptions(Array(12).fill('opt'));
      });

      await act(async () => {
        result.current.addPollOption();
      });

      expect(result.current.pollOptions).toHaveLength(12);
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.jobs.errors.maxOptions',
        })
      );
    });
  });

  // ── 12. removePollOption ─────────────────────────────────────
  describe('removePollOption', () => {
    it('should remove option when more than 2', async () => {
      const { result } = renderJobs();

      await act(async () => {
        result.current.setPollOptions(['A', 'B', 'C']);
      });

      await act(async () => {
        result.current.removePollOption(1);
      });

      expect(result.current.pollOptions).toEqual(['A', 'C']);
    });

    it('should keep options when exactly 2', async () => {
      const { result } = renderJobs();

      await act(async () => {
        result.current.setPollOptions(['A', 'B']);
      });

      await act(async () => {
        result.current.removePollOption(0);
      });

      expect(result.current.pollOptions).toEqual(['A', 'B']);
    });
  });

  // ── 13. updatePollOption ─────────────────────────────────────
  describe('updatePollOption', () => {
    it('should update option at correct index', async () => {
      const { result } = renderJobs();

      await act(async () => {
        result.current.setPollOptions(['A', 'B', 'C']);
      });

      await act(async () => {
        result.current.updatePollOption(1, 'Updated');
      });

      expect(result.current.pollOptions).toEqual(['A', 'Updated', 'C']);
    });
  });

  // ── 14. getStatusBadgeConfig ─────────────────────────────────
  describe('getStatusBadgeConfig', () => {
    it('should return correct config for pending', () => {
      const { result } = renderJobs();
      const config = result.current.getStatusBadgeConfig('pending');
      expect(config.label).toBe('pages.jobs.status.pending');
      expect(config.variant).toBe('default');
    });

    it('should return correct config for running', () => {
      const { result } = renderJobs();
      const config = result.current.getStatusBadgeConfig('running');
      expect(config.label).toBe('pages.jobs.status.running');
      expect(config.variant).toBe('secondary');
    });

    it('should return correct config for done', () => {
      const { result } = renderJobs();
      const config = result.current.getStatusBadgeConfig('done');
      expect(config.label).toBe('pages.jobs.status.done');
      // The implementation uses 'success' variant
      expect(config.variant).toBe('success');
    });

    it('should return correct config for failed', () => {
      const { result } = renderJobs();
      const config = result.current.getStatusBadgeConfig('failed');
      expect(config.label).toBe('pages.jobs.status.failed');
      expect(config.variant).toBe('destructive');
    });

    it('should return correct config for cancelled', () => {
      const { result } = renderJobs();
      const config = result.current.getStatusBadgeConfig('cancelled');
      expect(config.label).toBe('pages.jobs.status.cancelled');
      expect(config.variant).toBe('outline');
    });

    it('should fallback to pending config for unknown status', () => {
      const { result } = renderJobs();
      const config = result.current.getStatusBadgeConfig('unknown' as import('../use-jobs-connect').LocalJobStatus);
      expect(config.label).toBe('pages.jobs.status.pending');
      expect(config.variant).toBe('default');
    });
  });

  // ── 15. getActionLabel ───────────────────────────────────────
  describe('getActionLabel', () => {
    it.each([
      ['send_message', 'pages.jobs.actionTypes.sendMessage'],
      ['send_poll', 'pages.jobs.actionTypes.sendPoll'],
      ['update_description', 'pages.jobs.actionTypes.updateDescription'],
      ['close_groups', 'pages.jobs.actionTypes.closeGroups'],
      ['open_groups', 'pages.jobs.actionTypes.openGroups'],
      ['change_group_name', 'pages.jobs.actionTypes.changeGroupName'],
      ['change_group_photo', 'pages.jobs.actionTypes.changeGroupPhoto'],
    ])('should return i18n label for %s', (action, expected) => {
      const { result } = renderJobs();
      expect(result.current.getActionLabel(action)).toBe(expected);
    });

    it('should return raw string for unknown action type', () => {
      const { result } = renderJobs();
      expect(result.current.getActionLabel('custom_action')).toBe('custom_action');
    });
  });

  // ── 16. groups mapping ───────────────────────────────────────
  describe('groups mapping', () => {
    it('should map groupsData to Group interface', () => {
      const { result } = renderJobs();

      expect(result.current.groups).toEqual([
        { id: 'g1', name: 'Group 1', wa_group_id: '111@g.us', is_favorite: true },
        { id: 'g2', name: 'Group 2', wa_group_id: '222@g.us', is_favorite: false },
      ]);
    });
  });

  // ── 17. isPending ────────────────────────────────────────────
  describe('isPending', () => {
    it('should be false when no mutations are pending', () => {
      const { result } = renderJobs();
      expect(result.current.isPending).toBe(false);
    });
  });

  // ── 18. SSE (useJobsRealtime) ────────────────────────────────
  describe('useJobsRealtime', () => {
    it('should call useJobsRealtime with refetchJobs', () => {
      renderJobs();
      expect(mockUseJobsRealtime).toHaveBeenCalled();
    });
  });
});
