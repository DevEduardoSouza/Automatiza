import type { MlLastTest } from '@/hooks/use-affiliate-programs';
import { useTranslation } from 'react-i18next';

interface MlTestResultProps {
  result: MlLastTest;
  formatSoldQuantity: (q: number | null | undefined) => string | null;
}

/**
 * Displays the result of a Mercado Livre link test.
 * Shows product thumbnail, title, price, ratings, and generated affiliate links.
 */
export function MlTestResult({ result, formatSoldQuantity }: MlTestResultProps) {
  const { t } = useTranslation();

  if (!result.originUrl) {
    return null;
  }

  return (
    <div
      className="rounded-xl border p-3 space-y-2 text-sm"
      style={{ backgroundColor: 'rgba(141, 45, 83, 0.08)', borderColor: 'rgba(141, 45, 83, 0.2)' }}
    >
      <div className="font-medium">{t('pages.affiliatePrograms.ml.testResult')}</div>

      {result.thumbnail && (
        <img
          src={result.thumbnail}
          alt={result.title || 'Produto'}
          className="w-24 h-24 object-cover rounded"
        />
      )}

      {result.title ? (
        <div>
          <div className="font-medium">{result.title}</div>
          <div style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
            {[result.condition, formatSoldQuantity(result.soldQuantity)]
              .filter(Boolean)
              .join(' | ')}
          </div>

          {(result.ratingAverage != null || result.reviewsTotal != null) && (
            <div style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
              {result.ratingAverage != null && `${String(result.ratingAverage).slice(0, 4)} \u2605`}
              {result.ratingAverage != null && result.reviewsTotal != null && ' '}
              {result.reviewsTotal != null && `(${result.reviewsTotal})`}
            </div>
          )}

          {result.price != null && (
            <div style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
              {String(result.currency || 'BRL')} {String(result.price)}
            </div>
          )}

          {(result.originalPrice != null || result.discountPercent != null) && (
            <div style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
              {result.originalPrice != null &&
                `${t('pages.affiliatePrograms.ml.from')} ${String(result.currency || 'BRL')} ${String(result.originalPrice)}`}
              {result.originalPrice != null && result.discountPercent != null && ' \u2022 '}
              {result.discountPercent != null &&
                `${result.discountPercent}% ${t('pages.affiliatePrograms.ml.off')}`}
            </div>
          )}

          {result.installmentsText && (
            <div style={{ color: 'rgba(141, 45, 83, 0.6)' }}>{result.installmentsText}</div>
          )}
        </div>
      ) : (
        <div style={{ color: 'rgba(141, 45, 83, 0.6)' }}>{t('pages.affiliatePrograms.ml.noTitlePrice')}</div>
      )}

      {result.shortUrl && (
        <div className="break-all">
          <span className="font-medium">{t('pages.affiliatePrograms.ml.shortUrl')}</span>{' '}
          {result.shortUrl}
        </div>
      )}

      {result.longUrl && (
        <div className="break-all">
          <span className="font-medium">{t('pages.affiliatePrograms.ml.longUrl')}</span>{' '}
          {result.longUrl}
        </div>
      )}
    </div>
  );
}
