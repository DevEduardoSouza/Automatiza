import { useAdminConnect } from '@/hooks/use-admin-connect';
import { useSettingsConnect } from '@/hooks/use-settings-connect';
import { useWebhooksConnect } from '@/hooks/use-webhooks-connect';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import type {
  DashboardStats,
  DashboardTabProps,
  ObservabilityData,
  ShopeeAuditData,
  UseDashboardTabReturn,
  WebhookLog,
} from '../types/dashboard.types';

/**
 * Hook que gerencia a logica do dashboard administrativo.
 * Busca estatisticas, observabilidade e audit.
 * Usa ConnectRPC para todas as operacoes de dados.
 */
export function useDashboardTab(props?: DashboardTabProps): UseDashboardTabReturn {
  const { onStatsUpdate } = props || {};
  const { t } = useTranslation();

  // ConnectRPC hooks
  const {
    observability,
    obsLoading: connectObsLoading,
    refetchObservability,
    fetchShopeeAudit,
    listWebhookLogs,
    fetchAdminProfileStats,
  } = useAdminConnect();
  const { simulateKiwifyWebhook } = useWebhooksConnect();
  const { getSetting } = useSettingsConnect(['kiwify_webhook_token']);

  const [webhookLogs, setWebhookLogs] = useState<WebhookLog[]>([]);
  const [stats, setStats] = useState<DashboardStats>({
    totalUsers: 0,
    activeUsers: 0,
    inactiveUsers: 0,
    planStats: [],
    growthData: [],
  });
  const [audit, setAudit] = useState<ShopeeAuditData | null>(null);
  const [auditLoading, setAuditLoading] = useState(false);
  const [auditDays, setAuditDays] = useState<number>(7);

  // Map ConnectRPC observability to local format
  const obs: ObservabilityData | null = observability
    ? {
        generatedAt: observability.generatedAt,
        queue: observability.queue,
        dispatchLogs: {
          windowMinutes: observability.dispatchLogs.windowMinutes,
          total: observability.dispatchLogs.total,
          byStatus: observability.dispatchLogs.byStatus,
          topErrors: observability.dispatchLogs.topErrors,
          topUsers: observability.dispatchLogs.topUsers,
        },
        totals: observability.totals,
        topUsers: observability.topUsers,
      }
    : null;
  const obsLoading = connectObsLoading;

  const fetchObservability = useCallback(async () => {
    refetchObservability();
  }, [refetchObservability]);

  const handleFetchShopeeAudit = useCallback(
    async (days?: number) => {
      setAuditLoading(true);
      try {
        const result = await fetchShopeeAudit({
          days: days ?? auditDays,
          limit: 25,
          maxScan: 5000,
        });
        if (result) {
          setAudit({
            window: result.window,
            scanned: result.scanned,
            withIdentifier: result.withIdentifier,
            mismatches: result.mismatches,
            unknown: result.unknown,
            missingCreds: result.missingCreds,
            samples: result.samples,
          });
        } else {
          setAudit(null);
        }
      } catch (_e) {
        setAudit(null);
      } finally {
        setAuditLoading(false);
      }
    },
    [auditDays, fetchShopeeAudit]
  );

  const fetchDashboardStats = useCallback(async () => {
    try {
      const result = await fetchAdminProfileStats();
      if (!result) return;

      const newStats: DashboardStats = {
        totalUsers: result.totalUsers,
        activeUsers: result.activeUsers,
        inactiveUsers: result.inactiveUsers,
        planStats: result.planStats,
        growthData: result.growthData,
      };

      setStats(newStats);
      onStatsUpdate?.(newStats);
    } catch {
      // Error handled silently
    }
  }, [onStatsUpdate, fetchAdminProfileStats]);

  const fetchWebhookLogs = useCallback(async () => {
    const logs = await listWebhookLogs({ limit: 10, source: 'kiwify' });
    setWebhookLogs(logs);
  }, [listWebhookLogs]);

  const didMountRef = useRef(false);

  useEffect(() => {
    if (didMountRef.current) return;
    didMountRef.current = true;

    fetchWebhookLogs();
    fetchDashboardStats();
    handleFetchShopeeAudit(7);
  }, [fetchWebhookLogs, fetchDashboardStats, handleFetchShopeeAudit]);

  const handleSimulateWebhook = async (email: string, event: string, product: string) => {
    const token = getSetting<string>('kiwify_webhook_token', '');

    if (!token) {
      toast.error(t('admin.dashboard.webhook.simulator.tokenNotConfigured'));
      return;
    }

    const result = await simulateKiwifyWebhook({
      email,
      event,
      product: product || undefined,
      token,
    });

    if (result.error) throw new Error(result.error);

    toast.success(t('admin.dashboard.webhook.simulator.success'));

    await fetchWebhookLogs();
    await fetchDashboardStats();
  };

  return {
    stats,
    webhookLogs,
    obs,
    obsLoading,
    audit,
    auditLoading,
    auditDays,
    setAuditDays,
    fetchObservability,
    fetchShopeeAffiliateAudit: handleFetchShopeeAudit,
    handleSimulateWebhook,
  };
}
