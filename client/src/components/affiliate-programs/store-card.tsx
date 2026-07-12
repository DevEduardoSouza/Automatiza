import { Switch } from '@/components/ui/switch';
import type { AffiliateCredential, MlLastTest, StoreConfig } from '@/hooks/use-affiliate-programs';
import { AlertCircle, CheckCircle2, CreditCard, Loader2 } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AwinBrandsSelector } from './awin-brands-selector';
import { MlConnectionForm } from './ml-connection-form';

function EyeIcon({ className }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" color="currentColor" fill="none" stroke="currentColor" strokeWidth="1.5" className={className}>
      <path d="M21.544 11.045C21.848 11.4713 22 11.6845 22 12C22 12.3155 21.848 12.5287 21.544 12.955C20.1779 14.8706 16.6892 19 12 19C7.31078 19 3.8221 14.8706 2.45604 12.955C2.15201 12.5287 2 12.3155 2 12C2 11.6845 2.15201 11.4713 2.45604 11.045C3.8221 9.12944 7.31078 5 12 5C16.6892 5 20.1779 9.12944 21.544 11.045Z" />
      <path d="M15 12C15 10.3431 13.6569 9 12 9C10.3431 9 9 10.3431 9 12C9 13.6569 10.3431 15 12 15C13.6569 15 15 13.6569 15 12Z" />
    </svg>
  );
}

function EyeOffIcon({ className }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" color="currentColor" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" className={className}>
      <path d="M22 8C22 8 18 14 12 14C6 14 2 8 2 8" />
      <path d="M15 13.5L16.5 16" />
      <path d="M20 11L22 13" />
      <path d="M2 13L4 11" />
      <path d="M9 13.5L7.5 16" />
    </svg>
  );
}

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

const inputClasses =
  'w-full rounded-2xl border-2 border-gray-300 bg-gray-50 px-4 py-2 text-xs xs:py-2.5 xs:text-sm text-gray-900 placeholder-gray-400 outline-none transition-all hover:border-[#520B27] focus:border-[#520B27] focus:ring-1 focus:ring-[#3a0718]/30';

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
  const [visiblePasswords, setVisiblePasswords] = useState<Record<string, boolean>>({});

  const togglePasswordVisibility = (fieldKey: string) => {
    setVisiblePasswords((prev) => ({ ...prev, [fieldKey]: !prev[fieldKey] }));
  };

  const allFieldsFilled =
    config.key === 'ml'
      ? Boolean(credential?.is_active)
      : config.fields.every(
          (field) => field.optional || credential?.credentials?.[field.key]?.trim()
        );

  const isMl = config.key === 'ml';
  const isAwin = config.key === 'awin';
  const isShein = config.key === 'shein';

  return (
    <div
      className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
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
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between">
            <span className="text-base lg:text-lg font-semibold" style={{ color: '#18181b' }}>
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
          <p className="text-xs xs:text-sm tracking-tight" style={{ color: colors.wineTextMuted }}>
            {description}
          </p>
        </div>
      </div>

      <div className="h-px mb-4" style={{ backgroundColor: colors.wineBorder }} />

      <div className="space-y-4 lg:space-y-5">
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
          config.fields.map((field) => {
            const isPassword = field.type === 'password';
            const isVisible = visiblePasswords[field.key];

            return (
              <div key={field.key} className="space-y-2">
                <label
                  htmlFor={`${config.key}-${field.key}`}
                  className="mb-1.5 block text-xs lg:text-sm font-medium pl-2.5 text-gray-700"
                >
                  {field.label}
                </label>
                <div className="relative">
                  <input
                    id={`${config.key}-${field.key}`}
                    type={isPassword && !isVisible ? 'password' : 'text'}
                    placeholder={field.placeholder}
                    value={credential?.credentials?.[field.key] || ''}
                    onChange={(e) => onFieldChange(field.key, e.target.value)}
                    // autofill do navegador preenchia a senha SALVA DO SITE
                    // neste campo e um "Salvar" persistia o valor errado por
                    // cima do secret válido da loja (corrupção silenciosa de
                    // credencial — incidente 11/07). new-password desativa.
                    autoComplete={isPassword ? 'new-password' : 'off'}
                    data-lpignore="true"
                    data-1p-ignore="true"
                    className={`${inputClasses}${isPassword ? ' pr-10' : ''}`}
                  />
                  {isPassword && (
                    <button
                      type="button"
                      onClick={() => togglePasswordVisibility(field.key)}
                      className="absolute inset-y-0 right-0 flex cursor-pointer items-center pr-3 text-gray-400 transition hover:text-[#520B27]"
                      aria-label={
                        isVisible
                          ? t('pages.auth.form.hidePassword')
                          : t('pages.auth.form.showPassword')
                      }
                    >
                      {isVisible ? (
                        <EyeOffIcon className="h-4 w-4" />
                      ) : (
                        <EyeIcon className="h-4 w-4" />
                      )}
                    </button>
                  )}
                </div>
                {isShein && field.key === 'affiliateLink' && (
                  <p className="text-xs" style={{ color: colors.wineTextMuted }}>
                    {t('pages.affiliatePrograms.shein.linkHint')}
                  </p>
                )}
              </div>
            );
          })}

        {isAwin && allFieldsFilled && (
          <AwinBrandsSelector
            brands={awinBrands}
            selectedBrands={credential?.selected_brands || []}
            onToggleBrand={onToggleBrand}
          />
        )}

        <div
          className="flex items-center justify-between pt-2 lg:pt-3 border-t"
          style={{ borderColor: 'rgba(141, 45, 83, 0.2)' }}
        >
          <div className="flex items-center space-x-2">
            <Switch
              id={`${config.key}-auto`}
              checked={credential?.auto_generate || false}
              onCheckedChange={onToggleAutoGenerate}
              disabled={!allFieldsFilled}
            />
            <label
              htmlFor={`${config.key}-auto`}
              className="text-xs lg:text-sm font-medium text-gray-700 cursor-pointer"
            >
              {t('pages.affiliatePrograms.form.autoGenerate')}
            </label>
          </div>
        </div>

        <div className="flex flex-col xs:flex-row gap-2 xs:gap-3 pt-2 lg:pt-3">
          <button
            type="button"
            onClick={onSave}
            disabled={isSaving}
            className="auth-btn-glow inline-flex flex-1 cursor-pointer items-center justify-center gap-2 rounded-full px-5 py-2.5 text-xs xs:text-sm font-semibold text-white shadow-lg transition-all hover:-translate-y-0.5 hover:shadow-xl disabled:pointer-events-none disabled:opacity-50"
            style={{ backgroundColor: '#520B27' }}
          >
            {isSaving && <Loader2 className="h-4 w-4 animate-spin" />}
            {t('pages.affiliatePrograms.form.save')}
          </button>
          <button
            type="button"
            onClick={onTest}
            disabled={isTesting || !allFieldsFilled}
            className="inline-flex cursor-pointer items-center justify-center gap-2 rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md disabled:pointer-events-none disabled:opacity-50"
          >
            {isTesting && <Loader2 className="h-4 w-4 animate-spin" />}
            {t('pages.affiliatePrograms.form.test')}
          </button>
        </div>
      </div>
    </div>
  );
}
