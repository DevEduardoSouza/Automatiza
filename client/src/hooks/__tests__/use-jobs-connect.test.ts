import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Hoisted mocks (available before vi.mock factories)
// ============================================================

const {
  mockToast,
  mockClientCreateJob,
  mockClientUpdateJob,
  mockClientDeleteJob,
  mockClientProcessScheduledJobs,
  mockClient,
  mockRefetchJobsQuery,
  mockRefetchQueueQuery,
  mockUseQuery,
  mockTransport,
  mockInvalidateQueries,
  mockQueryClient,
  mockUseMutation,
  capturedMutations,
  mockUserRef,
  mockJobsDataRef,
  mockQueueDataRef,
} = vi.hoisted(() => {
  const mockToast = vi.fn();
  const mockClientCreateJob = vi.fn();
  const mockClientUpdateJob = vi.fn();
  const mockClientDeleteJob = vi.fn();
  const mockClientProcessScheduledJobs = vi.fn();
  const mockClient = {
    createJob: mockClientCreateJob,
    updateJob: mockClientUpdateJob,
    deleteJob: mockClientDeleteJob,
    processScheduledJobs: mockClientProcessScheduledJobs,
  };
  const mockRefetchJobsQuery = vi.fn();
  const mockRefetchQueueQuery = vi.fn();

  const mockJobsDataRef = { current: { jobs: [] as unknown[], nextPageToken: '', totalCount: 0 } };
  const mockQueueDataRef = { current: undefined as unknown };
  const mockUserRef = { current: { id: 'user-1' } as { id: string } | null };

  const mockUseQuery = vi.fn().mockImplementation((schema: unknown) => {
    if (schema === 'listJobs') {
      return { data: mockJobsDataRef.current, isLoading: false, refetch: mockRefetchJobsQuery };
    }
    return { data: mockQueueDataRef.current, isLoading: false, refetch: mockRefetchQueueQuery };
  });
  const mockTransport = {};
  const mockInvalidateQueries = vi.fn();
  const mockQueryClient = { invalidateQueries: mockInvalidateQueries };

  type MutationConfig = {
    mutationFn: (...args: unknown[]) => Promise<unknown>;
    onSuccess?: (data: unknown) => void;
    onError?: (error: unknown) => void;
  };
  const capturedMutations: MutationConfig[] = [];

  const mockUseMutation = vi.fn().mockImplementation((config: MutationConfig) => {
    capturedMutations.push(config);
    return {
      mutateAsync: config.mutationFn,
      mutate: (...args: unknown[]) => {
        const result = config.mutationFn(...args);
        result.then((data: unknown) => config.onSuccess?.(data)).catch((err: unknown) => config.onError?.(err));
      },
      isPending: false,
    };
  });

  return {
    mockToast,
    mockClientCreateJob,
    mockClientUpdateJob,
    mockClientDeleteJob,
    mockClientProcessScheduledJobs,
    mockClient,
    mockRefetchJobsQuery,
    mockRefetchQueueQuery,
    mockUseQuery,
    mockTransport,
    mockInvalidateQueries,
    mockQueryClient,
    mockUseMutation,
    capturedMutations,
    mockUserRef,
    mockJobsDataRef,
    mockQueueDataRef,
  };
});

// ============================================================
// Mock Modules
// ============================================================

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

vi.mock('@/contexts', () => ({
  useAuth: () => ({
    user: mockUserRef.current,
  }),
}));

vi.mock('@connectrpc/connect', () => ({
  createClient: () => mockClient,
}));

vi.mock('@connectrpc/connect-query', () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useTransport: () => mockTransport,
  createConnectQueryKey: () => ['mock-key'],
}));

vi.mock('@tanstack/react-query', () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
  useQueryClient: () => mockQueryClient,
}));

vi.mock('@/lib/connect-transport', () => ({
  getErrorInfo: () => ({ userMessage: 'error' }),
}));

vi.mock('@bufbuild/protobuf', () => ({
  create: (_schema: unknown, data: unknown) => data,
}));

vi.mock('@bufbuild/protobuf/wkt', () => ({
  timestampFromDate: (d: Date) => ({
    seconds: BigInt(Math.floor(d.getTime() / 1000)),
    nanos: 0,
  }),
}));

