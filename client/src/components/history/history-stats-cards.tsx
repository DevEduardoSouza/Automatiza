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
    <div className="grid gap-4 grid-cols-2 md:grid-cols-4">
      <div
        className="rounded-xl xs:rounded-2xl border p-4 xs:p-5"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <p className="text-xs md:text-sm font-medium mb-2" style={{ color: colors.wineTextMuted }}>
          {t('pages.history.stats.total')}
        </p>
        <div className="text-xl md:text-2xl font-bold">{stats.total}</div>
      </div>

      <div
        className="rounded-xl xs:rounded-2xl border p-4 xs:p-5"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <p className="text-xs md:text-sm font-medium mb-2" style={{ color: colors.wineTextMuted }}>
          {t('pages.history.stats.completed')}
        </p>
        <div className="text-xl md:text-2xl font-bold text-green-600">{stats.completed}</div>
      </div>

      <div
        className="rounded-xl xs:rounded-2xl border p-4 xs:p-5"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <p className="text-xs md:text-sm font-medium mb-2" style={{ color: colors.wineTextMuted }}>
          {t('pages.history.stats.running')}
        </p>
        <div className="text-xl md:text-2xl font-bold text-blue-600">{stats.running}</div>
      </div>

      <div
        className="rounded-xl xs:rounded-2xl border p-4 xs:p-5"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <p className="text-xs md:text-sm font-medium mb-2" style={{ color: colors.wineTextMuted }}>
          {t('pages.history.stats.failed')}
        </p>
        <div className="text-xl md:text-2xl font-bold text-red-600">{stats.failed}</div>
      </div>
    </div>
  );
}
