import { Button } from '@/components/ui/button';
import type { ActionHistory } from '@/types/action-history';
import { format } from 'date-fns';
import { ptBR } from 'date-fns/locale';
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-react';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { HistoryStatusBadge } from './history-status-badge';

const colors = {
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

interface HistoryTableProps {
  history: ActionHistory[];
  // Pagination props
  currentPage: number;
  pageSize: number;
  totalCount: number;
  totalPages: number;
  onPageChange: (page: number) => void;
}

/**
 * Table for action history with server-side pagination.
 * Visual style matches GroupListTable pagination.
 */
export function HistoryTable({
  history,
  currentPage,
  pageSize,
  totalCount,
  totalPages,
  onPageChange,
}: HistoryTableProps) {
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

  // Calculate display range for pagination info
  const startItem = (currentPage - 1) * pageSize + 1;
  const endItem = Math.min(currentPage * pageSize, totalCount);

  const columns = useMemo(
    () => [
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
                  (new Date(item.completed_at).getTime() - new Date(item.created_at).getTime()) /
                    1000
                )}s`
              : '-'}
          </span>
        ),
      },
    ],
    [t]
  );

  return (
    <div>
      {/* Desktop Table */}
      <div className="hidden md:block">
        <table className="w-full">
          <thead>
            <tr className="border-b">
              {columns.map((col) => (
                <th
                  key={col.key}
                  className="h-12 px-4 text-left align-middle font-medium"
                  style={{ width: col.width, color: colors.wineTextMuted }}
                >
                  {col.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {history.map((item, index) => (
              <tr key={item.id} className="border-b transition-colors hover:bg-muted/50">
                {columns.map((col) => (
                  <td key={col.key} className="p-4 align-middle">
                    {col.render(item, index)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Mobile Cards */}
      <div className="md:hidden space-y-3">
        {history.map((item) => (
          <div
            key={item.id}
            className="rounded-xl border p-4 space-y-2 overflow-hidden"
            style={{ backgroundColor: '#ffffff', borderColor: colors.wineBorder }}
          >
            <div className="flex items-start justify-between">
              <div>
                <span className="font-medium">{getActionTypeLabel(item.action_type)}</span>
                <p className="text-xs mt-1" style={{ color: colors.wineTextMuted }}>
                  {format(new Date(item.created_at), 'dd/MM/yyyy HH:mm', { locale: ptBR })}
                </p>
              </div>
              <HistoryStatusBadge status={item.status} />
            </div>
            <p className="text-sm" style={{ color: colors.wineTextMuted }}>
              {item.description}
            </p>
            {item.error_message && <p className="text-xs text-destructive">{item.error_message}</p>}
            {item.target_count > 0 && (
              <div className="flex gap-3 text-xs">
                <span style={{ color: colors.wineTextMuted }}>Total: {item.target_count}</span>
                {item.success_count > 0 && (
                  <span className="text-green-600">✓ {item.success_count}</span>
                )}
                {item.error_count > 0 && <span className="text-red-600">✗ {item.error_count}</span>}
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Pagination Controls */}
      <div className="flex items-center justify-between gap-4 mt-4">
        {/* Page info - left side */}
        <div className="text-sm" style={{ color: colors.wineTextMuted }}>
          {totalCount > 0
            ? t('pages.history.pagination.showing', {
                start: startItem,
                end: endItem,
                total: totalCount,
                defaultValue: `Mostrando ${startItem}-${endItem} de ${totalCount} registros`,
              })
            : t('pages.history.table.empty')}
        </div>

        {/* Page navigation - right side */}
        <div className="flex items-center gap-1">
          {/* First page */}
          <Button
            variant="outline"
            size="icon"
            className="h-8 w-8"
            onClick={() => onPageChange(1)}
            disabled={currentPage === 1}
            aria-label={t('pages.history.pagination.first', { defaultValue: 'Primeira página' })}
          >
            <ChevronsLeft className="h-4 w-4" />
          </Button>

          {/* Previous page */}
          <Button
            variant="outline"
            size="icon"
            className="h-8 w-8"
            onClick={() => onPageChange(currentPage - 1)}
            disabled={currentPage === 1}
            aria-label={t('pages.history.pagination.previous', { defaultValue: 'Página anterior' })}
          >
            <ChevronLeft className="h-4 w-4" />
          </Button>

          {/* Page indicator */}
          <span className="px-3 text-sm" style={{ color: colors.wineTextMuted }}>
            {currentPage} / {totalPages || 1}
          </span>

          {/* Next page */}
          <Button
            variant="outline"
            size="icon"
            className="h-8 w-8"
            onClick={() => onPageChange(currentPage + 1)}
            disabled={currentPage === totalPages || totalPages === 0}
            aria-label={t('pages.history.pagination.next', { defaultValue: 'Próxima página' })}
          >
            <ChevronRight className="h-4 w-4" />
          </Button>

          {/* Last page */}
          <Button
            variant="outline"
            size="icon"
            className="h-8 w-8"
            onClick={() => onPageChange(totalPages)}
            disabled={currentPage === totalPages || totalPages === 0}
            aria-label={t('pages.history.pagination.last', { defaultValue: 'Última página' })}
          >
            <ChevronsRight className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
