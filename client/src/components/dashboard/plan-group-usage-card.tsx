import { Progress } from '@/components/ui/progress';
import { useProfilesConnect } from '@/hooks/use-profiles-connect';
import { Users } from 'lucide-react';
import { useTranslation } from 'react-i18next';

/**
 * Compact, presentational group-quota indicator: "used / limit" with a thin
 * progress bar. Pure (no data fetching) so it can be previewed/tested in
 * isolation. The bar turns amber near the limit and red when reached.
 */
export function PlanGroupUsage({ used, limit }: { used: number; limit: number }) {
  const { t } = useTranslation();
  const pct = limit > 0 ? Math.min(100, Math.round((used / limit) * 100)) : 0;
  const atLimit = used >= limit;
  const accent = atLimit ? 'text-destructive' : 'text-foreground';
  const bar = atLimit ? '[&>div]:bg-destructive' : '';

  return (
    <div className="inline-flex w-full max-w-xs flex-col gap-1.5 rounded-xl border bg-card px-4 py-3">
      <div className="flex items-center justify-between gap-3">
        <span className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
          <Users className="h-3.5 w-3.5" />
          {t('pages.dashboard.planGroups.title')}
        </span>
        <span className={`text-sm font-semibold tabular-nums ${accent}`}>
          {used}
          <span className="font-normal text-muted-foreground">/{limit}</span>
        </span>
      </div>
      <Progress value={pct} className={`h-1.5 bg-muted ${bar}`} />
      {atLimit && (
        <span className="text-[11px] leading-tight text-destructive">
          {t('pages.dashboard.planGroups.full')}
        </span>
      )}
    </div>
  );
}

/**
 * Dashboard container: fetches the user's limits and renders the compact
 * indicator. Only shown for tiered plans (group_limit > 0); legacy/unlimited
 * plans render nothing.
 */
export function PlanGroupUsageCard() {
  const { limits, loading } = useProfilesConnect();
  const limit = limits?.group_limit ?? 0;
  if (loading || limit <= 0) {
    return null;
  }
  return <PlanGroupUsage used={limits?.groups_used ?? 0} limit={limit} />;
}
