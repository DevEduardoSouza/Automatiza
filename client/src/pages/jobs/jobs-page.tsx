import { ErrorBoundary } from '@/components/error-boundary';
import { ScheduleMultipleGroups } from '@/components/groups';
import { useJobs } from '@/hooks/use-jobs';
import { AlertCircle, HelpCircle, List } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const SchedulesIcon = ({ className, style }: { className?: string; style?: React.CSSProperties }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    style={style}
  >
    <path d="M16 2V6M8 2V6" />
    <path d="M13 4H11C7.22876 4 5.34315 4 4.17157 5.17157C3 6.34315 3 8.22876 3 12V14C3 17.7712 3 19.6569 4.17157 20.8284C5.34315 22 7.22876 22 11 22H13C16.7712 22 18.6569 22 19.8284 20.8284C21 19.6569 21 17.7712 21 14V12C21 8.22876 21 6.34315 19.8284 5.17157C18.6569 4 16.7712 4 13 4Z" />
    <path d="M3 10H21" />
    <path d="M11.9955 14H12.0045M11.9955 18H12.0045M15.991 14H16M8 14H8.00897M8 18H8.00897" />
  </svg>
);
import { JobFormDialog } from './components/job-form-dialog';
import { JobsMobileList } from './components/jobs-mobile-list';
import { JobsTable } from './components/jobs-table';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
  iconBgHover: 'rgba(141, 45, 83, 0.15)',
};

/**
 * Página de agendamentos de jobs.
 * UI apenas - toda lógica vem do useJobs().
 */
