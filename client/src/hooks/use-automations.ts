import {
  GENERAL_CATEGORY,
  clampIntervalMinutes,
  isBelezaSubcategoryKey,
  isCasaSubcategoryKey,
  isEletrodomesticosSubcategoryKey,
  isEletronicosSubcategoryKey,
  isEsportesSubcategoryKey,
  isHomensSubcategoryKey,
  isModaSubcategoryKey,
  translateShopeeAffiliateError,
} from '@/config/automation-categories';
import { useAuth } from '@/contexts';
import { useAffiliateCredentialsConnect } from '@/hooks/use-affiliate-credentials-connect';
import { useAiToolsConnect } from '@/hooks/use-ai-tools-connect';
import {
  type CreateAutomationParams,
  type LocalAutomation,
  type LocalMonitoringTextRules,
  type LocalStoreKey,
  type UpdateAutomationParams,
  useAutomationsConnect,
} from '@/hooks/use-automations-connect';
import { useCoupons } from '@/hooks/use-coupons';
import { useCTAsConnect } from '@/hooks/use-ctas-connect';
import { useDispatchTextsConnect } from '@/hooks/use-dispatch-texts-connect';
import { useGroupsConnect } from '@/hooks/use-groups-connect';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { useMlLinksConnect } from '@/hooks/use-ml-links-connect';
import { useProfilesConnect } from '@/hooks/use-profiles-connect';
import { useShopeeConnect } from '@/hooks/use-shopee-connect';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import type { CTA } from '@/entities/cta';
import type {
  Automation,
  AutomationCoupon,
  AutomationGroup,
  AutomationStatus,
  DispatchText,
  MlLinkList,
  PreviewOffer,
  StoreKey,
} from '@/types/automation';
import { DEFAULT_MONITOR_RULES } from '@/types/automation';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

/** Default form state for a new automation */
const DEFAULT_FORM_DATA: Partial<Automation> = {
  mode: 'search',
  start_time: '00:01',
  end_time: '23:59',
  interval_minutes: 30,
  send_groups: [],
  monitor_groups: [],
  stores: [],
  categories: [],
  priority: 'discount',
  filter_type: 'light',
  texts: [''],
  ctas: [],
  cta_ids: [],
  status: 'active',
  delay_seconds: 10,
  monitoring_text_rules: DEFAULT_MONITOR_RULES,
  coupon_id: null,
  dispatch_text_id: null,
  ml_link_list_id: null,
  list_items_per_run: 1,
  list_include_image: true,
  list_send_as_image: false,
};

export interface RefinementState {
  eletronicos: boolean;
  eletrodomesticos: boolean;
  homens: boolean;
  esportes: boolean;
  moda: boolean;
  casa: boolean;
  beleza: boolean;
}

export interface UseAutomationsReturn {
  automations: Automation[];
  maxAutomations: number;
  groups: AutomationGroup[];
  allGroups: AutomationGroup[];
  loading: boolean;
  dialogOpen: boolean;
  editingId: string | null;
  whatsappConnected: boolean;
  activeStores: StoreKey[];
  runningId: string | null;
  resettingId: string | null;
  suggestingText: boolean;
  previewOffers: PreviewOffer[];
  loadingPreview: boolean;
  previewDialogOpen: boolean;
  formData: Partial<Automation>;
  monitorDelayUnit: 'seconds' | 'minutes';
  monitorDelayInput: string;
  availableCoupons: AutomationCoupon[];
  availableCTAs: CTA[];
  dispatchTexts: DispatchText[];
  mlLinkLists: MlLinkList[];
  loadingMlLinkLists: boolean;
  limitReached: boolean;
  nearLimit: boolean;
  refinementOpen: RefinementState;
  savingAutomation: boolean;
  deleteTargetId: string | null;
  setDeleteTargetId: (id: string | null) => void;
  confirmDelete: () => Promise<void>;
  resetTargetId: string | null;
  resetTargetName: string | null;
  setResetTarget: (id: string | null, name?: string | null) => void;
  confirmResetHistory: () => Promise<void>;
  setDialogOpen: (open: boolean) => void;
  setPreviewDialogOpen: (open: boolean) => void;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  setMonitorDelayUnit: (unit: 'seconds' | 'minutes') => void;
  setMonitorDelayInput: (value: string) => void;
  setRefinementOpen: React.Dispatch<React.SetStateAction<RefinementState>>;
  toggleFavorite: (groupId: string) => Promise<void>;
  handleSave: () => Promise<void>;
  toggleStatus: (id: string, currentStatus: AutomationStatus) => Promise<void>;
  handleDelete: (id: string) => void;
  openEdit: (automation: Automation) => void;
  resetForm: () => void;
  handleRunNow: (automation: Automation) => Promise<void>;
  handleResetHistory: (automationId: string, automationName: string) => void;
  addText: () => void;
  updateText: (index: number, value: string) => void;
  removeText: (index: number) => void;
  handlePreviewOffers: (automation: Automation) => Promise<void>;
  suggestTexts: () => Promise<void>;
  loadMlLinkLists: () => Promise<void>;
}