vi.mock('@/gen/jobs/v1/jobs-JobsService_connectquery', () => ({
  listJobs: 'listJobs',
  getQueueStatus: 'getQueueStatus',
}));

vi.mock('@/gen/jobs/v1/jobs_pb', () => ({
  CreateJobRequestSchema: 'CreateJobRequestSchema',
  DeleteJobRequestSchema: 'DeleteJobRequestSchema',
  UpdateJobRequestSchema: 'UpdateJobRequestSchema',
  ProcessScheduledJobsRequestSchema: 'ProcessScheduledJobsRequestSchema',
  JobPayloadSchema: 'JobPayloadSchema',
  JobsService: 'JobsService',
  JobActionType: {
    UNSPECIFIED: 0,
    SEND_MESSAGE: 1,
    SEND_POLL: 2,
    SEND_ML_OFFER: 3,
    UPDATE_DESCRIPTION: 4,
    CLOSE_GROUPS: 5,
    OPEN_GROUPS: 6,
    CHANGE_GROUP_NAME: 7,
    CHANGE_GROUP_PHOTO: 8,
  },
}));

// Import after mocks
import { useJobsConnect } from '../use-jobs-connect';

// ============================================================
// Constants (mirror proto enum values for test readability)
// ============================================================

const ACTION = {
  UNSPECIFIED: 0,
  SEND_MESSAGE: 1,
  SEND_POLL: 2,
  SEND_ML_OFFER: 3,
  UPDATE_DESCRIPTION: 4,
  CLOSE_GROUPS: 5,
  OPEN_GROUPS: 6,
  CHANGE_GROUP_NAME: 7,
  CHANGE_GROUP_PHOTO: 8,
} as const;

// ============================================================
// Helpers
// ============================================================

function makeProtoTimestamp(date: Date) {
  return {
    seconds: BigInt(Math.floor(date.getTime() / 1000)),
    nanos: 0,
  };
}

function makeProtoJob(overrides: Record<string, unknown> = {}) {
  const now = new Date('2026-01-15T10:00:00Z');
  return {
    id: 'job-1',
    actionType: ACTION.SEND_MESSAGE,
    scheduledFor: makeProtoTimestamp(now),
    status: 1,
    payload: {
      groups: ['group-1', 'group-2'],
      message: 'Hello',
      media: undefined,
      mediaType: undefined,
      fileName: undefined,
      mimeType: undefined,
      type: undefined,
      question: undefined,
      options: undefined,
      selectableCount: undefined,
      originUrl: undefined,
      text: undefined,
      includeImage: undefined,
      description: undefined,
      name: undefined,
      autoNumber: undefined,
      image: undefined,
    },
    errorMessage: '',
    createdAt: makeProtoTimestamp(now),
    updatedAt: makeProtoTimestamp(now),
    ...overrides,
  };
}

function renderJobsConnect() {
  return renderHook(() => useJobsConnect());
}

// ============================================================
// Tests
// ============================================================

