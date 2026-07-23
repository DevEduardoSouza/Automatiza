import { useAuth } from '@/contexts';
import { useAiToolsConnect } from '@/hooks/use-ai-tools-connect';
import { useGroupsConnect } from '@/hooks/use-groups-connect';
import {
  type LocalJob,
  type LocalJobActionType,
  type LocalJobPayload,
  type LocalJobStatus,
  useJobsConnect,
} from '@/hooks/use-jobs-connect';
import { useJobsRealtime } from '@/hooks/use-jobs-realtime';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import { ConnectError } from '@connectrpc/connect';
import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

interface Group {
  id: string;
  name: string;
  wa_group_id: string;
  is_favorite?: boolean;
}

export type RecurrenceType = 'once' | 'daily' | 'weekly' | 'interval';

// Recorrência sem "repetir até" é materializada até este horizonte, e o total de
// jobs gerados por submit é limitado — proteção contra criar milhares de jobs de
// uma vez (600 chips em prod). ponytail: teto fixo; upgrade = recorrência nativa
// no backend (1 job repetível em vez de N materializados).
const RECUR_HORIZON_DAYS = 90;
const MAX_OCCURRENCES = 200;

const startOfDay = (d: Date): Date => new Date(d.getFullYear(), d.getMonth(), d.getDate());

/**
 * Materializa as ocorrências (data × horário) de um agendamento conforme a
 * recorrência. "Uma vez" usa a data escolhida como está (pode ser passado — o
 * backend valida). Recorrente vai da maior data entre início e hoje até
 * "repetir até" (ou o horizonte), pulando horários já passados e limitando o
 * total. Retorna Dates locais prontos para virar jobs single-shot.
 */
