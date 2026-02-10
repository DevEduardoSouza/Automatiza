import { Alert, AlertDescription } from '@/components/ui/alert';
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

  return (
    <div className="space-y-2">
      <div className="flex items-start gap-2">
        <Label>{t('pages.offerAutomations.form.stores')} *</Label>
      </div>
      {formData.mode === 'search' && activeStores.length < STORES.length && (
        <Alert>
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>
            {t('pages.offerAutomations.storesInactive')}{' '}
            <Link to="/affiliate-programs" className="font-semibold underline hover:text-primary">
              {t('sidebar.affiliatePrograms')}
            </Link>
          </AlertDescription>
        </Alert>
      )}

      {formData.mode === 'monitor' && (
        <Alert>
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>
            {t('pages.offerAutomations.form.monitorModeStoreHint')}
          </AlertDescription>
        </Alert>
      )}
      <Card>
        <CardContent className="pt-4 grid grid-cols-2 gap-3">
          {STORES.map((store) => {
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
