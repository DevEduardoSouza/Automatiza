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
      <div className="flex flex-col w-full h-full">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8">
            <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
              <div className="flex items-center gap-2 xs:gap-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                  style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
                >
                  <FileText
                    className="w-5 h-5 xs:w-6 xs:h-6"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <div>
                  <h1
                    className="text-xl xs:text-2xl md:text-3xl font-semibold tracking-tight"
                    style={{ color: '#18181b' }}
                  >
                    {t('pages.history.title')}
                  </h1>
                  <p
                    className="text-xs xs:text-sm md:text-base tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.history.subtitle')}
                  </p>
                </div>
              </div>
            </div>

            <HistoryStatsCards stats={stats} />

            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-center justify-between mb-4 xs:mb-5">
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
                      className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
                      style={{ color: colors.wineText }}
                    >
                      {t('pages.history.table.title')}
                    </span>
                    <p className="text-sm mt-0.5" style={{ color: colors.wineTextMuted }}>
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
                <div className="text-center py-8" style={{ color: colors.wineTextMuted }}>
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
