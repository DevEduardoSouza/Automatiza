import type { MlLastTest } from '@/hooks/use-affiliate-programs';
import { useTranslation } from 'react-i18next';
import { MlTestResult } from './ml-test-result';

interface MlConnectionFormProps {
  connectionToken: string;
  tag: string;
  testProductUrl: string;
  mlLastTest: MlLastTest | null;
  onConnectionTokenChange: (value: string) => void;
  onTagChange: (value: string) => void;
  onTestProductUrlChange: (value: string) => void;
  onValidateToken: () => void;
  formatSoldQuantity: (q: number | null | undefined) => string | null;
}

const inputClasses =
  'w-full rounded-2xl border-2 border-gray-300 bg-gray-50 px-4 py-2 text-xs xs:py-2.5 xs:text-sm text-gray-900 placeholder-gray-400 outline-none transition-all hover:border-[#520B27] focus:border-[#520B27] focus:ring-1 focus:ring-[#3a0718]/30';

/**
 * Form fields specific to Mercado Livre connection.
 * Handles connection token input, validation, tag field, and test product URL.
 */
export function MlConnectionForm({
  connectionToken,
  tag,
  testProductUrl,
  mlLastTest,
  onConnectionTokenChange,
  onTagChange,
  onTestProductUrlChange,
  onValidateToken,
  formatSoldQuantity,
}: MlConnectionFormProps) {
  const { t } = useTranslation();

  return (
    <>
      <div className="space-y-2">
        <label
          htmlFor="ml-connection-token"
          className="mb-1.5 block text-xs lg:text-sm font-medium pl-2.5 text-gray-700"
        >
          {t('pages.affiliatePrograms.ml.connectionToken')}
        </label>
        <textarea
          id="ml-connection-token"
          placeholder={t('pages.affiliatePrograms.ml.connectionTokenPlaceholder')}
          value={connectionToken}
          onChange={(e) => onConnectionTokenChange(e.target.value)}
          className="w-full min-h-[90px] rounded-2xl border-2 border-gray-300 bg-gray-50 px-4 py-3 text-xs xs:text-sm text-gray-900 placeholder-gray-400 outline-none transition-all hover:border-[#520B27] focus:border-[#520B27] focus:ring-1 focus:ring-[#3a0718]/30 resize-y"
        />
        <button
          type="button"
          onClick={onValidateToken}
          className="inline-flex w-full xs:w-auto cursor-pointer items-center justify-center gap-2 rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
        >
          {t('pages.affiliatePrograms.ml.validateToken')}
        </button>
        <p className="text-xs" style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
          {t('pages.affiliatePrograms.ml.extensionNote')}
        </p>
        <p className="text-xs" style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
          {t('pages.affiliatePrograms.ml.securityNote')}
        </p>
      </div>

      <div className="space-y-2">
        <label
          htmlFor="ml-tag"
          className="mb-1.5 block text-xs lg:text-sm font-medium pl-2.5 text-gray-700"
        >
          {t('pages.affiliatePrograms.form.tag')}
        </label>
        <input
          id="ml-tag"
          type="text"
          placeholder={t('pages.affiliatePrograms.form.tagPlaceholder')}
          value={tag}
          onChange={(e) => onTagChange(e.target.value)}
          className={inputClasses}
        />
      </div>

      <div className="space-y-2">
        <label
          htmlFor="ml-testProductUrl"
          className="mb-1.5 block text-xs lg:text-sm font-medium pl-2.5 text-gray-700"
        >
          {t('pages.affiliatePrograms.ml.testProductUrl')}
        </label>
        <input
          id="ml-testProductUrl"
          type="text"
          placeholder={t('pages.affiliatePrograms.ml.testProductUrlPlaceholder')}
          value={testProductUrl}
          onChange={(e) => onTestProductUrlChange(e.target.value)}
          className={inputClasses}
        />
        <p className="text-xs" style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
          {t('pages.affiliatePrograms.ml.testProductUrlHint')}
        </p>

        {mlLastTest?.originUrl && (
          <MlTestResult result={mlLastTest} formatSoldQuantity={formatSoldQuantity} />
        )}
      </div>
    </>
  );
}
