import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import type { AffiliateCredential, MlLastTest, StoreConfig } from '@/hooks/use-affiliate-programs';
import { AlertCircle, CheckCircle2, Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { AwinBrandsSelector } from './awin-brands-selector';
import { MlConnectionForm } from './ml-connection-form';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

interface StoreCardProps {
  config: StoreConfig;
  credential: AffiliateCredential;
  description: string;
  isSaving: boolean;
  isTesting: boolean;
  mlLastTest: MlLastTest | null;
  awinBrands: string[];
  onFieldChange: (field: string, value: string) => void;
  onToggleAutoGenerate: () => void;
  onToggleBrand: (brand: string) => void;
  onSave: () => void;
  onTest: () => void;
  onValidateMlToken: () => void;
  formatSoldQuantity: (q: number | null | undefined) => string | null;
}

/**
 * Card component for configuring a single affiliate store.
 * Handles generic fields, ML-specific form, Awin brand selection, and save/test actions.
 */
export function StoreCard({
  config,
  credential,
  description,
  isSaving,
  isTesting,
  mlLastTest,
  awinBrands,
  onFieldChange,
  onToggleAutoGenerate,
  onToggleBrand,
  onSave,
  onTest,
  onValidateMlToken,
  formatSoldQuantity,
}: StoreCardProps) {
  const { t } = useTranslation();

  const allFieldsFilled =
    config.key === 'ml'
      ? Boolean(credential?.is_active)
      : config.fields.every((field) => credential?.credentials?.[field.key]?.trim());

  const isMl = config.key === 'ml';
  const isAwin = config.key === 'awin';
  const isShein = config.key === 'shein';

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
                {config.name}
              </span>
              {credential?.is_active ? (
                <span
                  className="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium"
                  style={{ backgroundColor: colors.iconBg, color: colors.wine }}
                >
                  <CheckCircle2 className="h-3 w-3" />
                  {t('pages.affiliatePrograms.status.active')}
                </span>
              ) : (
                <span
                  className="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium"
                  style={{ backgroundColor: 'rgba(141, 45, 83, 0.06)', color: colors.wineTextLight }}
                >
                  <AlertCircle className="h-3 w-3" />
                  {t('pages.affiliatePrograms.status.incomplete')}
                </span>
              )}
            </div>
            <p className="mt-1 text-sm" style={{ color: colors.wineTextMuted }}>
              {description}
            </p>
          </div>
        </div>
      </div>
      <div className="space-y-4">
        {isMl && (
          <MlConnectionForm
            connectionToken={credential?.credentials?.connectionToken || ''}
            tag={credential?.credentials?.tag || ''}
            testProductUrl={credential?.credentials?.testProductUrl || ''}
            mlLastTest={mlLastTest}
            onConnectionTokenChange={(v) => onFieldChange('connectionToken', v)}
            onTagChange={(v) => onFieldChange('tag', v)}
            onTestProductUrlChange={(v) => onFieldChange('testProductUrl', v)}
            onValidateToken={onValidateMlToken}
            formatSoldQuantity={formatSoldQuantity}
          />
        )}

        {!isMl &&
          config.fields.map((field) => (
            <div key={field.key} className="space-y-2">
              <Label htmlFor={`${config.key}-${field.key}`}>{field.label}</Label>
              <Input
                id={`${config.key}-${field.key}`}
                type={field.type}
                placeholder={field.placeholder}
                value={credential?.credentials?.[field.key] || ''}
                onChange={(e) => onFieldChange(field.key, e.target.value)}
              />
              {isShein && field.key === 'affiliateLink' && (
                <p className="text-xs" style={{ color: colors.wineTextMuted }}>
                  {t('pages.affiliatePrograms.shein.linkHint')}
                </p>
              )}
            </div>
          ))}

        {isAwin && allFieldsFilled && (
          <AwinBrandsSelector
            brands={awinBrands}
            selectedBrands={credential?.selected_brands || []}
            onToggleBrand={onToggleBrand}
          />
        )}

        <div className="flex items-center justify-between pt-2">
          <div className="flex items-center space-x-2">
            <Switch
              id={`${config.key}-auto`}
              checked={credential?.auto_generate || false}
              onCheckedChange={onToggleAutoGenerate}
              disabled={!allFieldsFilled}
            />
            <Label htmlFor={`${config.key}-auto`} className="cursor-pointer">
              {t('pages.affiliatePrograms.form.autoGenerate')}
            </Label>
          </div>
        </div>

        <div className="flex gap-2 pt-2">
          <Button onClick={onSave} disabled={isSaving} className="flex-1">
            {isSaving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {t('pages.affiliatePrograms.form.save')}
          </Button>
          <Button variant="outline" onClick={onTest} disabled={isTesting || !allFieldsFilled}>
            {isTesting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {t('pages.affiliatePrograms.form.test')}
          </Button>
        </div>
      </div>
    </div>
  );
}
