import { Card, CardContent } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { STORES } from '@/config/automation-categories';
import { useToast } from '@/hooks/use-toast';
import type { Automation, StoreKey } from '@/types/automation';
import { AlertCircle } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';

interface StoresSelectorProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  activeStores: StoreKey[];
}

export function StoresSelector({ formData, setFormData, activeStores }: StoresSelectorProps) {
  const { t } = useTranslation();
  const { toast } = useToast();

  const visibleStores = STORES.filter(
    (store) => !(formData.mode === 'search' && store.value === 'ml'),
  );

  return (
    <div className="space-y-1.5 xs:space-y-2">
      <div className="flex items-start gap-2">
        <Label className="cursor-pointer">{t('pages.offerAutomations.form.stores')} *</Label>
      </div>
      {formData.mode === 'search' && activeStores.length < visibleStores.length && (
        <div
          className="rounded-xl border px-4 py-3 text-sm"
          style={{
            backgroundColor: 'rgba(141, 45, 83, 0.06)',
            borderColor: 'rgba(141, 45, 83, 0.15)',
          }}
        >
          <div className="flex items-start gap-3">
            <AlertCircle
              className="h-4 w-4 mt-0.5 shrink-0"
              style={{ color: 'rgba(141, 45, 83, 0.6)' }}
            />
            <p style={{ color: 'rgba(141, 45, 83, 0.8)' }}>
              {t('pages.offerAutomations.storesInactive')}{' '}
              <Link to="/affiliate-programs" className="font-semibold underline hover:text-primary">
                {t('sidebar.affiliatePrograms')}
              </Link>
            </p>
          </div>
        </div>
      )}

      {formData.mode === 'monitor' && (
        <div
          className="rounded-xl border px-4 py-3 text-sm"
          style={{
            backgroundColor: 'rgba(141, 45, 83, 0.06)',
            borderColor: 'rgba(141, 45, 83, 0.15)',
          }}
        >
          <div className="flex items-start gap-3">
            <AlertCircle
              className="h-4 w-4 mt-0.5 shrink-0"
              style={{ color: 'rgba(141, 45, 83, 0.6)' }}
            />
            <p style={{ color: 'rgba(141, 45, 83, 0.8)' }}>
              {t('pages.offerAutomations.form.monitorModeStoreHint')}
            </p>
          </div>
        </div>
      )}
      <Card>
        <CardContent className="pt-4 grid grid-cols-2 gap-3 lg:gap-4">
          {visibleStores.map((store) => {
            const isActive = activeStores.includes(store.value);
            const isMonitoring = formData.mode === 'monitor';
            const isShopee = store.value === 'shopee';
            const disabledByMode = isMonitoring && !isShopee;
            const isSelected = formData.stores?.includes(store.value) || false;
            const isLastSelectedStore = isSelected && (formData.stores?.length || 0) <= 1;
            return (
              <div key={store.value} className="space-y-1">
                <div className="flex items-center space-x-2">
                  <Checkbox
                    checked={isSelected}
                    disabled={!isActive || disabledByMode || isLastSelectedStore}
                    onCheckedChange={(checked) => {
                      setFormData((prev) => {
                        const current = prev.stores || [];
                        if (!checked && current.includes(store.value) && current.length <= 1) {
                          toast({
                            title: t('pages.offerAutomations.validation.storesRequired'),
                            description: t(
                              'pages.offerAutomations.validation.storesRequiredMessage'
                            ),
                            variant: 'destructive',
                          });
                          return prev;
                        }
                        const next = checked
                          ? [...current, store.value]
                          : current.filter((s) => s !== store.value);
                        return { ...prev, stores: next };
                      });
                    }}
                  />
                  <Label
                    className={`cursor-pointer ${
                      !isActive || disabledByMode ? 'text-muted-foreground' : ''
                    }`}
                  >
                    {store.label}
                  </Label>
                </div>
                {!isActive && formData.mode === 'search' && (
                  <p className="text-xs text-muted-foreground ml-6">
                    {t('pages.offerAutomations.form.configureCredentials')}
                  </p>
                )}
              </div>
            );
          })}
        </CardContent>
      </Card>
    </div>
  );
}
