import { ErrorBoundary } from '@/components/error-boundary';
import {
  HistoryClearDialog,
  HistoryFilters,
  HistoryStatsCards,
  HistoryTable,
} from '@/components/history';
import { useHistory } from '@/hooks/use-history';
import { FileText } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const HistoryIcon = ({ className, style }: { className?: string; style?: React.CSSProperties }) => (
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
    <path d="M19 10.5V9.99995C19 6.22876 18.9999 4.34311 17.8284 3.17154C16.6568 2 14.7712 2 11 2C7.22889 2 5.34326 2.00006 4.17169 3.17159C3.00015 4.34315 3.00013 6.22872 3.0001 9.99988L3.00006 14.5C3.00003 17.7874 3.00002 19.4312 3.90794 20.5375C4.07418 20.7401 4.25992 20.9258 4.46249 21.0921C5.56883 22 7.21255 22 10.5 22" />
    <path d="M7 7H15M7 11H11" />
    <path d="M18 18.5L16.5 17.95V15.5M12 17.5C12 19.9853 14.0147 22 16.5 22C18.9853 22 21 19.9853 21 17.5C21 15.0147 18.9853 13 16.5 13C14.0147 13 12 15.0147 12 17.5Z" />
  </svg>
);

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

/**
 * Action history page component.
 * Displays history of bulk actions with statistics and detailed logs.
 */
export default function History() {
  const { t } = useTranslation();
  const {
    history,
    loading,
    stats,
    filters,
    setFilters,
    clearFilters,
    clearHistory,
    clearingHistory,
    // Pagination
    currentPage,
    pageSize,
    totalCount,
    totalPages,
    setCurrentPage,
  } = useHistory();

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div
          className="h-8 w-8 animate-spin rounded-full border-4 border-t-transparent"
          style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
        />
      </div>
    );
  }

  return (
    <ErrorBoundary>
      <div className="flex flex-col w-full h-full 2xl:max-w-[1600px] 2xl:mx-auto">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl lg:rounded-3xl p-4 xs:p-5 sm:p-6 md:p-6 lg:p-8 xl:p-10"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8 lg:space-y-10">
            <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
              <div className="flex items-center gap-2 xs:gap-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                  style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
                >
                  <HistoryIcon
                    className="w-5 h-5 xs:w-6 xs:h-6"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <div>
                  <h2
                    className="text-xl xs:text-2xl md:text-3xl xl:text-4xl font-semibold tracking-tight"
                    style={{ color: '#18181b' }}
                  >
                    {t('pages.history.title')}
                  </h2>
                  <p
                    className="text-xs xs:text-sm tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.history.subtitle')}
                  </p>
                </div>
              </div>
            </div>

            <HistoryStatsCards stats={stats} />

            <div
              className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-center justify-between mb-4 xs:mb-5 lg:mb-6">
                <div className="flex items-center gap-2 xs:gap-3">
                  <div
                    className="p-1.5 xs:p-2 rounded-lg"
                    style={{ backgroundColor: colors.iconBg }}
                  >
                    <FileText
                      className="w-4 h-4 xs:w-5 xs:h-5"
                      style={{ color: colors.wine }}
                      aria-hidden="true"
                    />
                  </div>
                  <div>
                    <span
                      className="text-xs xs:text-sm lg:text-base font-semibold uppercase tracking-[0.1em] xs:tracking-[0.12em]"
                      style={{ color: colors.wineText }}
                    >
                      {t('pages.history.table.title')}
                    </span>
                    <p className="text-xs xs:text-sm lg:text-base mt-0.5" style={{ color: colors.wineTextMuted }}>
                      {t('pages.history.table.description')}
                    </p>
                  </div>
                </div>
                <HistoryClearDialog
                  onConfirm={clearHistory}
                  loading={clearingHistory}
                  disabled={history.length === 0}
                />
              </div>
              <HistoryFilters
                filters={filters}
                onFiltersChange={setFilters}
                onClearFilters={clearFilters}
              />
              {history.length === 0 && !loading ? (
                <div className="text-center py-8 lg:py-12" style={{ color: colors.wineTextMuted }}>
                  {t('pages.history.table.empty')}
                </div>
              ) : (
                <HistoryTable
                  history={history}
                  currentPage={currentPage}
                  pageSize={pageSize}
                  totalCount={totalCount}
                  totalPages={totalPages}
                  onPageChange={setCurrentPage}
                />
              )}
            </div>
          </div>
        </div>
      </div>
    </ErrorBoundary>
  );
}
