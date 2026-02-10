import type { ActionHistoryStatus, StatusVariant } from '@/types/action-history';
import { CheckCircle2, Clock, Loader2, XCircle } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const colors = {
  wine: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

interface HistoryStatusBadgeProps {
  status: ActionHistoryStatus;
}

/**
 * Reusable status badge component for action history entries.
 * Displays appropriate icon and styling based on status.
 */
export function HistoryStatusBadge({ status }: HistoryStatusBadgeProps) {
  const { t } = useTranslation();

  const variants: Record<ActionHistoryStatus, StatusVariant & { bg: string; color: string }> = {
    pending: {
      label: t('pages.history.status.pending'),
      className: '',
      icon: Clock,
      bg: 'rgba(141, 45, 83, 0.06)',
      color: colors.wineTextLight,
    },
    running: {
      label: t('pages.history.status.running'),
      className: 'animate-pulse',
      icon: Loader2,
      bg: colors.iconBg,
      color: colors.wine,
    },
    completed: {
      label: t('pages.history.status.completed'),
      className: '',
      icon: CheckCircle2,
      bg: 'rgba(34, 197, 94, 0.1)',
      color: '#16a34a',
    },
    failed: {
      label: t('pages.history.status.failed'),
      className: '',
      icon: XCircle,
      bg: 'rgba(239, 68, 68, 0.1)',
      color: '#dc2626',
    },
  };

  const variant = variants[status] || variants.pending;
  const Icon = variant.icon;

  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${variant.className}`}
      style={{ backgroundColor: variant.bg, color: variant.color }}
    >
      <Icon className="mr-1 h-3 w-3" aria-hidden="true" />
      {variant.label}
    </span>
  );
}
