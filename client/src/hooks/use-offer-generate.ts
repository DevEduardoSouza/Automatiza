import { useAuth } from '@/contexts';
import {
  type CreateAutomationParams,
  type LocalAutomation,
  type LocalStoreKey,
  type UpdateAutomationParams,
  useAutomationsConnect,
} from '@/hooks/use-automations-connect';
import { useAffiliateCredentialsConnect } from '@/hooks/use-affiliate-credentials-connect';
import { useDispatchLogsConnect } from '@/hooks/use-dispatch-logs-connect';
import { useDispatchTextsConnect } from '@/hooks/use-dispatch-texts-connect';
import { useGroupsConnect } from '@/hooks/use-groups-connect';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import type {
  Automation,
  AutomationGroup,
  StoreKey,
} from '@/types/automation';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { fromLocalAutomation } from './use-automations';

const DEFAULT_FORM: Partial<Automation> = {
  name: 'Gerar Oferta',
  mode: 'offer_generate',
  start_time: '00:01',
  end_time: '23:59',
  interval_minutes: 30,
  send_groups: [],
  monitor_groups: [],
  stores: ['shopee'],
  categories: [],
  priority: 'discount',
  filter_type: 'light',
  texts: [],
  ctas: [],
  cta_ids: [],
  status: 'active',
  delay_seconds: 10,
  coupon_id: null,
  dispatch_text_id: null,
};

export type DispatchLogStatus = 'sent' | 'failed' | 'skipped' | 'queued' | 'error';

