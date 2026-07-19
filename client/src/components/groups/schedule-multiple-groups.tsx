import { Dialog, DialogContent, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { useGroupsConnect } from '@/hooks/use-groups-connect';
import { type RecurrenceType, buildOccurrences } from '@/hooks/use-jobs';
import {
  type CreateJobParams,
  type LocalJobActionType,
  useJobsConnect,
} from '@/hooks/use-jobs-connect';
import { useToast } from '@/hooks/use-toast';
import { GroupPicker } from '@/pages/jobs/components/group-picker';
import { HandoffStyles } from '@/pages/jobs/components/handoff-styles';
import { T } from '@/pages/jobs/components/handoff-tokens';
import { JobScheduleFields } from '@/pages/jobs/components/job-schedule-fields';
import * as Sentry from '@sentry/react';
import { Plus } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

type Status = 'none' | 'open' | 'closed';

// Teto de jobs gerados por submit (grupos x acoes x ocorrencias) — protecao
// contra explosao com recorrencia. ponytail: teto fixo; upgrade = recorrencia
// nativa no backend. Alinhado com o modelo single-shot (createBulkJobs).
const MAX_TOTAL_JOBS = 500;
const overline = {
  fontSize: 11,
  fontWeight: 700,
  letterSpacing: '0.06em',
  color: T.ink,
  textTransform: 'uppercase' as const,
  marginBottom: 8,
};

/**
 * Modal "Agendar Ações em Múltiplos Grupos" — visual do handoff
 * `design_handoff_multiplos_grupos`: seletor de grupos com numeração por ordem
 * de clique, card "Quando executar" (recorrência + múltiplos horários) e as ações
 * em lote (nome base, status, descrição, foto, mensagem). As ações são
 * materializadas em N jobs single-shot via createBulkJobs — backend inalterado.
 */
export function ScheduleMultipleGroups() {
  const { t } = useTranslation();
  const {
    groups: availableGroups,
    loading: loadingGroups,
    toggleGroupFavorite,
  } = useGroupsConnect();
  const { createBulkJobs, isCreatingJob } = useJobsConnect();
  const { toast } = useToast();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  // Ordem do array = ordem de clique (define a numeração #1, #2…).
  const [selectedGroups, setSelectedGroups] = useState<string[]>([]);

  const [recurrence, setRecurrence] = useState<RecurrenceType>('once');
  const [weekdays, setWeekdays] = useState<number[]>([1, 3, 5]);
  const [intervalDays, setIntervalDays] = useState(2);
  const [times, setTimes] = useState<string[]>(['09:00']);
  const [scheduledDate, setScheduledDate] = useState('');
  const [repeatUntil, setRepeatUntil] = useState('');

  const [groupName, setGroupName] = useState('');
  const [description, setDescription] = useState('');
  const [status, setStatus] = useState<Status>('none');
  const [groupPhoto, setGroupPhoto] = useState<File | null>(null);
  const [message, setMessage] = useState('');

  const orderIndex = useMemo(() => {
    const m = new Map<string, number>();
    selectedGroups.forEach((id, i) => m.set(id, i + 1));
    return m;
  }, [selectedGroups]);
  const orderOf = (id: string) => orderIndex.get(id) ?? null;

  const toggleGroup = (id: string) =>
    setSelectedGroups((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  const toggleGroups = (ids: string[], selected: boolean) =>
    setSelectedGroups((prev) => {
      if (!selected) {
        const rm = new Set(ids);
        return prev.filter((x) => !rm.has(x));
      }
      const next = [...prev];
      for (const id of ids) if (!next.includes(id)) next.push(id);
      return next;
    });
  const selectAll = (ids: string[]) =>
    setSelectedGroups((prev) => {
      const next = [...prev];
      for (const id of ids) if (!next.includes(id)) next.push(id);
      return next;
    });

  const toggleFavorite = (id: string, next: boolean) => {
    void toggleGroupFavorite(id, next).catch((error: unknown) => {
      Sentry.captureException(error, { tags: { component: 'ScheduleMultipleGroups' } });
    });
  };

  const handlePhotoUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.errors.invalidPhotoType'),
        description: t('pages.groups.errors.invalidPhotoTypeMessage'),
      });
      e.target.value = '';
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.errors.photoTooLarge'),
        description: t('pages.groups.errors.photoExceedsLimitSchedule'),
      });
      e.target.value = '';
      return;
    }
    setGroupPhoto(file);
  };

  const actionsCount =
    (groupName.trim() ? 1 : 0) +
    (groupPhoto ? 1 : 0) +
    (description.trim() ? 1 : 0) +
    (status !== 'none' ? 1 : 0) +
    (message.trim() ? 1 : 0);

  const ready =
    selectedGroups.length > 0 &&
    actionsCount > 0 &&
    times.length > 0 &&
    (recurrence === 'once' ? !!scheduledDate : true) &&
    (recurrence === 'weekly' ? weekdays.length > 0 : true);

  const resetForm = () => {
    setSelectedGroups([]);
    setSearchTerm('');
    setRecurrence('once');
    setWeekdays([1, 3, 5]);
    setIntervalDays(2);
    setTimes(['09:00']);
    setScheduledDate('');
    setRepeatUntil('');
    setGroupName('');
    setDescription('');
    setStatus('none');
    setGroupPhoto(null);
    setMessage('');
    setDialogOpen(false);
  };

  const handleSchedule = async () => {
    if (selectedGroups.length === 0) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.schedule.errors.noGroupSelected'),
        description: t('pages.groups.schedule.errors.selectAtLeastOne'),
      });
      return;
    }
    if (actionsCount === 0) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.schedule.errors.noActionConfigured'),
        description: t('pages.groups.schedule.errors.configureAtLeastOneAction'),
      });
      return;
    }
    if (
      times.length === 0 ||
      (recurrence === 'once' && !scheduledDate) ||
      (recurrence === 'weekly' && weekdays.length === 0)
    ) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.schedule.errors.requiredFields'),
        description: t('pages.groups.schedule.errors.fillDateAndTime'),
      });
      return;
    }

    const occurrences = buildOccurrences({
      recurrence,
      scheduledDate,
      timeList: [...times].sort(),
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
      let photoBase64: string | null = null;
      if (groupPhoto) {
        const dataUrl = await new Promise<string>((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = () => resolve(reader.result as string);
          reader.onerror = reject;
          reader.readAsDataURL(groupPhoto);
        });
        photoBase64 = dataUrl.split(',')[1] || dataUrl;
      }

      const byId = new Map(availableGroups.map((g) => [g.id, g]));
      const orderedGroups = selectedGroups.map((id) => byId.get(id)).filter((g) => g != null);

      // Limita ocorrencias para nao estourar o teto de jobs por submit.
      const jobsPerOccurrence = orderedGroups.length * actionsCount;
      const maxOcc = Math.max(1, Math.floor(MAX_TOTAL_JOBS / jobsPerOccurrence));
      const occ = occurrences.slice(0, maxOcc);

      const jobs: CreateJobParams[] = [];
      for (const occDate of occ) {
        for (let gi = 0; gi < orderedGroups.length; gi++) {
          const group = orderedGroups[gi];
          const baseTime = occDate.getTime() + gi * 8000;
          let delay = 0;
          if (groupName.trim()) {
            jobs.push({
              action_type: 'change_group_name' as LocalJobActionType,
              scheduled_for: new Date(baseTime + delay),
              payload: {
                groups: [group.id],
                name: `#${gi + 1} ${groupName.trim()}`,
                autoNumber: false,
              },
            });
            delay += 2000;
          }
          if (photoBase64) {
            jobs.push({
              action_type: 'change_group_photo' as LocalJobActionType,
              scheduled_for: new Date(baseTime + delay),
              payload: { groups: [group.id], image: photoBase64 },
            });
            delay += 2000;
          }
          if (description.trim()) {
            jobs.push({
              action_type: 'update_description' as LocalJobActionType,
              scheduled_for: new Date(baseTime + delay),
              payload: { groups: [group.id], description: description.trim() },
            });
            delay += 2000;
          }
          if (status !== 'none') {
            jobs.push({
              action_type: (status === 'closed'
                ? 'close_groups'
                : 'open_groups') as LocalJobActionType,
              scheduled_for: new Date(baseTime + delay),
              payload: { groups: [group.id] },
            });
            delay += 2000;
          }
          if (message.trim()) {
            jobs.push({
              action_type: 'send_message' as LocalJobActionType,
              scheduled_for: new Date(baseTime + delay),
              payload: { groups: [group.id], message: message.trim() },
            });
          }
        }
      }

      await createBulkJobs(jobs);
      toast({
        title: t('pages.groups.schedule.success.scheduled'),
        description: t('pages.groups.schedule.success.scheduledDescription', {
          actions: jobs.length,
          groups: selectedGroups.length,
        }),
      });
      resetForm();
    } catch (error) {
      Sentry.captureException(error, {
        tags: { component: 'ScheduleMultipleGroups', action: 'handleSchedule' },
        extra: { selectedGroupsCount: selectedGroups.length },
      });
      toast({
        variant: 'destructive',
        title: t('pages.groups.schedule.errors.scheduleError'),
        description: error instanceof Error ? error.message : t('common.tryAgain'),
      });
    }
  };

  const namePreview = groupName.trim()
    ? t('pages.groups.schedule.handoff.namePreview', { name: groupName.trim() })
    : t('pages.groups.schedule.handoff.namePreviewDefault');

  const resumo = buildResumo(t, {
    groupName,
    description,
    status,
    hasPhoto: !!groupPhoto,
    message,
    count: selectedGroups.length,
    recurrence,
    weekdays,
    intervalDays,
    times,
    scheduledDate,
    repeatUntil,
  });

  const sectionPad = { padding: '22px 28px 0' };
  const inputStyle = {
    width: '100%',
    fontSize: 14,
    color: T.ink,
    padding: '11px 14px',
    borderRadius: 12,
    border: `1.5px solid ${T.border}`,
    background: T.white,
    outline: 'none',
  } as const;

  return (
    <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
      <DialogTrigger asChild>
        <button
          type="button"
          className="auth-btn-glow relative inline-flex flex-1 cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs font-semibold text-white transition-all md:flex-none xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base"
        >
          <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
            <div className="auth-btn-glow__glow absolute inset-0" />
          </div>
          <span className="relative z-[2] flex select-none items-center gap-2 font-semibold">
            <Plus className="h-4 w-4" aria-hidden="true" />
            <span className="hidden sm:inline">{t('pages.groups.schedule.scheduleButton')}</span>
            <span className="sm:hidden">{t('pages.groups.schedule.scheduleButtonShort')}</span>
          </span>
        </button>
      </DialogTrigger>
      <DialogContent
        aria-describedby={undefined}
        className="w-[calc(100vw-1.25rem)] max-w-[760px] max-h-[92vh] gap-0 overflow-y-auto rounded-[20px] border-0 bg-white p-0 [&>button]:hidden"
      >
        <HandoffStyles />

        {/* Header */}
        <div
          className="flex items-start justify-between"
          style={{ padding: '22px 28px', borderBottom: `1px solid ${T.divider}` }}
        >
          <div>
            <DialogTitle
              style={{ fontSize: 19, fontWeight: 700, color: T.ink, letterSpacing: '-0.01em' }}
            >
              {t('pages.groups.schedule.title')}
            </DialogTitle>
            <div style={{ fontSize: 13.5, color: T.ink, opacity: 0.55, marginTop: 2 }}>
              {t('pages.groups.schedule.handoff.descriptionFull')}
            </div>
          </div>
          <button
            type="button"
            onClick={() => setDialogOpen(false)}
            className="hf-close flex flex-none cursor-pointer items-center justify-center"
            style={{
              width: 34,
              height: 34,
              borderRadius: 10,
              border: `1px solid ${T.border}`,
              color: T.ink,
              fontSize: 15,
            }}
            aria-label={t('common.close')}
          >
            ✕
          </button>
        </div>

        {/* Como funciona */}
        <div
          className="flex items-start gap-2.5"
          style={{
            margin: '20px 28px 0',
            background: T.rose,
            border: `1px solid ${T.bannerBorder}`,
            borderRadius: 12,
            padding: '12px 16px',
          }}
        >
          <span
            className="flex flex-none items-center justify-center"
            style={{
              width: 20,
              height: 20,
              borderRadius: '50%',
              border: `1.5px solid ${T.wine}`,
              color: T.wine,
              fontSize: 12,
              fontWeight: 700,
              marginTop: 1,
            }}
          >
            i
          </span>
          <span style={{ fontSize: 13.5, color: T.ink, lineHeight: 1.5 }}>
            <strong>{t('pages.groups.schedule.howItWorks.title')}</strong>{' '}
            {t('pages.groups.schedule.howItWorks.description')}
          </span>
        </div>

        {/* Selecionar Grupos */}
        <div style={sectionPad}>
          <div className="mb-2.5 flex flex-wrap items-center justify-between gap-3">
            <div>
              <div style={{ fontSize: 14, fontWeight: 700, color: T.ink }}>
                {t('pages.groups.schedule.selectGroups')} <span style={{ color: T.wine }}>*</span>
              </div>
              <div style={{ fontSize: 12.5, color: T.ink, opacity: 0.55, marginTop: 2 }}>
                {t('pages.groups.schedule.handoff.orderHelper')}
              </div>
            </div>
            <span
              style={{
                fontSize: 12.5,
                fontWeight: 700,
                color: T.white,
                background: T.wine,
                borderRadius: 999,
                padding: '5px 14px',
              }}
            >
              {t('pages.jobs.schedule.groupsCounter', {
                selected: selectedGroups.length,
                total: availableGroups.length,
              })}
            </span>
          </div>
          <GroupPicker
            groups={availableGroups}
            selectedGroups={selectedGroups}
            searchTerm={searchTerm}
            loading={loadingGroups}
            onSearchChange={setSearchTerm}
            onToggleGroup={toggleGroup}
            onSelectAll={selectAll}
            onClearAll={() => setSelectedGroups([])}
            onToggleGroups={toggleGroups}
            onToggleFavorite={toggleFavorite}
            orderOf={orderOf}
          />
        </div>

        {/* Quando executar */}
        <div style={sectionPad}>
          <JobScheduleFields
            recurrence={recurrence}
            weekdays={weekdays}
            intervalDays={intervalDays}
            times={times}
            scheduledDate={scheduledDate}
            repeatUntil={repeatUntil}
            onRecurrenceChange={setRecurrence}
            onToggleWeekday={(d) =>
              setWeekdays((p) => (p.includes(d) ? p.filter((x) => x !== d) : [...p, d].sort()))
            }
            onIntervalChange={(v) => setIntervalDays(Math.max(2, Math.min(30, Math.trunc(v) || 2)))}
            onAddTime={(tm) => setTimes((p) => (p.includes(tm) ? p : [...p, tm].sort()))}
            onRemoveTime={(tm) => setTimes((p) => p.filter((x) => x !== tm))}
            onScheduledDateChange={setScheduledDate}
            onRepeatUntilChange={setRepeatUntil}
          />
        </div>

        {/* Ações nos grupos */}
        <div style={sectionPad}>
          <div className="mb-2.5" style={{ fontSize: 14, fontWeight: 700, color: T.ink }}>
            {t('pages.groups.schedule.handoff.actionsTitle')}{' '}
            <span style={{ fontWeight: 500, opacity: 0.5, fontSize: 13 }}>
              {t('pages.groups.schedule.handoff.actionsHint')}
            </span>
          </div>
          <div
            className="flex flex-col gap-4"
            style={{ border: `1px solid ${T.border}`, borderRadius: 16, padding: '16px 18px' }}
          >
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <div style={overline}>{t('pages.groups.schedule.handoff.baseNameLabel')}</div>
                <input
                  value={groupName}
                  onChange={(e) => setGroupName(e.target.value)}
                  placeholder={t('pages.groups.schedule.baseNamePlaceholder')}
                  className="hf-input"
                  style={inputStyle}
                />
                <div style={{ fontSize: 12, color: T.ink, opacity: 0.5, marginTop: 5 }}>
                  {namePreview}
                </div>
              </div>
              <div>
                <div style={overline}>{t('pages.groups.schedule.handoff.statusLabel')}</div>
                <select
                  value={status}
                  onChange={(e) => setStatus(e.target.value as Status)}
                  className="hf-input cursor-pointer"
                  style={inputStyle}
                >
                  <option value="none">{t('pages.groups.schedule.statusNoChange')}</option>
                  <option value="open">
                    {t('pages.groups.schedule.handoff.statusOpenAction')}
                  </option>
                  <option value="closed">
                    {t('pages.groups.schedule.handoff.statusClosedAction')}
                  </option>
                </select>
              </div>
            </div>
            <div>
              <div style={overline}>{t('pages.groups.schedule.handoff.descriptionLabel')}</div>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder={t('pages.groups.schedule.handoff.descriptionPlaceholder')}
                rows={3}
                className="hf-input"
                style={{ ...inputStyle, resize: 'vertical' }}
              />
            </div>
            <div>
              <div style={overline}>{t('pages.groups.schedule.handoff.photoLabel')}</div>
              <div
                className="hf-addtime relative flex cursor-pointer items-center gap-3"
                style={{
                  border: `1.5px dashed ${T.borderSel}`,
                  borderRadius: 12,
                  padding: '12px 14px',
                }}
              >
                <input
                  type="file"
                  accept="image/*"
                  onChange={handlePhotoUpload}
                  className="absolute inset-0 w-full cursor-pointer opacity-0"
                  aria-label={t('pages.groups.schedule.handoff.photoLabel')}
                />
                <span
                  className="flex-none"
                  style={{
                    fontSize: 13,
                    fontWeight: 600,
                    color: T.wine,
                    background: T.tagWine,
                    borderRadius: 8,
                    padding: '6px 12px',
                  }}
                >
                  {t('pages.groups.schedule.handoff.chooseFile')}
                </span>
                <span
                  className="overflow-hidden text-ellipsis whitespace-nowrap"
                  style={{ fontSize: 13.5, color: groupPhoto ? T.ink : T.placeholder }}
                >
                  {groupPhoto ? groupPhoto.name : t('pages.groups.schedule.handoff.noFile')}
                </span>
                {groupPhoto && (
                  <button
                    type="button"
                    onClick={() => setGroupPhoto(null)}
                    className="relative z-[2] ml-auto flex flex-none cursor-pointer items-center justify-center"
                    style={{
                      width: 22,
                      height: 22,
                      borderRadius: '50%',
                      background: T.pillNeutral,
                      color: T.ink,
                      fontSize: 11,
                      fontWeight: 700,
                    }}
                  >
                    ✕
                  </button>
                )}
              </div>
            </div>
            <div>
              <div style={overline}>{t('pages.groups.schedule.handoff.messageLabel')}</div>
              <textarea
                value={message}
                onChange={(e) => setMessage(e.target.value)}
                placeholder={t('pages.groups.schedule.messagePlaceholder')}
                rows={4}
                className="hf-input"
                style={{ ...inputStyle, resize: 'vertical' }}
              />
              <div className="mt-1.5 flex items-center justify-between">
                <span style={{ fontSize: 12, color: T.ink, opacity: 0.5 }}>
                  {t('pages.groups.schedule.handoff.messageHint')}
                </span>
                <span style={{ fontSize: 12, fontWeight: 600, color: T.ink, opacity: 0.5 }}>
                  {t('pages.groups.schedule.handoff.charCount', { count: message.length })}
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Resumo */}
        <div
          className="flex items-start gap-2.5"
          style={{
            margin: '18px 28px 0',
            background: T.rose,
            border: `1px solid ${T.bannerBorder}`,
            borderRadius: 12,
            padding: '12px 16px',
          }}
        >
          <span
            className="flex flex-none items-center justify-center"
            style={{
              width: 20,
              height: 20,
              borderRadius: '50%',
              border: `1.5px solid ${T.wine}`,
              color: T.wine,
              fontSize: 12,
              fontWeight: 700,
              marginTop: 1,
            }}
          >
            i
          </span>
          <span style={{ fontSize: 13.5, color: T.ink, lineHeight: 1.5 }}>{resumo}</span>
        </div>

        {/* Footer */}
        <div
          style={{
            padding: '20px 28px 24px',
            borderTop: `1px solid ${T.divider}`,
            marginTop: 20,
            background: T.toolbar,
            borderRadius: '0 0 20px 20px',
          }}
        >
          <button
            type="button"
            onClick={handleSchedule}
            disabled={!ready || isCreatingJob}
            className="hf-cta w-full whitespace-nowrap"
            style={{
              fontSize: 14,
              fontWeight: 700,
              color: T.white,
              background: ready && !isCreatingJob ? T.wine : T.wineDisabled,
              border: 'none',
              borderRadius: 12,
              padding: '13px 26px',
              cursor: !ready || isCreatingJob ? 'not-allowed' : 'pointer',
              boxShadow: '0 6px 18px rgba(125,34,72,0.35)',
            }}
          >
            {isCreatingJob
              ? t('common.loading')
              : `🗓 ${t('pages.groups.schedule.scheduleCount', { count: selectedGroups.length })}`}
          </button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

