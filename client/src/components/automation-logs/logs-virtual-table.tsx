import { Button } from '@/components/ui/button';
import { VirtualTable } from '@/components/ui/virtual-table';
import type { DispatchLog, DispatchStatus } from '@/types/dispatch-log';
import { STORES, isInstanceMissingError } from '@/types/dispatch-log';
import { format } from 'date-fns';
import { ExternalLink } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
  iconBgHover: 'rgba(141, 45, 83, 0.15)',
};

interface LogsVirtualTableProps {
  logs: DispatchLog[];
  getGroupName: (groupId: string, groupName?: string) => string;
}

/**
 * Tabela virtualizada para logs de automação.
 */
export function LogsVirtualTable({ logs, getGroupName }: LogsVirtualTableProps) {
  const { t } = useTranslation();

  const getProductHref = (log: DispatchLog): string => {
    if (log.affiliate_url) return log.affiliate_url;
    if (log.product_page_url) return log.product_page_url;
    return log.product_url;
  };

  const getStatusBadge = (status: DispatchStatus) => {
    const variants: Record<DispatchStatus, { label: string; bg: string; color: string }> = {
      queued: {
        label: t('pages.automationLogs.status.queued'),
        bg: 'rgba(141, 45, 83, 0.06)',
        color: colors.wineTextLight,
      },
      sent: {
        label: t('pages.automationLogs.status.sent'),
        bg: colors.iconBg,
        color: colors.wine,
      },
      skipped: {
        label: t('pages.automationLogs.status.skipped'),
        bg: 'rgba(234, 179, 8, 0.1)',
        color: '#a16207',
      },
      error: {
        label: t('pages.automationLogs.status.error'),
        bg: 'rgba(239, 68, 68, 0.1)',
        color: '#dc2626',
      },
    };
    const v = variants[status] || variants.queued;
    return (
      <span
        className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
        style={{ backgroundColor: v.bg, color: v.color }}
      >
        {v.label}
      </span>
    );
  };

  const columns = [
    {
      key: 'dateTime',
      header: t('pages.automationLogs.table.dateTime'),
      width: '120px',
      render: (log: DispatchLog) => (
        <span className="text-xs">{format(new Date(log.created_at), 'dd/MM/yyyy HH:mm')}</span>
      ),
    },
    {
      key: 'automation',
      header: t('pages.automationLogs.table.automation'),
      width: '150px',
      render: (log: DispatchLog) => <span className="font-medium">{log.automation_name}</span>,
    },
    {
      key: 'store',
      header: t('pages.automationLogs.table.store'),
      width: '100px',
      render: (log: DispatchLog) => (
        <span>{STORES.find((s) => s.value === log.store)?.label || log.store}</span>
      ),
    },
    {
      key: 'group',
      header: t('pages.automationLogs.table.group'),
      width: '150px',
      render: (log: DispatchLog) => (
        <span className="text-xs">{getGroupName(log.group_id, log.group_name)}</span>
      ),
    },
    {
      key: 'category',
      header: t('pages.automationLogs.table.category'),
      width: '100px',
      render: (log: DispatchLog) => <span className="text-xs">{log.product_category || '-'}</span>,
    },
    {
      key: 'product',
      header: t('pages.automationLogs.table.product'),
      width: '100px',
      render: (log: DispatchLog) => (
        <a
          href={getProductHref(log)}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-1 text-xs text-primary hover:underline"
        >
          {t('pages.automationLogs.table.viewProduct')}
          <ExternalLink className="h-3 w-3" aria-hidden="true" />
        </a>
      ),
    },
    {
      key: 'coupon',
      header: t('pages.automationLogs.table.coupon'),
      width: '150px',
      render: (log: DispatchLog) => {
        if (!log.coupon_id) {
          return <span style={{ color: colors.wineTextMuted }}>-</span>;
        }
        if (log.coupon_applied) {
          return (
            <span
              className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
              style={{ backgroundColor: colors.iconBg, color: colors.wine }}
            >
              {log.coupon_code || t('pages.automationLogs.table.coupon')} ✓
            </span>
          );
        }
        return (
          <div className="space-y-1">
            <span
              className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
              style={{ backgroundColor: 'rgba(141, 45, 83, 0.06)', color: colors.wineTextLight }}
            >
              {log.coupon_code || t('pages.automationLogs.table.coupon')} ✕
            </span>
            {log.coupon_skip_reason ? (
              <div
                className="text-xs truncate max-w-[150px]"
                style={{ color: colors.wineTextMuted }}
                title={log.coupon_skip_reason}
              >
                {log.coupon_skip_reason}
              </div>
            ) : null}
          </div>
        );
      },
    },
    {
      key: 'status',
      header: t('pages.automationLogs.table.status'),
      width: '100px',
      render: (log: DispatchLog) => getStatusBadge(log.status),
    },
    {
      key: 'error',
      header: t('pages.automationLogs.table.error'),
      width: '200px',
      render: (log: DispatchLog) => {
        if (!log.error) return <span>-</span>;
        return (
          <div
            className="flex items-center gap-2 text-xs max-w-[200px]"
            style={{ color: colors.wineTextMuted }}
          >
            <span className="truncate">{log.error}</span>
            {isInstanceMissingError(log.error) ? (
              <Button variant="link" className="h-auto p-0 text-xs" asChild>
                <Link to="/whatsapp">{t('pages.automationLogs.table.reconnect')}</Link>
              </Button>
            ) : null}
          </div>
        );
      },
    },
  ];

  if (logs.length === 0) {
    return null;
  }

  return (
    <VirtualTable
      data={logs}
      columns={columns}
      getRowKey={(log) => log.id}
      estimateSize={56}
      maxHeight={600}
    />
  );
}
