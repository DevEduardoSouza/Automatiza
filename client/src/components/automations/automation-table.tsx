import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { formatDelayWithI18n, getModeKey, getModeVariant } from '@/lib/automation-formatters';
import type { Automation } from '@/types/automation';
import { format } from 'date-fns';
import { Edit, FileText, Play, RotateCcw, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

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
              <TableCell>{formatDelayWithI18n(auto, t)}</TableCell>
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
                <div className="flex gap-1 xl:gap-2">
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => onEdit(auto)}
                    title={t('pages.offerAutomations.actions.edit')}
                  >
                    <Edit className="h-4 w-4" />
                  </Button>
                  {auto.mode === 'search' && (
                    <>
                      {auto.stores.includes('shopee') && (
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => onPreview(auto)}
                          disabled={loadingPreview}
                          title={t('pages.offerAutomations.actions.preview')}
                        >
                          <FileText className="h-4 w-4" />
                        </Button>
                      )}
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => onRunNow(auto)}
                        disabled={runningId === auto.id || auto.status !== 'active'}
                        title={t('pages.offerAutomations.actions.runNow')}
                      >
                        <Play
                          className={`h-4 w-4 ${runningId === auto.id ? 'animate-pulse' : ''}`}
                        />
                      </Button>
                    </>
                  )}
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => onResetHistory(auto.id, auto.name)}
                    disabled={resettingId === auto.id}
                    title={t('pages.offerAutomations.actions.resetHistory')}
                  >
                    <RotateCcw
                      className={`h-4 w-4 ${resettingId === auto.id ? 'animate-spin' : ''}`}
                    />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => onDelete(auto.id)}
                    title={t('pages.offerAutomations.actions.delete')}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