export default function JobsPage() {
  const { t } = useTranslation();
  const jobs = useJobs();

  return (
    <ErrorBoundary>
      <div className="jobs-page flex flex-col w-full h-full 2xl:max-w-[1600px] 2xl:mx-auto">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl lg:rounded-3xl p-4 xs:p-5 sm:p-6 md:p-6 lg:p-8 xl:p-10"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8 lg:space-y-10">
            {/* Header */}
            <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
              <div className="flex items-center gap-2 xs:gap-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                  style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
                >
                  <SchedulesIcon className="w-5 h-5 xs:w-6 xs:h-6" style={{ color: colors.wine }} />
                </div>
                <div>
                  <h2
                    className="text-xl xs:text-2xl md:text-3xl xl:text-4xl font-semibold tracking-tight"
                    style={{ color: '#18181b' }}
                  >
                    {t('pages.jobs.title')}
                  </h2>
                  <p
                    className="text-xs xs:text-sm tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.jobs.subtitle')}
                  </p>
                </div>
              </div>

              <div className="flex flex-col xs:flex-row gap-2 flex-wrap">
                <JobFormDialog
                  dialogOpen={jobs.dialogOpen}
                  editingJobId={jobs.editingJobId}
                  actionType={jobs.actionType}
                  scheduledDate={jobs.scheduledDate}
                  scheduledTime={jobs.scheduledTime}
                  payload={jobs.payload}
                  mediaFile={jobs.mediaFile}
                  isEnhancing={jobs.isEnhancing}
                  selectedGroups={jobs.selectedGroups}
                  mentionOpen={jobs.mentionOpen}
                  autoNumberGroups={jobs.autoNumberGroups}
                  searchTerm={jobs.searchTerm}
                  pollQuestion={jobs.pollQuestion}
                  pollOptions={jobs.pollOptions}
                  isPending={jobs.isPending}
                  groups={jobs.groups}
                  loadingGroups={jobs.loadingGroups}
                  onDialogOpenChange={jobs.setDialogOpen}
                  onActionTypeChange={jobs.setActionType}
                  onScheduledDateChange={jobs.setScheduledDate}
                  onScheduledTimeChange={jobs.setScheduledTime}
                  onPayloadChange={jobs.setPayload}
                  onMediaFileChange={jobs.setMediaFile}
                  onMentionOpenChange={jobs.setMentionOpen}
                  onAutoNumberGroupsChange={jobs.setAutoNumberGroups}
                  onSearchTermChange={jobs.setSearchTerm}
                  onPollQuestionChange={jobs.setPollQuestion}
                  onToggleGroup={jobs.toggleGroup}
                  onInsertMention={jobs.insertMention}
                  onAddPollOption={jobs.addPollOption}
                  onRemovePollOption={jobs.removePollOption}
                  onUpdatePollOption={jobs.updatePollOption}
                  onEnhanceMessage={jobs.handleEnhanceMessage}
                  onSubmit={jobs.handleScheduleJob}
                />

                <ScheduleMultipleGroups />
              </div>
            </div>

            {/* Warning Alert - Glassmorphism */}
            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8"
              role="alert"
              style={{
                backgroundColor: 'rgba(180, 120, 60, 0.08)',
                borderColor: 'rgba(180, 120, 60, 0.25)',
                backdropFilter: 'blur(12px) saturate(180%)',
                WebkitBackdropFilter: 'blur(12px) saturate(180%)',
              }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-2 xs:mb-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg"
                  style={{ backgroundColor: 'rgba(180, 120, 60, 0.12)' }}
                >
                  <AlertCircle
                    className="w-4 h-4 xs:w-5 xs:h-5"
                    style={{ color: 'rgb(160, 100, 40)' }}
                    aria-hidden="true"
                  />
                </div>
                <span
                  className="text-sm xs:text-base font-semibold tracking-tight"
                  style={{ color: 'rgb(120, 75, 30)' }}
                >
                  {t('pages.jobs.alert.title')}
                </span>
              </div>
              <p
                className="text-xs xs:text-sm leading-relaxed"
                style={{ color: 'rgba(120, 75, 30, 0.8)' }}
              >
                {t('pages.jobs.alert.message')}
              </p>
            </div>

            {/* Agendamentos Card */}
            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
                <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
                  <List
                    className="w-4 h-4 xs:w-5 xs:h-5"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <div>
                  <span
                    className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em] font-semibold"
                    style={{ color: colors.wineTextLight }}
                  >
                    {t('pages.jobs.table.title')}
                  </span>
                  <p
                    className="text-xs xs:text-sm tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.jobs.table.description')}
                  </p>
                </div>
              </div>

              <ErrorBoundary>
              {jobs.loadingJobs ? (
                <div className="flex items-center justify-center py-8">
                  <div
                    className="h-8 w-8 animate-spin rounded-full border-4 border-t-transparent"
                    style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
                  />
                </div>
              ) : jobs.jobs.length === 0 ? (
                <div className="text-center py-8">
                  <p style={{ color: colors.wineTextMuted }}>{t('pages.jobs.empty')}</p>
                </div>
              ) : (
                <>
                  <div className="hidden md:block">
                    <JobsTable
                      jobs={jobs.jobs}
                      onEditJob={jobs.handleEditJob}
                      onDeleteJob={jobs.handleDeleteJob}
                      getStatusBadgeConfig={jobs.getStatusBadgeConfig}
                      getActionLabel={jobs.getActionLabel}
                    />
                  </div>

                  <div className="md:hidden">
                    <JobsMobileList
                      jobs={jobs.jobs}
                      onEditJob={jobs.handleEditJob}
                      onDeleteJob={jobs.handleDeleteJob}
                      getStatusBadgeConfig={jobs.getStatusBadgeConfig}
                      getActionLabel={jobs.getActionLabel}
                    />
                  </div>

                  <div
                    className="flex flex-col xs:flex-row items-start xs:items-center justify-between gap-3 xs:gap-0 mt-4 pt-4 border-t"
                    style={{ borderColor: colors.wineBorder }}
                  >
                    <p className="text-sm" style={{ color: colors.wineTextMuted }}>
                      {t('pages.jobs.table.showing', {
                        count: jobs.jobs.length,
                        total: jobs.totalCount,
                      })}
                    </p>
                    {jobs.hasNextPage && (
                      <button
                        type="button"
                        onClick={jobs.loadNextPage}
                        className="inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
                      >
                        {t('pages.jobs.table.loadMore')}
                      </button>
                    )}
                  </div>
                </>
              )}
              </ErrorBoundary>
            </div>

            {/* Como funciona Card */}
            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8 xl:p-10"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
                <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
                  <HelpCircle
                    className="w-4 h-4 xs:w-5 xs:h-5"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <span
                  className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em] font-semibold"
                  style={{ color: colors.wineTextLight }}
                >
                  {t('pages.jobs.howItWorks.title')}
                </span>
              </div>

              <div className="space-y-3">
                <p className="text-xs xs:text-sm" style={{ color: '#71717a' }}>
                  {t('pages.jobs.howItWorks.description')}
                </p>
                <ul
                  className="list-disc list-inside space-y-1.5 text-xs xs:text-sm"
                  style={{ color: '#71717a' }}
                >
                  <li>
                    <strong>{t('pages.jobs.actionTypes.sendMessage')}:</strong>{' '}
                    {t('pages.jobs.howItWorks.sendMessage')}
                  </li>
                  <li>
                    <strong>{t('pages.jobs.actionTypes.updateDescription')}:</strong>{' '}
                    {t('pages.jobs.howItWorks.updateDescription')}
                  </li>
                  <li>
                    <strong>
                      {t('pages.jobs.actionTypes.closeGroups')}/
                      {t('pages.jobs.actionTypes.openGroups')}:
                    </strong>{' '}
                    {t('pages.jobs.howItWorks.closeOpenGroups')}
                  </li>
                  <li>
                    <strong>{t('pages.jobs.actionTypes.changeGroupName')}:</strong>{' '}
                    {t('pages.jobs.howItWorks.changeName')}
                  </li>
                  <li>
                    <strong>{t('pages.jobs.actionTypes.changeGroupPhoto')}:</strong>{' '}
                    {t('pages.jobs.howItWorks.changePhoto')}
                  </li>
                </ul>
                <div
                  className="p-2.5 xs:p-3 rounded-lg xs:rounded-xl border"
                  style={{
                    backgroundColor: colors.iconBg,
                    borderColor: colors.wineBorder,
                  }}
                >
                  <p className="text-xs xs:text-sm">
                    <strong>{t('common.tip')}:</strong> {t('pages.jobs.howItWorks.note')}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </ErrorBoundary>
  );
}
