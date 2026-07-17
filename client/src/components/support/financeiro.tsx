import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from '@/components/ui/toast-utils';
import {
  CreateFinanceEntryRequestSchema,
  DeleteFinanceEntryRequestSchema,
  GetFinanceOverviewRequestSchema,
} from '@/gen/support/v1/support_pb';
import {
  createFinanceEntry,
  deleteFinanceEntry,
  getFinanceOverview,
} from '@/gen/support/v1/support-SupportService_connectquery';
import { create } from '@bufbuild/protobuf';
import { useMutation, useQuery } from '@connectrpc/connect-query';
import { AlertTriangle, Plus, Trash2, TrendingUp } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  CartesianGrid,
  Cell,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  XAxis,
  YAxis,
} from 'recharts';
import { FinanceEntryDialog, type NewFinanceEntry } from './finance-entry-dialog';

const CHART_COLORS = [
  'hsl(var(--chart-1))',
  'hsl(var(--chart-2))',
  'hsl(var(--chart-3))',
  'hsl(var(--chart-4))',
  'hsl(var(--chart-5))',
];

const pad = (n: number) => String(n).padStart(2, '0');
const fmtDate = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
const brl = (n: number) => (n ?? 0).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });
const pct = (n: number) => `${(n ?? 0).toFixed(1)}%`;

type Granularity = 'month' | 'quarter' | 'year';

interface ProjPoint {
  periodStart: string;
  label: string;
  revenue: number;
  expenses: number;
  net: number;
}

/**
 * Agrega a série mensal (36 pontos) em mês/trimestre/ano por período de
 * calendário — buckets parciais no início são somados corretamente.
 */
function aggregate(points: ProjPoint[], gran: Granularity) {
  if (gran === 'month') return points.slice(0, 12);
  const buckets = new Map<string, ProjPoint>();
  for (const p of points) {
    const year = p.periodStart.slice(0, 4);
    const month = Number(p.periodStart.slice(5, 7));
    const q = Math.floor((month - 1) / 3) + 1;
    const key = gran === 'quarter' ? `T${q}/${year}` : year;
    const cur = buckets.get(key);
    if (cur) {
      cur.revenue += p.revenue;
      cur.expenses += p.expenses;
      cur.net += p.net;
    } else {
      buckets.set(key, { ...p, label: key });
    }
  }
  return Array.from(buckets.values()).slice(0, gran === 'quarter' ? 8 : 3);
}

