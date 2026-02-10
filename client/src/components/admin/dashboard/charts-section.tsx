import { ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart';
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

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

interface PlanStat {
  plan: string;
  count: number;
}

interface GrowthData {
  date: string;
  users: number;
}

interface ChartsSectionProps {
  planStats: PlanStat[];
  growthData: GrowthData[];
}

const PLAN_COLORS = {
  Free: 'hsl(var(--chart-1))',
  Starter: 'hsl(var(--chart-2))',
  Pro: 'hsl(var(--chart-3))',
  Master: 'hsl(var(--chart-4))',
} as const;

/**
 * Seção de gráficos do dashboard.
 * Exibe crescimento de usuários e distribuição por plano.
 */
export function ChartsSection({ planStats, growthData }: ChartsSectionProps) {
  const { t } = useTranslation();

  return (
    <div className="grid gap-3 xs:gap-4 grid-cols-1 md:grid-cols-2">
      <div
        className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <div className="mb-3 xs:mb-4">
          <span
            className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
            style={{ color: colors.wineText }}
          >
            {t('admin.dashboard.charts.userGrowth.title')}
          </span>
          <p className="text-xs md:text-sm mt-0.5" style={{ color: colors.wineTextMuted }}>
            {t('admin.dashboard.charts.userGrowth.last7Days')}
          </p>
        </div>
        <ChartContainer
          config={{
            users: {
              label: t('admin.dashboard.charts.userGrowth.users'),
              color: 'hsl(var(--chart-1))',
            },
          }}
          className="h-[160px] xs:h-[180px] md:h-[200px] lg:h-[240px] xl:h-[280px]"
        >
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={growthData}>
              <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
              <XAxis dataKey="date" className="text-xs" />
              <YAxis className="text-xs" />
              <ChartTooltip content={<ChartTooltipContent />} />
              <Line type="monotone" dataKey="users" stroke="hsl(var(--chart-1))" strokeWidth={2} />
            </LineChart>
          </ResponsiveContainer>
        </ChartContainer>
      </div>

      <div
        className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <div className="mb-3 xs:mb-4">
          <span
            className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
            style={{ color: colors.wineText }}
          >
            {t('admin.dashboard.charts.planDistribution.title')}
          </span>
          <p className="text-xs md:text-sm mt-0.5" style={{ color: colors.wineTextMuted }}>
            {t('admin.dashboard.charts.planDistribution.description')}
          </p>
        </div>
        <ChartContainer
          config={Object.fromEntries(
            planStats.map(({ plan }) => [
              plan.toLowerCase(),
              {
                label: plan,
                color: PLAN_COLORS[plan as keyof typeof PLAN_COLORS] || 'hsl(var(--chart-1))',
              },
            ])
          )}
          className="h-[160px] xs:h-[180px] md:h-[200px] lg:h-[240px] xl:h-[280px]"
        >
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <ChartTooltip content={<ChartTooltipContent />} />
              <Pie
                data={planStats}
                dataKey="count"
                nameKey="plan"
                cx="50%"
                cy="50%"
                outerRadius={80}
                label
              >
                {planStats.map((entry) => (
                  <Cell
                    key={`cell-${entry.plan}`}
                    fill={
                      PLAN_COLORS[entry.plan as keyof typeof PLAN_COLORS] || 'hsl(var(--chart-1))'
                    }
                  />
                ))}
              </Pie>
            </PieChart>
          </ResponsiveContainer>
        </ChartContainer>
      </div>
    </div>
  );
}
