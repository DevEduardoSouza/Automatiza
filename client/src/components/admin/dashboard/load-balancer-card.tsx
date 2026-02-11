import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

interface ServerStats {
  serverId: number;
  serverName: string;
  url: string;
  currentUsers: number;
  maxUsers: number;
  availableSlots: number;
  usagePercent: number;
  status: string;
  wahaInstancesTotal?: number;
  wahaInstancesOpen?: number;
  wahaInstancesConnecting?: number;
  wahaInstancesClose?: number;
}

interface LoadBalancerSummary {
  totalServers: number;
  totalCapacity: number;
  totalUsed: number;
  totalAvailable: number;
  usagePercent: number;
  wahaInstancesTotal?: number;
  wahaInstancesOpen?: number;
  wahaInstancesConnecting?: number;
  wahaInstancesClose?: number;
}

interface LoadBalancerStats {
  servers: ServerStats[];
  summary: LoadBalancerSummary;
}

interface LoadBalancerCardProps {
  stats: LoadBalancerStats | null;
  loading: boolean;
  ensureWebhookLoading: boolean;
  onRefresh: () => void;
  onForceWebhooks: () => void;
}

/**
 * Card de estatísticas do Load Balancer.
 * Exibe uso de servidores e permite forçar webhooks.
 */
