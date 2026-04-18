import { useAuth } from '@/contexts';
import { getQueueStatus, listJobs } from '@/gen/jobs/v1/jobs-JobsService_connectquery';
import {
  CreateJobRequestSchema,
  DeleteJobRequestSchema,
  type GetQueueStatusResponse,
  type Job,
  JobActionType,
  type JobPayload,
  JobPayloadSchema,
  JobsService,
  ProcessScheduledJobsRequestSchema,
  UpdateJobRequestSchema,
} from '@/gen/jobs/v1/jobs_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { createClient } from '@connectrpc/connect';
import { createConnectQueryKey, useQuery, useTransport } from '@connectrpc/connect-query';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

// ============================================================
// Local Types
// ============================================================

export type LocalJobActionType =
  | 'send_message'
  | 'send_poll'
  | 'update_description'
  | 'close_groups'
  | 'open_groups'
  | 'change_group_name'
  | 'change_group_photo';

export type LocalJobStatus = 'pending' | 'running' | 'done' | 'failed' | 'cancelled';

export interface LocalJobPayload {
  groups: string[];
  message?: string;
  media?: string;
  mediaType?: string;
  fileName?: string;
  mimeType?: string;
  type?: string;
  question?: string;
  options?: string[];
  selectableCount?: number;
  description?: string;
  name?: string;
  autoNumber?: boolean;
  image?: string;
}

