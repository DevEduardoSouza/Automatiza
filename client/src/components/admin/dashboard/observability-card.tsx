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

interface DispatchUser {
  userId: string;
  sent: number;
  error: number;
  skipped: number;
  queued: number;
}

interface TopUserByGroups {
  userId: string;
  automations: number;
  groupsSlots: number;
}

interface TopError {
  error: string;
  count: number;
}

interface ObservabilityData {
  generatedAt: string;
  queue: {
    waiting: number;
    delayed: number;
    active: number;
    queued: number;
    limitPerMinute: number | null;
    workerConcurrency: number | null;
    estimatedMinutes: number | null;
  };
  dispatchLogs: {
    windowMinutes: number;
    total: number;
    byStatus: Record<string, number>;
    topErrors: TopError[];
    topUsers: DispatchUser[];
  };
  totals: {
    usersWithAnyAutomation: number;
    usersWithActiveAutomations: number;
    automationsTotal: number;
    automationsActive: number;
    groupsTotal: number;
    avgGroupsPerAutomation: number;
  };
  topUsers: {
    byGroupsSlots: TopUserByGroups[];
  };
}

interface ObservabilityCardProps {
  data: ObservabilityData | null;
  loading: boolean;
  onRefresh: () => void;
}

/**
 * Card de observabilidade do sistema.
 * Exibe métricas de fila, envios, automações e grupos.
 */