/**
 * Converts LocalAutomation from ConnectRPC to the UI Automation type.
 */
export function fromLocalAutomation(local: LocalAutomation): Automation {
  return {
    id: local.id,
    name: local.name,
    mode: local.mode,
    type: local.mode === 'monitor' ? 'monitoring' : local.mode === 'list' ? 'list' : local.mode === 'offer_generate' ? 'offer_generate' : 'search',
    dispatch_text_id: local.dispatchTextId ?? null,
    monitoring_text_rules: local.monitoringTextRules
      ? {
          substituicoes: local.monitoringTextRules.substituicoes,
          remover_termos: local.monitoringTextRules.remover_termos,
          prefixo: local.monitoringTextRules.prefixo,
          sufixo: local.monitoringTextRules.sufixo,
          limpar_telefones: local.monitoringTextRules.limpar_telefones,
          limpar_arrobas: local.monitoringTextRules.limpar_arrobas,
        }
      : null,
    start_time: local.startTime,
    end_time: local.endTime,
    interval_minutes: clampIntervalMinutes(local.intervalMinutes),
    send_groups: local.sendGroups,
    monitor_groups: local.monitorGroupIds,
    monitor_group_ids: local.monitorGroupIds ?? null,
    stores: local.stores as StoreKey[],
    categories: local.categories,
    priority: local.priority,
    filter_type: local.filterType,
    min_discount: local.minDiscount ?? null,
    min_price: local.minPrice ?? null,
    max_price: local.maxPrice ?? null,
    texts: local.texts,
    ctas: local.ctas,
    cta_ids: local.ctaIds,
    last_run_at: local.lastRunAt,
    next_run_at: local.nextRunAt,
    status: local.status,
    last_error: local.lastError,
    delay_seconds: local.delaySeconds,
    coupon_id: local.couponId ?? null,
    ml_link_list_id: local.mlLinkListId ?? null,
    list_items_per_run: local.listItemsPerRun,
    list_include_image: local.listIncludeImage,
    list_send_as_image: local.listSendAsImage,
    instance_id: local.instanceId ?? null,
    instance_ids: local.instanceIds ?? [],
    chip_overrides: local.chipOverrides ?? {},
    apply_coupon_rules: local.applyCouponRules ?? true,
    repeat_when_finished: local.repeatWhenFinished ?? true,
  };
}

/**
 * Converts UI MonitoringTextRules to LocalMonitoringTextRules for ConnectRPC.
 */
export function toLocalMonitoringRules(
  rules: Automation['monitoring_text_rules']
): LocalMonitoringTextRules | undefined {
  if (!rules) return undefined;
  return {
    substituicoes: rules.substituicoes ?? [],
    remover_termos: rules.remover_termos ?? [],
    prefixo: rules.prefixo ?? '',
    sufixo: rules.sufixo ?? '',
    limpar_telefones: rules.limpar_telefones ?? false,
    limpar_arrobas: rules.limpar_arrobas ?? false,
  };
}

/**
 * Custom hook that encapsulates all automation business logic.
 * Uses ConnectRPC for all data operations.
 */
