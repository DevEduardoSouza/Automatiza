import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import type { RefinementState } from '@/hooks/use-automations';
import type {
  Automation,
  AutomationCoupon,
  AutomationGroup,
  DispatchText,
  MlLinkList,
  StoreKey,
} from '@/types/automation';
import { AlertCircle, Info, Plus } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import {
  CategoriesSection,
  CouponSelector,
  DispatchTextSelector,
  FiltersSection,
  IntervalSelector,
  ListModeConfig,
  ModeSection,
  MonitorDelayInput,
  MonitorGroupsSelector,
  MonitoringTextProcessing,
  PriorityFilterSection,
  SendGroupsSelector,
  StoresSelector,
  TimeWindowInputs,
} from './form-sections';

interface AutomationFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editingId: string | null;
  limitReached: boolean;
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  groups: AutomationGroup[];
  allGroups: AutomationGroup[];
  activeStores: StoreKey[];
  availableCoupons: AutomationCoupon[];
  dispatchTexts: DispatchText[];
  mlLinkLists: MlLinkList[];
  loadingMlLinkLists: boolean;
  monitorDelayUnit: 'seconds' | 'minutes';
  monitorDelayInput: string;
  setMonitorDelayUnit: (unit: 'seconds' | 'minutes') => void;
  setMonitorDelayInput: (value: string) => void;
  refinementOpen: RefinementState;
  setRefinementOpen: React.Dispatch<React.SetStateAction<RefinementState>>;
  onSave: () => Promise<void>;
  onReset: () => void;
  onToggleFavorite: (groupId: string) => Promise<void>;
  onLimitReachedToast: () => void;
}

