import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { useProfilesConnect } from '@/hooks/use-profiles-connect';
import { Users } from 'lucide-react';
import { useTranslation } from 'react-i18next';

/**
 * Shows the user's plan group quota as "used / limit" with a progress bar.
 * Only rendered for tiered plans (group_limit > 0); legacy/unlimited plans
 * (group_limit === 0) render nothing. Mirrors the backend's per-plan count
 * (distinct destination groups across active automations).
 */
export function PlanGroupUsageCard() {
  const { t } = useTranslation();
  const { limits, loading } = useProfilesConnect();

  const limit = limits?.group_limit ?? 0;
  if (loading || limit <= 0) {
    return null;
  }

  const used = limits?.groups_used ?? 0;
  const pct = Math.min(100, Math.round((used / limit) * 100));
  const atLimit = used >= limit;
  const nearLimit = used >= limit - 1;
  const numberColor = atLimit
    ? 'text-destructive'
    : nearLimit
      ? 'text-amber-500'
      : 'text-foreground';

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-base">
          <Users className="h-4 w-4" />
          {t('pages.dashboard.planGroups.title')}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        <div className="flex items-baseline justify-between">
          <span className={`text-2xl font-semibold ${numberColor}`}>
            {used}
            <span className="text-base font-normal text-muted-foreground"> / {limit}</span>
          </span>
          <span className="text-sm text-muted-foreground">{t('pages.dashboard.planGroups.groups')}</span>
        </div>
        <Progress value={pct} />
        {atLimit && (
          <p className="text-sm text-destructive">{t('pages.dashboard.planGroups.full')}</p>
        )}
      </CardContent>
    </Card>
  );
}
