import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { ChipSelector } from '@/components/whatsapp/chip-selector';
import type { RefinementState } from '@/hooks/use-automations';
import type { CTA } from '@/entities/cta';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import type {
  Automation,
  AutomationCoupon,
  AutomationGroup,
  DispatchText,
  MlLinkList,
  StoreKey,
} from '@/types/automation';
import { AlertCircle, Info, Plus, Save } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  CTASelector,
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
  availableCTAs: CTA[];
  dispatchTexts: DispatchText[];
  mlLinkLists: MlLinkList[];
  loadingMlLinkLists: boolean;
  monitorDelayUnit: 'seconds' | 'minutes';
  monitorDelayInput: string;
  setMonitorDelayUnit: (unit: 'seconds' | 'minutes') => void;
  setMonitorDelayInput: (value: string) => void;
  refinementOpen: RefinementState;
  setRefinementOpen: React.Dispatch<React.SetStateAction<RefinementState>>;
  saving?: boolean;
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
  availableCTAs,
  dispatchTexts,
  mlLinkLists,
  loadingMlLinkLists,
  monitorDelayUnit,
  monitorDelayInput,
  setMonitorDelayUnit,
  setMonitorDelayInput,
  refinementOpen,
  setRefinementOpen,
  saving,
  onSave,
  onReset,
  onToggleFavorite,
  onLimitReachedToast,
}: AutomationFormDialogProps) {
  const { t } = useTranslation();
  const { instances } = useInstancesConnect();

  // Pick which chip this automation runs on. State lives in the dialog so
  // it resets every open/close cycle. Default-chip resolution:
  //  - on CREATE: the user's is_default instance (or the first instance)
  //  - on EDIT:   the chip that owns the first existing send_group, so the
  //               user sees their saved selection right away
  // Switching chips clears send_groups/monitor_groups (selections from the
  // old chip wouldn't match the new chip's groups anyway).
  const [selectedChipId, setSelectedChipId] = useState<string | null>(null);

  // biome-ignore lint/correctness/useExhaustiveDependencies: only re-resolve on open or initial editing target
  useEffect(() => {
    if (!open) return;
    if (instances.length === 0) {
      setSelectedChipId(null);
      return;
    }
    const firstSendGroup = formData.send_groups?.[0];
    const fromExisting = firstSendGroup
      ? groups.find((g) => g.wa_group_id === firstSendGroup)?.instance_id
      : undefined;
    if (fromExisting) {
      setSelectedChipId(fromExisting);
      return;
    }
    const defaultInstance = instances.find((i) => i.is_default) ?? instances[0];
    setSelectedChipId(defaultInstance?.id ?? null);
  }, [open, editingId]);

  // Groups/allGroups filtered to the selected chip — what the user actually
  // sees in the SendGroupsSelector / MonitorGroupsSelector.
  //
  // Match against participating_instance_ids (every chip whose sync has
  // seen the group) instead of just instance_id (the single owner).
  // Without this, a group both chips are admin in only shows under the
  // owning chip, so the user can't pick it from the other chip's view.
  // instance_id stays as the fallback when the array is empty (legacy
  // rows from before migration 044).
  const matchesChip = (g: AutomationGroup, chipId: string): boolean => {
    const participants = g.participating_instance_ids;
    if (participants && participants.length > 0) {
      return participants.includes(chipId);
    }
    return g.instance_id === chipId;
  };

  const filteredGroups = useMemo(() => {
    if (!selectedChipId) return groups;
    return groups.filter((g) => matchesChip(g, selectedChipId));
  }, [groups, selectedChipId]);

  const filteredAllGroups = useMemo(() => {
    if (!selectedChipId) return allGroups;
    return allGroups.filter((g) => matchesChip(g, selectedChipId));
  }, [allGroups, selectedChipId]);

  const handleChipChange = (id: string | null) => {
    if (id === selectedChipId) return;
    setSelectedChipId(id);
    // Clear selections so the user does not accidentally save references
    // to groups owned by a chip they are no longer targeting.
    setFormData((prev) => ({
      ...prev,
      send_groups: [],
      monitor_groups: [],
    }));
  };

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
            <span className="w-full xs:w-auto">
              <DialogTrigger asChild>
                <button
                  type="button"
                  onClick={onReset}
                  disabled={limitReached}
                  className="auth-btn-glow relative w-full xs:w-auto xs:flex-1 md:flex-none inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
                    <div className="auth-btn-glow__glow absolute inset-0" />
                  </div>
                  <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
                    <Plus className="h-4 w-4" />
                    {t('pages.offerAutomations.newAutomation')}
                  </span>
                </button>
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
      <DialogContent className="automation-form-dialog max-w-[calc(100vw-2.5rem)] xs:max-w-[calc(100vw-3rem)] sm:max-w-[calc(100vw-4rem)] md:max-w-3xl xl:max-w-4xl 2xl:max-w-5xl max-h-[85vh] xs:max-h-[90vh] overflow-y-auto p-4 xs:p-5 sm:p-6 rounded-xl xs:rounded-2xl">
        <DialogHeader>
          <DialogTitle className="text-base xs:text-lg md:text-xl">
            {editingId
              ? t('pages.offerAutomations.editTitle')
              : t('pages.offerAutomations.createTitle')}
          </DialogTitle>
          <DialogDescription className="text-xs xs:text-sm">
            {editingId
              ? t('pages.offerAutomations.dialog.editDescription')
              : t('pages.offerAutomations.dialog.createDescription')}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 xs:space-y-4">
          {/* Mode */}
          <ModeSection formData={formData} setFormData={setFormData} />

          {/* Name */}
          <div className="space-y-1.5 xs:space-y-2">
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

          {/* Delay (monitor mode) */}
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

          {/* Chip selector — picks which WhatsApp number this automation
              uses. Filters the group lists below to that chip's groups. */}
          {instances.length > 0 && (
            <div className="space-y-1.5 xs:space-y-2">
              <Label>
                {t('pages.offerAutomations.form.chip', 'Número')} *{' '}
                <span className="text-xs text-muted-foreground">
                  {t(
                    'pages.offerAutomations.form.chipHint',
                    'Os grupos abaixo são deste número. Trocar limpa a seleção.'
                  )}
                </span>
              </Label>
              <ChipSelector
                value={selectedChipId}
                onChange={handleChipChange}
                instances={instances}
              />
            </div>
          )}

          {/* Send Groups */}
          <SendGroupsSelector
            formData={formData}
            setFormData={setFormData}
            groups={filteredGroups}
            onToggleFavorite={onToggleFavorite}
          />

          {/* Monitor Groups (monitor mode) */}
          {formData.mode === 'monitor' && (
            <MonitorGroupsSelector
              formData={formData}
              setFormData={setFormData}
              allGroups={filteredAllGroups}
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
            <div
              className="rounded-xl border px-4 py-3 text-sm"
              style={{
                backgroundColor: 'rgba(141, 45, 83, 0.06)',
                borderColor: 'rgba(141, 45, 83, 0.15)',
              }}
            >
              <div className="flex items-start gap-3">
                <Info
                  className="h-4 w-4 mt-0.5 shrink-0"
                  style={{ color: 'rgba(141, 45, 83, 0.6)' }}
                />
                <p style={{ color: 'rgba(141, 45, 83, 0.8)' }}>
                  {t('pages.offerAutomations.form.listModeStoreInfo')}
                </p>
              </div>
            </div>
          )}

          {/* Coupon Selection (list mode) */}
          {formData.mode === 'list' && (
            <CouponSelector
              formData={formData}
              setFormData={setFormData}
              availableCoupons={availableCoupons}
            />
          )}

          {/* CTA Selection (search + list mode) */}
          {(formData.mode === 'search' || formData.mode === 'list') && (
            <CTASelector
              formData={formData}
              setFormData={setFormData}
              availableCTAs={availableCTAs}
            />
          )}

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
            <div
              className="rounded-xl border px-4 py-3 text-sm"
              style={{
                backgroundColor: 'rgba(141, 45, 83, 0.06)',
                borderColor: 'rgba(141, 45, 83, 0.15)',
              }}
            >
              <div className="flex items-start gap-3">
                <Info
                  className="h-4 w-4 mt-0.5 shrink-0"
                  style={{ color: 'rgba(141, 45, 83, 0.6)' }}
                />
                <p style={{ color: 'rgba(141, 45, 83, 0.8)' }}>
                  {t('pages.offerAutomations.form.dispatchTextActive')}
                </p>
              </div>
            </div>
          )}

          {/* Monitoring: Text Processing */}
          {formData.mode === 'monitor' && (
            <MonitoringTextProcessing formData={formData} setFormData={setFormData} />
          )}

          {formData.mode === 'search' && (
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
                  {t('pages.offerAutomations.form.affiliateCredentialsHint')}
                </p>
              </div>
            </div>
          )}

          <div className="flex flex-col xs:flex-row justify-end gap-2 lg:gap-3">
            <button
              type="button"
              onClick={() => onOpenChange(false)}
              className="inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
            >
              {t('common.cancel')}
            </button>
            <button
              type="button"
              onClick={onSave}
              disabled={saving}
              className="auth-btn-glow relative inline-flex w-full xs:w-auto cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
            >
              <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
                <div className="auth-btn-glow__glow absolute inset-0" />
              </div>
              <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
                <Save className="h-3.5 w-3.5 xs:h-4 xs:w-4" />
                {editingId ? t('common.save') : t('pages.offerAutomations.form.createAndActivate')}
              </span>
            </button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