export function LoadBalancerCard({
  stats,
  loading,
  ensureWebhookLoading,
  onRefresh,
  onForceWebhooks,
}: LoadBalancerCardProps) {
  const { t } = useTranslation();

  const renderCapacityAlert = () => {
    if (!stats) return null;

    const isFull =
      (stats.summary.totalCapacity > 0 && stats.summary.totalUsed >= stats.summary.totalCapacity) ||
      stats.servers.some((s) => (s.availableSlots ?? 0) <= 0);

    const isNearLimit =
      !isFull &&
      ((stats.summary.totalCapacity > 0 &&
        stats.summary.totalUsed / stats.summary.totalCapacity >= 0.8) ||
        stats.servers.some((s) => Number.isFinite(s.usagePercent) && (s.usagePercent ?? 0) >= 80));

    if (isFull) {
      return (
        <Alert>
          <AlertTitle>{t('admin.dashboard.loadBalancer.capacityLimitTitle')}</AlertTitle>
          <AlertDescription>{t('admin.dashboard.loadBalancer.capacityLimitDesc')}</AlertDescription>
        </Alert>
      );
    }

    if (isNearLimit) {
      return (
        <Alert>
          <AlertTitle>{t('admin.dashboard.loadBalancer.nearLimitTitle')}</AlertTitle>
          <AlertDescription>{t('admin.dashboard.loadBalancer.nearLimitDesc')}</AlertDescription>
        </Alert>
      );
    }

    return null;
  };

  return (
    <div
      className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between mb-3 xs:mb-4 sm:mb-5">
        <div>
          <span
            className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
            style={{ color: colors.wineText }}
          >
            {t('admin.dashboard.loadBalancer.title')}
          </span>
          <p className="text-xs md:text-sm mt-0.5" style={{ color: colors.wineTextMuted }}>
            {t('admin.dashboard.loadBalancer.description')}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={onRefresh}
            disabled={loading}
            className="inline-flex cursor-pointer items-center justify-center gap-2 rounded-full border border-gray-300 bg-white px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold text-[#8D2D53] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
          >
            {loading ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" />
                {t('admin.dashboard.loadBalancer.refreshing')}
              </>
            ) : (
              t('admin.dashboard.loadBalancer.refresh')
            )}
          </button>
          <button
            type="button"
            onClick={onForceWebhooks}
            disabled={ensureWebhookLoading}
            title={t('admin.dashboard.loadBalancer.forceWebhooksTitle')}
            className="inline-flex cursor-pointer items-center justify-center gap-2 rounded-full border border-gray-300 bg-white px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold text-[#8D2D53] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
          >
            {ensureWebhookLoading ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" />
                {t('admin.dashboard.loadBalancer.forcing')}
              </>
            ) : (
              t('admin.dashboard.loadBalancer.forceWebhooks')
            )}
          </button>
        </div>
      </div>
      <div className="space-y-3 xs:space-y-4">
        {stats ? (
          <div className="space-y-3">
            {renderCapacityAlert()}

            <div className="text-sm" style={{ color: colors.wineTextMuted }}>
              {t('admin.dashboard.loadBalancer.summary')}:{' '}
              <strong className="text-foreground">{stats.summary.totalUsed}</strong> /{' '}
              <strong className="text-foreground">{stats.summary.totalCapacity}</strong> (
              {stats.summary.usagePercent}%)
              {typeof stats.summary.wahaInstancesTotal === 'number' ? (
                <span>
                  {' '}
                  • WAHA:{' '}
                  <strong className="text-foreground">
                    {stats.summary.wahaInstancesOpen ?? '-'}
                  </strong>{' '}
                  |{' '}
                  <strong className="text-foreground">
                    {stats.summary.wahaInstancesConnecting ?? '-'}
                  </strong>{' '}
                  |{' '}
                  <strong className="text-foreground">
                    {stats.summary.wahaInstancesClose ?? '-'}
                  </strong>
                  /
                  <strong className="text-foreground">
                    {stats.summary.wahaInstancesTotal ?? '-'}
                  </strong>{' '}
                  {t('admin.dashboard.loadBalancer.wahaLabels')}
                </span>
              ) : null}
            </div>

            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('admin.dashboard.loadBalancer.server')}</TableHead>
                    <TableHead>{t('admin.dashboard.loadBalancer.url')}</TableHead>
                    <TableHead className="text-right">
                      {t('admin.dashboard.loadBalancer.users')}
                    </TableHead>
                    <TableHead className="text-right">
                      {t('admin.dashboard.loadBalancer.waha')}
                    </TableHead>
                    <TableHead className="text-right">
                      {t('admin.dashboard.loadBalancer.slots')}
                    </TableHead>
                    <TableHead className="text-right">
                      {t('admin.dashboard.loadBalancer.usage')}
                    </TableHead>
                    <TableHead>{t('common.status')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {stats.servers.map((s, index) => (
                    <TableRow key={s.serverId ?? `server-${index}`}>
                      <TableCell className="font-medium">{s.serverName}</TableCell>
                      <TableCell className="text-xs" style={{ color: colors.wineTextMuted }}>
                        {s.url}
                      </TableCell>
                      <TableCell className="text-right">
                        {s.currentUsers}/{s.maxUsers}
                      </TableCell>
                      <TableCell className="text-right">
                        {typeof s.wahaInstancesTotal === 'number' ? (
                          s.wahaInstancesTotal < 0 ? (
                            <span style={{ color: colors.wineTextMuted }}>
                              {t('admin.dashboard.loadBalancer.unavailableShort')}
                            </span>
                          ) : (
                            <span>
                              {s.wahaInstancesOpen ?? '-'} | {s.wahaInstancesConnecting ?? '-'} |{' '}
                              {s.wahaInstancesClose ?? '-'} / {s.wahaInstancesTotal}
                            </span>
                          )
                        ) : (
                          <span style={{ color: colors.wineTextMuted }}>—</span>
                        )}
                      </TableCell>
                      <TableCell className="text-right">{s.availableSlots}</TableCell>
                      <TableCell className="text-right">{s.usagePercent}%</TableCell>
                      <TableCell>
                        <Badge variant={s.status === 'active' ? 'default' : 'secondary'}>
                          {s.status}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </div>
        ) : (
          <div className="text-sm" style={{ color: colors.wineTextMuted }}>
            {t('admin.dashboard.loadBalancer.unavailable')}
          </div>
        )}
      </div>
    </div>
  );
}
