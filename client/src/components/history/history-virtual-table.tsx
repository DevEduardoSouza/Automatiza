import { VirtualTable } from '@/components/ui/virtual-table';
import type { ActionHistory } from '@/types/action-history';
import { format } from 'date-fns';
import { ptBR } from 'date-fns/locale';
import { useTranslation } from 'react-i18next';
import { HistoryStatusBadge } from './history-status-badge';

const colors = {
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

interface HistoryVirtualTableProps {
  history: ActionHistory[];
}

/**
 * Virtualized table for action history.
 * Optimized for displaying large lists with smooth scrolling.
 */
export function HistoryVirtualTable({ history }: HistoryVirtualTableProps) {
  const { t } = useTranslation();

  const getActionTypeLabel = (actionType: string) => {
    const labels: Record<string, string> = {
      close_groups: t('pages.history.actions.closeGroups'),
      open_groups: t('pages.history.actions.openGroups'),
      send_message: t('pages.history.actions.sendMessage'),
      change_name: t('pages.history.actions.changeName'),
      change_photo: t('pages.history.actions.changePhoto'),
      change_description: t('pages.history.actions.changeDescription'),
      import_groups: t('pages.history.actions.importGroups'),
      message_sent: t('pages.history.actions.sendMessage'),
      automation_run: t('pages.history.actions.automationRun'),
      coupon_used: t('pages.history.actions.couponUsed'),
      link_clicked: t('pages.history.actions.linkClicked'),
      group_sync: t('pages.history.actions.importGroups'),
      error: t('pages.history.actions.error'),
      send_campaign: t('pages.history.actions.sendCampaign'),
      generate_image: t('pages.history.actions.generateImage'),
      add_member: t('pages.history.actions.addMember'),
      remove_member: t('pages.history.actions.removeMember'),
      promote_member: t('pages.history.actions.promoteMember'),
      demote_member: t('pages.history.actions.demoteMember'),
      leave_group: t('pages.history.actions.leaveGroup'),
    };
    return labels[actionType] || actionType;
  };

  const columns = [
    {
      key: 'dateTime',
      header: t('pages.history.table.columns.dateTime'),
      width: '140px',
      render: (item: ActionHistory) => (
        <span className="font-mono text-sm">
          {format(new Date(item.created_at), 'dd/MM/yyyy HH:mm', {
            locale: ptBR,
          })}
        </span>
      ),
    },
    {
      key: 'action',
      header: t('pages.history.table.columns.action'),
      width: '150px',
      render: (item: ActionHistory) => (
        <span className="font-medium">{getActionTypeLabel(item.action_type)}</span>
      ),
    },
    {
      key: 'description',
      header: t('pages.history.table.columns.description'),
      width: '250px',
      render: (item: ActionHistory) => (
        <div className="max-w-xs truncate">
          {item.description}
          {item.error_message && (
            <p className="text-xs text-destructive mt-1">{item.error_message}</p>
          )}
        </div>
      ),
    },
    {
      key: 'status',
      header: t('pages.history.table.columns.status'),
      width: '120px',
      render: (item: ActionHistory) => <HistoryStatusBadge status={item.status} />,
    },
    {
      key: 'results',
      header: t('pages.history.table.columns.results'),
      width: '100px',
      render: (item: ActionHistory) => (
        <div className="text-right">
          {item.target_count > 0 && (
            <div className="space-y-1">
              <div className="text-xs" style={{ color: colors.wineTextMuted }}>
                {t('pages.history.table.total')} {item.target_count}
              </div>
              {item.success_count > 0 && (
                <div className="text-xs text-green-600">
                  <span aria-hidden="true">✓</span> {item.success_count}
                </div>
              )}
              {item.error_count > 0 && (
                <div className="text-xs text-red-600">
                  <span aria-hidden="true">✗</span> {item.error_count}
                </div>
              )}
            </div>
          )}
        </div>
      ),
    },
    {
      key: 'duration',
      header: t('pages.history.table.columns.duration'),
      width: '80px',
      render: (item: ActionHistory) => (
        <span className="text-xs" style={{ color: colors.wineTextMuted }}>
          {item.completed_at
            ? `${Math.round(
                (new Date(item.completed_at).getTime() - new Date(item.created_at).getTime()) / 1000
              )}s`
            : '-'}
        </span>
      ),
    },
  ];

  if (history.length === 0) {
    return null;
  }

  return (
    <VirtualTable
      data={history}
      columns={columns}
      getRowKey={(item) => item.id}
      estimateSize={56}
      maxHeight={600}
    />
  );
}