describe('useJobsConnect', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    capturedMutations.length = 0;
    mockUserRef.current = { id: 'user-1' };
    mockJobsDataRef.current = { jobs: [], nextPageToken: '', totalCount: 0 };
    mockQueueDataRef.current = undefined;

    mockClientCreateJob.mockResolvedValue({ job: makeProtoJob() });
    mockClientUpdateJob.mockResolvedValue({ job: makeProtoJob() });
    mockClientDeleteJob.mockResolvedValue({});
    mockClientProcessScheduledJobs.mockResolvedValue({ processed: 1, failed: 0 });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ============================================================
  // 1. Converter Tests (indirect via hook output)
  // ============================================================

  describe('Converters (fromProtoJob / timestampToIso / fromProtoQueueStatus)', () => {
    it('should map SEND_MESSAGE action type correctly', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({ actionType: ACTION.SEND_MESSAGE })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(result.current.jobs[0].action_type).toBe('send_message');
    });

    it('should map SEND_POLL action type correctly', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({ actionType: ACTION.SEND_POLL })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(result.current.jobs[0].action_type).toBe('send_poll');
    });

    it('should map SEND_ML_OFFER action type correctly', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({ actionType: ACTION.SEND_ML_OFFER })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(result.current.jobs[0].action_type).toBe('send_ml_offer');
    });

    it('should map UPDATE_DESCRIPTION action type correctly', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({ actionType: ACTION.UPDATE_DESCRIPTION })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(result.current.jobs[0].action_type).toBe('update_description');
    });

    it('should map CLOSE_GROUPS action type correctly', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({ actionType: ACTION.CLOSE_GROUPS })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(result.current.jobs[0].action_type).toBe('close_groups');
    });

    it('should map OPEN_GROUPS action type correctly', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({ actionType: ACTION.OPEN_GROUPS })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(result.current.jobs[0].action_type).toBe('open_groups');
    });

    it('should map CHANGE_GROUP_NAME action type correctly', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({ actionType: ACTION.CHANGE_GROUP_NAME })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(result.current.jobs[0].action_type).toBe('change_group_name');
    });

    it('should map CHANGE_GROUP_PHOTO action type correctly', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({ actionType: ACTION.CHANGE_GROUP_PHOTO })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(result.current.jobs[0].action_type).toBe('change_group_photo');
    });

    it('should map UNSPECIFIED action type to send_message', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({ actionType: ACTION.UNSPECIFIED })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(result.current.jobs[0].action_type).toBe('send_message');
    });

    it('should map all proto status values correctly', () => {
      const statusMap: Record<number, string> = {
        0: 'pending',
        1: 'pending',
        2: 'running',
        3: 'done',
        4: 'failed',
        5: 'cancelled',
      };

      for (const [protoStatus, expected] of Object.entries(statusMap)) {
        mockJobsDataRef.current = {
          jobs: [makeProtoJob({ status: Number(protoStatus) })],
          nextPageToken: '',
          totalCount: 1,
        };

        const { result } = renderJobsConnect();
        expect(result.current.jobs[0].status).toBe(expected);
      }
    });

    it('should handle null payload by returning empty groups', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({ payload: null })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(result.current.jobs[0].payload.groups).toEqual([]);
    });

    it('should handle undefined payload by returning empty groups', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({ payload: undefined })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(result.current.jobs[0].payload.groups).toEqual([]);
    });

    it('should convert valid timestamp to ISO string', () => {
      const date = new Date('2026-01-15T10:00:00Z');
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({
          scheduledFor: makeProtoTimestamp(date),
          createdAt: makeProtoTimestamp(date),
          updatedAt: makeProtoTimestamp(date),
        })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(result.current.jobs[0].scheduled_for).toBe(date.toISOString());
      expect(result.current.jobs[0].created_at).toBe(date.toISOString());
      expect(result.current.jobs[0].updated_at).toBe(date.toISOString());
    });

    it('should handle timestamp with nanos', () => {
      const date = new Date('2026-01-15T10:00:00.500Z');
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({
          scheduledFor: {
            seconds: BigInt(Math.floor(date.getTime() / 1000)),
            nanos: 500_000_000,
          },
        })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(result.current.jobs[0].scheduled_for).toContain('2026-01-15T10:00:00');
    });

    it('should handle undefined timestamp by returning current date ISO', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({ scheduledFor: undefined })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(() => new Date(result.current.jobs[0].scheduled_for)).not.toThrow();
    });

    it('should handle timestamp with 0 seconds by returning current date ISO', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({ scheduledFor: { seconds: BigInt(0), nanos: 0 } })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(() => new Date(result.current.jobs[0].scheduled_for)).not.toThrow();
    });

    it('should map full payload fields correctly', () => {
      const fullPayload = {
        groups: ['g1', 'g2'],
        message: 'test msg',
        media: 'media-url',
        mediaType: 'image',
        fileName: 'photo.jpg',
        mimeType: 'image/jpeg',
        type: 'broadcast',
        question: 'What?',
        options: ['a', 'b'],
        selectableCount: 1,
        originUrl: 'https://example.com',
        text: 'some text',
        includeImage: true,
        description: 'desc',
        name: 'my-job',
        autoNumber: false,
        image: 'img-data',
      };

      mockJobsDataRef.current = {
        jobs: [makeProtoJob({ payload: fullPayload })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      const p = result.current.jobs[0].payload;
      expect(p.groups).toEqual(['g1', 'g2']);
      expect(p.message).toBe('test msg');
      expect(p.media).toBe('media-url');
      expect(p.mediaType).toBe('image');
      expect(p.fileName).toBe('photo.jpg');
      expect(p.mimeType).toBe('image/jpeg');
      expect(p.type).toBe('broadcast');
      expect(p.question).toBe('What?');
      expect(p.options).toEqual(['a', 'b']);
      expect(p.selectableCount).toBe(1);
      expect(p.originUrl).toBe('https://example.com');
      expect(p.text).toBe('some text');
      expect(p.includeImage).toBe(true);
      expect(p.description).toBe('desc');
      expect(p.name).toBe('my-job');
      expect(p.autoNumber).toBe(false);
      expect(p.image).toBe('img-data');
    });

    it('should map errorMessage correctly', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob({ errorMessage: 'something went wrong' })],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(result.current.jobs[0].error_message).toBe('something went wrong');
    });

    it('should convert fromProtoQueueStatus with valid data', () => {
      mockQueueDataRef.current = {
        queue: { waiting: 5, delayed: 2, active: 1, queued: 8 },
        settings: { limitPerMinute: 60, workerConcurrency: 4 },
        estimatedMinutes: 3,
      };

      const { result } = renderJobsConnect();
      expect(result.current.queueStatus).toEqual({
        queue: { waiting: 5, delayed: 2, active: 1, queued: 8 },
        settings: { limitPerMinute: 60, workerConcurrency: 4 },
        estimatedMinutes: 3,
      });
    });

    it('should return null queueStatus when response is undefined', () => {
      mockQueueDataRef.current = undefined;

      const { result } = renderJobsConnect();
      expect(result.current.queueStatus).toBeNull();
    });

    it('should return null queueStatus when queue is missing', () => {
      mockQueueDataRef.current = { settings: { limitPerMinute: 10, workerConcurrency: 2 } };

      const { result } = renderJobsConnect();
      expect(result.current.queueStatus).toBeNull();
    });

    it('should return null queueStatus when settings is missing', () => {
      mockQueueDataRef.current = { queue: { waiting: 1, delayed: 0, active: 0, queued: 1 } };

      const { result } = renderJobsConnect();
      expect(result.current.queueStatus).toBeNull();
    });

    it('should handle null limitPerMinute and workerConcurrency', () => {
      mockQueueDataRef.current = {
        queue: { waiting: 0, delayed: 0, active: 0, queued: 0 },
        settings: { limitPerMinute: undefined, workerConcurrency: undefined },
        estimatedMinutes: undefined,
      };

      const { result } = renderJobsConnect();
      expect(result.current.queueStatus).toEqual({
        queue: { waiting: 0, delayed: 0, active: 0, queued: 0 },
        settings: { limitPerMinute: null, workerConcurrency: null },
        estimatedMinutes: null,
      });
    });
  });

  // ============================================================
  // 2. Query Tests
  // ============================================================

  describe('Queries', () => {
    it('should disable listJobs query when user is null', () => {
      mockUserRef.current = null;
      renderJobsConnect();

      const listJobsCall = mockUseQuery.mock.calls.find(
        (call: unknown[]) => call[0] === 'listJobs'
      );
      expect(listJobsCall).toBeDefined();
      expect(listJobsCall![2]?.enabled).toBe(false);
    });

    it('should enable listJobs query when user exists', () => {
      mockUserRef.current = { id: 'user-1' };
      renderJobsConnect();

      const listJobsCall = mockUseQuery.mock.calls.find(
        (call: unknown[]) => call[0] === 'listJobs'
      );
      expect(listJobsCall).toBeDefined();
      expect(listJobsCall![2]?.enabled).toBe(true);
    });

    it('should return mapped jobs from listJobs query', () => {
      mockJobsDataRef.current = {
        jobs: [
          makeProtoJob({ id: 'job-1', actionType: ACTION.SEND_MESSAGE }),
          makeProtoJob({ id: 'job-2', actionType: ACTION.SEND_POLL }),
        ],
        nextPageToken: '',
        totalCount: 2,
      };

      const { result } = renderJobsConnect();
      expect(result.current.jobs).toHaveLength(2);
      expect(result.current.jobs[0].id).toBe('job-1');
      expect(result.current.jobs[0].action_type).toBe('send_message');
      expect(result.current.jobs[1].id).toBe('job-2');
      expect(result.current.jobs[1].action_type).toBe('send_poll');
    });

    it('should set hasNextPage to true when nextPageToken is present', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob()],
        nextPageToken: 'next-token',
        totalCount: 100,
      };

      const { result } = renderJobsConnect();
      expect(result.current.hasNextPage).toBe(true);
    });

    it('should set hasNextPage to false when nextPageToken is empty', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob()],
        nextPageToken: '',
        totalCount: 1,
      };

      const { result } = renderJobsConnect();
      expect(result.current.hasNextPage).toBe(false);
    });

    it('should return totalCount from query response', () => {
      mockJobsDataRef.current = { jobs: [], nextPageToken: '', totalCount: 42 };

      const { result } = renderJobsConnect();
      expect(result.current.totalCount).toBe(42);
    });

    it('should return mapped queue status from getQueueStatus query', () => {
      mockQueueDataRef.current = {
        queue: { waiting: 10, delayed: 3, active: 2, queued: 15 },
        settings: { limitPerMinute: 120, workerConcurrency: 8 },
        estimatedMinutes: 5,
      };

      const { result } = renderJobsConnect();
      expect(result.current.queueStatus).toEqual({
        queue: { waiting: 10, delayed: 3, active: 2, queued: 15 },
        settings: { limitPerMinute: 120, workerConcurrency: 8 },
        estimatedMinutes: 5,
      });
    });

    it('should handle empty jobs array', () => {
      mockJobsDataRef.current = { jobs: [], nextPageToken: '', totalCount: 0 };

      const { result } = renderJobsConnect();
      expect(result.current.jobs).toEqual([]);
      expect(result.current.totalCount).toBe(0);
    });
  });

  // ============================================================
  // 3. Pagination Tests
  // ============================================================

  describe('Pagination', () => {
    it('should call refetchJobs resetting pagination state', () => {
      mockJobsDataRef.current = {
        jobs: [makeProtoJob()],
        nextPageToken: 'page-2',
        totalCount: 100,
      };

      const { result } = renderJobsConnect();

      act(() => {
        result.current.refetchJobs();
      });

      expect(mockRefetchJobsQuery).toHaveBeenCalled();
    });

    it('should not crash when loadNextPage is called with no nextPageToken', () => {
      mockJobsDataRef.current = { jobs: [], nextPageToken: '', totalCount: 0 };

      const { result } = renderJobsConnect();

      act(() => {
        result.current.loadNextPage();
      });

      expect(result.current.jobs).toEqual([]);
    });
  });

  // ============================================================
  // 4. Mutation Tests
  // ============================================================

  describe('Mutations', () => {
    describe('createJob', () => {
      it('should call client.createJob with correct proto structure', async () => {
        const scheduledFor = new Date('2026-02-01T12:00:00Z');
        const createdJob = makeProtoJob({
          id: 'new-job-1',
          actionType: ACTION.SEND_MESSAGE,
        });
        mockClientCreateJob.mockResolvedValue({ job: createdJob });

        const { result } = renderJobsConnect();

        const localJob = await act(async () => {
          return result.current.createJob({
            action_type: 'send_message',
            scheduled_for: scheduledFor,
            payload: { groups: ['g1'] },
          });
        });

        expect(mockClientCreateJob).toHaveBeenCalledWith(
          expect.objectContaining({
            actionType: ACTION.SEND_MESSAGE,
            payload: expect.objectContaining({ groups: ['g1'] }),
          })
        );
        expect(localJob.id).toBe('new-job-1');
      });

      it('should throw when response.job is null', async () => {
        mockClientCreateJob.mockResolvedValue({ job: null });

        const { result } = renderJobsConnect();

        await expect(
          act(async () => {
            return result.current.createJob({
              action_type: 'send_message',
              scheduled_for: new Date(),
              payload: { groups: ['g1'] },
            });
          })
        ).rejects.toThrow('pages.jobs.errors.createFailed');
      });

      it('should throw when user is null', async () => {
        mockUserRef.current = null;

        const { result } = renderJobsConnect();

        await expect(
          act(async () => {
            return result.current.createJob({
              action_type: 'send_message',
              scheduled_for: new Date(),
              payload: { groups: ['g1'] },
            });
          })
        ).rejects.toThrow('common.errors.notAuthenticated');
      });

      it('should map all action types to proto correctly when creating', async () => {
        const actionMap: Record<string, number> = {
          send_message: ACTION.SEND_MESSAGE,
          send_poll: ACTION.SEND_POLL,
          send_ml_offer: ACTION.SEND_ML_OFFER,
          update_description: ACTION.UPDATE_DESCRIPTION,
          close_groups: ACTION.CLOSE_GROUPS,
          open_groups: ACTION.OPEN_GROUPS,
          change_group_name: ACTION.CHANGE_GROUP_NAME,
          change_group_photo: ACTION.CHANGE_GROUP_PHOTO,
        };

        for (const [localType, protoType] of Object.entries(actionMap)) {
          mockClientCreateJob.mockResolvedValue({ job: makeProtoJob({ actionType: protoType }) });

          const { result } = renderJobsConnect();

          await act(async () => {
            await result.current.createJob({
              action_type: localType as any,
              scheduled_for: new Date(),
              payload: { groups: [] },
            });
          });

          expect(mockClientCreateJob).toHaveBeenCalledWith(
            expect.objectContaining({ actionType: protoType })
          );
          mockClientCreateJob.mockClear();
        }
      });

      it('should pass payload options as empty array when undefined', async () => {
        mockClientCreateJob.mockResolvedValue({ job: makeProtoJob() });

        const { result } = renderJobsConnect();

        await act(async () => {
          await result.current.createJob({
            action_type: 'send_message',
            scheduled_for: new Date(),
            payload: { groups: ['g1'], options: undefined },
          });
        });

        expect(mockClientCreateJob).toHaveBeenCalledWith(
          expect.objectContaining({
            payload: expect.objectContaining({ options: [] }),
          })
        );
      });
    });

    describe('createBulkJobs', () => {
      it('should create multiple jobs sequentially', async () => {
        const job1 = makeProtoJob({ id: 'bulk-1' });
        const job2 = makeProtoJob({ id: 'bulk-2' });
        mockClientCreateJob
          .mockResolvedValueOnce({ job: job1 })
          .mockResolvedValueOnce({ job: job2 });

        const { result } = renderJobsConnect();

        const created = await act(async () => {
          return result.current.createBulkJobs([
            { action_type: 'send_message', scheduled_for: new Date(), payload: { groups: ['g1'] } },
            { action_type: 'send_poll', scheduled_for: new Date(), payload: { groups: ['g2'] } },
          ]);
        });

        expect(mockClientCreateJob).toHaveBeenCalledTimes(2);
        expect(created).toHaveLength(2);
        expect(created[0].id).toBe('bulk-1');
        expect(created[1].id).toBe('bulk-2');
      });

      it('should throw when user is null for bulk creation', async () => {
        mockUserRef.current = null;

        const { result } = renderJobsConnect();

        await expect(
          act(async () => {
            return result.current.createBulkJobs([
              { action_type: 'send_message', scheduled_for: new Date(), payload: { groups: [] } },
            ]);
          })
        ).rejects.toThrow('common.errors.notAuthenticated');
      });

      it('should skip jobs where response.job is null', async () => {
        mockClientCreateJob
          .mockResolvedValueOnce({ job: makeProtoJob({ id: 'ok-1' }) })
          .mockResolvedValueOnce({ job: null })
          .mockResolvedValueOnce({ job: makeProtoJob({ id: 'ok-3' }) });

        const { result } = renderJobsConnect();

        const created = await act(async () => {
          return result.current.createBulkJobs([
            { action_type: 'send_message', scheduled_for: new Date(), payload: { groups: [] } },
            { action_type: 'send_message', scheduled_for: new Date(), payload: { groups: [] } },
            { action_type: 'send_message', scheduled_for: new Date(), payload: { groups: [] } },
          ]);
        });

        expect(created).toHaveLength(2);
        expect(created[0].id).toBe('ok-1');
        expect(created[1].id).toBe('ok-3');
      });

      it('should invalidate queries after bulk creation', async () => {
        mockClientCreateJob.mockResolvedValue({ job: makeProtoJob() });

        const { result } = renderJobsConnect();

        await act(async () => {
          await result.current.createBulkJobs([
            { action_type: 'send_message', scheduled_for: new Date(), payload: { groups: [] } },
          ]);
        });

        expect(mockInvalidateQueries).toHaveBeenCalled();
      });
    });

    describe('updateJob', () => {
      it('should call client.updateJob with correct proto', async () => {
        const updatedJob = makeProtoJob({ id: 'job-1', actionType: ACTION.SEND_POLL });
        mockClientUpdateJob.mockResolvedValue({ job: updatedJob });

        const { result } = renderJobsConnect();

        const localJob = await act(async () => {
          return result.current.updateJob({
            id: 'job-1',
            action_type: 'send_poll',
            scheduled_for: new Date('2026-03-01T00:00:00Z'),
            payload: { groups: ['g3'] },
          });
        });

        expect(mockClientUpdateJob).toHaveBeenCalledWith(
          expect.objectContaining({
            id: 'job-1',
            actionType: ACTION.SEND_POLL,
          })
        );
        expect(localJob.id).toBe('job-1');
      });

      it('should throw when response.job is null', async () => {
        mockClientUpdateJob.mockResolvedValue({ job: null });

        const { result } = renderJobsConnect();

        await expect(
          act(async () => {
            return result.current.updateJob({ id: 'job-1' });
          })
        ).rejects.toThrow('pages.jobs.errors.updateFailed');
      });

      it('should throw when user is null', async () => {
        mockUserRef.current = null;

        const { result } = renderJobsConnect();

        await expect(
          act(async () => {
            return result.current.updateJob({ id: 'job-1' });
          })
        ).rejects.toThrow('common.errors.notAuthenticated');
      });

      it('should pass undefined for optional fields not provided', async () => {
        mockClientUpdateJob.mockResolvedValue({ job: makeProtoJob() });

        const { result } = renderJobsConnect();

        await act(async () => {
          await result.current.updateJob({ id: 'job-1' });
        });

        expect(mockClientUpdateJob).toHaveBeenCalledWith(
          expect.objectContaining({
            id: 'job-1',
            actionType: undefined,
            scheduledFor: undefined,
            payload: undefined,
          })
        );
      });
    });

    describe('deleteJob', () => {
      it('should call client.deleteJob with correct id', async () => {
        const { result } = renderJobsConnect();

        await act(async () => {
          await result.current.deleteJob('job-to-delete');
        });

        expect(mockClientDeleteJob).toHaveBeenCalledWith(
          expect.objectContaining({ id: 'job-to-delete' })
        );
      });

      it('should throw when user is null', async () => {
        mockUserRef.current = null;

        const { result } = renderJobsConnect();

        await expect(
          act(async () => {
            await result.current.deleteJob('job-1');
          })
        ).rejects.toThrow('common.errors.notAuthenticated');
      });
    });

    describe('processScheduledJobs', () => {
      it('should call processScheduledJobs RPC', () => {
        const { result } = renderJobsConnect();

        act(() => {
          result.current.processScheduledJobs();
        });

        expect(mockClientProcessScheduledJobs).toHaveBeenCalled();
      });

      it('should call toast with success on resolved', async () => {
        mockClientProcessScheduledJobs.mockResolvedValue({ processed: 5, failed: 1 });

        const { result } = renderJobsConnect();

        await act(async () => {
          result.current.processScheduledJobs();
          await vi.waitFor(() => {
            expect(mockToast).toHaveBeenCalled();
          });
        });

        expect(mockToast).toHaveBeenCalledWith(
          expect.objectContaining({
            title: 'pages.jobs.success.processed',
          })
        );
      });
    });

    describe('Error handlers', () => {
      it('should call toast with destructive variant on createJob error via onError', () => {
        renderJobsConnect();

        // createJob mutation is the 2nd (processJobs=0, createJob=1)
        const mutations = capturedMutations.filter((m) => m.onError !== undefined);
        expect(mutations.length).toBeGreaterThanOrEqual(2);

        mutations[1].onError!(new Error('create failed'));

        expect(mockToast).toHaveBeenCalledWith(
          expect.objectContaining({
            variant: 'destructive',
            description: 'error',
          })
        );
      });

      it('should call toast with destructive variant on updateJob error via onError', () => {
        renderJobsConnect();

        // updateJob mutation is the 3rd (processJobs=0, createJob=1, updateJob=2)
        const mutations = capturedMutations.filter((m) => m.onError !== undefined);
        expect(mutations.length).toBeGreaterThanOrEqual(3);

        mutations[2].onError!(new Error('update failed'));

        expect(mockToast).toHaveBeenCalledWith(
          expect.objectContaining({
            variant: 'destructive',
          })
        );
      });

      it('should call toast with destructive variant on deleteJob error via onError', () => {
        renderJobsConnect();

        // deleteJob mutation is the 4th (processJobs=0, createJob=1, updateJob=2, deleteJob=3)
        const mutations = capturedMutations.filter((m) => m.onError !== undefined);
        expect(mutations.length).toBeGreaterThanOrEqual(4);

        mutations[3].onError!(new Error('delete failed'));

        expect(mockToast).toHaveBeenCalledWith(
          expect.objectContaining({
            variant: 'destructive',
          })
        );
      });

      it('should call toast with destructive variant on processScheduledJobs error', async () => {
        mockClientProcessScheduledJobs.mockRejectedValue(new Error('process failed'));

        const { result } = renderJobsConnect();

        act(() => {
          result.current.processScheduledJobs();
        });

        await act(async () => {
          await vi.waitFor(() => {
            expect(mockToast).toHaveBeenCalled();
          });
        });

        expect(mockToast).toHaveBeenCalledWith(
          expect.objectContaining({
            variant: 'destructive',
            title: 'pages.jobs.errors.process',
            description: 'error',
          })
        );
      });
    });
  });

  // ============================================================
  // 5. Return Shape Tests
  // ============================================================

  describe('Return shape', () => {
    it('should return all 18 expected properties', () => {
      const { result } = renderJobsConnect();

      const expectedKeys = [
        'jobs',
        'loadingJobs',
        'hasNextPage',
        'totalCount',
        'loadNextPage',
        'queueStatus',
        'loading',
        'isPending',
        'isCreatingJob',
        'isUpdatingJob',
        'isDeletingJob',
        'refetch',
        'refetchJobs',
        'processScheduledJobs',
        'createJob',
        'createBulkJobs',
        'updateJob',
        'deleteJob',
      ];

      for (const key of expectedKeys) {
        expect(result.current).toHaveProperty(key);
      }
      expect(Object.keys(result.current)).toHaveLength(18);
    });

    it('should have correct types for all properties', () => {
      const { result } = renderJobsConnect();

      expect(Array.isArray(result.current.jobs)).toBe(true);
      expect(typeof result.current.loadingJobs).toBe('boolean');
      expect(typeof result.current.hasNextPage).toBe('boolean');
      expect(typeof result.current.totalCount).toBe('number');
      expect(typeof result.current.loadNextPage).toBe('function');
      expect(typeof result.current.loading).toBe('boolean');
      expect(typeof result.current.isPending).toBe('boolean');
      expect(typeof result.current.isCreatingJob).toBe('boolean');
      expect(typeof result.current.isUpdatingJob).toBe('boolean');
      expect(typeof result.current.isDeletingJob).toBe('boolean');
      expect(typeof result.current.refetch).toBe('function');
      expect(typeof result.current.refetchJobs).toBe('function');
      expect(typeof result.current.processScheduledJobs).toBe('function');
      expect(typeof result.current.createJob).toBe('function');
      expect(typeof result.current.createBulkJobs).toBe('function');
      expect(typeof result.current.updateJob).toBe('function');
      expect(typeof result.current.deleteJob).toBe('function');
    });

    it('should default boolean states to false', () => {
      const { result } = renderJobsConnect();

      expect(result.current.loadingJobs).toBe(false);
      expect(result.current.loading).toBe(false);
      expect(result.current.isPending).toBe(false);
      expect(result.current.isCreatingJob).toBe(false);
      expect(result.current.isUpdatingJob).toBe(false);
      expect(result.current.isDeletingJob).toBe(false);
    });
  });
});