export function useAutomations(): UseAutomationsReturn {
  const { user } = useAuth();
  const { toast } = useToast();
  const { t } = useTranslation();

  // ConnectRPC hooks for data fetching
  const automationsConnect = useAutomationsConnect();
  const groupsConnect = useGroupsConnect();
  const dispatchTextsConnect = useDispatchTextsConnect();
  const mlLinksConnect = useMlLinksConnect();
  const profilesConnect = useProfilesConnect();
  const instancesConnect = useInstancesConnect();
  const affiliateCredentialsConnect = useAffiliateCredentialsConnect();
  const { coupons: allCoupons } = useCoupons();
  const ctasConnect = useCTAsConnect();
  const { searchOffers } = useShopeeConnect();
  const { suggestOfferText } = useAiToolsConnect();

  // Core state
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  // Action state
  const [runningId, setRunningId] = useState<string | null>(null);
  const [resettingId, setResettingId] = useState<string | null>(null);
  const [suggestingText, setSuggestingText] = useState(false);

  // Preview state
  const [previewOffers, setPreviewOffers] = useState<PreviewOffer[]>([]);
  const [loadingPreview, setLoadingPreview] = useState(false);
  const [previewDialogOpen, setPreviewDialogOpen] = useState(false);

  // Refinement accordion state
  const [refinementOpen, setRefinementOpen] = useState<RefinementState>({
    eletronicos: false,
    eletrodomesticos: false,
    homens: false,
    esportes: false,
    moda: false,
    casa: false,
    beleza: false,
  });

  // Form state
  const [formData, setFormData] = useState<Partial<Automation>>(DEFAULT_FORM_DATA);
  const [monitorDelayUnit, setMonitorDelayUnit] = useState<'seconds' | 'minutes'>('seconds');
  const [monitorDelayInput, setMonitorDelayInput] = useState<string>('10');

  // ============================================================
  // Derived state from ConnectRPC hooks
  // ============================================================

  // Automations from ConnectRPC - fetch on mount
  const [automationsData, setAutomationsData] = useState<LocalAutomation[]>([]);
  const [automationsLoading, setAutomationsLoading] = useState(true);

  const automations: Automation[] = useMemo(
    () => automationsData.filter((a) => a.mode !== 'offer_generate').map(fromLocalAutomation),
    [automationsData]
  );

  // Load automations
  const loadAutomations = useCallback(async () => {
    if (!user?.id) return;
    setAutomationsLoading(true);
    try {
      const data = await automationsConnect.listAutomations();
      setAutomationsData(data);
    } catch (error: unknown) {
      captureHookError(error, 'useAutomations', 'loadAutomations');
      toast({
        title: t('pages.offerAutomations.errors.loadAutomations'),
        description: t('pages.offerAutomations.errors.loadAutomationsMessage'),
        variant: 'destructive',
      });
    } finally {
      setAutomationsLoading(false);
    }
  }, [user?.id, automationsConnect.listAutomations, toast, t]);

  // Groups from ConnectRPC (all groups for send selection)
  const groups: AutomationGroup[] = useMemo(
    () =>
      groupsConnect.groups
        .map((g) => ({
          id: g.id,
          name: g.name,
          wa_group_id: g.wa_group_id,
          is_favorite: g.is_favorite,
          instance_id: g.instance_id,
          participating_instance_ids: g.participating_instance_ids,
        }))
        .sort((a, b) => {
          if (a.is_favorite && !b.is_favorite) return -1;
          if (!a.is_favorite && b.is_favorite) return 1;
          return a.name.localeCompare(b.name);
        }),
    [groupsConnect.groups]
  );

  // All groups (including non-admin) for monitoring group selection
  const allGroups: AutomationGroup[] = useMemo(
    () =>
      groupsConnect.allGroups
        .map((g) => ({
          id: g.id,
          name: g.name,
          wa_group_id: g.wa_group_id,
          is_favorite: g.is_favorite,
          instance_id: g.instance_id,
          participating_instance_ids: g.participating_instance_ids,
        }))
        .sort((a, b) => {
          if (a.is_favorite && !b.is_favorite) return -1;
          if (!a.is_favorite && b.is_favorite) return 1;
          return a.name.localeCompare(b.name);
        }),
    [groupsConnect.allGroups]
  );

  // Dispatch texts from ConnectRPC (only active texts available for selection)
  const dispatchTexts: DispatchText[] = useMemo(
    () =>
      dispatchTextsConnect.dispatchTexts
        .filter((dt) => dt.is_active)
        .map((dt) => ({
          id: dt.id,
          name: dt.name,
          is_active: dt.is_active,
        })),
    [dispatchTextsConnect.dispatchTexts]
  );

  // CTAs from ConnectRPC (only active CTAs available for selection)
  const availableCTAs: CTA[] = useMemo(
    () => ctasConnect.ctas.filter((cta) => cta.is_active),
    [ctasConnect.ctas]
  );

  // ML Link Lists from ConnectRPC
  const mlLinkLists: MlLinkList[] = useMemo(
    () =>
      mlLinksConnect.lists
        .filter((l) => l.is_active)
        .map((l) => ({
          id: l.id,
          name: l.name,
        })),
    [mlLinksConnect.lists]
  );

  // User limits from ConnectRPC
  const maxAutomations = useMemo(() => {
    const v = profilesConnect.limits?.max_automations;
    if (typeof v === 'number' && Number.isFinite(v) && v > 0) {
      return Math.floor(v);
    }
    return 10; // default
  }, [profilesConnect.limits?.max_automations]);

  // WhatsApp connection status from ConnectRPC.
  // Multi-instance aware: any connected instance qualifies as "WhatsApp
  // available". The legacy check on latestInstance flagged the user as
  // disconnected whenever the most-recently-updated chip was the wrong one
  // (e.g. they paired a 2nd chip and it later went into pending).
  const whatsappConnected = useMemo(() => {
    return instancesConnect.instances.some((inst) => inst.status === 'connected');
  }, [instancesConnect.instances]);

  // Active stores from ConnectRPC
  const activeStores: StoreKey[] = useMemo(
    () =>
      affiliateCredentialsConnect.activeStores
        .filter((s) => s.is_configured)
        .map((s) => s.store as StoreKey),
    [affiliateCredentialsConnect.activeStores]
  );

  // Coupons from ConnectRPC
  const availableCoupons: AutomationCoupon[] = useMemo(
    () =>
      allCoupons
        .filter((c) => c.is_active)
        .map((c) => ({
          id: c.id,
          code: c.code,
          title: c.title,
          store: c.store,
        })),
    [allCoupons]
  );

  // Combined loading state
  const loading =
    automationsLoading ||
    groupsConnect.loading ||
    dispatchTextsConnect.loading ||
    profilesConnect.loading ||
    instancesConnect.loading ||
    affiliateCredentialsConnect.loading;

  const loadingMlLinkLists = mlLinksConnect.loadingLists;

  const limitReached = automations.length >= maxAutomations;
  const nearLimit = maxAutomations > 0 && automations.length >= maxAutomations - 1;

  // Initial data load
  useEffect(() => {
    if (user) {
      loadAutomations();
    }
  }, [user, loadAutomations]);

  // Monitor delay unit handling
  useEffect(() => {
    if (!dialogOpen) return;
    if (formData.mode !== 'monitor') return;
    const s = Number(formData.delay_seconds ?? 10);
    const unit = s >= 60 && s % 60 === 0 ? 'minutes' : 'seconds';
    setMonitorDelayUnit(unit);
    setMonitorDelayInput(
      unit === 'minutes' ? String(Math.max(1, Math.round(s / 60))) : String(Math.max(10, s))
    );
  }, [dialogOpen, formData.mode, formData.delay_seconds]);

  // Load ML link lists when in list mode
  const loadMlLinkLists = useCallback(async () => {
    await mlLinksConnect.refetchLists();
  }, [mlLinksConnect.refetchLists]);

  useEffect(() => {
    if (user && dialogOpen && formData.mode === 'list') {
      loadMlLinkLists();
    }
  }, [user, dialogOpen, formData.mode, loadMlLinkLists]);

  // ============================================================
  // Action handlers
  // ============================================================

  const toggleFavorite = useCallback(
    async (groupId: string) => {
      const group = groups.find((g) => g.id === groupId);
      if (!group) return;

      try {
        await groupsConnect.toggleGroupFavorite(groupId, !group.is_favorite);

        toast({
          title: group.is_favorite
            ? t('pages.offerAutomations.success.removedFavorite')
            : t('pages.offerAutomations.success.addedFavorite'),
          description: group.is_favorite
            ? t('pages.offerAutomations.success.removedFavoriteMessage', {
                name: group.name,
              })
            : t('pages.offerAutomations.success.addedFavoriteMessage', {
                name: group.name,
              }),
        });
      } catch (error: unknown) {
        captureHookError(error, 'useAutomations', 'toggleFavorite');
        toast({
          title: t('common.error'),
          description: t('pages.offerAutomations.errors.favoriteUpdate'),
          variant: 'destructive',
        });
      }
    },
    [groups, groupsConnect.toggleGroupFavorite, toast, t]
  );

  const resetForm = useCallback(() => {
    setEditingId(null);
    setRefinementOpen({
      eletronicos: false,
      eletrodomesticos: false,
      homens: false,
      esportes: false,
      moda: false,
      casa: false,
      beleza: false,
    });
    setFormData({ ...DEFAULT_FORM_DATA });
  }, []);

  const handleSave = useCallback(async () => {
    if (!formData.name || formData.send_groups?.length === 0) {
      toast({
        title: t('pages.offerAutomations.validation.requiredFields'),
        description: t('pages.offerAutomations.validation.requiredFieldsMessage'),
        variant: 'destructive',
      });
      return;
    }

    if ((formData.stores?.length || 0) === 0) {
      toast({
        title: t('pages.offerAutomations.validation.storesRequired'),
        description: t('pages.offerAutomations.validation.storesRequiredMessage'),
        variant: 'destructive',
      });
      return;
    }

    if (
      formData.mode === 'monitor' &&
      (!formData.monitor_groups || formData.monitor_groups.length === 0)
    ) {
      toast({
        title: t('pages.offerAutomations.validation.monitorGroupsRequired'),
        description: t('pages.offerAutomations.validation.monitorGroupsRequiredMessage'),
        variant: 'destructive',
      });
      return;
    }

    if (formData.mode === 'search' && (!formData.categories || formData.categories.length === 0)) {
      toast({
        title: t('pages.offerAutomations.validation.categoriesRequired'),
        description: t('pages.offerAutomations.validation.categoriesRequiredMessage'),
        variant: 'destructive',
      });
      return;
    }

    if (formData.mode === 'monitor') {
      const monitorGroups = formData.monitor_groups || [];
      const sendGroups = formData.send_groups || [];
      const commonGroups = monitorGroups.filter((groupId) => sendGroups.includes(groupId));

      if (commonGroups.length > 0) {
        toast({
          title: t('pages.offerAutomations.validation.invalidConfiguration'),
          description: t('pages.offerAutomations.validation.conflictingGroupsMessage'),
          variant: 'destructive',
        });
        return;
      }
    }

    try {
      // Prepare data for ConnectRPC
      const mode = formData.mode ?? 'search';
      const intervalMinutes =
        mode === 'search' || mode === 'list'
          ? clampIntervalMinutes(formData.interval_minutes)
          : (formData.interval_minutes ?? 30);

      const isEventDriven = mode === 'monitor';

      // Handle categories based on mode
      let categories: string[] = [];
      if (mode === 'search') {
        const cats = formData.categories || [];
        categories = cats.includes(GENERAL_CATEGORY) ? [GENERAL_CATEGORY] : cats;
      }

      // Handle dispatch text and ML link list
      let dispatchTextId: string | undefined;
      let mlLinkListId: string | undefined;

      if (mode === 'search') {
        dispatchTextId = formData.dispatch_text_id || undefined;
      } else if (mode === 'list') {
        dispatchTextId = formData.dispatch_text_id || undefined;
        mlLinkListId = formData.ml_link_list_id || undefined;
      }

      // Handle texts
      const texts = mode === 'search' ? formData.texts?.filter((t) => t.trim()) || [] : [];

      // Handle stores
      const stores =
        mode === 'list'
          ? (['ml'] as LocalStoreKey[])
          : ((formData.stores || []) as LocalStoreKey[]);

      if (editingId) {
        // Update existing automation
        const updateParams: UpdateAutomationParams = {
          id: editingId,
          name: formData.name,
          mode,
          dispatchTextId,
          monitoringTextRules:
            mode === 'monitor' ? toLocalMonitoringRules(formData.monitoring_text_rules) : undefined,
          startTime: formData.start_time,
          endTime: formData.end_time,
          intervalMinutes,
          sendGroups: formData.send_groups,
          monitorGroupIds: isEventDriven ? formData.monitor_groups : [],
          stores,
          categories,
          priority: mode === 'search' ? formData.priority : 'discount',
          filterType: mode === 'search' ? formData.filter_type : 'light',
          minDiscount: mode === 'search' ? (formData.min_discount ?? undefined) : undefined,
          minPrice: mode === 'search' ? (formData.min_price ?? undefined) : undefined,
          maxPrice: mode === 'search' ? (formData.max_price ?? undefined) : undefined,
          texts,
          ctas: isEventDriven ? [] : formData.ctas,
          ctaIds: isEventDriven ? [] : formData.cta_ids || [],
          status: formData.status,
          delaySeconds: isEventDriven ? formData.delay_seconds : undefined,
          couponId: formData.coupon_id || undefined,
          mlLinkListId,
          listItemsPerRun:
            mode === 'list'
              ? Math.max(1, Math.min(20, Math.floor(Number(formData.list_items_per_run ?? 1))))
              : undefined,
          listIncludeImage:
            mode === 'list' ? Boolean(formData.list_include_image ?? true) : undefined,
          listSendAsImage:
            mode === 'list' ? Boolean(formData.list_send_as_image ?? false) : undefined,
          // Empty string sent on update explicitly clears the saved
          // preference (when the user picks "all chips"). undefined
          // would leave the previous value untouched.
          instanceId: formData.instance_id ?? '',
          instanceIds: formData.instance_ids ?? [],
          chipOverrides: formData.chip_overrides ?? {},
          applyCouponRules: formData.apply_coupon_rules,
          repeatWhenFinished: formData.repeat_when_finished,
        };

        await automationsConnect.updateAutomation(updateParams);
        toast({ title: t('pages.offerAutomations.success.updated') });
      } else {
        // Create new automation
        if (automations.length >= maxAutomations) {
          toast({
            title: t('pages.offerAutomations.limits.automationsReached'),
            description: t('pages.offerAutomations.limits.automationsReachedMessage', {
              count: automations.length,
              max: maxAutomations,
            }),
            variant: 'destructive',
          });
          return;
        }

        const createParams: CreateAutomationParams = {
          name: formData.name || '',
          mode,
          dispatchTextId,
          monitoringTextRules:
            mode === 'monitor' ? toLocalMonitoringRules(formData.monitoring_text_rules) : undefined,
          startTime: formData.start_time || '00:01',
          endTime: formData.end_time || '23:59',
          intervalMinutes,
          sendGroups: formData.send_groups || [],
          monitorGroupIds: isEventDriven ? formData.monitor_groups : [],
          stores,
          categories,
          priority: mode === 'search' ? formData.priority || 'discount' : 'discount',
          filterType: mode === 'search' ? formData.filter_type || 'light' : 'light',
          minDiscount: mode === 'search' ? (formData.min_discount ?? undefined) : undefined,
          minPrice: mode === 'search' ? (formData.min_price ?? undefined) : undefined,
          maxPrice: mode === 'search' ? (formData.max_price ?? undefined) : undefined,
          texts,
          ctas: isEventDriven ? [] : formData.ctas || [],
          ctaIds: isEventDriven ? [] : formData.cta_ids || [],
          status: formData.status || 'active',
          delaySeconds: isEventDriven ? formData.delay_seconds : undefined,
          couponId: formData.coupon_id || undefined,
          mlLinkListId,
          listItemsPerRun:
            mode === 'list'
              ? Math.max(1, Math.min(20, Math.floor(Number(formData.list_items_per_run ?? 1))))
              : undefined,
          listIncludeImage:
            mode === 'list' ? Boolean(formData.list_include_image ?? true) : undefined,
          listSendAsImage:
            mode === 'list' ? Boolean(formData.list_send_as_image ?? false) : undefined,
          instanceId: formData.instance_id ?? undefined,
          instanceIds: formData.instance_ids ?? [],
          chipOverrides: formData.chip_overrides ?? {},
          // Mirrors the form default (see automation-form-dialog): every
          // new automation here starts with the rule engine OFF, so the
          // user opts in explicitly when they want coupon injection.
          applyCouponRules: formData.apply_coupon_rules ?? false,
          // Default true: lista LIST mode roda em loop por padrão
          // (auto-reseta items consumidos quando esgota).
          repeatWhenFinished: formData.repeat_when_finished ?? true,
        };

        await automationsConnect.createAutomation(createParams);
        toast({ title: t('pages.offerAutomations.success.created') });
      }

      setDialogOpen(false);
      resetForm();
      loadAutomations();
    } catch (error: unknown) {
      captureHookError(error, 'useAutomations', 'handleSave');
      toast({
        title: t('pages.offerAutomations.errors.save'),
        description: t('pages.offerAutomations.errors.saveMessage'),
        variant: 'destructive',
      });
    }
  }, [
    formData,
    editingId,
    automations.length,
    maxAutomations,
    toast,
    t,
    resetForm,
    loadAutomations,
    automationsConnect.createAutomation,
    automationsConnect.updateAutomation,
  ]);

  const toggleStatus = useCallback(
    async (id: string, currentStatus: AutomationStatus) => {
      const newStatus = currentStatus === 'active' ? 'paused' : 'active';
      try {
        await automationsConnect.updateAutomation({
          id,
          status: newStatus,
        });
        loadAutomations();
        toast({
          title:
            newStatus === 'active'
              ? t('pages.offerAutomations.success.activated')
              : t('pages.offerAutomations.success.paused'),
        });
      } catch (error: unknown) {
        captureHookError(error, 'useAutomations', 'toggleStatus');
        toast({
          title: t('common.error'),
          description: t('pages.offerAutomations.errors.save'),
          variant: 'destructive',
        });
      }
    },
    [loadAutomations, toast, t, automationsConnect.updateAutomation]
  );

  // Delete confirmation state (replaces window.confirm with AlertDialog)
  const [deleteTargetId, setDeleteTargetId] = useState<string | null>(null);

  // Reset history confirmation state
  const [resetTargetId, setResetTargetId] = useState<string | null>(null);
  const [resetTargetName, setResetTargetName] = useState<string | null>(null);

  const handleDelete = useCallback((id: string) => {
    setDeleteTargetId(id);
  }, []);

  const confirmDelete = useCallback(async () => {
    if (!deleteTargetId) return;

    try {
      await automationsConnect.deleteAutomation(deleteTargetId);
      loadAutomations();
      toast({ title: t('pages.offerAutomations.success.deleted') });
    } catch (error: unknown) {
      captureHookError(error, 'useAutomations', 'handleDelete');
      toast({
        title: t('common.error'),
        description: t('pages.offerAutomations.errors.save'),
        variant: 'destructive',
      });
    } finally {
      setDeleteTargetId(null);
    }
  }, [deleteTargetId, loadAutomations, toast, t, automationsConnect.deleteAutomation]);

  const openEdit = useCallback((automation: Automation) => {
    setEditingId(automation.id);
    const nextForm = {
      ...automation,
      interval_minutes: clampIntervalMinutes(automation.interval_minutes),
      categories: automation.categories || [],
      mode:
        automation.type === 'monitoring'
          ? 'monitor'
          : automation.type === 'list'
            ? 'list'
            : (automation.mode ?? 'search'),
      monitor_groups: automation.monitor_group_ids ?? automation.monitor_groups ?? [],
      monitoring_text_rules: automation.monitoring_text_rules || DEFAULT_MONITOR_RULES,
    } as Partial<Automation>;
    setFormData(nextForm);

    const cats = nextForm.categories || [];
    const hasEletronicosSubcats = cats.some(isEletronicosSubcategoryKey);
    const hasEletroSubcats = cats.some(isEletrodomesticosSubcategoryKey);
    const hasHomensSubcats = cats.some(isHomensSubcategoryKey);
    const hasEsportesSubcats = cats.some(isEsportesSubcategoryKey);
    const hasModaSubcats = cats.some(isModaSubcategoryKey);
    const hasCasaSubcats = cats.some(isCasaSubcategoryKey);
    const hasBelezaSubcats = cats.some(isBelezaSubcategoryKey);

    setRefinementOpen({
      eletronicos: hasEletronicosSubcats,
      eletrodomesticos: !hasEletronicosSubcats && hasEletroSubcats,
      homens: !hasEletronicosSubcats && !hasEletroSubcats && hasHomensSubcats,
      esportes:
        !hasEletronicosSubcats && !hasEletroSubcats && !hasHomensSubcats && hasEsportesSubcats,
      moda:
        !hasEletronicosSubcats &&
        !hasEletroSubcats &&
        !hasHomensSubcats &&
        !hasEsportesSubcats &&
        hasModaSubcats,
      casa:
        !hasEletronicosSubcats &&
        !hasEletroSubcats &&
        !hasHomensSubcats &&
        !hasEsportesSubcats &&
        !hasModaSubcats &&
        hasCasaSubcats,
      beleza:
        !hasEletronicosSubcats &&
        !hasEletroSubcats &&
        !hasHomensSubcats &&
        !hasEsportesSubcats &&
        !hasModaSubcats &&
        !hasCasaSubcats &&
        hasBelezaSubcats,
    });
    setDialogOpen(true);
  }, []);

  const handleRunNow = useCallback(
    async (automation: Automation) => {
      setRunningId(automation.id);

      try {
        const diagResult = await automationsConnect.diagnoseAutomation(automation.id);

        const code = diagResult.code || '';
        const message = diagResult.message || '';
        const steps = diagResult.steps || [];

        const failedStep = steps.find((s) => !s.ok);
        const errorDetail = failedStep?.detail || '';

        if (code === 'SENT_OK' || code === 'ENVIO_PRONTO') {
          await automationsConnect.runAutomation(automation.id);
          toast({
            title: t('pages.offerAutomations.success.executed'),
            description: t('pages.offerAutomations.success.executedMessage', {
              name: automation.name,
            }),
            variant: 'success',
            duration: 5000,
          });
        } else {
          toast({
            title: t('pages.offerAutomations.errors.execute'),
            description: errorDetail || message,
            variant: 'destructive',
            duration: 5000,
          });
        }

        loadAutomations();
      } catch (error: unknown) {
        const errorMessage =
          error instanceof Error ? error.message : t('pages.offerAutomations.errors.executeFailed');
        toast({
          title: t('pages.offerAutomations.errors.execute'),
          description: errorMessage,
          variant: 'destructive',
          duration: 5000,
        });
      } finally {
        setRunningId(null);
      }
    },
    [
      toast,
      t,
      loadAutomations,
      automationsConnect.diagnoseAutomation,
      automationsConnect.runAutomation,
    ]
  );

  const handleResetHistory = useCallback((automationId: string, automationName: string) => {
    setResetTargetId(automationId);
    setResetTargetName(automationName);
  }, []);

  const setResetTarget = useCallback((id: string | null, name?: string | null) => {
    setResetTargetId(id);
    setResetTargetName(name ?? null);
  }, []);

  const confirmResetHistory = useCallback(async () => {
    if (!resetTargetId) return;

    setResettingId(resetTargetId);

    try {
      const result = await automationsConnect.resetAutomationHistory(resetTargetId);

      toast({
        title: t('pages.offerAutomations.success.historyReset'),
        description: result.message || t('pages.offerAutomations.success.historyResetMessage'),
      });

      loadAutomations();
    } catch (error: unknown) {
      toast({
        title: t('pages.offerAutomations.errors.resetHistory'),
        description:
          error instanceof Error
            ? error.message
            : t('pages.offerAutomations.errors.resetHistoryMessage'),
        variant: 'destructive',
      });
    } finally {
      setResettingId(null);
      setResetTargetId(null);
      setResetTargetName(null);
    }
  }, [resetTargetId, toast, t, loadAutomations, automationsConnect.resetAutomationHistory]);

  const addText = useCallback(() => {
    if ((formData.texts?.length || 0) < 3) {
      setFormData((prev) => ({
        ...prev,
        texts: [...(prev.texts || []), ''],
      }));
    }
  }, [formData.texts?.length]);

  const updateText = useCallback((index: number, value: string) => {
    setFormData((prev) => {
      const newTexts = [...(prev.texts || [])];
      newTexts[index] = value;
      return { ...prev, texts: newTexts };
    });
  }, []);

  const removeText = useCallback((index: number) => {
    setFormData((prev) => ({
      ...prev,
      texts: prev.texts?.filter((_, i) => i !== index) || [],
    }));
  }, []);

  const handlePreviewOffers = useCallback(
    async (automation: Automation) => {
      if (!automation.stores.includes('shopee')) {
        toast({
          title: t('pages.offerAutomations.validation.shopeeOnly'),
          description: t('pages.offerAutomations.validation.shopeeOnlyMessage'),
          variant: 'destructive',
        });
        return;
      }

      setLoadingPreview(true);
      setPreviewDialogOpen(true);
      setPreviewOffers([]);

      try {
        const result = await searchOffers({
          categories: automation.categories,
          minPrice:
            automation.min_price === null || automation.min_price === undefined
              ? undefined
              : Number(automation.min_price),
          maxPrice:
            automation.max_price === null || automation.max_price === undefined
              ? undefined
              : Number(automation.max_price),
          minDiscount:
            automation.min_discount === null || automation.min_discount === undefined
              ? undefined
              : Number(automation.min_discount),
          sortBy:
            automation.priority === 'discount'
              ? 'discount'
              : automation.priority === 'commission'
                ? 'commission'
                : 'saving',
          limit: 20,
          filterType: automation?.filter_type || 'light',
        });

        // Map LocalShopeeOffer to PreviewOffer
        const offers: PreviewOffer[] = result.map((offer) => ({
          title: offer.title,
          originalPrice: offer.originalPrice,
          finalPrice: offer.finalPrice,
          discountPercent: offer.discountPercent,
          imageUrl: offer.imageUrl,
          productUrl: offer.productUrl,
          storeName: offer.storeName,
          commissionRate: offer.commissionRate,
        }));
        // Filter out offers with no real discount (matches backend behavior)
        const filteredOffers = offers.filter(
          (o) => o.finalPrice > 0 && o.originalPrice > 0 && o.finalPrice < o.originalPrice
        );
        setPreviewOffers(filteredOffers);

        if (filteredOffers.length === 0) {
          toast({
            title: t('pages.offerAutomations.errors.noOffersFound'),
            description: t('pages.offerAutomations.errors.adjustFilters'),
            variant: 'destructive',
          });
        }
      } catch (error: unknown) {
        const errorMessage =
          error instanceof Error
            ? error.message
            : typeof error === 'object' && error !== null && 'error' in error
              ? String((error as { error: unknown }).error)
              : t('pages.offerAutomations.errors.fetchOffers');
        toast({
          title: t('pages.offerAutomations.errors.fetchOffers'),
          description: translateShopeeAffiliateError(errorMessage, t),
          variant: 'destructive',
        });
      } finally {
        setLoadingPreview(false);
      }
    },
    [toast, t, searchOffers]
  );

  const suggestTexts = useCallback(async () => {
    setSuggestingText(true);
    try {
      const texts = await suggestOfferText({
        category: formData.categories?.[0] ?? undefined,
        store: formData.stores?.[0] ?? undefined,
        mode: formData.mode,
      });

      if (texts && texts.length > 0) {
        setFormData((prev) => ({ ...prev, texts: texts.slice(0, 3) }));
        toast({
          title: t('pages.offerAutomations.success.textsSuggested'),
          description: t('pages.offerAutomations.success.textsSuggestedMessage'),
        });
      }
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error
          ? error.message
          : typeof error === 'object' && error !== null && 'error' in error
            ? String((error as { error: unknown }).error)
            : t('pages.offerAutomations.errors.suggestTexts');
      toast({
        title: t('pages.offerAutomations.errors.suggestTexts'),
        description: errorMessage,
        variant: 'destructive',
      });
    } finally {
      setSuggestingText(false);
    }
  }, [formData.categories, formData.stores, formData.mode, toast, t, suggestOfferText]);

  return {
    automations,
    maxAutomations,
    groups,
    allGroups,
    loading,
    dialogOpen,
    editingId,
    whatsappConnected,
    activeStores,
    runningId,
    resettingId,
    suggestingText,
    previewOffers,
    loadingPreview,
    previewDialogOpen,
    formData,
    monitorDelayUnit,
    monitorDelayInput,
    availableCoupons,
    availableCTAs,
    dispatchTexts,
    mlLinkLists,
    loadingMlLinkLists,
    limitReached,
    nearLimit,
    refinementOpen,
    savingAutomation:
      automationsConnect.isCreatingAutomation || automationsConnect.isUpdatingAutomation,
    deleteTargetId,
    setDeleteTargetId,
    confirmDelete,
    resetTargetId,
    resetTargetName,
    setResetTarget,
    confirmResetHistory,
    setDialogOpen,
    setPreviewDialogOpen,
    setFormData,
    setMonitorDelayUnit,
    setMonitorDelayInput,
    setRefinementOpen,
    toggleFavorite,
    handleSave,
    toggleStatus,
    handleDelete,
    openEdit,
    resetForm,
    handleRunNow,
    handleResetHistory,
    addText,
    updateText,
    removeText,
    handlePreviewOffers,
    suggestTexts,
    loadMlLinkLists,
  };
}
