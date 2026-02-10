import { Button } from '@/components/ui/button';
import type { ActionHistory } from '@/types/action-history';
import { format } from 'date-fns';
import { ptBR } from 'date-fns/locale';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { HistoryStatusBadge } from './history-status-badge';

const colors = {
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

const MOBILE_CARDS_LIMIT = 50;

interface HistoryMobileListProps {
  history: ActionHistory[];
  getActionTypeLabel: (actionType: string) => string;
}

/**
 * Mobile-optimized card list for action history with pagination.
 * Shows first 50 items by default with option to show all.
 */
export function HistoryMobileList({ history, getActionTypeLabel }: HistoryMobileListProps) {
  const { t } = useTranslation();
  const [showAll, setShowAll] = useState(false);

  const itemsToShow = showAll ? history : history.slice(0, MOBILE_CARDS_LIMIT);
  const hasMore = history.length > MOBILE_CARDS_LIMIT;

  return (
    <div className="space-y-4">
      {itemsToShow.map((item) => (
        <div
          key={item.id}
          className="rounded-xl border p-4 space-y-3 overflow-hidden"
          style={{ backgroundColor: '#ffffff', borderColor: colors.wineBorder }}
        >
          <div className="flex items-start justify-between gap-2">
            <div className="flex-1 min-w-0">
              <h3 className="font-medium text-sm truncate">
                {getActionTypeLabel(item.action_type)}
              </h3>
              <p className="text-xs mt-1" style={{ color: colors.wineTextMuted }}>
                {format(new Date(item.created_at), 'dd/MM/yyyy HH:mm', {
                  locale: ptBR,
                })}
              </p>
            </div>
            <HistoryStatusBadge status={item.status} />
          </div>

          <div className="space-y-2 text-sm">
            <div>
              <span className="text-xs" style={{ color: colors.wineTextMuted }}>
                {t('pages.history.table.columns.description')}:
              </span>
              <p className="text-xs break-words mt-1">{item.description}</p>
            </div>

            {item.target_count > 0 && (
              <div className="flex items-center gap-4 text-xs pt-2 border-t">
                <div>
                  <span style={{ color: colors.wineTextMuted }}>
                    {t('pages.history.table.total')}
                  </span>{' '}
                  <span className="font-medium">{item.target_count}</span>
                </div>
                {item.success_count > 0 && (
                  <div className="text-green-600">
                    <span aria-hidden="true">✓</span> {item.success_count}
                  </div>
                )}
                {item.error_count > 0 && (
                  <div className="text-red-600">
                    <span aria-hidden="true">✗</span> {item.error_count}
                  </div>
                )}
                {item.completed_at && (
                  <div style={{ color: colors.wineTextMuted }}>
                    <span aria-hidden="true">⏱️</span>{' '}
                    {Math.round(
                      (new Date(item.completed_at).getTime() -
                        new Date(item.created_at).getTime()) /
                        1000
                    )}
                    s
                  </div>
                )}
              </div>
            )}

            {item.error_message && (
              <div className="pt-2 border-t">
                <span className="text-xs" style={{ color: colors.wineTextMuted }}>
                  {t('pages.history.table.error')}
                </span>
                <p className="text-destructive text-xs break-words mt-1">{item.error_message}</p>
              </div>
            )}
          </div>
        </div>
      ))}

      {hasMore && !showAll && (
        <Button variant="outline" className="w-full" onClick={() => setShowAll(true)}>
          {t('common.showMore')} ({history.length - MOBILE_CARDS_LIMIT} {t('common.more')})
        </Button>
      )}
    </div>
  );
}
