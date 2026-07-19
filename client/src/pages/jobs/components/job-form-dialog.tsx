import { Dialog, DialogContent, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import type { RecurrenceType } from '@/hooks/use-jobs';
import { Plus } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { GroupNameForm } from './group-name-form';
import { GroupPicker } from './group-picker';
import { HandoffStyles } from './handoff-styles';
import { T } from './handoff-tokens';
import { JobScheduleFields } from './job-schedule-fields';
import { MessageForm } from './message-form';
import { PollOptionsForm } from './poll-options-form';

interface Group {
  id: string;
  name: string;
  wa_group_id: string;
  is_favorite?: boolean;
}

interface JobFormDialogProps {
  dialogOpen: boolean;
  editingJobId: string | null;
  actionType: string;
  scheduledDate: string;
  payload: string;
  mediaFile: File | null;
  isEnhancing: boolean;
  selectedGroups: string[];
  mentionOpen: boolean;
  autoNumberGroups: boolean;
  searchTerm: string;
  pollQuestion: string;
  pollOptions: string[];
  isPending: boolean;
  groups: Group[];
  loadingGroups: boolean;
  recurrence: RecurrenceType;
  weekdays: number[];
  intervalDays: number;
  times: string[];
  repeatUntil: string;
  onDialogOpenChange: (open: boolean) => void;
  onActionTypeChange: (value: string) => void;
  onScheduledDateChange: (value: string) => void;
  onPayloadChange: (value: string) => void;
  onMediaFileChange: (file: File | null) => void;
  onMentionOpenChange: (open: boolean) => void;
  onAutoNumberGroupsChange: (value: boolean) => void;
  onSearchTermChange: (value: string) => void;
  onPollQuestionChange: (value: string) => void;
  onToggleGroup: (groupId: string) => void;
  onSelectAllGroups: (groupIds: string[]) => void;
  onClearGroups: () => void;
  onToggleGroups: (ids: string[], selected: boolean) => void;
  onToggleFavorite: (id: string, next: boolean) => void;
  onInsertMention: (mention: string) => void;
  onAddPollOption: () => void;
  onRemovePollOption: (index: number) => void;
  onUpdatePollOption: (index: number, value: string) => void;
  onEnhanceMessage: () => void;
  onRecurrenceChange: (value: RecurrenceType) => void;
  onToggleWeekday: (day: number) => void;
  onIntervalChange: (value: number) => void;
  onAddTime: (time: string) => void;
  onRemoveTime: (time: string) => void;
  onRepeatUntilChange: (value: string) => void;
  onSubmit: () => void;
}

const ACTION_KEYS: Record<string, string> = {
  send_message: 'sendMessage',
  send_poll: 'sendPoll',
  update_description: 'updateDescription',
  close_groups: 'closeGroups',
  open_groups: 'openGroups',
  change_group_name: 'changeGroupName',
  change_group_photo: 'changeGroupPhoto',
};

/**
 * Modal "Agendar Nova Ação" — visual do handoff `design_handoff_grupos_agendamento`
 * (paleta vinho, seleção de grupos em seções, recorrência + múltiplos horários e
 * datepickers customizados). Mantém todos os tipos de ação e sub-formulários já
 * existentes; a recorrência é materializada em N jobs single-shot no hook.
 */
export function JobFormDialog(props: JobFormDialogProps) {
  const { t } = useTranslation();
  const {
    dialogOpen,
    editingJobId,
    actionType,
    scheduledDate,
    payload,
    mediaFile,
    isEnhancing,
    selectedGroups,
    mentionOpen,
    autoNumberGroups,
    searchTerm,
    pollQuestion,
    pollOptions,
    isPending,
    groups,
    loadingGroups,
    recurrence,
    weekdays,
    intervalDays,
    times,
    repeatUntil,
    onDialogOpenChange,
    onActionTypeChange,
    onScheduledDateChange,
    onPayloadChange,
    onMediaFileChange,
    onMentionOpenChange,
    onAutoNumberGroupsChange,
    onSearchTermChange,
    onPollQuestionChange,
    onToggleGroup,
    onSelectAllGroups,
    onClearGroups,
    onToggleGroups,
    onToggleFavorite,
    onInsertMention,
    onAddPollOption,
    onRemovePollOption,
    onUpdatePollOption,
    onEnhanceMessage,
    onRecurrenceChange,
    onToggleWeekday,
    onIntervalChange,
    onAddTime,
    onRemoveTime,
    onRepeatUntilChange,
    onSubmit,
  } = props;

  const selectedCount = selectedGroups.length;
  const totalGroups = groups.length;

  const ready =
    !!actionType &&
    selectedCount > 0 &&
    times.length > 0 &&
    (recurrence === 'once' ? !!scheduledDate : true) &&
    (recurrence === 'weekly' ? weekdays.length > 0 : true);

  const resumo = buildResumo(t, {
    actionLabel: actionType ? t(`pages.jobs.actionTypes.${ACTION_KEYS[actionType]}`) : '',
    count: selectedCount,
    recurrence,
    weekdays,
    intervalDays,
    times,
    scheduledDate,
    repeatUntil,
  });

  const sectionPad = { padding: '22px 28px 0' };
  const ctaBg = ready && !isPending ? T.wine : T.wineDisabled;

  return (
    <Dialog open={dialogOpen} onOpenChange={onDialogOpenChange}>
      <DialogTrigger asChild>
        <button
          type="button"
          className="auth-btn-glow relative inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs font-semibold text-white transition-all xs:w-auto xs:flex-1 xs:px-5 xs:py-2.5 xs:text-sm md:flex-none lg:py-3 lg:text-base"
        >
          <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
            <div className="auth-btn-glow__glow absolute inset-0" />
          </div>
          <span className="relative z-[2] flex select-none items-center gap-2 font-semibold">
            <Plus className="h-4 w-4" />
            <span className="hidden sm:inline">{t('pages.jobs.newJob')}</span>
            <span className="sm:hidden">{t('pages.jobs.newJobShort')}</span>
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
              {editingJobId ? t('pages.jobs.dialog.editTitle') : t('pages.jobs.dialog.createTitle')}
            </DialogTitle>
            <div style={{ fontSize: 13.5, color: T.ink, opacity: 0.55, marginTop: 2 }}>
              {editingJobId
                ? t('pages.jobs.dialog.editDescription')
                : t('pages.jobs.dialog.createDescription')}
            </div>
          </div>
          <button
            type="button"
            onClick={() => onDialogOpenChange(false)}
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

        {/* Tipo de Ação */}
        <div style={sectionPad}>
          <div className="mb-2" style={{ fontSize: 14, fontWeight: 700, color: T.ink }}>
            {t('pages.jobs.form.actionType')} <span style={{ color: T.wine }}>*</span>
          </div>
          <select
            value={actionType}
            onChange={(e) => onActionTypeChange(e.target.value)}
            className="w-full cursor-pointer outline-none"
            style={{
              fontSize: 14,
              color: actionType ? T.ink : T.placeholder,
              padding: '11px 14px',
              borderRadius: 12,
              border: `1.5px solid ${T.border}`,
              background: T.white,
            }}
          >
            <option value="">{t('pages.jobs.form.actionTypePlaceholder')}</option>
            {Object.entries(ACTION_KEYS).map(([value, key]) => (
              <option key={value} value={value}>
                {t(`pages.jobs.actionTypes.${key}`)}
              </option>
            ))}
          </select>
        </div>

        {/* Sub-formulário da ação */}
        {actionType && (
          <div style={sectionPad}>
            {actionType === 'send_poll' && (
              <PollOptionsForm
                pollQuestion={pollQuestion}
                pollOptions={pollOptions}
                onQuestionChange={onPollQuestionChange}
                onAddOption={onAddPollOption}
                onRemoveOption={onRemovePollOption}
                onUpdateOption={onUpdatePollOption}
              />
            )}
            {actionType === 'send_message' && (
              <MessageForm
                payload={payload}
                mediaFile={mediaFile}
                mentionOpen={mentionOpen}
                isEnhancing={isEnhancing}
                onPayloadChange={onPayloadChange}
                onMediaFileChange={onMediaFileChange}
                onMentionOpenChange={onMentionOpenChange}
                onInsertMention={onInsertMention}
                onEnhanceMessage={onEnhanceMessage}
              />
            )}
            {actionType === 'update_description' && (
              <div className="space-y-2">
                <Label htmlFor="payload" className="cursor-pointer">
                  {t('pages.jobs.form.newDescription')}
                </Label>
                <Textarea
                  id="payload"
                  placeholder={t('pages.jobs.form.newDescriptionPlaceholder')}
                  value={payload}
                  onChange={(e) => onPayloadChange(e.target.value)}
                  rows={3}
                  className="min-h-[80px] cursor-text lg:min-h-[120px]"
                />
              </div>
            )}
            {actionType === 'change_group_name' && (
              <GroupNameForm
                payload={payload}
                autoNumberGroups={autoNumberGroups}
                onPayloadChange={onPayloadChange}
                onAutoNumberChange={onAutoNumberGroupsChange}
              />
            )}
            {actionType === 'change_group_photo' && (
              <div className="space-y-2">
                <Label htmlFor="group-photo" className="cursor-pointer">
                  {t('pages.jobs.form.newPhoto')}
                </Label>
                <input
                  id="group-photo"
                  type="file"
                  accept="image/*"
                  onChange={(e) => onMediaFileChange(e.target.files?.[0] || null)}
                  className="w-full cursor-pointer text-sm"
                />
                {mediaFile && (
                  <p className="truncate text-sm text-muted-foreground">
                    {t('pages.jobs.form.fileSelected')} {mediaFile.name}
                  </p>
                )}
              </div>
            )}
          </div>
        )}

        {/* Selecionar Grupos */}
        <div style={sectionPad}>
          <div className="mb-2.5 flex flex-wrap items-center justify-between gap-3">
            <div style={{ fontSize: 14, fontWeight: 700, color: T.ink }}>
              {t('pages.jobs.form.selectGroups')} <span style={{ color: T.wine }}>*</span>
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
                selected: selectedCount,
                total: totalGroups,
              })}
            </span>
          </div>
          <GroupPicker
            groups={groups}
            selectedGroups={selectedGroups}
            searchTerm={searchTerm}
            loading={loadingGroups}
            onSearchChange={onSearchTermChange}
            onToggleGroup={onToggleGroup}
            onSelectAll={onSelectAllGroups}
            onClearAll={onClearGroups}
            onToggleGroups={onToggleGroups}
            onToggleFavorite={onToggleFavorite}
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
            onRecurrenceChange={onRecurrenceChange}
            onToggleWeekday={onToggleWeekday}
            onIntervalChange={onIntervalChange}
            onAddTime={onAddTime}
            onRemoveTime={onRemoveTime}
            onScheduledDateChange={onScheduledDateChange}
            onRepeatUntilChange={onRepeatUntilChange}
          />
        </div>

        {/* Resumo */}
        <div
          className="flex items-center gap-2.5"
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
            }}
          >
            i
          </span>
          <span style={{ fontSize: 13.5, color: T.ink }}>{resumo}</span>
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
            onClick={onSubmit}
            disabled={!ready || isPending}
            className="hf-cta w-full whitespace-nowrap"
            style={{
              fontSize: 14,
              fontWeight: 700,
              color: T.white,
              background: ctaBg,
              border: 'none',
              borderRadius: 12,
              padding: '13px 26px',
              cursor: !ready || isPending ? 'not-allowed' : 'pointer',
              boxShadow: '0 6px 18px rgba(125,34,72,0.35)',
            }}
          >
            {editingJobId
              ? isPending
                ? t('pages.jobs.form.saving')
                : t('pages.jobs.form.save')
              : isPending
                ? t('pages.jobs.form.submitting')
                : `🗓 ${t('pages.jobs.schedule.cta', { count: selectedCount })}`}
          </button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

/** Monta a frase-resumo em linguagem natural (porta a lógica do handoff). */
function buildResumo(
  t: (key: string, opts?: Record<string, unknown>) => string,
  opts: {
    actionLabel: string;
    count: number;
    recurrence: RecurrenceType;
    weekdays: number[];
    intervalDays: number;
    times: string[];
    scheduledDate: string;
    repeatUntil: string;
  }
): string {
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

  const timesPart = times.length
    ? ` ${t(`${S}.atTimes`, { times: [...times].sort().join(', ') })}`
    : `, ${t(`${S}.addTimeHint`)}`;
  const action = opts.actionLabel || t(`${S}.actionFallback`);
  return `${t(`${S}.prefix`)} ${action} ${t(`${S}.forGroups`, { count: opts.count })}, ${when}${timesPart}.`;
}
