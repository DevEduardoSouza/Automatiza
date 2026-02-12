import { useAdminConnect } from '@/hooks/use-admin-connect';
import { useLoadBalancerConnect } from '@/hooks/use-loadbalancer-connect';
import { useSettingsConnect } from '@/hooks/use-settings-connect';
import { useWebhooksConnect } from '@/hooks/use-webhooks-connect';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import type {
  DashboardStats,
  DashboardTabProps,
  LoadBalancerStats,
  ObservabilityData,
  ShopeeAuditData,
  UseDashboardTabReturn,
  WebhookLog,
} from '../types/dashboard.types';

/**
 * Hook que gerencia a logica do dashboard administrativo.
 * Busca estatisticas, dados de load balancer, observabilidade e audit.
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
    ensureWebhooks,
    isPending: adminPending,
    listWebhookLogs,
    fetchAdminProfileStats,
  } = useAdminConnect();
  const {
    stats: lbConnectStats,
    loading: lbConnectLoading,
    refetch: refetchLbStats,
  } = useLoadBalancerConnect();
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
  const [ensureWebhookLoading, setEnsureWebhookLoading] = useState(false);

  // Map ConnectRPC stats to local format
  const lbStats: LoadBalancerStats | null = lbConnectStats
    ? {
        servers: lbConnectStats.servers.map((s) => ({
          server_id: s.serverId,
          name: s.serverName,
          url: s.url,
          capacity: s.maxUsers,
          mapped_users: s.currentUsers,
          available: s.availableSlots,
          usage_percent: s.usagePercent,
          status: s.status,
          waha_instances_total: s.wahaInstancesTotal,
          waha_instances_open: s.wahaInstancesOpen,
          waha_instances_connecting: s.wahaInstancesConnecting,
          waha_instances_close: s.wahaInstancesClose,
        })),
        summary: {
          total_servers: lbConnectStats.summary.totalServers,
          total_capacity: lbConnectStats.summary.totalCapacity,
          total_mapped: lbConnectStats.summary.totalUsed,
          total_available: lbConnectStats.summary.totalAvailable,
          usage_percent: lbConnectStats.summary.usagePercent,
          waha_instances_total: lbConnectStats.summary.wahaInstancesTotal,
          waha_instances_open: lbConnectStats.summary.wahaInstancesOpen,
          waha_instances_connecting: lbConnectStats.summary.wahaInstancesConnecting,
          waha_instances_close: lbConnectStats.summary.wahaInstancesClose,
        },
      }
    : null;
  const lbLoading = lbConnectLoading;

  // Map ConnectRPC observability to local format
  const obs: ObservabilityData | null = observability
    ? {
        generatedAt: observability.generatedAt,
        queue: observability.queue,
        loadBalancer: observability.loadBalancer,
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

  const fetchLoadBalancerStats = useCallback(async () => {
    refetchLbStats();
  }, [refetchLbStats]);

  const forceWebhooksServer2 = async () => {
    setEnsureWebhookLoading(true);
    try {
      const result = await ensureWebhooks({
        serverId: 2,
        limit: 500,
        concurrency: 5,
      });
      toast.success(
        t('admin.dashboard.loadBalancer.forceWebhooksSuccess', {
          ok: result.ok,
          total: result.total,
          fail: result.fail,
        })
      );
    } catch (e: unknown) {
      const msg =
        e instanceof Error ? e.message : t('admin.dashboard.loadBalancer.forceWebhooksError');
      toast.error(msg);
    } finally {
      setEnsureWebhookLoading(false);
    }
  };

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
    lbStats,
    lbLoading,
    obs,
    obsLoading,
    audit,
    auditLoading,
    auditDays,
    setAuditDays,
    ensureWebhookLoading: ensureWebhookLoading || adminPending,
    fetchLoadBalancerStats,
    forceWebhooksServer2,
    fetchObservability,
    fetchShopeeAffiliateAudit: handleFetchShopeeAudit,
    handleSimulateWebhook,
  };
}