/** Frase-resumo em linguagem natural (porta a lógica do handoff). */
function buildResumo(
  t: (key: string, opts?: Record<string, unknown>) => string,
  opts: {
    groupName: string;
    description: string;
    status: Status;
    hasPhoto: boolean;
    message: string;
    count: number;
    recurrence: RecurrenceType;
    weekdays: number[];
    intervalDays: number;
    times: string[];
    scheduledDate: string;
    repeatUntil: string;
  }
): string {
  const H = 'pages.groups.schedule.handoff.summary';
  const S = 'pages.jobs.schedule.summary';
  const fmt = (iso: string) => (iso ? iso.split('-').reverse().join('/') : '');
  const { recurrence, weekdays, intervalDays, times, scheduledDate, repeatUntil } = opts;
  const isRecurring = recurrence !== 'once';

  let when = '';
  if (recurrence === 'once') {
    when = scheduledDate
      ? t(`${S}.onceWithDate`, { date: fmt(scheduledDate) })
      : t(`${S}.oncePickDate`);
  } else if (recurrence === 'daily') {
    when = t(`${S}.daily`);
  } else if (recurrence === 'weekly') {
    when = weekdays.length
      ? t(`${S}.weekly`, {
          days: [...weekdays]
            .sort()
            .map((i) => t(`pages.jobs.schedule.weekdays.${i}`))
            .join(', '),
        })
      : t(`${S}.weeklyPick`);
  } else {
    when = t(`${S}.interval`, { n: intervalDays });
  }
  if (isRecurring && scheduledDate) when += ` ${t(`${S}.fromDate`, { date: fmt(scheduledDate) })}`;
  if (isRecurring && repeatUntil) when += ` ${t(`${S}.untilDate`, { date: fmt(repeatUntil) })}`;

  const acoes: string[] = [];
  if (opts.groupName.trim()) acoes.push(t(`${H}.rename`, { name: opts.groupName.trim() }));
  if (opts.description.trim()) acoes.push(t(`${H}.description`));
  if (opts.status === 'open') acoes.push(t(`${H}.open`));
  if (opts.status === 'closed') acoes.push(t(`${H}.close`));
  if (opts.hasPhoto) acoes.push(t(`${H}.photo`));
  if (opts.message.trim()) acoes.push(t(`${H}.message`));

  const actionsStr = acoes.length ? acoes.join(', ') : t(`${H}.noAction`);
  const timesPart = times.length
    ? ` ${t(`${S}.atTimes`, { times: [...times].sort().join(', ') })}`
    : `, ${t(`${S}.addTimeHint`)}`;
  return `${t(`${H}.prefix`)} ${actionsStr} — ${t(`${H}.groups`, { count: opts.count })}, ${when}${timesPart}.`;
}
