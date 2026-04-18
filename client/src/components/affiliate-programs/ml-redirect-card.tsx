import { ArrowRight, ShoppingBag } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

/**
 * Card displayed on the affiliate-programs page for Mercado Livre.
 * The ML credential flow was retired in PR #62 — users now paste their
 * pre-generated affiliate URLs directly into ML link lists. This card
 * replaces the old credential form with a pointer to that flow.
 */
export function MlRedirectCard() {
  const { t } = useTranslation();

  return (
    <div
      className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="flex items-center gap-2 xs:gap-3">
        <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
          <ShoppingBag
            className="w-4 h-4 xs:w-5 xs:h-5"
            style={{ color: colors.wine }}
            aria-hidden="true"
          />
        </div>
        <div>
          <span className="text-base lg:text-lg font-semibold" style={{ color: '#18181b' }}>
            {t('pages.affiliatePrograms.mlRedirect.title')}
          </span>
          <p className="text-xs xs:text-sm tracking-tight" style={{ color: colors.wineTextMuted }}>
            {t('pages.affiliatePrograms.mlRedirect.subtitle')}
          </p>
        </div>
      </div>

      <p
        className="text-xs xs:text-sm mt-4 lg:mt-6 leading-relaxed"
        style={{ color: colors.wineTextMuted }}
      >
        {t('pages.affiliatePrograms.mlRedirect.message')}
      </p>

      <Link
        to="/ml-link-lists"
        className="inline-flex items-center gap-1.5 mt-4 lg:mt-5 rounded-full px-4 py-2 text-xs xs:text-sm font-semibold text-white transition-opacity hover:opacity-90"
        style={{ backgroundColor: colors.wine }}
      >
        {t('pages.affiliatePrograms.mlRedirect.cta')}
        <ArrowRight className="w-3.5 h-3.5" aria-hidden="true" />
      </Link>
    </div>
  );
}
