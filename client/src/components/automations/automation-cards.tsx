import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { formatDelayCompact, getModeKey, getModeVariant } from '@/lib/automation-formatters';
import type { Automation } from '@/types/automation';
import { format } from 'date-fns';
import { Edit, FileText, Play, RotateCcw, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const colors = {
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

interface AutomationCardsProps {
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
 * Mobile cards view for automations list
 */
export function AutomationCards({
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
}: AutomationCardsProps) {
  const { t } = useTranslation();

  return (
    <div className="md:hidden space-y-3">
      {automations.map((auto) => (
        <div
          key={auto.id}
          className="rounded-xl xs:rounded-2xl border p-3 xs:p-4 space-y-3"
          style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
        >
          <div className="flex items-start justify-between gap-2">
            <div className="flex-1 min-w-0">
              <h3 className="font-semibold text-sm xs:text-base mb-2 break-words">{auto.name}</h3>
              <div className="flex flex-wrap gap-2 text-xs">
                <Badge variant={getModeVariant(auto.mode)} className="text-xs">
                  {t(`pages.offerAutomations.tabs.${getModeKey(auto.mode)}`)}
                </Badge>
                <span style={{ color: colors.wineTextMuted }}>
                  {t('pages.offerAutomations.table.storeCount', {
                    count: auto.stores.length,
                  })}
                </span>
                <span style={{ color: colors.wineTextMuted }}>{formatDelayCompact(auto)}</span>
              </div>
              <p className="text-xs mt-2" style={{ color: colors.wineTextMuted }}>
                {auto.start_time} - {auto.end_time}
              </p>
              {auto.mode !== 'monitor' && auto.next_run_at && (
                <p className="text-xs" style={{ color: colors.wineTextMuted }}>
                  {t('pages.offerAutomations.table.nextRunLabel')}{' '}
                  {format(new Date(auto.next_run_at), 'dd/MM HH:mm')}
                </p>
              )}
            </div>
            <Switch
              checked={auto.status === 'active'}
              onCheckedChange={() => onToggleStatus(auto.id, auto.status)}
            />
          </div>

          <div
            className="flex gap-1 xs:gap-2 pt-2 border-t flex-wrap"
            style={{ borderColor: colors.wineBorder }}
          >
            <Button variant="outline" size="sm" onClick={() => onEdit(auto)} className="flex-1 min-w-0 xs:min-w-[100px]">
              <Edit className="h-3 w-3 mr-1" />
              {t('pages.offerAutomations.actions.editShort')}
            </Button>
            {auto.mode === 'search' && (
              <>
                {auto.stores.includes('shopee') && (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => onPreview(auto)}
                    disabled={loadingPreview}
                    className="flex-1 min-w-0 xs:min-w-[100px]"
                  >
                    <FileText className="h-3 w-3 mr-1" />
                    {t('pages.offerAutomations.actions.previewShort')}
                  </Button>
                )}
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => onRunNow(auto)}
                  disabled={runningId === auto.id || auto.status !== 'active'}
                  className="flex-1 min-w-0 xs:min-w-[100px]"
                >
                  <Play
                    className={`h-3 w-3 mr-1 ${runningId === auto.id ? 'animate-pulse' : ''}`}
                  />
                  {t('pages.offerAutomations.actions.runNowShort')}
                </Button>
              </>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={() => onResetHistory(auto.id, auto.name)}
              disabled={resettingId === auto.id}
            >
              <RotateCcw className={`h-3 w-3 ${resettingId === auto.id ? 'animate-spin' : ''}`} />
            </Button>
            <Button variant="outline" size="sm" onClick={() => onDelete(auto.id)}>
              <Trash2 className="h-3 w-3" />
            </Button>
          </div>
        </div>
      ))}
    </div>
  );
}
