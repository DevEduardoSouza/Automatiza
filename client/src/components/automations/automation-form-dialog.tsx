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

import { Switch } from '@/components/ui/switch';
import {
  CTASelector,
  CategoriesSection,
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

  // Multi-chip preference (049): the automation can fan out across N chips.
  // State lives in the dialog so it resets every open/close cycle.
  // Resolution order:
  //   - on CREATE: user's is_default chip (or first connected) as a 1-item list
  //   - on EDIT:   formData.instance_ids if set; else legacy instance_id; else
  //                inferred from the first existing send_group's owner
  // Switching chips clears send_groups/monitor_groups (selections from the
  // old chip set may not match the new union of participating groups).
  const [selectedChipIds, setSelectedChipIds] = useState<string[]>([]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: only re-resolve on open or initial editing target
  useEffect(() => {
    if (!open) return;
    if (instances.length === 0) {
      setSelectedChipIds([]);
      return;
    }
    // Prefer the saved multi-chip list (049). Fall back to legacy single
    // instance_id, then to the inferred chip from the first send group.
    if (formData.instance_ids && formData.instance_ids.length > 0) {
      setSelectedChipIds(formData.instance_ids);
      return;
    }
    if (formData.instance_id) {
      setSelectedChipIds([formData.instance_id]);
      return;
    }
    const firstSendGroup = formData.send_groups?.[0];
    const fromExisting = firstSendGroup
      ? groups.find((g) => g.wa_group_id === firstSendGroup)?.instance_id
      : undefined;
    if (fromExisting) {
      setSelectedChipIds([fromExisting]);
      return;
    }
    const defaultInstance = instances.find((i) => i.is_default) ?? instances[0];
    setSelectedChipIds(defaultInstance?.id ? [defaultInstance.id] : []);
  }, [open, editingId]);

  // Groups/allGroups filtered to the union of selected chips — what the
  // user actually sees in the SendGroupsSelector / MonitorGroupsSelector.
  //
  // A group is visible when AT LEAST ONE of the selected chips participates
  // (or owns it as fallback for pre-044 rows). Empty selection = show all,
  // matching the "no chip filter" behavior of the legacy single-chip mode.
  const matchesAnyChip = (g: AutomationGroup, chipIds: string[]): boolean => {
    if (chipIds.length === 0) return true;
    const participants = g.participating_instance_ids;
    if (participants && participants.length > 0) {
      return chipIds.some((id) => participants.includes(id));
    }
    return g.instance_id ? chipIds.includes(g.instance_id) : false;
  };

  const filteredGroups = useMemo(() => {
    if (selectedChipIds.length === 0) return groups;
    return groups.filter((g) => matchesAnyChip(g, selectedChipIds));
  }, [groups, selectedChipIds]);

  const filteredAllGroups = useMemo(() => {
    if (selectedChipIds.length === 0) return allGroups;
    return allGroups.filter((g) => matchesAnyChip(g, selectedChipIds));
  }, [allGroups, selectedChipIds]);

  const handleChipToggle = (chipId: string) => {
    setSelectedChipIds((prev) => {
      const next = prev.includes(chipId)
        ? prev.filter((id) => id !== chipId)
        : [...prev, chipId];
      // Persist into formData. Sync the legacy instance_id with the first
      // selected chip so older callers/migrations stay consistent.
      // Empty list = "no preference" → repo writes NULL.
      setFormData((current) => ({
        ...current,
        send_groups: [],
        monitor_groups: [],
        instance_ids: next,
        instance_id: next[0] ?? null,
      }));
      return next;
    });
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

          {/* Multi-chip preference (049). Marca os números pelos quais o
              dispatch da automação vai disparar — pra fan-out multi-chip
              em offer_generate, o worker chama a API 1x e dispara via cada
              chip selecionado nos grupos onde ele participa. Em search/
              monitor/list o dispatch atual usa o primeiro chip da lista
              (comportamento single-chip preservado). */}
          {instances.length > 0 && (
            <div className="space-y-1.5 xs:space-y-2">
              <Label>
                {t('pages.offerAutomations.form.chips', 'Números')} *{' '}
                <span className="text-xs text-muted-foreground">
                  {t(
                    'pages.offerAutomations.form.chipsHint',
                    'Marque um ou mais números. Os grupos abaixo são a união dos selecionados.'
                  )}
                </span>
              </Label>
              <div className="grid grid-cols-1 xs:grid-cols-2 gap-2">
                {instances.map((inst) => {
                  const checked = selectedChipIds.includes(inst.id);
                  const label =
                    inst.friendly_name ||
                    (inst.phone_number ? `+${inst.phone_number}` : inst.instance_id);
                  return (
                    <label
                      key={inst.id}
                      className={`flex items-center gap-2 rounded-lg border px-3 py-2 cursor-pointer text-sm transition-colors ${
                        checked
                          ? 'border-[#8D2D53] bg-[rgba(141,45,83,0.08)]'
                          : 'border-input hover:bg-muted/40'
                      }`}
                    >
                      <input
                        type="checkbox"
                        className="h-4 w-4 accent-[#8D2D53]"
                        checked={checked}
                        onChange={() => handleChipToggle(inst.id)}
                      />
                      <span className="flex-1 truncate">{label}</span>
                      {inst.is_default && (
                        <span className="text-[10px] uppercase tracking-wide text-muted-foreground">
                          padrão
                        </span>
                      )}
                    </label>
                  );
                })}
              </div>
              {selectedChipIds.length === 0 && (
                <p className="text-xs text-amber-600">
                  {t(
                    'pages.offerAutomations.form.chipsRequired',
                    'Selecione pelo menos 1 número.'
                  )}
                </p>
              )}
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

          {/* Coupon rule engine opt-out — substitui o seletor de cupom
              fixo retirado na migration 046. Quando ligado (default), o
              worker avalia as regras de Cupons Avançados em cada disparo
              dessa automação. Desligar é útil pra automações de modo
              busca cujos produtos já vêm com promoção da Shopee. */}
          <div className="flex items-start justify-between gap-4 rounded-lg border p-4">
            <div className="space-y-0.5">
              <div className="text-sm font-medium">Aplicar cupons automáticos</div>
              <p className="text-xs text-muted-foreground">
                Quando ligado, a automação consulta as regras de "Cupons Avançados" e
                injeta o cupom que casar com o produto. Desligue se essa automação
                envia produtos que já estão em promoção/cupom da própria loja.
              </p>
            </div>
            <Switch
              // Default OFF for every new automation in this form (search,
              // monitor, list). Operators here usually want full control
              // over the message and pull products from sources that may
              // already carry promo codes — opt-in to coupon injection.
              // The /offer-generate config page keeps its own default ON
              // since that flow is event-driven (chat → bot rewrite).
              checked={formData.apply_coupon_rules ?? false}
              onCheckedChange={(checked) =>
                setFormData((prev) => ({ ...prev, apply_coupon_rules: checked }))
              }
              aria-label="Aplicar cupons automáticos"
            />
          </div>

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
