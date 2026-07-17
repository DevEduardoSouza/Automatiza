import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { ResumePausedButton } from '@/components/support/resume-paused-button';
import type { GetSystemHealthResponse } from '@/gen/support/v1/support_pb';
import { GetSystemHealthRequestSchema } from '@/gen/support/v1/support_pb';
import { getSystemHealth } from '@/gen/support/v1/support-SupportService_connectquery';
import { create } from '@bufbuild/protobuf';
import { useQuery } from '@connectrpc/connect-query';
import {
  AlertTriangle,
  CheckCircle2,
  Database,
  HardDrive,
  Layers,
  ListChecks,
  Send,
  Server,
  Smartphone,
  XCircle,
  Zap,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';

const POLL_MS = 20_000;

const TYPE_LABEL: Record<string, string> = {
  search: 'Busca',
  list: 'Lista',
  monitoring: 'Monitoramento',
  offer_generate: 'Espelhamento',
};

/** Semáforo geral → cor, ícone e texto de uma linha. */
const OVERALL: Record<string, { cls: string; Icon: typeof CheckCircle2; key: string }> = {
  ok: { cls: 'border-emerald-200 bg-emerald-50 text-emerald-900', Icon: CheckCircle2, key: 'ok' },
  atencao: { cls: 'border-amber-200 bg-amber-50 text-amber-900', Icon: AlertTriangle, key: 'warn' },
  critico: { cls: 'border-red-200 bg-red-50 text-red-900', Icon: XCircle, key: 'crit' },
};

function formatBytes(n: number | bigint): string {
  const b = Number(n);
  if (b <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.min(Math.floor(Math.log(b) / Math.log(1024)), units.length - 1);
  return `${(b / 1024 ** i).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

export function SystemDashboard() {
  const { t } = useTranslation();
  const { data, isLoading, error, dataUpdatedAt, refetch } = useQuery(
    getSystemHealth,
    create(GetSystemHealthRequestSchema, {}),
    { refetchInterval: POLL_MS, staleTime: 0 }
  );

  if (isLoading) {
    return (
      <div className="mx-auto w-full max-w-5xl space-y-4">
        <Skeleton className="h-16 w-full rounded-xl" />
        <div className="grid gap-3 sm:grid-cols-2">
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-40 rounded-xl" />
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="mx-auto w-full max-w-5xl">
        <p className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
          {error.message}
        </p>
      </div>
    );
  }

  const h = data as GetSystemHealthResponse;
  const overall = OVERALL[h.overall] ?? OVERALL.ok;
  const queuePending = (h.queues ?? []).reduce((sum, q) => sum + q.pending, 0);
  const queueActive = (h.queues ?? []).reduce((sum, q) => sum + q.active, 0);

  return (
    <div className="mx-auto w-full max-w-5xl space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="text-xl font-semibold tracking-tight sm:text-2xl">
            {t('pages.support.dash.title')}
          </h1>
          <p className="text-sm text-muted-foreground">{t('pages.support.dash.subtitle')}</p>
        </div>
        <div className="flex items-center gap-3">
          {dataUpdatedAt > 0 && (
            <span className="text-xs text-muted-foreground">
              {t('pages.support.dash.updatedAt', {
                time: new Date(dataUpdatedAt).toLocaleTimeString('pt-BR'),
              })}
            </span>
          )}
          <ResumePausedButton onDone={() => refetch()} />
        </div>
      </div>

      {/* Semáforo geral — a primeira coisa que o não técnico lê. */}
      <div className={`flex items-center gap-3 rounded-xl border p-4 ${overall.cls}`}>
        <overall.Icon className="h-6 w-6 shrink-0" />
        <div>
          <p className="font-semibold">{t(`pages.support.dash.overall.${overall.key}`)}</p>
          <p className="text-xs opacity-80">{t(`pages.support.dash.overallHint.${overall.key}`)}</p>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        {/* Sessões WhatsApp */}
        <Card icon={Smartphone} title={t('pages.support.dash.sessions')}>
          <BigStat
            value={`${h.waha?.sessionsConnected ?? 0}/${h.waha?.sessionsTotal ?? 0}`}
            label={t('pages.support.dash.sessionsConnected')}
          />
          <Meter value={h.waha?.sessionsConnected ?? 0} max={h.waha?.sessionsTotal ?? 0} good />
          <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
            <span className={(h.waha?.sessionsDisconnected ?? 0) > 0 ? 'text-red-600' : ''}>
              {t('pages.support.dash.disconnected', { n: h.waha?.sessionsDisconnected ?? 0 })}
            </span>
            <span>{t('pages.support.dash.pending', { n: h.waha?.sessionsPending ?? 0 })}</span>
          </div>
        </Card>

        {/* Capacidade dos servidores WAHA */}
        <Card icon={Server} title={t('pages.support.dash.capacity')}>
          {(h.waha?.servers ?? 0) > 0 ? (
            <>
              <BigStat
                value={`${(h.waha?.capacityPercent ?? 0).toFixed(0)}%`}
                label={t('pages.support.dash.capacityUsed')}
              />
              <Meter value={h.waha?.mapped ?? 0} max={h.waha?.capacity ?? 0} />
              <p className="mt-2 text-xs text-muted-foreground">
                {t('pages.support.dash.capacityDetail', {
                  mapped: h.waha?.mapped ?? 0,
                  capacity: h.waha?.capacity ?? 0,
                  servers: h.waha?.servers ?? 0,
                })}
              </p>
            </>
          ) : (
            <p className="py-4 text-sm text-muted-foreground">
              {t('pages.support.dash.capacityUnconfigured')}
            </p>
          )}
        </Card>

        {/* Redis */}
        <Card icon={Database} title={t('pages.support.dash.redis')}>
          {h.redis?.available ? (
            <>
              <BigStat
                value={`${(h.redis.usagePercent ?? 0).toFixed(1)}%`}
                label={t('pages.support.dash.memoryUsed')}
                danger={(h.redis.usagePercent ?? 0) > 90}
              />
              {Number(h.redis.maxMemoryBytes) > 0 && (
                <Meter value={Number(h.redis.usagePercent)} max={100} warnAt={75} critAt={90} />
              )}
              <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                <span>
                  {formatBytes(h.redis.usedMemoryBytes)}
                  {Number(h.redis.maxMemoryBytes) > 0
                    ? ` / ${formatBytes(h.redis.maxMemoryBytes)}`
                    : ` (${t('pages.support.dash.noLimit')})`}
                </span>
                <span>{t('pages.support.dash.clients', { n: h.redis.connectedClients })}</span>
                {Number(h.redis.evictedKeys) > 0 && (
                  <span className="text-red-600">
                    {t('pages.support.dash.evicted', { n: Number(h.redis.evictedKeys) })}
                  </span>
                )}
              </div>
            </>
          ) : (
            <Unavailable text={t('pages.support.dash.redisUnavailable')} />
          )}
        </Card>

        {/* Filas */}
        <Card icon={Layers} title={t('pages.support.dash.queues')}>
          <BigStat
            value={String(queuePending)}
            label={t('pages.support.dash.queuePending')}
            danger={queuePending > 5000}
          />
          <p className="mt-1 text-xs text-muted-foreground">
            {t('pages.support.dash.queueActive', { n: queueActive })}
          </p>
          {(h.queues ?? []).length > 0 ? (
            <div className="mt-2 flex flex-wrap gap-1.5">
              {h.queues.map((q) => (
                <Badge key={q.queue} variant="outline" className="font-mono text-[11px]">
                  {q.queue}: {q.pending}
                </Badge>
              ))}
            </div>
          ) : (
            <p className="mt-2 text-xs text-muted-foreground">
              {t('pages.support.dash.queueEmpty')}
            </p>
          )}
        </Card>

        {/* Automações por tipo — pausadas / com erro */}
        <Card icon={Zap} title={t('pages.support.dash.automations')} wide>
          <div className="space-y-1.5">
            <div className="grid grid-cols-5 gap-2 text-[11px] font-medium text-muted-foreground">
              <span className="col-span-2">{t('pages.support.dash.type')}</span>
              <span className="text-right">{t('pages.support.dash.active')}</span>
              <span className="text-right">{t('pages.support.dash.paused')}</span>
              <span className="text-right">{t('pages.support.dash.withError')}</span>
            </div>
            {(h.app?.automationTypes ?? []).length === 0 && (
              <p className="py-2 text-sm text-muted-foreground">
                {t('pages.support.dash.noAutomations')}
              </p>
            )}
            {(h.app?.automationTypes ?? []).map((a) => (
              <div key={a.type} className="grid grid-cols-5 items-center gap-2 text-sm">
                <span className="col-span-2 truncate">{TYPE_LABEL[a.type] ?? a.type}</span>
                <span className="text-right">{a.active}</span>
                <span
                  className={`text-right ${a.paused > 0 ? 'text-amber-600' : 'text-muted-foreground'}`}
                >
                  {a.paused}
                </span>
                <span
                  className={`text-right font-medium ${a.withError > 0 ? 'text-red-600' : 'text-muted-foreground'}`}
                >
                  {a.withError}
                </span>
              </div>
            ))}
          </div>
        </Card>

        {/* Listas de disparo */}
        <Card icon={ListChecks} title={t('pages.support.dash.lists')}>
          <BigStat
            value={String(h.app?.listsActive ?? 0)}
            label={t('pages.support.dash.listsActive')}
          />
          <p className="mt-1 text-xs text-muted-foreground">
            {t('pages.support.dash.listsDetail', {
              total: h.app?.listsTotal ?? 0,
              inactive: h.app?.listsInactive ?? 0,
            })}
          </p>
        </Card>

        {/* Disparos última hora */}
        <Card icon={Send} title={t('pages.support.dash.dispatch')}>
          <div className="flex items-end gap-6">
            <BigStat
              value={String(h.app?.sentLastHour ?? 0)}
              label={t('pages.support.dash.sent')}
            />
            <BigStat
              value={String(h.app?.failedLastHour ?? 0)}
              label={t('pages.support.dash.failed')}
              danger={(h.app?.failedLastHour ?? 0) > 0}
            />
          </div>
          <p className="mt-2 text-xs text-muted-foreground">
            {t('pages.support.dash.dispatchWindow')} ·{' '}
            {t('pages.support.dash.dbSize', {
              size: formatBytes(h.app?.dbSizeBytes ?? 0n),
            })}
          </p>
        </Card>

        {/* Host (VPS) — coletado pela sentinela */}
        <Card icon={HardDrive} title={t('pages.support.dash.host')}>
          {h.host?.available ? (
            <div className="grid grid-cols-3 gap-2 text-center">
              <HostStat label="CPU" value={`${(h.host.cpuPercent ?? 0).toFixed(0)}%`} />
              <HostStat label="Disco" value={`${(h.host.diskUsedPercent ?? 0).toFixed(0)}%`} />
              <HostStat label="Load" value={(h.host.loadAvg1m ?? 0).toFixed(2)} />
            </div>
          ) : (
            <Unavailable text={h.host?.note || t('pages.support.dash.hostUnavailable')} />
          )}
        </Card>
      </div>
    </div>
  );
}

function Card({
  icon: Icon,
  title,
  children,
  wide,
}: {
  icon: typeof Server;
  title: string;
  children: React.ReactNode;
  wide?: boolean;
}) {
  return (
    <div className={`rounded-xl border bg-background p-4 ${wide ? 'sm:col-span-2' : ''}`}>
      <div className="mb-2 flex items-center gap-2">
        <Icon className="h-4 w-4 text-muted-foreground" />
        <h2 className="text-sm font-semibold">{title}</h2>
      </div>
      {children}
    </div>
  );
}

function BigStat({
  value,
  label,
  danger,
}: {
  value: string;
  label: string;
  danger?: boolean;
}) {
  return (
    <div>
      <p className={`text-2xl font-semibold ${danger ? 'text-red-600' : ''}`}>{value}</p>
      <p className="text-xs text-muted-foreground">{label}</p>
    </div>
  );
}

/** Barra de progresso. `good` pinta de verde (mais é melhor); senão escala por
 *  limiar (verde→âmbar→vermelho conforme enche). */
function Meter({
  value,
  max,
  good,
  warnAt = 85,
  critAt = 95,
}: {
  value: number;
  max: number;
  good?: boolean;
  warnAt?: number;
  critAt?: number;
}) {
  const pct = max > 0 ? Math.min((value / max) * 100, 100) : 0;
  let color = 'bg-emerald-500';
  if (good) {
    // "Mais conectado = melhor": vermelho quando POUCO preenchido.
    if (pct < 40) color = 'bg-red-500';
    else if (pct < 70) color = 'bg-amber-500';
  } else if (pct >= critAt) color = 'bg-red-500';
  else if (pct >= warnAt) color = 'bg-amber-500';

  return (
    <div className="mt-2 h-2 w-full overflow-hidden rounded-full bg-muted">
      <div className={`h-full rounded-full ${color}`} style={{ width: `${pct}%` }} />
    </div>
  );
}

function HostStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border p-2">
      <p className="text-lg font-semibold">{value}</p>
      <p className="text-[10px] text-muted-foreground">{label}</p>
    </div>
  );
}

function Unavailable({ text }: { text: string }) {
  return (
    <div className="flex items-start gap-2 py-2 text-xs text-muted-foreground">
      <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
      <span>{text}</span>
    </div>
  );
}
