import type { HistoryStats } from '@/types/action-history';
import { useTranslation } from 'react-i18next';

const colors = {
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

interface HistoryStatsCardsProps {
  stats: HistoryStats;
}

/**
 * Statistics cards component for action history.
 * Displays total, completed, running, and failed counts.
 */
export function HistoryStatsCards({ stats }: HistoryStatsCardsProps) {
  const { t } = useTranslation();

  return (
    <div className="grid gap-3 xs:gap-4 lg:gap-6 grid-cols-2 md:grid-cols-4">
      <div
        className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-3 xs:p-4 sm:p-5 lg:p-6"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <p className="text-[11px] xs:text-xs md:text-sm lg:text-base font-medium mb-1.5 xs:mb-2" style={{ color: colors.wineTextMuted }}>
          {t('pages.history.stats.total')}
        </p>
        <div className="text-lg xs:text-xl md:text-2xl lg:text-3xl font-bold">{stats.total}</div>
      </div>

      <div
        className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-3 xs:p-4 sm:p-5 lg:p-6"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <p className="text-[11px] xs:text-xs md:text-sm lg:text-base font-medium mb-1.5 xs:mb-2" style={{ color: colors.wineTextMuted }}>
          {t('pages.history.stats.completed')}
        </p>
        <div className="text-lg xs:text-xl md:text-2xl lg:text-3xl font-bold text-green-600">{stats.completed}</div>
      </div>

      <div
        className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-3 xs:p-4 sm:p-5 lg:p-6"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <p className="text-[11px] xs:text-xs md:text-sm lg:text-base font-medium mb-1.5 xs:mb-2" style={{ color: colors.wineTextMuted }}>
          {t('pages.history.stats.running')}
        </p>
        <div className="text-lg xs:text-xl md:text-2xl lg:text-3xl font-bold text-blue-600">{stats.running}</div>
      </div>

      <div
        className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-3 xs:p-4 sm:p-5 lg:p-6"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <p className="text-[11px] xs:text-xs md:text-sm lg:text-base font-medium mb-1.5 xs:mb-2" style={{ color: colors.wineTextMuted }}>
          {t('pages.history.stats.failed')}
        </p>
        <div className="text-lg xs:text-xl md:text-2xl lg:text-3xl font-bold text-red-600">{stats.failed}</div>
      </div>
    </div>
  );
}