export function useOfferGenerate() {
  const { user } = useAuth();
  const { toast } = useToast();
  const { t } = useTranslation();

  const automationsConnect = useAutomationsConnect();
  const groupsConnect = useGroupsConnect();
  const instancesConnect = useInstancesConnect();
  const affiliateCredentialsConnect = useAffiliateCredentialsConnect();
  const dispatchTextsConnect = useDispatchTextsConnect();

  // State
  const [config, setConfig] = useState<Automation | null>(null);
  const [formData, setFormData] = useState<Partial<Automation>>({ ...DEFAULT_FORM });
  const [configLoading, setConfigLoading] = useState(true);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  // Log filters
  const [logFilterStore, setLogFilterStore] = useState<string>('');
  const [logFilterStatus, setLogFilterStatus] = useState<string>('');

  // Dispatch logs
  const logsConnect = useDispatchLogsConnect({
    automationId: config?.id ?? '',
    limit: 50,
    enabled: !!config?.id,
    store: logFilterStore || undefined,
    status: (logFilterStatus as DispatchLogStatus) || undefined,
  });

  // Groups for send selection (admin groups, sorted by favorite). instance_id
  // is preserved so the page can filter by the user's selected chip.
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
    [groupsConnect.groups],
  );

  // All groups for monitor/router selection
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
    [groupsConnect.allGroups],
  );

  // Active stores — use all configured stores as default for the automation
  const activeStores: StoreKey[] = useMemo(
    () => affiliateCredentialsConnect.activeStores.filter((s) => s.is_configured).map((s) => s.store as StoreKey),
    [affiliateCredentialsConnect.activeStores],
  );

  // Use primaryInstance (default+connected → connected → any) instead of
  // latestInstance — the latter returns whichever chip was most recently
  // updated, which can be a disconnected secondary chip and would falsely
  // flag whatsappConnected=false when the user has a connected default.
  const whatsappConnected = useMemo(
    () => instancesConnect.primaryInstance?.status === 'connected',
    [instancesConnect.primaryInstance?.status],
  );

  // Expose the instances list so the page can render a chip selector.
  const instances = instancesConnect.instances;

  const dispatchTexts = useMemo(
    () => dispatchTextsConnect.dispatchTexts.filter((dt) => dt.is_active),
    [dispatchTextsConnect.dispatchTexts],
  );

  const loading = configLoading || groupsConnect.loading || instancesConnect.loading;
  const saving = automationsConnect.isCreatingAutomation || automationsConnect.isUpdatingAutomation;

  // Load the single offer_generate config
  const loadConfig = useCallback(async () => {
    if (!user?.id) return;
    setConfigLoading(true);
    try {
      const data = await automationsConnect.listAutomations();
      const offerGen = data.find((a: LocalAutomation) => a.mode === 'offer_generate');
      if (offerGen) {
        const converted = fromLocalAutomation(offerGen);
        setConfig(converted);
        setFormData({
          ...converted,
          monitor_groups: converted.monitor_group_ids ?? converted.monitor_groups ?? [],
        });
      } else {
        setConfig(null);
        setFormData({ ...DEFAULT_FORM });
      }
    } catch (error: unknown) {
      captureHookError(error, 'useOfferGenerate', 'loadConfig');
      toast({ title: t('pages.offerGenerate.errors.loadFailed'), variant: 'destructive' });
    } finally {
      setConfigLoading(false);
    }
  }, [user?.id, automationsConnect.listAutomations, toast, t]);

  useEffect(() => {
    if (user) loadConfig();
  }, [user, loadConfig]);

  // Save — only groups matter, everything else uses sensible defaults
  const handleSave = useCallback(async () => {
    if (!formData.monitor_groups || formData.monitor_groups.length === 0) {
      toast({ title: t('pages.offerGenerate.validation.sourceGroupRequired'), variant: 'destructive' });
      return;
    }
    if (!formData.send_groups || formData.send_groups.length === 0) {
      toast({ title: t('pages.offerGenerate.validation.destinationGroupsRequired'), variant: 'destructive' });
      return;
    }

    const monitorGroups = formData.monitor_groups || [];
    const sendGroups = formData.send_groups || [];
    if (monitorGroups.some((g) => sendGroups.includes(g))) {
      toast({ title: t('pages.offerGenerate.validation.conflictingGroups'), variant: 'destructive' });
      return;
    }

    try {
      // Use all active stores if none set
      const stores: LocalStoreKey[] =
        formData.stores && formData.stores.length > 0
          ? (formData.stores as LocalStoreKey[])
          : activeStores.length > 0
            ? (activeStores as LocalStoreKey[])
            : ['shopee'];

      const commonParams = {
        mode: 'offer_generate' as const,
        name: formData.name || 'Gerar Oferta',
        startTime: formData.start_time || '00:01',
        endTime: formData.end_time || '23:59',
        intervalMinutes: formData.interval_minutes ?? 30,
        sendGroups: formData.send_groups || [],
        monitorGroupIds: formData.monitor_groups || [],
        stores,
        categories: [] as string[],
        priority: 'discount' as const,
        filterType: 'light' as const,
        texts: [] as string[],
        ctas: [] as string[],
        ctaIds: [] as string[],
        delaySeconds: formData.delay_seconds ?? 10,
        couponId: formData.coupon_id || undefined,
        dispatchTextId: formData.dispatch_text_id || undefined,
        // Multi-chip preference (049). Empty list = "no preference" (server
        // falls back to first connected). instanceId é mantido em sync com
        // instanceIds[0] no backend pra rollback compat.
        instanceIds: formData.instance_ids ?? [],
        instanceId: formData.instance_id ?? undefined,
        // Mirror the toggle on the offer-generate config page; default
        // to true when unset so the rule engine still kicks in for
        // existing rows that pre-date the field.
        applyCouponRules: formData.apply_coupon_rules ?? true,
      };

      if (config) {
        await automationsConnect.updateAutomation({
          id: config.id,
          ...commonParams,
          status: formData.status,
        } as UpdateAutomationParams);
      } else {
        await automationsConnect.createAutomation({
          ...commonParams,
          status: 'active',
        } as CreateAutomationParams);
      }

      toast({ title: t('pages.offerGenerate.success.saved') });
      await loadConfig();
    } catch (error: unknown) {
      captureHookError(error, 'useOfferGenerate', 'handleSave');
      toast({ title: t('pages.offerGenerate.errors.saveFailed'), variant: 'destructive' });
    }
  }, [formData, config, activeStores, automationsConnect, toast, t, loadConfig]);

  // Toggle active/paused
  const handleToggleStatus = useCallback(async () => {
    if (!config) return;
    const newStatus = config.status === 'active' ? 'paused' : 'active';
    try {
      await automationsConnect.updateAutomation({ id: config.id, status: newStatus });
      toast({
        title: newStatus === 'active'
          ? t('pages.offerGenerate.success.activated')
          : t('pages.offerGenerate.success.paused'),
      });
      await loadConfig();
    } catch (error: unknown) {
      captureHookError(error, 'useOfferGenerate', 'handleToggleStatus');
      toast({ title: t('pages.offerGenerate.errors.toggleFailed'), variant: 'destructive' });
    }
  }, [config, automationsConnect, toast, t, loadConfig]);

  // Delete
  const handleDelete = useCallback(() => {
    setDeleteDialogOpen(true);
  }, []);

  const confirmDelete = useCallback(async () => {
    if (!config) return;
    try {
      await automationsConnect.deleteAutomation(config.id);
      toast({ title: t('pages.offerGenerate.success.deleted') });
      setConfig(null);
      setFormData({ ...DEFAULT_FORM });
      setDeleteDialogOpen(false);
    } catch (error: unknown) {
      captureHookError(error, 'useOfferGenerate', 'confirmDelete');
      toast({ title: t('pages.offerGenerate.errors.deleteFailed'), variant: 'destructive' });
    }
  }, [config, automationsConnect, toast, t]);

  // Toggle favorite group
  const toggleFavorite = useCallback(
    async (groupId: string) => {
      const group = groups.find((g) => g.id === groupId) || allGroups.find((g) => g.id === groupId);
      if (!group) return;
      try {
        await groupsConnect.toggleGroupFavorite(groupId, !group.is_favorite);
      } catch {
        // silent
      }
    },
    [groups, allGroups, groupsConnect.toggleGroupFavorite],
  );

  // ── DEV MOCK DATA ── remove after testing ──
  const DEV_MOCK = import.meta.env.DEV && !config;

  const mockConfig: Automation | null = DEV_MOCK
    ? {
        id: 'mock-1',
        name: 'Gerar Oferta',
        mode: 'offer_generate',
        type: 'offer_generate',
        start_time: '00:01',
        end_time: '23:59',
        interval_minutes: 30,
        send_groups: [],
        monitor_groups: [],
        stores: ['shopee'],
        categories: [],
        priority: 'discount',
        filter_type: 'light',
        texts: [],
        ctas: [],
        cta_ids: [],
        status: 'active',
        delay_seconds: 10,
      }
    : config;

  const mockLogs = DEV_MOCK
    ? [
        { id: '1', automationId: 'mock-1', groupId: 'g1', groupName: 'Ofertas VIP', canonicalKey: 'shopee-123', offerTitle: 'Conjunto Feminino Tricô Premium Gola V', store: 'shopee', status: 'sent' as const, createdAt: new Date('2026-03-09T22:35:00'), productPageUrl: '' },
        { id: '2', automationId: 'mock-1', groupId: 'g1', groupName: 'Ofertas VIP', canonicalKey: 'shopee-456', offerTitle: 'Combo 4 Produtos - Trivitt Itallian Color', store: 'shopee', status: 'sent' as const, createdAt: new Date('2026-03-09T17:26:00'), productPageUrl: '' },
        { id: '3', automationId: 'mock-1', groupId: 'g2', groupName: 'Promoções Top', canonicalKey: 'amazon-789', offerTitle: 'Echo Dot 5ª Geração Smart Speaker com Alexa', store: 'amazon', status: 'sent' as const, createdAt: new Date('2026-03-09T08:35:00'), productPageUrl: '' },
        { id: '4', automationId: 'mock-1', groupId: 'g1', groupName: 'Ofertas VIP', canonicalKey: 'shopee-101', offerTitle: 'Celular iPhone 14/14 Plus/14 Pro/ 14 Pro Max', store: 'shopee', status: 'failed' as const, createdAt: new Date('2026-03-08T21:13:00'), errorMessage: 'Timeout ao enviar', productPageUrl: '' },
        { id: '5', automationId: 'mock-1', groupId: 'g2', groupName: 'Promoções Top', canonicalKey: 'magalu-202', offerTitle: 'Smart TV Samsung 55" Crystal UHD 4K', store: 'magalu', status: 'sent' as const, createdAt: new Date('2026-03-08T20:25:00'), productPageUrl: '' },
        { id: '6', automationId: 'mock-1', groupId: 'g1', groupName: 'Ofertas VIP', canonicalKey: 'shopee-303', offerTitle: 'Wella Invigo Nutri-enrich - Máscara De Nutrição', store: 'shopee', status: 'skipped' as const, createdAt: new Date('2026-03-08T20:25:00'), productPageUrl: '' },
        { id: '7', automationId: 'mock-1', groupId: 'g2', groupName: 'Promoções Top', canonicalKey: 'amazon-404', offerTitle: 'Fone de Ouvido Bluetooth JBL Tune 520BT', store: 'amazon', status: 'sent' as const, createdAt: new Date('2026-03-08T20:22:00'), productPageUrl: '' },
        { id: '8', automationId: 'mock-1', groupId: 'g1', groupName: 'Ofertas VIP', canonicalKey: 'shopee-505', offerTitle: 'Medicube Booster Pro Mini 3 Cores', store: 'shopee', status: 'sent' as const, createdAt: new Date('2026-03-06T22:20:00'), productPageUrl: '' },
        { id: '9', automationId: 'mock-1', groupId: 'g2', groupName: 'Promoções Top', canonicalKey: 'ml-606', offerTitle: 'Cafeteira Nespresso Vertuo Pop', store: 'ml', status: 'failed' as const, createdAt: new Date('2026-03-05T09:15:00'), errorMessage: 'Grupo não encontrado', productPageUrl: '' },
        { id: '10', automationId: 'mock-1', groupId: 'g1', groupName: 'Ofertas VIP', canonicalKey: 'shopee-707', offerTitle: 'ANUA Niacinamide 10% + TXA 4% Serum 30ml', store: 'shopee', status: 'sent' as const, createdAt: new Date('2026-03-04T23:41:00'), productPageUrl: '' },
      ]
    : logsConnect.logs;

  const filteredMockLogs = DEV_MOCK && logFilterStatus
    ? mockLogs.filter((l) => l.status === logFilterStatus)
    : mockLogs;
  // ── END DEV MOCK ──

  return {
    config: mockConfig,
    formData,
    setFormData,
    groups,
    allGroups,
    instances,
    whatsappConnected,
    logs: DEV_MOCK ? filteredMockLogs : logsConnect.logs,
    logsLoading: DEV_MOCK ? false : logsConnect.isLoading,
    logsTotalCount: DEV_MOCK ? filteredMockLogs.length : logsConnect.totalCount,
    logsHasMore: DEV_MOCK ? false : logsConnect.hasMore,
    refetchLogs: logsConnect.refetch,
    logFilterStore,
    setLogFilterStore,
    logFilterStatus,
    setLogFilterStatus,
    loading: DEV_MOCK ? false : loading,
    saving,
    handleSave,
    handleToggleStatus,
    handleDelete,
    toggleFavorite,
    dispatchTexts,
    deleteDialogOpen,
    setDeleteDialogOpen,
    confirmDelete,
  };
}
