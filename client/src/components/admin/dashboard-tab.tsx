import {
  ChartsSection,
  ObservabilityCard,
  ShopeeAuditSection,
  StatsCards,
  WebhookSection,
} from './dashboard';
import { useDashboardTab } from './hooks';
import type { DashboardTabProps } from './types/dashboard.types';

/**
 * Tab do dashboard administrativo.
 * Exibe estatísticas, gráficos, observabilidade e webhooks.
 */
export default function DashboardTab({ onStatsUpdate }: DashboardTabProps) {
  const {
    stats,
    webhookLogs,
    obs,
    obsLoading,
    audit,
    auditLoading,
    auditDays,
    setAuditDays,
    fetchObservability,
    fetchShopeeAffiliateAudit,
    handleSimulateWebhook,
  } = useDashboardTab({ onStatsUpdate });

  return (
    <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8 xl:space-y-10">
      <StatsCards stats={stats} obs={obs} />

      <ObservabilityCard data={obs} loading={obsLoading} onRefresh={fetchObservability} />

      <ShopeeAuditSection
        audit={audit}
        auditLoading={auditLoading}
        auditDays={auditDays}
        onAuditDaysChange={setAuditDays}
        onRefresh={fetchShopeeAffiliateAudit}
      />

      <ChartsSection planStats={stats.planStats} growthData={stats.growthData} />

      <WebhookSection webhookLogs={webhookLogs} onSimulate={handleSimulateWebhook} />
    </div>
  );
}
