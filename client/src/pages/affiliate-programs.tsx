import { ComingSoonCard, StoreCard } from '@/components/affiliate-programs';
import { ErrorBoundary } from '@/components/error-boundary';
import { useAffiliatePrograms } from '@/hooks';
import { useToast } from '@/hooks/use-toast';
import { Store } from 'lucide-react';
import { useTranslation } from 'react-i18next';

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
      <div className="flex flex-col w-full h-full">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8"
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
      <div className="flex flex-col w-full h-full">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8">
            <div className="flex items-center gap-3 xs:gap-4">
              <div className="p-2 xs:p-2.5 rounded-xl" style={{ backgroundColor: colors.iconBg }}>
                <Store
                  className="w-5 h-5 xs:w-6 xs:h-6"
                  style={{ color: colors.wine }}
                  aria-hidden="true"
                />
              </div>
              <div>
                <h1
                  className="text-xl xs:text-2xl md:text-3xl font-semibold tracking-tight"
                  style={{ color: '#18181b' }}
                >
                  {t('pages.affiliatePrograms.title')}
                </h1>
                <p
                  className="text-xs xs:text-sm md:text-base"
                  style={{ color: colors.wineTextMuted }}
                >
                  {t('pages.affiliatePrograms.subtitle')}
                </p>
              </div>
            </div>

            <div className="grid gap-6 md:grid-cols-2">
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
                    onFieldChange={(field, value) => updateCredentialField(config.key, field, value)}
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
        </div>
      </div>
    </ErrorBoundary>
  );
}
