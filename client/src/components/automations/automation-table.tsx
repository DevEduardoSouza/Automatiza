import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  formatDelayWithI18n,
  formatMonitorQueue,
  getModeKey,
  getModeVariant,
} from '@/lib/automation-formatters';
import type { Automation } from '@/types/automation';
import { format } from 'date-fns';
import { FileText, Play, RotateCcw, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const colors = {
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

interface AutomationTableProps {
  automations: Automation[];
  runningId: string | null;
  resettingId: string | null;
  loadingPreview: boolean;
  onEdit: (automation: Automation) => void;
  onToggleStatus: (id: string, currentStatus: 'active' | 'paused') => void;
  onPreview: (automation: Automation) => void;
  onRunNow: (automation: Automation) => void;
  onResetHistory: (id: string, name: string) => void;
  onDelete: (id: string) => void;
}

/**
 * Desktop table view for automations list
 */
export function AutomationTable({
  automations,
  runningId,
  resettingId,
  loadingPreview,
  onEdit,
  onToggleStatus,
  onPreview,
  onRunNow,
  onResetHistory,
  onDelete,
}: AutomationTableProps) {
  const { t } = useTranslation();

  return (
    <div className="hidden md:block">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('pages.offerAutomations.table.headers.name')}</TableHead>
            <TableHead>{t('pages.offerAutomations.table.headers.mode')}</TableHead>
            <TableHead>{t('pages.offerAutomations.table.headers.stores')}</TableHead>
            <TableHead>{t('pages.offerAutomations.table.headers.intervalDelay')}</TableHead>
            <TableHead>{t('pages.offerAutomations.table.headers.window')}</TableHead>
            <TableHead>{t('pages.offerAutomations.table.headers.status')}</TableHead>
            <TableHead>{t('pages.offerAutomations.table.headers.nextRun')}</TableHead>
            <TableHead>{t('pages.offerAutomations.table.headers.actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {automations.map((auto) => (
            <TableRow key={auto.id}>
              <TableCell className="font-medium">{auto.name}</TableCell>
              <TableCell>
                <Badge variant={getModeVariant(auto.mode)}>
                  {t(`pages.offerAutomations.tabs.${getModeKey(auto.mode)}`)}
                </Badge>
              </TableCell>
              <TableCell>
                {t('pages.offerAutomations.table.storeCount', {
                  count: auto.stores.length,
                })}
              </TableCell>
              <TableCell>
                <div className="flex flex-col">
                  <span>{formatDelayWithI18n(auto, t)}</span>
                  {formatMonitorQueue(auto, t) && (
                    <span className="text-xs text-muted-foreground">
                      {formatMonitorQueue(auto, t)}
                    </span>
                  )}
                </div>
              </TableCell>
              <TableCell className="text-xs xl:text-sm">
                {auto.start_time} - {auto.end_time}
              </TableCell>
              <TableCell>
                <Switch
                  checked={auto.status === 'active'}
                  onCheckedChange={() => onToggleStatus(auto.id, auto.status)}
                />
              </TableCell>
              <TableCell className="text-xs xl:text-sm">
                {auto.mode !== 'monitor' && auto.next_run_at
                  ? format(new Date(auto.next_run_at), 'dd/MM HH:mm')
                  : auto.mode === 'monitor'
                    ? t('pages.offerAutomations.table.reactive')
                    : '-'}
              </TableCell>
              <TableCell>
                <div className="flex items-center gap-1 xl:gap-2">
                  <button
                    type="button"
                    onClick={() => onEdit(auto)}
                    className="cursor-pointer rounded-full px-3 py-1.5 text-sm font-medium transition-colors hover:bg-accent"
                  >
                    {t('pages.offerAutomations.actions.editShort')}
                  </button>
                  {auto.mode === 'search' && (
                    <>
                      {auto.stores.includes('shopee') && (
                        <button
                          type="button"
                          onClick={() => onPreview(auto)}
                          disabled={loadingPreview}
                          className="cursor-pointer rounded-full p-1.5 transition-colors hover:bg-accent disabled:cursor-not-allowed disabled:opacity-50"
                          title={t('pages.offerAutomations.actions.preview')}
                        >
                          <FileText className="h-4 w-4" />
                        </button>
                      )}
                      <button
                        type="button"
                        onClick={() => onRunNow(auto)}
                        disabled={runningId === auto.id || auto.status !== 'active'}
                        className="cursor-pointer rounded-full p-1.5 transition-colors hover:bg-accent disabled:cursor-not-allowed disabled:opacity-50"
                        title={t('pages.offerAutomations.actions.runNow')}
                      >
                        <Play
                          className={`h-4 w-4 ${runningId === auto.id ? 'animate-pulse' : ''}`}
                        />
                      </button>
                    </>
                  )}
                  <button
                    type="button"
                    onClick={() => onResetHistory(auto.id, auto.name)}
                    disabled={resettingId === auto.id}
                    className="cursor-pointer rounded-full p-1.5 transition-colors hover:bg-accent disabled:cursor-not-allowed disabled:opacity-50"
                    title={t('pages.offerAutomations.actions.resetHistory')}
                  >
                    <RotateCcw
                      className={`h-4 w-4 ${resettingId === auto.id ? 'animate-spin' : ''}`}
                    />
                  </button>
                  <button
                    type="button"
                    onClick={() => onDelete(auto.id)}
                    className="cursor-pointer rounded-full p-1.5 text-destructive transition-colors hover:bg-destructive/10"
                    title={t('pages.offerAutomations.actions.delete')}
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
