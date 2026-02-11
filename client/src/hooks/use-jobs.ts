import { useAuth } from '@/contexts';
import { useAiToolsConnect } from '@/hooks/use-ai-tools-connect';
import { useGroupsConnect } from '@/hooks/use-groups-connect';
import {
  type CreateJobParams,
  type LocalJob,
  type LocalJobActionType,
  type LocalJobPayload,
  type LocalJobStatus,
  type UpdateJobParams,
  useJobsConnect,
} from '@/hooks/use-jobs-connect';
import { useJobsRealtime } from '@/hooks/use-jobs-realtime';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

interface Group {
  id: string;
  name: string;
  wa_group_id: string;
  is_favorite?: boolean;
}

interface StatusBadgeConfig {
  label: string;
  variant: 'default' | 'secondary' | 'destructive' | 'outline';
}

interface UseJobsReturn {
  jobs: LocalJob[];
  groups: Group[];
  loadingJobs: boolean;
  loadingGroups: boolean;
  hasNextPage: boolean;
  totalCount: number;
  loadNextPage: () => void;
  actionType: string;
  scheduledDate: string;
  scheduledTime: string;
  payload: string;
  mlProductUrl: string;
  mlIncludeImage: boolean;
  mediaFile: File | null;
  isEnhancing: boolean;
  selectedGroups: string[];
  mentionOpen: boolean;
  dialogOpen: boolean;
  editingJobId: string | null;
  autoNumberGroups: boolean;
  searchTerm: string;
  pollQuestion: string;
  pollOptions: string[];
  isPending: boolean;
  setActionType: (value: string) => void;
  setScheduledDate: (value: string) => void;
  setScheduledTime: (value: string) => void;
  setPayload: (value: string) => void;
  setMlProductUrl: (value: string) => void;
  setMlIncludeImage: (value: boolean) => void;
  setMediaFile: (file: File | null) => void;
  setSelectedGroups: React.Dispatch<React.SetStateAction<string[]>>;
  setMentionOpen: (value: boolean) => void;
  setDialogOpen: (value: boolean) => void;
  setAutoNumberGroups: (value: boolean) => void;
  setSearchTerm: (value: string) => void;
  setPollQuestion: (value: string) => void;
  setPollOptions: React.Dispatch<React.SetStateAction<string[]>>;
  resetForm: () => void;
  handleEditJob: (job: LocalJob) => void;
  toggleGroup: (groupId: string) => void;
  insertMention: (mention: string) => void;
  addPollOption: () => void;
  removePollOption: (index: number) => void;
  updatePollOption: (index: number, value: string) => void;
  handleScheduleJob: () => Promise<void>;
  handleDeleteJob: (jobId: string) => Promise<void>;
  handleEnhanceMessage: () => Promise<void>;
  getStatusBadgeConfig: (status: LocalJobStatus) => StatusBadgeConfig;
  getActionLabel: (action: string) => string;
}

/**
 * Hook for jobs page logic
 * Manages job scheduling with ConnectRPC
 */