export function buildOccurrences(opts: {
  recurrence: RecurrenceType;
  scheduledDate: string;
  timeList: string[];
  weekdays: number[];
  intervalDays: number;
  repeatUntil: string;
}): Date[] {
  const { recurrence, scheduledDate, timeList, weekdays, intervalDays, repeatUntil } = opts;
  const parseDay = (iso: string): Date => {
    const [y, mo, d] = iso.split('-').map(Number);
    return new Date(y, mo - 1, d);
  };
  const atTimes = (day: Date): Date[] =>
    timeList.map((t) => {
      const [h, m] = t.split(':').map(Number);
      return new Date(day.getFullYear(), day.getMonth(), day.getDate(), h, m, 0, 0);
    });

  if (recurrence === 'once') {
    return scheduledDate ? atTimes(parseDay(scheduledDate)) : [];
  }

  const now = new Date();
  const today = startOfDay(now);
  let start = today;
  if (scheduledDate) {
    const s = startOfDay(parseDay(scheduledDate));
    if (s > today) start = s;
  }
  const end = repeatUntil ? startOfDay(parseDay(repeatUntil)) : new Date(start);
  if (!repeatUntil) end.setDate(end.getDate() + RECUR_HORIZON_DAYS);
  if (end < start) return [];

  const wdSet = new Set(weekdays);
  const step = Math.max(2, intervalDays);
  const out: Date[] = [];
  const day = new Date(start);
  let idx = 0;
  while (day <= end && out.length < MAX_OCCURRENCES) {
    const include =
      recurrence === 'daily' ||
      (recurrence === 'weekly' && wdSet.has(day.getDay())) ||
      (recurrence === 'interval' && idx % step === 0);
    if (include) {
      for (const dt of atTimes(day)) {
        if (dt > now && out.length < MAX_OCCURRENCES) out.push(dt);
      }
    }
    day.setDate(day.getDate() + 1);
    idx++;
  }
  return out;
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
  // Agendamento avançado (recorrência + múltiplos horários) — expande em N jobs
  // single-shot via createBulkJobs; contrato do backend inalterado.
  recurrence: RecurrenceType;
  weekdays: number[];
  intervalDays: number;
  times: string[];
  repeatUntil: string;
  setRecurrence: (value: RecurrenceType) => void;
  toggleWeekday: (day: number) => void;
  setIntervalDays: (value: number) => void;
  addTime: (time: string) => void;
  removeTime: (time: string) => void;
  setRepeatUntil: (value: string) => void;
  setActionType: (value: string) => void;
  setScheduledDate: (value: string) => void;
  setScheduledTime: (value: string) => void;
  setPayload: (value: string) => void;
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
  selectAllGroups: (groupIds: string[]) => void;
  clearGroups: () => void;
  toggleGroups: (ids: string[], selected: boolean) => void;
  toggleFavorite: (id: string, next: boolean) => void;
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
  const { groups: groupsData, loading: loadingGroups, toggleGroupFavorite } = useGroupsConnect();
  const {
    jobs,
    loadingJobs,
    hasNextPage,
    totalCount,
    loadNextPage,
    createJob,
    createBulkJobs,
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
  const [mediaFile, setMediaFile] = useState<File | null>(null);
  const [selectedGroups, setSelectedGroups] = useState<string[]>([]);
  const [mentionOpen, setMentionOpen] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingJobId, setEditingJobId] = useState<string | null>(null);
  const [autoNumberGroups, setAutoNumberGroups] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const [pollQuestion, setPollQuestion] = useState('');
  const [pollOptions, setPollOptions] = useState<string[]>(['', '']);
  const [recurrence, setRecurrence] = useState<RecurrenceType>('once');
  const [weekdays, setWeekdays] = useState<number[]>([1, 3, 5]);
  const [intervalDays, setIntervalDaysState] = useState(2);
  const [times, setTimes] = useState<string[]>([]);
  const [repeatUntil, setRepeatUntil] = useState('');

  const toggleWeekday = useCallback((day: number) => {
    setWeekdays((prev) =>
      prev.includes(day) ? prev.filter((d) => d !== day) : [...prev, day].sort()
    );
  }, []);

  const setIntervalDays = useCallback((value: number) => {
    setIntervalDaysState(Math.max(2, Math.min(30, Math.trunc(value) || 2)));
  }, []);

  // Adiciona horário "HH:MM" mantendo a lista ordenada e sem duplicatas.
  const addTime = useCallback((time: string) => {
    setTimes((prev) => (prev.includes(time) ? prev : [...prev, time].sort()));
  }, []);

  const removeTime = useCallback((time: string) => {
    setTimes((prev) => prev.filter((t) => t !== time));
  }, []);

  // Refetch groups when dialog opens to ensure fresh data
  // Use invalidateQueries to force refetch, bypassing staleTime
  useEffect(() => {
    if (dialogOpen) {
      queryClient.invalidateQueries({ queryKey: ['groups', 'list'] });
    }
  }, [dialogOpen, queryClient]);

  // Ao abrir o modal de CRIAÇÃO, pré-preenche um horário padrão (09:00) como no
  // handoff. Edição já traz o horário do próprio job via handleEditJob.
  useEffect(() => {
    if (dialogOpen && !editingJobId) {
      setTimes((prev) => (prev.length > 0 ? prev : ['09:00']));
    }
  }, [dialogOpen, editingJobId]);

  const resetForm = useCallback(() => {
    setActionType('');
    setScheduledDate('');
    setScheduledTime('');
    setPayload('');
    setMediaFile(null);
    setSelectedGroups([]);
    setAutoNumberGroups(false);
    setSearchTerm('');
    setPollQuestion('');
    setPollOptions(['', '']);
    setRecurrence('once');
    setWeekdays([1, 3, 5]);
    setIntervalDaysState(2);
    setTimes([]);
    setRepeatUntil('');
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
    }

    const scheduledDate = new Date(job.scheduled_for);
    const hhmm = scheduledDate.toTimeString().slice(0, 5);
    setScheduledDate(scheduledDate.toISOString().split('T')[0]);
    setScheduledTime(hhmm);
    // Editar = 1 job existente → agendamento pontual com o horário do próprio job.
    setRecurrence('once');
    setTimes([hhmm]);
    setRepeatUntil('');

    setDialogOpen(true);
  }, []);

  const toggleGroup = useCallback((groupId: string) => {
    setSelectedGroups((prev) =>
      prev.includes(groupId) ? prev.filter((id) => id !== groupId) : [...prev, groupId]
    );
  }, []);

  // Seleção em lote (merge, dedup) dos grupos passados — o GroupSelector
  // manda os visíveis após a busca, então "Selecionar todos" respeita o filtro.
  const selectAllGroups = useCallback((groupIds: string[]) => {
    setSelectedGroups((prev) => Array.from(new Set([...prev, ...groupIds])));
  }, []);

  const clearGroups = useCallback(() => {
    setSelectedGroups([]);
  }, []);

  // Marca/desmarca um conjunto de grupos de uma vez (seção/atalho do picker) —
  // um único update de estado em vez de N toggles.
  const toggleGroups = useCallback((ids: string[], selected: boolean) => {
    setSelectedGroups((prev) => {
      if (selected) return Array.from(new Set([...prev, ...ids]));
      const remove = new Set(ids);
      return prev.filter((id) => !remove.has(id));
    });
  }, []);

  // Favoritar a partir do modal (persistido). Fail-open: erro não quebra o modal.
  const toggleFavorite = useCallback(
    (id: string, next: boolean) => {
      void toggleGroupFavorite(id, next).catch((error: unknown) => {
        captureHookError(error, 'useJobs', 'toggleFavorite');
      });
    },
    [toggleGroupFavorite]
  );

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
    // Horários: a lista nova é a fonte da verdade; cai no scheduledTime legado
    // (fluxo de edição / testes) quando a lista está vazia.
    const timeList = times.length > 0 ? [...times].sort() : scheduledTime ? [scheduledTime] : [];

    // actionType + horário sempre obrigatórios; data só em "uma vez"; dias da
    // semana só em "dias da semana". Mesmo toast do fluxo antigo.
    const missingCore =
      !actionType ||
      timeList.length === 0 ||
      (recurrence === 'once' && !scheduledDate) ||
      (recurrence === 'weekly' && weekdays.length === 0);
    if (missingCore) {
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
    } else if (actionType === 'change_group_photo') {
      // O input de foto usa mediaFile (novo arquivo). Ao editar sem trocar o
      // arquivo, handleEditJob deixa a imagem existente (base64) em `payload`.
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
          const imageDataUrl = await new Promise<string>((resolve, reject) => {
            const reader = new FileReader();
            reader.onload = () => resolve(reader.result as string);
            reader.onerror = reject;
            reader.readAsDataURL(mediaFile);
          });
          jobPayload.image = imageDataUrl;
        } catch (error: unknown) {
          captureHookError(error, 'useJobs', 'readGroupPhotoFile');
          toast({
            variant: 'destructive',
            title: t('pages.jobs.errors.fileReadError'),
            description: t('pages.jobs.errors.fileReadMessage'),
          });
          return;
        }
      } else if (payload) {
        // Edição sem novo arquivo: reaproveita a imagem já salva no job.
        jobPayload.image = payload;
      }

      if (!jobPayload.image) {
        toast({
          variant: 'destructive',
          title: t('pages.jobs.errors.noImage'),
          description: t('pages.jobs.errors.noImageMessage'),
        });
        return;
      }
    }

    const occurrences = buildOccurrences({
      recurrence,
      scheduledDate,
      timeList,
      weekdays,
      intervalDays,
      repeatUntil,
    });

    if (occurrences.length === 0) {
      toast({
        variant: 'destructive',
        title: t('pages.jobs.errors.noOccurrences'),
        description: t('pages.jobs.errors.noOccurrencesMessage'),
      });
      return;
    }

    try {
      if (editingJobId) {
        // Edição continua sendo 1 job (primeira ocorrência).
        await updateJob({
          id: editingJobId,
          action_type: finalActionType,
          scheduled_for: occurrences[0],
          payload: jobPayload,
        });

        toast({
          title: t('pages.jobs.success.updated'),
          description: t('pages.jobs.success.updatedMessage'),
        });
      } else if (occurrences.length === 1) {
        // Caminho single-shot inalterado (retrocompatível com o backend/tests).
        await createJob({
          action_type: finalActionType,
          scheduled_for: occurrences[0],
          payload: jobPayload,
        });

        toast({
          title: t('pages.jobs.success.scheduled'),
          description: t('pages.jobs.success.scheduledMessage'),
        });
      } else {
        // Recorrência/múltiplos horários: N jobs single-shot via bulk.
        const result = await createBulkJobs(
          occurrences.map((scheduled_for) => ({
            action_type: finalActionType,
            scheduled_for,
            payload: jobPayload,
          }))
        );

        toast({
          title: t('pages.jobs.success.scheduled'),
          // Aviso neutro: agendamento repetido é engano comum (dois cliques),
          // não erro do usuário — e recorrência legítima nunca cai aqui,
          // porque cada ocorrência tem horário próprio.
          description:
            result.deduplicated > 0
              ? t('pages.jobs.success.scheduledBulkDeduped', {
                  count: occurrences.length,
                  deduplicated: result.deduplicated,
                })
              : t('pages.jobs.success.scheduledBulkMessage', { count: occurrences.length }),
        });
      }

      resetForm();
      refetchJobs();
    } catch (error: unknown) {
      captureHookError(error, 'useJobs', 'handleScheduleJob');
      toast({
        variant: 'destructive',
        title: t('pages.jobs.errors.schedule'),
        // rawMessage drops the "[code]" prefix connect puts on .message, so
        // the backend's user-facing text (e.g. "Este agendamento já foi
        // executado…") shows clean instead of "[internal] failed to update…".
        description:
          error instanceof ConnectError
            ? error.rawMessage
            : error instanceof Error
              ? error.message
              : t('pages.jobs.errors.schedule'),
      });
    }
  }, [
    actionType,
    scheduledDate,
    scheduledTime,
    times,
    recurrence,
    weekdays,
    intervalDays,
    repeatUntil,
    selectedGroups,
    payload,
    mediaFile,
    pollQuestion,
    pollOptions,
    autoNumberGroups,
    editingJobId,
    toast,
    t,
    createJob,
    createBulkJobs,
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
          description:
            error instanceof ConnectError
              ? error.rawMessage
              : error instanceof Error
                ? error.message
                : t('pages.jobs.errors.delete'),
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
    recurrence,
    weekdays,
    intervalDays,
    times,
    repeatUntil,
    setRecurrence,
    toggleWeekday,
    setIntervalDays,
    addTime,
    removeTime,
    setRepeatUntil,
    isPending: isCreatingJob || isUpdatingJob || isDeletingJob,
    setActionType,
    setScheduledDate,
    setScheduledTime,
    setPayload,
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
    selectAllGroups,
    clearGroups,
    toggleGroups,
    toggleFavorite,
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