export interface LocalJob {
  id: string;
  action_type: LocalJobActionType;
  scheduled_for: string;
  status: LocalJobStatus;
  payload: LocalJobPayload;
  error_message?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateJobParams {
  action_type: LocalJobActionType;
  scheduled_for: Date;
  payload: LocalJobPayload;
}

/**
 * Local queue status interface matching UI expectations (snake_case).
 */
interface LocalQueueStatus {
  queue: {
    waiting: number;
    delayed: number;
    active: number;
    queued: number;
  };
  settings: {
    limitPerMinute: number | null;
    workerConcurrency: number | null;
  };
  estimatedMinutes: number | null;
}

export interface UpdateJobParams {
  id: string;
  action_type?: LocalJobActionType;
  scheduled_for?: Date;
  payload?: LocalJobPayload;
}

interface UseJobsConnectReturn {
  /** List of jobs (accumulated across pages) */
  jobs: LocalJob[];
  /** Whether jobs are loading */
  loadingJobs: boolean;
  /** Whether there is a next page of results */
  hasNextPage: boolean;
  /** Total count of jobs */
  totalCount: number;
  /** Load next page of jobs */
  loadNextPage: () => void;
  /** Current queue status */
  queueStatus: LocalQueueStatus | null;
  /** Whether queue status is loading */
  loading: boolean;
  /** Whether a mutation is pending */
  isPending: boolean;
  /** Whether job creation is pending */
  isCreatingJob: boolean;
  /** Whether job update is pending */
  isUpdatingJob: boolean;
  /** Whether job deletion is pending */
  isDeletingJob: boolean;
  /** Refetch queue status */
  refetch: () => void;
  /** Refetch jobs list (resets pagination) */
  refetchJobs: () => void;
  /** Process scheduled jobs manually */
  processScheduledJobs: () => void;
  /** Create a single job */
  createJob: (params: CreateJobParams) => Promise<LocalJob>;
  /** Create multiple jobs at once */
  createBulkJobs: (jobs: CreateJobParams[]) => Promise<LocalJob[]>;
  /** Update an existing job */
  updateJob: (params: UpdateJobParams) => Promise<LocalJob>;
  /** Delete a job */
  deleteJob: (jobId: string) => Promise<void>;
}

// ============================================================
// Converters
// ============================================================

const ACTION_TYPE_TO_PROTO: Record<LocalJobActionType, JobActionType> = {
  send_message: JobActionType.SEND_MESSAGE,
  send_poll: JobActionType.SEND_POLL,
  update_description: JobActionType.UPDATE_DESCRIPTION,
  close_groups: JobActionType.CLOSE_GROUPS,
  open_groups: JobActionType.OPEN_GROUPS,
  change_group_name: JobActionType.CHANGE_GROUP_NAME,
  change_group_photo: JobActionType.CHANGE_GROUP_PHOTO,
};

const PROTO_TO_ACTION_TYPE: Record<JobActionType, LocalJobActionType> = {
  [JobActionType.UNSPECIFIED]: 'send_message',
  [JobActionType.SEND_MESSAGE]: 'send_message',
  [JobActionType.SEND_POLL]: 'send_poll',
  [JobActionType.UPDATE_DESCRIPTION]: 'update_description',
  [JobActionType.CLOSE_GROUPS]: 'close_groups',
  [JobActionType.OPEN_GROUPS]: 'open_groups',
  [JobActionType.CHANGE_GROUP_NAME]: 'change_group_name',
  [JobActionType.CHANGE_GROUP_PHOTO]: 'change_group_photo',
};

const PROTO_TO_STATUS: Record<number, LocalJobStatus> = {
  0: 'pending', // UNSPECIFIED
  1: 'pending', // PENDING
  2: 'running', // RUNNING
  3: 'done', // DONE
  4: 'failed', // FAILED
  5: 'cancelled', // CANCELLED
};

function timestampToIso(timestamp: { seconds?: bigint; nanos?: number } | undefined): string {
  if (!timestamp?.seconds) return new Date().toISOString();
  const ms = Number(timestamp.seconds) * 1000 + (timestamp.nanos || 0) / 1_000_000;
  return new Date(ms).toISOString();
}

function fromProtoJob(job: Job): LocalJob {
  return {
    id: job.id,
    action_type: PROTO_TO_ACTION_TYPE[job.actionType] || 'send_message',
    scheduled_for: timestampToIso(job.scheduledFor),
    status: PROTO_TO_STATUS[job.status as number] || 'pending',
    payload: {
      groups: job.payload?.groups || [],
      message: job.payload?.message,
      media: job.payload?.media,
      mediaType: job.payload?.mediaType,
      fileName: job.payload?.fileName,
      mimeType: job.payload?.mimeType,
      type: job.payload?.type,
      question: job.payload?.question,
      options: job.payload?.options,
      selectableCount: job.payload?.selectableCount,
      description: job.payload?.description,
      name: job.payload?.name,
      autoNumber: job.payload?.autoNumber,
      image: job.payload?.image,
    },
    error_message: job.errorMessage,
    created_at: timestampToIso(job.createdAt),
    updated_at: timestampToIso(job.updatedAt),
  };
}

function toProtoPayload(payload: LocalJobPayload): JobPayload {
  return create(JobPayloadSchema, {
    groups: payload.groups,
    message: payload.message,
    media: payload.media,
    mediaType: payload.mediaType,
    fileName: payload.fileName,
    mimeType: payload.mimeType,
    type: payload.type,
    question: payload.question,
    options: payload.options || [],
    selectableCount: payload.selectableCount,
    description: payload.description,
    name: payload.name,
    autoNumber: payload.autoNumber,
    image: payload.image,
  });
}

/**
 * Converts proto GetQueueStatusResponse to local format.
 */
function fromProtoQueueStatus(
  response: GetQueueStatusResponse | undefined
): LocalQueueStatus | null {
  if (!response?.queue || !response?.settings) {
    return null;
  }

  return {
    queue: {
      waiting: response.queue.waiting,
      delayed: response.queue.delayed,
      active: response.queue.active,
      queued: response.queue.queued,
    },
    settings: {
      limitPerMinute: response.settings.limitPerMinute ?? null,
      workerConcurrency: response.settings.workerConcurrency ?? null,
    },
    estimatedMinutes: response.estimatedMinutes ?? null,
  };
}

/**
 * Hook for jobs queue operations via ConnectRPC.
 * Provides queue status, job listing, and job creation.
 */
export function useJobsConnect(): UseJobsConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const transport = useTransport();