export function ObservabilityCard({ data, loading, onRefresh }: ObservabilityCardProps) {
  const { t } = useTranslation();

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
            {t('admin.dashboard.observability.title')}
          </span>
          <p className="text-xs md:text-sm mt-0.5" style={{ color: colors.wineTextMuted }}>
            {t('admin.dashboard.observability.description')}
          </p>
        </div>
        <button
          type="button"
          onClick={onRefresh}
          disabled={loading}
          className="inline-flex cursor-pointer items-center justify-center gap-2 rounded-full border border-gray-300 bg-white px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold text-[#8D2D53] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
        >
          {loading ? (
            <>
              <Loader2 className="h-4 w-4 animate-spin" />
              {t('common.refreshing')}
            </>
          ) : (
            t('common.refresh')
          )}
        </button>
      </div>
      <div className="space-y-3 xs:space-y-4">
        {data ? (
          <div className="space-y-6">
            <div className="text-xs" style={{ color: colors.wineTextMuted }}>
              {t('admin.dashboard.observability.generatedAt')}{' '}
              <span className="text-foreground font-medium">
                {new Date(data.generatedAt).toLocaleString()}
              </span>
            </div>

            {/* Queue Stats */}
            <div className="grid gap-3 xs:gap-4 grid-cols-2 md:grid-cols-4">
              <div
                className="rounded-xl border p-3 xs:p-4"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="text-xs font-medium" style={{ color: colors.wineText }}>
                  {t('admin.dashboard.observability.queue.title')}
                </div>
                <div className="text-xl font-bold mt-1">{data.queue.queued}</div>
                <div className="text-xs mt-0.5" style={{ color: colors.wineTextMuted }}>
                  {t('admin.dashboard.observability.queue.waiting')} {data.queue.waiting} •{' '}
                  {t('admin.dashboard.observability.queue.delayed')} {data.queue.delayed}
                </div>
              </div>
              <div
                className="rounded-xl border p-3 xs:p-4"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="text-xs font-medium" style={{ color: colors.wineText }}>
                  {t('admin.dashboard.observability.workers.title')}
                </div>
                <div className="text-xl font-bold mt-1">{data.queue.active}</div>
                <div className="text-xs mt-0.5" style={{ color: colors.wineTextMuted }}>
                  {t('admin.dashboard.observability.workers.concurrency')}{' '}
                  {data.queue.workerConcurrency ?? '—'}
                </div>
              </div>
              <div
                className="rounded-xl border p-3 xs:p-4"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="text-xs font-medium" style={{ color: colors.wineText }}>
                  {t('admin.dashboard.observability.sendLimit.title')}
                </div>
                <div className="text-xl font-bold mt-1">{data.queue.limitPerMinute ?? '—'}</div>
                <div className="text-xs mt-0.5" style={{ color: colors.wineTextMuted }}>
                  {t('admin.dashboard.observability.sendLimit.perMinute')}
                </div>
              </div>
              <div
                className="rounded-xl border p-3 xs:p-4"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="text-xs font-medium" style={{ color: colors.wineText }}>
                  {t('admin.dashboard.observability.estimatedWait.title')}
                </div>
                <div className="text-xl font-bold mt-1">{data.queue.estimatedMinutes ?? '—'}</div>
                <div className="text-xs mt-0.5" style={{ color: colors.wineTextMuted }}>
                  {t('admin.dashboard.observability.estimatedWait.minutes')}
                </div>
              </div>
            </div>

            {/* Automation Stats */}
            <div className="grid gap-3 xs:gap-4 grid-cols-2 md:grid-cols-4">
              <div
                className="rounded-xl border p-3 xs:p-4"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="text-xs font-medium" style={{ color: colors.wineText }}>
                  {t('admin.dashboard.observability.automations.title')}
                </div>
                <div className="text-xl font-bold mt-1">{data.totals.automationsTotal}</div>
                <div className="text-xs mt-0.5" style={{ color: colors.wineTextMuted }}>
                  {t('admin.dashboard.observability.automations.active')}{' '}
                  {data.totals.automationsActive}
                </div>
              </div>
              <div
                className="rounded-xl border p-3 xs:p-4"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="text-xs font-medium" style={{ color: colors.wineText }}>
                  {t('admin.dashboard.observability.usersAutomations.title')}
                </div>
                <div className="text-xl font-bold mt-1">{data.totals.usersWithAnyAutomation}</div>
                <div className="text-xs mt-0.5" style={{ color: colors.wineTextMuted }}>
                  {t('admin.dashboard.observability.usersAutomations.withAutomation')} •{' '}
                  {t('admin.dashboard.observability.usersAutomations.active')}{' '}
                  {data.totals.usersWithActiveAutomations}
                </div>
              </div>
              <div
                className="rounded-xl border p-3 xs:p-4"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="text-xs font-medium" style={{ color: colors.wineText }}>
                  {t('admin.dashboard.observability.groups.title')}
                </div>
                <div className="text-xl font-bold mt-1">{data.totals.groupsTotal}</div>
                <div className="text-xs mt-0.5" style={{ color: colors.wineTextMuted }}>
                  {t('admin.dashboard.observability.groups.total')}
                </div>
              </div>
              <div
                className="rounded-xl border p-3 xs:p-4"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="text-xs font-medium" style={{ color: colors.wineText }}>
                  {t('admin.dashboard.observability.avgGroups.title')}
                </div>
                <div className="text-xl font-bold mt-1">{data.totals.avgGroupsPerAutomation}</div>
                <div className="text-xs mt-0.5" style={{ color: colors.wineTextMuted }}>
                  {t('admin.dashboard.observability.avgGroups.slots')}
                </div>
              </div>
            </div>

            {/* Dispatch Logs */}
            <div className="grid gap-3 xs:gap-4 grid-cols-1 md:grid-cols-2">
              <div
                className="rounded-xl border p-3 xs:p-4"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="text-sm font-medium" style={{ color: colors.wineText }}>
                  {t('admin.dashboard.observability.sends.title', {
                    minutes: data.dispatchLogs.windowMinutes,
                  })}
                </div>
                <p className="text-xs mt-0.5 mb-3" style={{ color: colors.wineTextMuted }}>
                  {t('admin.dashboard.observability.sends.total')}: {data.dispatchLogs.total}
                </p>
                <div className="space-y-3">
                  <div className="flex flex-wrap gap-2">
                    {Object.entries(data.dispatchLogs.byStatus || {}).map(([k, v]) => (
                      <Badge key={k} variant={k === 'error' ? 'destructive' : 'secondary'}>
                        {k}: {v}
                      </Badge>
                    ))}
                  </div>
                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>
                            {t('admin.dashboard.observability.topErrors.title')}
                          </TableHead>
                          <TableHead className="text-right">
                            {t('admin.dashboard.observability.topErrors.quantity')}
                          </TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {(data.dispatchLogs.topErrors || []).length ? (
                          data.dispatchLogs.topErrors.map((e) => (
                            <TableRow key={e.error}>
                              <TableCell
                                className="text-xs"
                                style={{ color: colors.wineTextMuted }}
                              >
                                {e.error}
                              </TableCell>
                              <TableCell className="text-right">{e.count}</TableCell>
                            </TableRow>
                          ))
                        ) : (
                          <TableRow>
                            <TableCell className="text-sm" style={{ color: colors.wineTextMuted }}>
                              {t('admin.dashboard.observability.topErrors.noErrors')}
                            </TableCell>
                            <TableCell className="text-right">—</TableCell>
                          </TableRow>
                        )}
                      </TableBody>
                    </Table>
                  </div>
                </div>
              </div>

              <div
                className="rounded-xl border p-3 xs:p-4"
                style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
              >
                <div className="text-sm font-medium" style={{ color: colors.wineText }}>
                  {t('admin.dashboard.observability.topUsers.title')}
                </div>
                <p className="text-xs mt-0.5 mb-3" style={{ color: colors.wineTextMuted }}>
                  {t('admin.dashboard.observability.topUsers.description', {
                    minutes: data.dispatchLogs.windowMinutes,
                  })}
                </p>
                <div className="space-y-4">
                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t('admin.dashboard.observability.topUsers.user')}</TableHead>
                          <TableHead className="text-right">
                            {t('admin.dashboard.observability.topUsers.sent')}
                          </TableHead>
                          <TableHead className="text-right">
                            {t('admin.dashboard.observability.topUsers.queued')}
                          </TableHead>
                          <TableHead className="text-right">
                            {t('admin.dashboard.observability.topUsers.skipped')}
                          </TableHead>
                          <TableHead className="text-right">
                            {t('admin.dashboard.observability.topUsers.error')}
                          </TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {(data.dispatchLogs.topUsers || []).length ? (
                          data.dispatchLogs.topUsers.map((u) => (
                            <TableRow key={u.userId}>
                              <TableCell className="font-mono text-xs">{u.userId}</TableCell>
                              <TableCell className="text-right">{u.sent}</TableCell>
                              <TableCell className="text-right">{u.queued}</TableCell>
                              <TableCell className="text-right">{u.skipped}</TableCell>
                              <TableCell className="text-right">{u.error}</TableCell>
                            </TableRow>
                          ))
                        ) : (
                          <TableRow>
                            <TableCell className="text-sm" style={{ color: colors.wineTextMuted }}>
                              {t('admin.dashboard.observability.topUsers.noData')}
                            </TableCell>
                            <TableCell className="text-right">—</TableCell>
                            <TableCell className="text-right">—</TableCell>
                            <TableCell className="text-right">—</TableCell>
                            <TableCell className="text-right">—</TableCell>
                          </TableRow>
                        )}
                      </TableBody>
                    </Table>
                  </div>

                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>
                            {t('admin.dashboard.observability.topByGroups.title')}
                          </TableHead>
                          <TableHead className="text-right">
                            {t('admin.dashboard.observability.topByGroups.automations')}
                          </TableHead>
                          <TableHead className="text-right">
                            {t('admin.dashboard.observability.topByGroups.groupsSlots')}
                          </TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {(data.topUsers.byGroupsSlots || []).length ? (
                          data.topUsers.byGroupsSlots.map((u) => (
                            <TableRow key={u.userId}>
                              <TableCell className="font-mono text-xs">{u.userId}</TableCell>
                              <TableCell className="text-right">{u.automations}</TableCell>
                              <TableCell className="text-right">{u.groupsSlots}</TableCell>
                            </TableRow>
                          ))
                        ) : (
                          <TableRow>
                            <TableCell className="text-sm" style={{ color: colors.wineTextMuted }}>
                              {t('admin.dashboard.observability.topByGroups.noData')}
                            </TableCell>
                            <TableCell className="text-right">—</TableCell>
                            <TableCell className="text-right">—</TableCell>
                          </TableRow>
                        )}
                      </TableBody>
                    </Table>
                  </div>
                </div>
              </div>
            </div>
          </div>
        ) : (
          <div className="text-sm" style={{ color: colors.wineTextMuted }}>
            {t('admin.dashboard.observability.unavailable')}
          </div>
        )}
      </div>
    </div>
  );
}