export function AutomationFormDialog({
  open,
  onOpenChange,
  editingId,
  limitReached,
  formData,
  setFormData,
  groups,
  allGroups,
  activeStores,
  availableCoupons,
  dispatchTexts,
  mlLinkLists,
  loadingMlLinkLists,
  monitorDelayUnit,
  monitorDelayInput,
  setMonitorDelayUnit,
  setMonitorDelayInput,
  refinementOpen,
  setRefinementOpen,
  onSave,
  onReset,
  onToggleFavorite,
  onLimitReachedToast,
}: AutomationFormDialogProps) {
  const { t } = useTranslation();

  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen && limitReached && !editingId) {
      onLimitReachedToast();
      return;
    }
    onOpenChange(nextOpen);
  };

  const closeAllRefinements = () => {
    setRefinementOpen({
      eletronicos: false,
      eletrodomesticos: false,
      homens: false,
      esportes: false,
      moda: false,
      casa: false,
      beleza: false,
    });
  };

  const toggleRefinement = (key: keyof RefinementState) => {
    setRefinementOpen((prev) => {
      const isOpening = !prev[key];
      if (isOpening) {
        return {
          eletronicos: false,
          eletrodomesticos: false,
          homens: false,
          esportes: false,
          moda: false,
          casa: false,
          beleza: false,
          [key]: true,
        };
      }
      return { ...prev, [key]: false };
    });
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="w-full md:w-auto">
              <DialogTrigger asChild>
                <Button onClick={onReset} className="w-full md:w-auto" disabled={limitReached}>
                  <Plus className="h-4 w-4 mr-2" />
                  {t('pages.offerAutomations.newAutomation')}
                </Button>
              </DialogTrigger>
            </span>
          </TooltipTrigger>
          {limitReached && (
            <TooltipContent>
              <p>{t('pages.offerAutomations.limits.automationsReached')}</p>
            </TooltipContent>
          )}
        </Tooltip>
      </TooltipProvider>
      <DialogContent className="automation-form-dialog max-w-3xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {editingId
              ? t('pages.offerAutomations.editTitle')
              : t('pages.offerAutomations.createTitle')}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* Mode */}
          <ModeSection formData={formData} setFormData={setFormData} />

          {/* Name */}
          <div className="space-y-2">
            <Label>{t('pages.offerAutomations.form.name')} *</Label>
            <Input
              value={formData.name || ''}
              onChange={(e) => setFormData((prev) => ({ ...prev, name: e.target.value }))}
              placeholder={t('pages.offerAutomations.form.namePlaceholder')}
            />
          </div>

          {/* Time Window */}
          <TimeWindowInputs formData={formData} setFormData={setFormData} />

          {/* Interval (search + list mode) */}
          {(formData.mode === 'search' || formData.mode === 'list') && (
            <IntervalSelector formData={formData} setFormData={setFormData} />
          )}

          {/* Delay (only for monitor mode) */}
          {formData.mode === 'monitor' && (
            <MonitorDelayInput
              monitorDelayUnit={monitorDelayUnit}
              monitorDelayInput={monitorDelayInput}
              setMonitorDelayUnit={setMonitorDelayUnit}
              setMonitorDelayInput={setMonitorDelayInput}
              setFormData={setFormData}
            />
          )}

          {/* List config (only for list mode) */}
          {formData.mode === 'list' && (
            <ListModeConfig
              formData={formData}
              setFormData={setFormData}
              mlLinkLists={mlLinkLists}
              loadingMlLinkLists={loadingMlLinkLists}
            />
          )}

          {/* Send Groups */}
          <SendGroupsSelector
            formData={formData}
            setFormData={setFormData}
            groups={groups}
            onToggleFavorite={onToggleFavorite}
          />

          {/* Monitor Groups (only for monitor mode) */}
          {formData.mode === 'monitor' && (
            <MonitorGroupsSelector
              formData={formData}
              setFormData={setFormData}
              allGroups={allGroups}
              onToggleFavorite={onToggleFavorite}
            />
          )}

          {/* Stores */}
          {formData.mode !== 'list' && (
            <StoresSelector
              formData={formData}
              setFormData={setFormData}
              activeStores={activeStores}
            />
          )}

          {formData.mode === 'list' && (
            <Alert>
              <Info className="h-4 w-4" />
              <AlertDescription>
                {t('pages.offerAutomations.form.listModeStoreInfo')}
              </AlertDescription>
            </Alert>
          )}

          {/* Coupon Selection */}
          <CouponSelector
            formData={formData}
            setFormData={setFormData}
            availableCoupons={availableCoupons}
          />

          {/* Categories (only for search mode) */}
          {formData.mode === 'search' && (
            <CategoriesSection
              formData={formData}
              setFormData={setFormData}
              refinementOpen={refinementOpen}
              toggleRefinement={toggleRefinement}
              closeAllRefinements={closeAllRefinements}
            />
          )}

          {/* Priority & Filter Type (only for search mode) */}
          {formData.mode === 'search' && (
            <PriorityFilterSection formData={formData} setFormData={setFormData} />
          )}

          {/* Filters (only for search mode) */}
          {formData.mode === 'search' && (
            <FiltersSection formData={formData} setFormData={setFormData} />
          )}

          {/* Dispatch Text */}
          {(formData.mode === 'search' || formData.mode === 'list') && (
            <DispatchTextSelector
              formData={formData}
              setFormData={setFormData}
              dispatchTexts={dispatchTexts}
            />
          )}

          {formData.mode === 'search' && formData.dispatch_text_id && (
            <Alert>
              <AlertDescription>
                {t('pages.offerAutomations.form.dispatchTextActive')}
              </AlertDescription>
            </Alert>
          )}

          {/* Monitoring: Text Processing */}
          {formData.mode === 'monitor' && (
            <MonitoringTextProcessing formData={formData} setFormData={setFormData} />
          )}

          {formData.mode === 'search' && (
            <Alert>
              <AlertCircle className="h-4 w-4" />
              <AlertDescription>
                {t('pages.offerAutomations.form.affiliateCredentialsHint')}
              </AlertDescription>
            </Alert>
          )}

          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              {t('common.cancel')}
            </Button>
            <Button onClick={onSave}>
              {editingId ? t('common.save') : t('pages.offerAutomations.form.createAndActivate')}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