  // ============================================================
  // Query: List Jobs (cursor-based pagination)
  // ============================================================

  const PAGE_SIZE = 50;
  const [pageToken, setPageToken] = useState('');
  const [accumulatedJobs, setAccumulatedJobs] = useState<LocalJob[]>([]);
  const [isAccumulating, setIsAccumulating] = useState(false);

  const {
    data: jobsResponse,
    isLoading: loadingJobs,
    refetch: refetchJobsQuery,
  } = useQuery(
    listJobs,
    { pageSize: PAGE_SIZE, pageToken },
    {
      enabled: !!user,
      staleTime: 30_000,
      refetchInterval: pageToken ? false : 60_000, // Only auto-refresh first page
    }
  );

  const currentPageJobs = useMemo(() => {
    return (jobsResponse?.jobs || []).map(fromProtoJob);
  }, [jobsResponse]);

  const nextPageToken = jobsResponse?.nextPageToken || '';
  const totalCount = jobsResponse?.totalCount || 0;

  // Update accumulated jobs when new page data arrives
  useEffect(() => {
    if (isAccumulating && currentPageJobs.length > 0) {
      const seen = new Set(accumulatedJobs.map((j) => j.id));
      const newJobs = currentPageJobs.filter((j) => !seen.has(j.id));
      if (newJobs.length > 0) {
        setAccumulatedJobs((prev) => [...prev, ...newJobs]);
      }
      setIsAccumulating(false);
    }
  }, [isAccumulating, currentPageJobs, accumulatedJobs]);

  // Merge accumulated jobs with current page
  const jobs = useMemo(() => {
    if (pageToken === '') {
      // First page - just return current
      return currentPageJobs;
    }
    // Accumulated + current (deduplicated)
    const seen = new Set(accumulatedJobs.map((j) => j.id));
    const newJobs = currentPageJobs.filter((j) => !seen.has(j.id));
    return [...accumulatedJobs, ...newJobs];
  }, [currentPageJobs, accumulatedJobs, pageToken]);

  const hasNextPage = nextPageToken !== '';

  const loadNextPage = useCallback(() => {
    if (nextPageToken) {
      setAccumulatedJobs(jobs);
      setIsAccumulating(true);
      setPageToken(nextPageToken);
    }
  }, [nextPageToken, jobs]);

  const refetchJobs = useCallback(() => {
    // Reset pagination and refetch from first page
    setPageToken('');
    setAccumulatedJobs([]);
    setIsAccumulating(false);
    refetchJobsQuery();
  }, [refetchJobsQuery]);

  // ============================================================
  // Query: Queue Status
  // ============================================================

  const {
    data: queueResponse,
    isLoading: loading,
    refetch,
  } = useQuery(
    getQueueStatus,
    {},
    {
      enabled: !!user,
      staleTime: 30_000,
      refetchInterval: 60_000,
    }
  );

  const queueStatus = useMemo(() => fromProtoQueueStatus(queueResponse), [queueResponse]);

  // ============================================================
  // Mutation: Process Scheduled Jobs
  // ============================================================