export function Financeiro() {
  const { t } = useTranslation();

  const now = new Date();
  const [from, setFrom] = useState(fmtDate(new Date(now.getFullYear(), now.getMonth(), 1)));
  const [to, setTo] = useState(fmtDate(new Date(now.getFullYear(), now.getMonth() + 1, 0)));
  const [gran, setGran] = useState<Granularity>('month');
  const [modalOpen, setModalOpen] = useState(false);

  const { data, isLoading, error, refetch } = useQuery(
    getFinanceOverview,
    create(GetFinanceOverviewRequestSchema, { from, to }),
    { staleTime: 0 }
  );

  const createMut = useMutation(createFinanceEntry);
  const deleteMut = useMutation(deleteFinanceEntry);

  const onCreate = async (e: NewFinanceEntry) => {
    await createMut.mutateAsync(create(CreateFinanceEntryRequestSchema, e));
    await refetch();
  };
  const onDelete = async (id: string) => {
    try {
      await deleteMut.mutateAsync(create(DeleteFinanceEntryRequestSchema, { id }));
      await refetch();
      toast.success(t('pages.support.financeiro.entryDeleted'));
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const k = data?.kpis;
  const projData = useMemo(
    () => aggregate((data?.projection ?? []) as ProjPoint[], gran),
    [data?.projection, gran]
  );
  const pieData = useMemo(
    () => (data?.planRevenue ?? []).map((p) => ({ name: p.plan, value: p.mrr })),
    [data?.planRevenue]
  );

  if (error) {
    return (
      <div className="mx-auto max-w-6xl p-4">
        <div className="flex items-center gap-2 rounded-lg border border-red-300 bg-red-50 p-4 text-sm text-red-700">
          <AlertTriangle className="h-4 w-4" /> {error.message}
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-6xl space-y-5 p-4">
      {/* Cabeçalho + filtro + ação */}
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight sm:text-2xl">
            {t('pages.support.financeiro.title')}
          </h1>
          <p className="text-sm text-muted-foreground">{t('pages.support.financeiro.subtitle')}</p>
        </div>
        <div className="flex flex-wrap items-end gap-2">
          <div className="flex flex-col">
            <span className="text-[10px] uppercase text-muted-foreground">
              {t('pages.support.financeiro.from')}
            </span>
            <Input
              type="date"
              value={from}
              onChange={(e) => setFrom(e.target.value)}
              className="h-8 w-[140px]"
            />
          </div>
          <div className="flex flex-col">
            <span className="text-[10px] uppercase text-muted-foreground">
              {t('pages.support.financeiro.to')}
            </span>
            <Input
              type="date"
              value={to}
              onChange={(e) => setTo(e.target.value)}
              className="h-8 w-[140px]"
            />
          </div>
          <Button size="sm" className="gap-1.5" onClick={() => setModalOpen(true)}>
            <Plus className="h-3.5 w-3.5" />
            {t('pages.support.financeiro.newEntry')}
          </Button>
        </div>
      </div>

      {isLoading || !k ? (
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: esqueleto estático
            <Skeleton key={i} className="h-24 rounded-xl" />
          ))}
        </div>
      ) : (
        <>
          {/* KPIs */}
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <StatCard
              label={t('pages.support.financeiro.kpi.mrr')}
              value={brl(k.mrrEstimated)}
              hint={
                <Badge variant="outline" className="text-[10px]">
                  {t('pages.support.financeiro.kpi.coverage', {
                    mapped: k.mappedSubscribers,
                    active: k.activeSubscribers,
                  })}
                </Badge>
              }
              estimate
            />
            <StatCard
              label={t('pages.support.financeiro.kpi.arr')}
              value={brl(k.arrEstimated)}
              estimate
            />
            <StatCard
              label={t('pages.support.financeiro.kpi.arpu')}
              value={brl(k.arpuEstimated)}
              estimate
            />
            <StatCard
              label={t('pages.support.financeiro.kpi.activeSubs')}
              value={String(k.activeSubscribers)}
              hint={
                <span className="text-[10px] text-muted-foreground">
                  {t('pages.support.financeiro.kpi.activeHint')}
                </span>
              }
            />
            <StatCard
              label={t('pages.support.financeiro.kpi.netResult')}
              value={brl(k.netResultPeriod)}
              danger={k.netResultPeriod < 0}
              positive={k.netResultPeriod >= 0}
            />
            <StatCard
              label={t('pages.support.financeiro.kpi.expenses')}
              value={brl(k.expensesPeriod)}
              danger={k.expensesPeriod > 0}
            />
            <StatCard
              label={t('pages.support.financeiro.kpi.margin')}
              value={pct(k.marginPercent)}
            />
            <StatCard
              label={t('pages.support.financeiro.kpi.churnRisk')}
              value={String(k.suspended + k.late)}
              hint={
                <span className="text-[10px] text-muted-foreground">
                  {t('pages.support.financeiro.kpi.churnHint', {
                    suspended: k.suspended,
                    late: k.late,
                    refunded: k.refunded,
                  })}
                </span>
              }
              danger={k.suspended + k.late > 0}
            />
          </div>

          {/* Gráficos */}
          <div className="grid gap-4 lg:grid-cols-3">
            {/* Projeção */}
            <div className="rounded-xl border p-4 lg:col-span-2">
              <div className="mb-3 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <TrendingUp className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm font-medium">
                    {t('pages.support.financeiro.projection.title')}
                  </span>
                </div>
                <div className="flex gap-1 rounded-lg bg-muted p-0.5 text-xs">
                  {(['month', 'quarter', 'year'] as Granularity[]).map((g) => (
                    <button
                      key={g}
                      type="button"
                      onClick={() => setGran(g)}
                      className={`rounded-md px-2 py-1 ${
                        gran === g ? 'bg-background font-medium shadow-sm' : 'text-muted-foreground'
                      }`}
                    >
                      {t(`pages.support.financeiro.projection.${g}`)}
                    </button>
                  ))}
                </div>
              </div>
              <ChartContainer
                config={{
                  revenue: {
                    label: t('pages.support.financeiro.projection.revenue'),
                    color: 'hsl(var(--chart-2))',
                  },
                  expenses: {
                    label: t('pages.support.financeiro.projection.expenses'),
                    color: 'hsl(var(--chart-4))',
                  },
                  net: {
                    label: t('pages.support.financeiro.projection.net'),
                    color: 'hsl(var(--chart-1))',
                  },
                }}
                className="h-[240px] w-full"
              >
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={projData}>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                    <XAxis dataKey="label" className="text-xs" />
                    <YAxis className="text-xs" width={48} tickFormatter={(v) => brl(Number(v))} />
                    <ChartTooltip content={<ChartTooltipContent />} />
                    <Line
                      type="monotone"
                      dataKey="revenue"
                      stroke="hsl(var(--chart-2))"
                      strokeWidth={2}
                      dot={false}
                    />
                    <Line
                      type="monotone"
                      dataKey="expenses"
                      stroke="hsl(var(--chart-4))"
                      strokeWidth={2}
                      dot={false}
                    />
                    <Line
                      type="monotone"
                      dataKey="net"
                      stroke="hsl(var(--chart-1))"
                      strokeWidth={2}
                      dot={false}
                    />
                  </LineChart>
                </ResponsiveContainer>
              </ChartContainer>
            </div>

            {/* Mix de plano (receita estimada) */}
            <div className="rounded-xl border p-4">
              <span className="text-sm font-medium">
                {t('pages.support.financeiro.planMix.title')}
              </span>
              {pieData.length === 0 ? (
                <p className="py-8 text-center text-xs text-muted-foreground">
                  {t('pages.support.financeiro.planMix.empty')}
                </p>
              ) : (
                <ChartContainer config={{}} className="h-[240px] w-full">
                  <ResponsiveContainer width="100%" height="100%">
                    <PieChart>
                      <ChartTooltip content={<ChartTooltipContent />} />
                      <Pie
                        data={pieData}
                        dataKey="value"
                        nameKey="name"
                        cx="50%"
                        cy="50%"
                        outerRadius={80}
                        label
                      >
                        {pieData.map((entry, i) => (
                          <Cell key={entry.name} fill={CHART_COLORS[i % CHART_COLORS.length]} />
                        ))}
                      </Pie>
                    </PieChart>
                  </ResponsiveContainer>
                </ChartContainer>
              )}
            </div>
          </div>

          {/* Receita por plano */}
          {(data?.planRevenue?.length ?? 0) > 0 && (
            <div className="rounded-xl border p-4">
              <span className="text-sm font-medium">
                {t('pages.support.financeiro.byPlan.title')}
              </span>
              <div className="mt-3 overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="text-xs text-muted-foreground">
                    <tr className="text-left">
                      <th className="pb-2">{t('pages.support.financeiro.byPlan.plan')}</th>
                      <th className="pb-2 text-right">
                        {t('pages.support.financeiro.byPlan.price')}
                      </th>
                      <th className="pb-2 text-right">
                        {t('pages.support.financeiro.byPlan.subs')}
                      </th>
                      <th className="pb-2 text-right">MRR</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data?.planRevenue.map((p) => (
                      <tr key={p.plan} className="border-t">
                        <td className="py-1.5">{p.plan}</td>
                        <td className="py-1.5 text-right">{brl(p.price)}</td>
                        <td className="py-1.5 text-right">{p.subscribers}</td>
                        <td className="py-1.5 text-right font-medium">{brl(p.mrr)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* Lançamentos manuais */}
          <div className="rounded-xl border p-4">
            <div className="mb-3 flex items-center justify-between">
              <span className="text-sm font-medium">
                {t('pages.support.financeiro.entries.title')}
              </span>
            </div>
            {(data?.entries?.length ?? 0) === 0 ? (
              <p className="py-6 text-center text-xs text-muted-foreground">
                {t('pages.support.financeiro.entries.empty')}
              </p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="text-xs text-muted-foreground">
                    <tr className="text-left">
                      <th className="pb-2">{t('pages.support.financeiro.entries.date')}</th>
                      <th className="pb-2">{t('pages.support.financeiro.entries.description')}</th>
                      <th className="pb-2">{t('pages.support.financeiro.entries.recurrence')}</th>
                      <th className="pb-2 text-right">
                        {t('pages.support.financeiro.entries.amount')}
                      </th>
                      <th className="pb-2" />
                    </tr>
                  </thead>
                  <tbody>
                    {data?.entries.map((e) => (
                      <tr key={e.id} className="border-t">
                        <td className="py-1.5 text-muted-foreground">{e.entryDate}</td>
                        <td className="py-1.5">
                          {e.description}
                          {e.category ? (
                            <span className="ml-1 text-xs text-muted-foreground">
                              · {e.category}
                            </span>
                          ) : null}
                        </td>
                        <td className="py-1.5 text-xs text-muted-foreground">
                          {e.recurring
                            ? e.recurrenceMonths
                              ? t('pages.support.financeiro.entries.recMonths', {
                                  n: e.recurrenceMonths,
                                })
                              : t('pages.support.financeiro.entries.recInfinite')
                            : t('pages.support.financeiro.entries.once')}
                        </td>
                        <td
                          className={`py-1.5 text-right font-medium ${
                            e.kind === 'expense' ? 'text-red-600' : 'text-emerald-600'
                          }`}
                        >
                          {e.kind === 'expense' ? '−' : '+'}
                          {brl(e.amount)}
                        </td>
                        <td className="py-1.5 text-right">
                          <button
                            type="button"
                            onClick={() => onDelete(e.id)}
                            className="text-muted-foreground hover:text-red-600"
                            aria-label={t('common.delete')}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          {/* Aviso honesto de cobertura */}
          {data?.coverageNote ? (
            <p className="flex items-start gap-2 text-xs text-muted-foreground">
              <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
              {data.coverageNote}
            </p>
          ) : null}
        </>
      )}

      <FinanceEntryDialog open={modalOpen} onOpenChange={setModalOpen} onSubmit={onCreate} />
    </div>
  );
}

function StatCard({
  label,
  value,
  hint,
  danger,
  positive,
  estimate,
}: {
  label: string;
  value: string;
  hint?: React.ReactNode;
  danger?: boolean;
  positive?: boolean;
  estimate?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <div className="rounded-xl border p-3">
      <div className="flex items-center gap-1 text-xs text-muted-foreground">
        {label}
        {estimate ? (
          <span className="text-[9px] uppercase tracking-wide text-amber-600">
            {t('pages.support.financeiro.estimate')}
          </span>
        ) : null}
      </div>
      <p
        className={`mt-1 text-xl font-semibold ${
          danger ? 'text-red-600' : positive ? 'text-emerald-600' : ''
        }`}
      >
        {value}
      </p>
      {hint ? <div className="mt-1">{hint}</div> : null}
    </div>
  );
}
