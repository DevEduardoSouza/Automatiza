import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Edit, Trash2 } from 'lucide-react';
import type { Plan } from '../types/plans.types';

interface PlansMobileListProps {
  plans: Plan[];
  formatStorage: (bytes: number) => string;
  onEdit: (plan: Plan) => void;
  onDelete: (id: string) => void;
  t: (key: string) => string;
}

export function PlansMobileList({
  plans,
  formatStorage,
  onEdit,
  onDelete,
  t,
}: PlansMobileListProps) {
  return (
    <div className="space-y-3">
      {plans.map((plan) => (
        <Card key={plan.id} className="border">
          <CardContent className="p-3 space-y-3">
            <PlanHeader plan={plan} t={t} />
            <PlanDetails plan={plan} formatStorage={formatStorage} t={t} />
            <PlanActions plan={plan} onEdit={onEdit} onDelete={onDelete} t={t} />
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

function PlanHeader({ plan, t }: { plan: Plan; t: (key: string) => string }) {
  return (
    <div className="flex items-start justify-between gap-2">
      <div className="flex-1">
        <h3 className="font-medium text-sm">{plan.name}</h3>
        <p className="text-xs text-muted-foreground mt-1">
          {plan.price_monthly
            ? `R$ ${plan.price_monthly.toFixed(2)}/${t('admin.plans.perMonth')}`
            : t('admin.plans.noMonthlyPrice')}
        </p>
      </div>
      {plan.is_active ? (
        <Badge className="bg-green-500 text-xs">{t('common.active')}</Badge>
      ) : (
        <Badge variant="secondary" className="text-xs">
          {t('common.inactive')}
        </Badge>
      )}
    </div>
  );
}

function PlanDetails({
  plan,
  formatStorage,
  t,
}: {
  plan: Plan;
  formatStorage: (bytes: number) => string;
  t: (key: string) => string;
}) {
  return (
    <div className="space-y-2 text-xs pt-2 border-t">
      <div className="flex items-center gap-2">
        <span className="text-muted-foreground">{t('admin.plans.columns.groups')}:</span>
        <span className="font-medium">{plan.group_limit}</span>
      </div>
      <div className="flex items-center gap-2">
        <span className="text-muted-foreground">{t('admin.plans.columns.storage')}:</span>
        <span className="font-medium">{formatStorage(plan.storage_limit)}</span>
      </div>
      <KiwifyIdsMobileDisplay plan={plan} t={t} />
    </div>
  );
}

function KiwifyIdsMobileDisplay({ plan, t }: { plan: Plan; t: (key: string) => string }) {
  const hasAnyId =
    plan.kiwify_product_id_monthly ||
    plan.kiwify_product_id_semester ||
    plan.kiwify_product_id_annual;

  if (!hasAnyId) return null;

  return (
    <div className="pt-1">
      <span className="text-muted-foreground block mb-1">
        {t('admin.plans.columns.kiwifyIds')}:
      </span>
      {plan.kiwify_product_id_monthly && (
        <KiwifyIdRow
          label={t('admin.plans.kiwifyLabels.monthly')}
          value={plan.kiwify_product_id_monthly}
        />
      )}
      {plan.kiwify_product_id_semester && (
        <KiwifyIdRow
          label={t('admin.plans.kiwifyLabels.semester')}
          value={plan.kiwify_product_id_semester}
        />
      )}
      {plan.kiwify_product_id_annual && (
        <KiwifyIdRow
          label={t('admin.plans.kiwifyLabels.annual')}
          value={plan.kiwify_product_id_annual}
        />
      )}
    </div>
  );
}

function KiwifyIdRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="text-xs">
      <span className="text-muted-foreground">{label}:</span>{' '}
      <code className="bg-muted px-1 rounded">{value}</code>
    </div>
  );
}

function PlanActions({
  plan,
  onEdit,
  onDelete,
  t,
}: {
  plan: Plan;
  onEdit: (plan: Plan) => void;
  onDelete: (id: string) => void;
  t: (key: string) => string;
}) {
  return (
    <div className="flex gap-2 pt-2 border-t">
      <Button size="sm" variant="outline" onClick={() => onEdit(plan)} className="flex-1">
        <Edit className="h-3 w-3 mr-1" />
        {t('common.edit')}
      </Button>
      <Button size="sm" variant="outline" onClick={() => onDelete(plan.id)} className="flex-1">
        <Trash2 className="h-3 w-3 mr-1" />
        {t('common.delete')}
      </Button>
    </div>
  );
}