  const processJobsMutation = useMutation({
    mutationFn: async () => {
      const client = createClient(JobsService, transport);
      return client.processScheduledJobs(create(ProcessScheduledJobsRequestSchema, {}));
    },
    onSuccess: (response) => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: getQueueStatus, cardinality: 'finite' }),
      });
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listJobs, cardinality: 'finite' }),
      });
      toast({
        title: t('pages.jobs.success.processed'),
        description: t('pages.jobs.success.processedDetails', {
          processed: response.processed,
          failed: response.failed,
        }),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('pages.jobs.errors.process'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Mutation: Create Job
  // ============================================================

  const createJobMutation = useMutation({
    mutationFn: async (params: CreateJobParams): Promise<LocalJob> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(JobsService, transport);
      const response = await client.createJob(
        create(CreateJobRequestSchema, {
          actionType: ACTION_TYPE_TO_PROTO[params.action_type],
          scheduledFor: timestampFromDate(params.scheduled_for),
          payload: toProtoPayload(params.payload),
        })
      );

      if (!response.job) {
        throw new Error(t('pages.jobs.errors.createFailed'));
      }

      return fromProtoJob(response.job);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listJobs, cardinality: 'finite' }),
      });
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('pages.jobs.errors.schedule'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Callback: Create Job
  // ============================================================

  const createJob = useCallback(
    async (params: CreateJobParams): Promise<LocalJob> => {
      return createJobMutation.mutateAsync(params);
    },
    [createJobMutation]
  );

  // ============================================================
  // Callback: Create Bulk Jobs
  // ============================================================

  const createBulkJobs = useCallback(
    async (jobsList: CreateJobParams[]): Promise<LocalJob[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(JobsService, transport);
      const createdJobs: LocalJob[] = [];

      for (const params of jobsList) {
        const response = await client.createJob(
          create(CreateJobRequestSchema, {
            actionType: ACTION_TYPE_TO_PROTO[params.action_type],
            scheduledFor: timestampFromDate(params.scheduled_for),
            payload: toProtoPayload(params.payload),
          })
        );

        if (response.job) {
          createdJobs.push(fromProtoJob(response.job));
        }
      }

      // Invalidate queries after bulk creation
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listJobs, cardinality: 'finite' }),
      });
      queryClient.invalidateQueries({ queryKey: ['jobs'] });

      return createdJobs;
    },
    [user?.id, transport, queryClient, t]
  );

  // ============================================================
  // Mutation: Update Job
  // ============================================================

  const updateJobMutation = useMutation({
    mutationFn: async (params: UpdateJobParams): Promise<LocalJob> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(JobsService, transport);
      const response = await client.updateJob(
        create(UpdateJobRequestSchema, {
          id: params.id,
          actionType: params.action_type ? ACTION_TYPE_TO_PROTO[params.action_type] : undefined,
          scheduledFor: params.scheduled_for ? timestampFromDate(params.scheduled_for) : undefined,
          payload: params.payload ? toProtoPayload(params.payload) : undefined,
        })
      );

      if (!response.job) {
        throw new Error(t('pages.jobs.errors.updateFailed'));
      }

      return fromProtoJob(response.job);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listJobs, cardinality: 'finite' }),
      });
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('pages.jobs.errors.update'),
        description: errorInfo.userMessage,
      });
    },
  });

  const updateJob = useCallback(
    async (params: UpdateJobParams): Promise<LocalJob> => {
      return updateJobMutation.mutateAsync(params);
    },
    [updateJobMutation]
  );

  // ============================================================
  // Mutation: Delete Job
  // ============================================================

  const deleteJobMutation = useMutation({
    mutationFn: async (jobId: string): Promise<void> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(JobsService, transport);
      await client.deleteJob(create(DeleteJobRequestSchema, { id: jobId }));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listJobs, cardinality: 'finite' }),
      });
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('pages.jobs.errors.delete'),
        description: errorInfo.userMessage,
      });
    },
  });

  const deleteJob = useCallback(
    async (jobId: string): Promise<void> => {
      return deleteJobMutation.mutateAsync(jobId);
    },
    [deleteJobMutation]
  );

  return {
    jobs,
    loadingJobs,
    hasNextPage,
    totalCount,
    loadNextPage,
    queueStatus,
    loading,
    isPending: processJobsMutation.isPending,
    isCreatingJob: createJobMutation.isPending,
    isUpdatingJob: updateJobMutation.isPending,
    isDeletingJob: deleteJobMutation.isPending,
    refetch: () => refetch(),
    refetchJobs,
    processScheduledJobs: () => processJobsMutation.mutate(),
    createJob,
    createBulkJobs,
    updateJob,
    deleteJob,
  };
}
