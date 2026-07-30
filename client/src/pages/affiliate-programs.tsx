import { AffiliateHowItWorks, ComingSoonCard, StoreCard } from '@/components/affiliate-programs';
import { ErrorBoundary } from '@/components/error-boundary';
import { useAffiliatePrograms } from '@/hooks';
import { useToast } from '@/hooks/use-toast';
import { CreditCard } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const AffiliateIcon = ({
  className,
  style,
}: { className?: string; style?: React.CSSProperties }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    style={style}
  >
    <title>Affiliate Programs</title>
    <path d="M2 12C2 8.46252 2 6.69377 3.0528 5.5129C3.22119 5.32403 3.40678 5.14935 3.60746 4.99087C4.86213 4 6.74142 4 10.5 4H13.5C17.2586 4 19.1379 4 20.3925 4.99087C20.5932 5.14935 20.7788 5.32403 20.9472 5.5129C22 6.69377 22 8.46252 22 12C22 15.5375 22 17.3062 20.9472 18.4871C20.7788 18.676 20.5932 18.8506 20.3925 19.0091C19.1379 20 17.2586 20 13.5 20H10.5C6.74142 20 4.86213 20 3.60746 19.0091C3.40678 18.8506 3.22119 18.676 3.0528 18.4871C2 17.3062 2 15.5375 2 12Z" />
    <path d="M10 16H11.5" strokeMiterlimit="10" />
    <path d="M14.5 16L18 16" strokeMiterlimit="10" />
    <path d="M2 9H22" />
  </svg>
);

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
  iconBgHover: 'rgba(141, 45, 83, 0.15)',
};

/**
 * Affiliate Programs page.
 * Allows users to configure credentials for multiple affiliate platforms.
 * Uses useAffiliatePrograms hook for all state and logic.
 */
export default function AffiliatePrograms() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const {
    credentials,
    loading,
    saving,
    testing,
    mlLastTest,
    storeConfigs,
    enabledStores,
    awinBrands,
    updateCredentialField,
    toggleAutoGenerate,
    toggleBrand,
    handleSave,
    handleTest,
    decodeMlToken,
    formatSoldQuantity,
    getStoreDescription,
  } = useAffiliatePrograms();

  const handleValidateMlToken = (storeKey: string) => {
    const cred = credentials[storeKey as keyof typeof credentials];
    const decoded = decodeMlToken(String(cred?.credentials?.connectionToken || ''));
    if ('error' in decoded) {
      toast({
        title: t('pages.affiliatePrograms.ml.tokenInvalid'),
        description: decoded.error,
        variant: 'destructive',
      });
      return;
    }
    if (!String(cred?.credentials?.tag || '').trim()) {
      updateCredentialField('ml', 'tag', decoded.tag);
    }
    toast({
      title: t('pages.affiliatePrograms.ml.tokenValidated'),
      description: t('pages.affiliatePrograms.ml.tokenValidatedMessage'),
    });
  };

  if (loading) {
    return (
      <div className="flex flex-col w-full h-full 2xl:max-w-[1600px] 2xl:mx-auto">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl lg:rounded-3xl p-4 xs:p-5 sm:p-6 md:p-6 lg:p-8 xl:p-10"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="flex items-center justify-center py-16">
            <div
              className="h-8 w-8 animate-spin rounded-full border-4 border-t-transparent"
              style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
            />
          </div>
        </div>
      </div>
    );
  }

  return (
    <ErrorBoundary>
      <div className="flex flex-col w-full h-full 2xl:max-w-[1600px] 2xl:mx-auto">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl lg:rounded-3xl p-4 xs:p-5 sm:p-6 md:p-6 lg:p-8 xl:p-10"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8 lg:space-y-10">
            <div className="flex items-center gap-2 xs:gap-3">
              <div
                className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
              >
                <AffiliateIcon
                  className="w-5 h-5 xs:w-6 xs:h-6"
                  style={{ color: colors.wine }}
                  aria-hidden="true"
                />
              </div>
              <div>
                <h2
                  className="text-xl xs:text-2xl md:text-3xl xl:text-4xl font-semibold tracking-tight"
                  style={{ color: '#18181b' }}
                >
                  {t('pages.affiliatePrograms.title')}
                </h2>
                <p
                  className="text-xs xs:text-sm tracking-tight"
                  style={{ color: colors.wineTextMuted }}
                >
                  {t('pages.affiliatePrograms.subtitle')}
                </p>
              </div>
            </div>

            <div
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-3 xs:mb-4">
                <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
                  <CreditCard
                    className="w-4 h-4 xs:w-5 xs:h-5"
                    style={{ color: colors.wine }}
                    aria-hidden="true"
                  />
                </div>
                <div>
                  <span
                    className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em] font-semibold"
                    style={{ color: colors.wineTextLight }}
                  >
                    {t('pages.affiliatePrograms.sections.storesTitle')}
                  </span>
                  <p
                    className="text-xs xs:text-sm tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.affiliatePrograms.sections.storesDescription')}
                  </p>
                </div>
              </div>
              <div className="grid gap-4 xs:gap-5 sm:gap-6 lg:gap-8 md:grid-cols-2">
                {storeConfigs.map((config) => {
                  const isEnabled = enabledStores.includes(config.key);

                  if (!isEnabled) {
                    return <ComingSoonCard key={config.key} storeName={config.name} />;
                  }

                  return (
                    <StoreCard
                      key={config.key}
                      config={config}
                      credential={credentials[config.key]}
                      description={getStoreDescription(config.key)}
                      isSaving={saving === config.key}
                      isTesting={testing === config.key}
                      mlLastTest={config.key === 'ml' ? mlLastTest : null}
                      awinBrands={awinBrands}
                      onFieldChange={(field, value) =>
                        updateCredentialField(config.key, field, value)
                      }
                      onToggleAutoGenerate={() => toggleAutoGenerate(config.key)}
                      onToggleBrand={(brand) => toggleBrand(config.key, brand)}
                      onSave={() => handleSave(config.key)}
                      onTest={() => handleTest(config.key)}
                      onValidateMlToken={() => handleValidateMlToken(config.key)}
                      formatSoldQuantity={formatSoldQuantity}
                    />
                  );
                })}
              </div>
            </div>

            <AffiliateHowItWorks />
          </div>
        </div>
      </div>
    </ErrorBoundary>
  );
}
