import { useTranslation } from 'react-i18next';

const colors = {
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
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
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="mb-4">
        <div className="flex justify-between items-start">
          <div>
            <div className="flex items-center gap-2">
              <span className="text-base font-semibold" style={{ color: '#18181b' }}>
                {storeName}
              </span>
              <span
                className="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium"
                style={{ backgroundColor: 'rgba(141, 45, 83, 0.06)', color: colors.wineTextLight }}
              >
                {t('pages.affiliatePrograms.comingSoon')}
              </span>
            </div>
            <p className="mt-1 text-sm" style={{ color: colors.wineTextMuted }}>
              {t('pages.affiliatePrograms.comingSoonDesc', { store: storeName })}
            </p>
          </div>
        </div>
      </div>
      <p className="text-sm" style={{ color: colors.wineTextMuted }}>
        {t('pages.affiliatePrograms.comingSoonNote')}
      </p>
    </div>
  );
}
