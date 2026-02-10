import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Edit, Trash2 } from 'lucide-react';
import type { Plan } from '../types/plans.types';

const colors = {
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

interface PlansTableProps {
  plans: Plan[];
  formatStorage: (bytes: number) => string;
  onEdit: (plan: Plan) => void;
  onDelete: (id: string) => void;
  t: (key: string) => string;
}

export function PlansTable({ plans, formatStorage, onEdit, onDelete, t }: PlansTableProps) {
  return (
    <div className="border rounded-lg">
      <Table aria-label={t('admin.plans.tableAriaLabel')}>
        <TableHeader>
          <TableRow>
            <TableHead>{t('admin.plans.columns.name')}</TableHead>
            <TableHead>{t('admin.plans.columns.monthlyPrice')}</TableHead>
            <TableHead>{t('admin.plans.columns.kiwifyIds')}</TableHead>
            <TableHead>{t('admin.plans.columns.groups')}</TableHead>
            <TableHead>{t('admin.plans.columns.storage')}</TableHead>
            <TableHead>{t('admin.plans.columns.status')}</TableHead>
            <TableHead>{t('admin.plans.columns.actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {plans.map((plan) => (
            <TableRow key={plan.id}>
              <TableCell className="font-medium">{plan.name}</TableCell>
              <TableCell>
                <PriceCell plan={plan} t={t} />
              </TableCell>
              <TableCell>
                <KiwifyIdsBadges plan={plan} t={t} />
              </TableCell>
              <TableCell>{plan.group_limit}</TableCell>
              <TableCell>{formatStorage(plan.storage_limit)}</TableCell>
              <TableCell>
                <StatusBadge isActive={plan.is_active} t={t} />
              </TableCell>
              <TableCell>
                <ActionButtons plan={plan} onEdit={onEdit} onDelete={onDelete} t={t} />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function PriceCell({ plan, t }: { plan: Plan; t: (key: string) => string }) {
  return (
    <>
      {plan.price_monthly ? `R$ ${plan.price_monthly.toFixed(2)}` : '-'}
      {plan.price_semester && (
        <span className="text-xs block" style={{ color: colors.wineTextMuted }}>
          {t('admin.plans.kiwifyLabels.semesterShort')}: R$ {plan.price_semester.toFixed(2)}
        </span>
      )}
      {plan.price_annual && (
        <span className="text-xs block" style={{ color: colors.wineTextMuted }}>
          {t('admin.plans.kiwifyLabels.annual')}: R$ {plan.price_annual.toFixed(2)}
        </span>
      )}
    </>
  );
}

export function KiwifyIdsBadges({ plan, t }: { plan: Plan; t: (key: string) => string }) {
  const hasAnyId =
    plan.kiwify_product_id_monthly ||
    plan.kiwify_product_id_semester ||
    plan.kiwify_product_id_annual;

  if (!hasAnyId) {
    return <span style={{ color: colors.wineTextMuted }}>{t('common.none')}</span>;
  }

  return (
    <div className="flex flex-col gap-1 text-xs">
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
    <div>
      <span style={{ color: colors.wineTextMuted }}>{label}:</span>{' '}
      <code className="px-1 rounded" style={{ backgroundColor: colors.wineBg }}>
        {value}
      </code>
    </div>
  );
}

export function StatusBadge({ isActive, t }: { isActive: boolean; t: (key: string) => string }) {
  return isActive ? (
    <Badge className="bg-green-500">{t('common.active')}</Badge>
  ) : (
    <Badge variant="secondary">{t('common.inactive')}</Badge>
  );
}

function ActionButtons({
  plan,
  onEdit,
  onDelete,
  t,
}: {
  plan: Plan;
  onEdit: (plan: Plan) => void;
  onDelete: (id: string) => void;
  t: (key: string, options?: Record<string, unknown>) => string;
}) {
  return (
    <div className="flex gap-2">
      <Button
        size="sm"
        variant="outline"
        onClick={() => onEdit(plan)}
        aria-label={t('admin.plans.editAriaLabel', { name: plan.name })}
      >
        <Edit className="h-4 w-4" />
      </Button>
      <Button
        size="sm"
        variant="outline"
        onClick={() => onDelete(plan.id)}
        aria-label={t('admin.plans.deleteAriaLabel', { name: plan.name })}
      >
        <Trash2 className="h-4 w-4" />
      </Button>
    </div>
  );
}