export function useJobs(): UseJobsReturn {
  const { t } = useTranslation();
  const { toast } = useToast();
  const { user } = useAuth();

  // ConnectRPC hooks
  const queryClient = useQueryClient();
  const { enhanceMessage: enhanceMessageApi, isEnhancing: isEnhancingApi } = useAiToolsConnect();
  const { groups: groupsData, loading: loadingGroups } = useGroupsConnect();
  const {
    jobs,
    loadingJobs,
    hasNextPage,
    totalCount,
    loadNextPage,
    createJob,
    updateJob,
    deleteJob,
    isCreatingJob,
    isUpdatingJob,
    isDeletingJob,
    refetchJobs,
  } = useJobsConnect();

  // Listen for real-time job status updates via SSE
  useJobsRealtime(refetchJobs);

  // Map groups to expected format
  // Note: is_admin filter removed - backend query already handles admin fallback
  const groups: Group[] = groupsData.map((g) => ({
    id: g.id,
    name: g.name,
    wa_group_id: g.wa_group_id,
    is_favorite: g.is_favorite,
  }));

  const [actionType, setActionType] = useState('');
  const [scheduledDate, setScheduledDate] = useState('');
  const [scheduledTime, setScheduledTime] = useState('');
  const [payload, setPayload] = useState('');
  const [mlProductUrl, setMlProductUrl] = useState('');
  const [mlIncludeImage, setMlIncludeImage] = useState(true);
  const [mediaFile, setMediaFile] = useState<File | null>(null);
  const [selectedGroups, setSelectedGroups] = useState<string[]>([]);
  const [mentionOpen, setMentionOpen] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingJobId, setEditingJobId] = useState<string | null>(null);
  const [autoNumberGroups, setAutoNumberGroups] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const [pollQuestion, setPollQuestion] = useState('');
  const [pollOptions, setPollOptions] = useState<string[]>(['', '']);

  // Refetch groups when dialog opens to ensure fresh data
  // Use invalidateQueries to force refetch, bypassing staleTime
  useEffect(() => {
    if (dialogOpen) {
      queryClient.invalidateQueries({ queryKey: ['groups', 'list'] });
    }
  }, [dialogOpen, queryClient]);

  const resetForm = useCallback(() => {
    setActionType('');
    setScheduledDate('');
    setScheduledTime('');
    setPayload('');
    setMlProductUrl('');
    setMlIncludeImage(true);
    setMediaFile(null);
    setSelectedGroups([]);
    setAutoNumberGroups(false);
    setSearchTerm('');
    setPollQuestion('');
    setPollOptions(['', '']);
    setDialogOpen(false);
    setEditingJobId(null);
  }, []);

  const handleEditJob = useCallback((job: LocalJob) => {
    setEditingJobId(job.id);
    setActionType(job.action_type);
    setSelectedGroups(job.payload.groups || []);

    if (job.action_type === 'send_poll') {
      setPollQuestion(job.payload.question || '');
      setPollOptions(job.payload.options || ['', '']);
    } else if (job.action_type === 'send_message') {
      // Backward compat: old jobs may have type=poll under send_message
      if (job.payload.type === 'poll') {
        setActionType('send_poll');
        setPollQuestion(job.payload.question || '');
        setPollOptions(job.payload.options || ['', '']);
      } else {
        setPayload(job.payload.message || '');
      }
    } else if (job.action_type === 'update_description') {
      setPayload(job.payload.description || '');
    } else if (job.action_type === 'change_group_name') {
      setPayload(job.payload.name || '');
      setAutoNumberGroups(job.payload.autoNumber || false);
    } else if (job.action_type === 'change_group_photo') {
      setPayload(job.payload.image || '');
    } else if (job.action_type === 'send_ml_offer') {
      setMlProductUrl(job.payload.originUrl || '');
      setMlIncludeImage(Boolean(job.payload.includeImage ?? true));
      setPayload(job.payload.text || '');
    }

    const scheduledDate = new Date(job.scheduled_for);
    setScheduledDate(scheduledDate.toISOString().split('T')[0]);
    setScheduledTime(scheduledDate.toTimeString().slice(0, 5));

    setDialogOpen(true);
  }, []);

  const toggleGroup = useCallback((groupId: string) => {
    setSelectedGroups((prev) =>
      prev.includes(groupId) ? prev.filter((id) => id !== groupId) : [...prev, groupId]
    );
  }, []);

  const insertMention = useCallback((mention: string) => {
    setPayload((prev) => prev + mention);
    setMentionOpen(false);
  }, []);

  const addPollOption = useCallback(() => {
    if (pollOptions.length < 12) {
      setPollOptions((prev) => [...prev, '']);
    } else {
      toast({
        title: t('pages.jobs.errors.maxOptions'),
        description: t('pages.jobs.errors.maxOptionsMessage'),
        variant: 'destructive',
      });
    }
  }, [pollOptions.length, toast, t]);

  const removePollOption = useCallback((index: number) => {
    setPollOptions((prev) => {
      if (prev.length > 2) {
        return prev.filter((_, i) => i !== index);
      }
      return prev;
    });
  }, []);

  const updatePollOption = useCallback((index: number, value: string) => {
    setPollOptions((prev) => {
      const newOptions = [...prev];
      newOptions[index] = value;
      return newOptions;
    });
  }, []);

  const handleScheduleJob = useCallback(async () => {
    if (!actionType || !scheduledDate || !scheduledTime) {
      toast({
        variant: 'destructive',
        title: t('pages.jobs.errors.requiredFields'),
        description: t('pages.jobs.errors.requiredFieldsMessage'),
      });
      return;
    }

    if (selectedGroups.length === 0) {
      toast({
        variant: 'destructive',
        title: t('pages.jobs.errors.noGroups'),
        description: t('pages.jobs.errors.noGroupsMessage'),
      });
      return;
    }

    const jobPayload: LocalJobPayload = {
      groups: selectedGroups,
    };

    const finalActionType = actionType as LocalJobActionType;

    if (actionType === 'send_message') {
      if (!payload.trim()) {
        toast({
          variant: 'destructive',
          title: t('pages.jobs.errors.emptyMessage'),
          description: t('pages.jobs.errors.emptyMessageMessage'),
        });
        return;
      }
      jobPayload.message = payload;

      if (mediaFile) {
        const MAX_MEDIA_BYTES = 7 * 1024 * 1024;
        if (mediaFile.size > MAX_MEDIA_BYTES) {
          toast({
            variant: 'destructive',
            title: t('pages.jobs.errors.fileTooLarge'),
            description: t('pages.jobs.errors.fileTooLargeMessage'),
          });
          return;
        }

        try {
          const mediaDataUrl = await new Promise<string>((resolve, reject) => {
            const reader = new FileReader();
            reader.onload = () => resolve(reader.result as string);
            reader.onerror = reject;
            reader.readAsDataURL(mediaFile);
          });

          jobPayload.media = mediaDataUrl;
          jobPayload.mediaType = mediaFile.type?.startsWith('video/') ? 'video' : 'image';
          jobPayload.fileName = mediaFile.name;
          jobPayload.mimeType = mediaFile.type || undefined;
        } catch (error: unknown) {
          captureHookError(error, 'useJobs', 'readMediaFile');
          toast({
            variant: 'destructive',
            title: t('pages.jobs.errors.fileReadError'),
            description: t('pages.jobs.errors.fileReadMessage'),
          });
          return;
        }
      }
    } else if (actionType === 'send_ml_offer') {
      if (!mlProductUrl.trim()) {
        toast({
          variant: 'destructive',
          title: t('pages.jobs.errors.emptyUrl'),
          description: t('pages.jobs.errors.emptyUrlMessage'),
        });
        return;
      }
      jobPayload.originUrl = mlProductUrl.trim();
      jobPayload.text = payload.trim() || '';
      jobPayload.includeImage = Boolean(mlIncludeImage);
    } else if (actionType === 'send_poll') {
      if (!pollQuestion.trim()) {
        toast({
          variant: 'destructive',
          title: t('pages.jobs.errors.emptyPollQuestion'),
          description: t('pages.jobs.errors.emptyPollQuestionMessage'),
        });
        return;
      }

      const validOptions = pollOptions.filter((opt) => opt.trim());
      if (validOptions.length < 2) {
        toast({
          variant: 'destructive',
          title: t('pages.jobs.errors.insufficientOptions'),
          description: t('pages.jobs.errors.insufficientOptionsMessage'),
        });
        return;
      }

      jobPayload.question = pollQuestion;
      jobPayload.options = validOptions;
    } else if (actionType === 'update_description') {
      jobPayload.description = payload;
    } else if (actionType === 'change_group_name') {
      jobPayload.name = payload;
      jobPayload.autoNumber = autoNumberGroups;
    }

    const localDateTime = new Date(`${scheduledDate}T${scheduledTime}:00`);

    try {
      if (editingJobId) {
        const updateParams: UpdateJobParams = {
          id: editingJobId,
          action_type: finalActionType,
          scheduled_for: localDateTime,
          payload: jobPayload,
        };

        await updateJob(updateParams);

        toast({
          title: t('pages.jobs.success.updated'),
          description: t('pages.jobs.success.updatedMessage'),
        });
      } else {
        const createParams: CreateJobParams = {
          action_type: finalActionType,
          scheduled_for: localDateTime,
          payload: jobPayload,
        };

        await createJob(createParams);

        toast({
          title: t('pages.jobs.success.scheduled'),
          description: t('pages.jobs.success.scheduledMessage'),
        });
      }

      resetForm();
      refetchJobs();
    } catch (error: unknown) {
      captureHookError(error, 'useJobs', 'handleScheduleJob');
      toast({
        variant: 'destructive',
        title: t('pages.jobs.errors.schedule'),
        description: error instanceof Error ? error.message : t('pages.jobs.errors.schedule'),
      });
    }
  }, [
    actionType,
    scheduledDate,
    scheduledTime,
    selectedGroups,
    payload,
    mediaFile,
    mlProductUrl,
    mlIncludeImage,
    pollQuestion,
    pollOptions,
    autoNumberGroups,
    editingJobId,
    toast,
    t,
    createJob,
    updateJob,
    resetForm,
    refetchJobs,
  ]);

  const handleDeleteJob = useCallback(
    async (jobId: string) => {
      try {
        await deleteJob(jobId);
        toast({
          title: t('pages.jobs.success.deleted'),
          description: t('pages.jobs.success.deletedMessage'),
        });
        refetchJobs();
      } catch (error: unknown) {
        captureHookError(error, 'useJobs', 'handleDeleteJob');
        toast({
          variant: 'destructive',
          title: t('pages.jobs.errors.delete'),
          description: error instanceof Error ? error.message : t('pages.jobs.errors.delete'),
        });
      }
    },
    [deleteJob, toast, t, refetchJobs]
  );

  const handleEnhanceMessage = useCallback(async () => {
    if (!payload.trim()) {
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: t('pages.jobs.errors.emptyMessage'),
      });
      return;
    }

    try {
      const messages = await enhanceMessageApi({
        productLinks: payload,
      });

      if (messages.length > 0) {
        setPayload(messages[0]);
        toast({
          title: t('pages.jobs.success.enhanced'),
          description:
            messages.length > 1
              ? t('pages.jobs.success.enhancedMultipleMessage', { count: messages.length })
              : t('pages.jobs.success.enhancedMessage'),
        });
      } else {
        toast({
          variant: 'destructive',
          title: t('pages.jobs.errors.unexpectedResponse'),
          description: t('pages.jobs.errors.unexpectedResponseMessage'),
        });
      }
    } catch (error: unknown) {
      captureHookError(error, 'useJobs', 'handleEnhanceMessage');
      toast({
        variant: 'destructive',
        title: t('pages.jobs.errors.enhance'),
        description: t('pages.jobs.errors.enhanceMessage'),
      });
    }
  }, [payload, toast, t, enhanceMessageApi]);

  const getStatusBadgeConfig = useCallback(
    (status: LocalJobStatus): StatusBadgeConfig => {
      const statusConfig: Record<LocalJobStatus, StatusBadgeConfig> = {
        pending: { label: t('pages.jobs.status.pending'), variant: 'default' },
        running: { label: t('pages.jobs.status.running'), variant: 'secondary' },
        done: { label: t('pages.jobs.status.done'), variant: 'success' },
        failed: { label: t('pages.jobs.status.failed'), variant: 'destructive' },
        cancelled: { label: t('pages.jobs.status.cancelled'), variant: 'outline' },
      };
      return statusConfig[status] || statusConfig.pending;
    },
    [t]
  );

  const getActionLabel = useCallback(
    (action: string): string => {
      const labels: Record<string, string> = {
        send_message: t('pages.jobs.actionTypes.sendMessage'),
        send_poll: t('pages.jobs.actionTypes.sendPoll'),
        send_ml_offer: t('pages.jobs.actionTypes.sendMlOffer'),
        update_description: t('pages.jobs.actionTypes.updateDescription'),
        close_groups: t('pages.jobs.actionTypes.closeGroups'),
        open_groups: t('pages.jobs.actionTypes.openGroups'),
        change_group_name: t('pages.jobs.actionTypes.changeGroupName'),
        change_group_photo: t('pages.jobs.actionTypes.changeGroupPhoto'),
      };
      return labels[action] || action;
    },
    [t]
  );

  return {
    jobs,
    groups,
    loadingJobs,
    loadingGroups,
    hasNextPage,
    totalCount,
    loadNextPage,
    actionType,
    scheduledDate,
    scheduledTime,
    payload,
    mlProductUrl,
    mlIncludeImage,
    mediaFile,
    isEnhancing: isEnhancingApi,
    selectedGroups,
    mentionOpen,
    dialogOpen,
    editingJobId,
    autoNumberGroups,
    searchTerm,
    pollQuestion,
    pollOptions,
    isPending: isCreatingJob || isUpdatingJob || isDeletingJob,
    setActionType,
    setScheduledDate,
    setScheduledTime,
    setPayload,
    setMlProductUrl,
    setMlIncludeImage,
    setMediaFile,
    setSelectedGroups,
    setMentionOpen,
    setDialogOpen,
    setAutoNumberGroups,
    setSearchTerm,
    setPollQuestion,
    setPollOptions,
    resetForm,
    handleEditJob,
    toggleGroup,
    insertMention,
    addPollOption,
    removePollOption,
    updatePollOption,
    handleScheduleJob,
    handleDeleteJob,
    handleEnhanceMessage,
    getStatusBadgeConfig,
    getActionLabel,
  };
}
