import { Clock } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const colors = {
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

interface ComingSoonCardProps {
  storeName: string;
}

/**
 * Card displayed for stores that are not yet enabled.
 * Shows a "Coming Soon" badge and informative message.
 */
export function ComingSoonCard({ storeName }: ComingSoonCardProps) {
  const { t } = useTranslation();

  return (
    <div
      data-testid="coming-soon-card"
      className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="flex items-center gap-2 xs:gap-3">
        <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
          <Clock
            className="w-4 h-4 xs:w-5 xs:h-5"
            style={{ color: colors.wineTextLight }}
            aria-hidden="true"
          />
        </div>
        <div>
          <div className="flex items-center gap-2">
            <span className="text-base lg:text-lg font-semibold" style={{ color: '#18181b' }}>
              {storeName}
            </span>
            <span
              className="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium"
              style={{ backgroundColor: 'rgba(141, 45, 83, 0.06)', color: colors.wineTextLight }}
            >
              {t('pages.affiliatePrograms.comingSoon')}
            </span>
          </div>
          <p className="text-xs xs:text-sm tracking-tight" style={{ color: colors.wineTextMuted }}>
            {t('pages.affiliatePrograms.comingSoonDesc', { store: storeName })}
          </p>
        </div>
      </div>
      <p className="text-xs xs:text-sm mt-4 lg:mt-6" style={{ color: colors.wineTextMuted }}>
        {t('pages.affiliatePrograms.comingSoonNote')}
      </p>
    </div>
  );
}
