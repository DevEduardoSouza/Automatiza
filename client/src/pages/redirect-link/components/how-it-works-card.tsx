import { HelpCircle } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

export function HowItWorksCard() {
  const { t } = useTranslation();

  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
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
          {t('pages.redirectLink.howItWorks.title')}
        </span>
      </div>

      <div className="space-y-4">
        <p className="text-sm md:text-base" style={{ color: '#71717a' }}>
          {t('pages.redirectLink.howItWorks.description')}
        </p>
        <ol
          className="list-decimal list-inside space-y-2 text-sm md:text-base"
          style={{ color: '#71717a' }}
        >
          <li>{t('pages.redirectLink.howItWorks.step1')}</li>
          <li>{t('pages.redirectLink.howItWorks.step2')}</li>
          <li>{t('pages.redirectLink.howItWorks.step3')}</li>
          <li>{t('pages.redirectLink.howItWorks.step4')}</li>
          <li>{t('pages.redirectLink.howItWorks.step5')}</li>
        </ol>
        <div
          className="p-3 md:p-4 rounded-lg border"
          style={{
            backgroundColor: colors.iconBg,
            borderColor: colors.wineBorder,
          }}
        >
          <p className="text-xs md:text-sm">
            <strong>{t('common.tip')}:</strong> {t('pages.redirectLink.howItWorks.tip')}
          </p>
        </div>
      </div>
    </div>
  );
}
