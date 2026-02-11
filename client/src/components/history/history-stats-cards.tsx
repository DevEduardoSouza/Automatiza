import type { HistoryStats } from '@/types/action-history';
import { Activity, CheckCircle2, Loader2, XCircle } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

const STAT_CARDS = [
  { key: 'total', icon: Activity, valueClass: '' },
  { key: 'completed', icon: CheckCircle2, valueClass: 'text-green-600' },
  { key: 'running', icon: Loader2, valueClass: 'text-blue-600' },
  { key: 'failed', icon: XCircle, valueClass: 'text-red-600' },
] as const;

interface HistoryStatsCardsProps {
  stats: HistoryStats;
}

/**
 * Statistics cards component for action history.
 * Displays total, completed, running, and failed counts with icons and hover effects.
 */
export function HistoryStatsCards({ stats }: HistoryStatsCardsProps) {
  const { t } = useTranslation();
  const [hoveredCard, setHoveredCard] = useState<string | null>(null);

  return (
    <div className="grid gap-3 xs:gap-4 lg:gap-6 grid-cols-2 lg:grid-cols-4">
      {STAT_CARDS.map(({ key, icon: Icon, valueClass }) => (
        <div
          key={key}
          className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-3 xs:p-4 sm:p-5 lg:p-6 transition-colors"
          style={{
            backgroundColor: hoveredCard === key ? colors.wineBgHover : colors.wineBg,
            borderColor: colors.wineBorder,
          }}
          onMouseEnter={() => setHoveredCard(key)}
          onMouseLeave={() => setHoveredCard(null)}
        >
          <div className="flex items-center gap-2 xs:gap-3 mb-2 xs:mb-3">
            <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
              <Icon
                className="w-3.5 h-3.5 xs:w-4 xs:h-4"
                style={{ color: colors.wine }}
                aria-hidden="true"
              />
            </div>
            <span
              className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em] font-semibold"
              style={{ color: colors.wineTextLight }}
            >
              {t(`pages.history.stats.${key}`)}
            </span>
          </div>
          <div className={`text-xl xs:text-2xl font-semibold tracking-tight ${valueClass}`}>
            {stats[key as keyof HistoryStats]}
          </div>
        </div>
      ))}
    </div>
  );
}
