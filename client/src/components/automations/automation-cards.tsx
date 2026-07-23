import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import {
  formatDelayCompact,
  formatMonitorQueue,
  getModeKey,
  getModeVariant,
} from '@/lib/automation-formatters';
import type { Automation } from '@/types/automation';
import { format } from 'date-fns';
import { FileText, Play, RotateCcw, Trash2 } from 'lucide-react';
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
    <div className="md:hidden space-y-3 xs:space-y-4">
      {automations.map((auto) => (
        <div
          key={auto.id}
          className="rounded-xl xs:rounded-2xl border p-3 xs:p-4 sm:p-5 space-y-3"
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
              {formatMonitorQueue(auto, t) && (
                <p className="text-xs" style={{ color: colors.wineTextMuted }}>
                  {formatMonitorQueue(auto, t)}
                </p>
              )}
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
            className="flex flex-col xs:flex-row gap-2 pt-2 border-t"
            style={{ borderColor: colors.wineBorder }}
          >
            <button
              type="button"
              onClick={() => onEdit(auto)}
              className="flex-1 inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
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
                    className="flex-1 inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <FileText className="h-3 w-3" />
                    {t('pages.offerAutomations.actions.previewShort')}
                  </button>
                )}
                <button
                  type="button"
                  onClick={() => onRunNow(auto)}
                  disabled={runningId === auto.id || auto.status !== 'active'}
                  className="flex-1 inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Play className={`h-3 w-3 ${runningId === auto.id ? 'animate-pulse' : ''}`} />
                  {t('pages.offerAutomations.actions.runNowShort')}
                </button>
              </>
            )}
            <button
              type="button"
              onClick={() => onResetHistory(auto.id, auto.name)}
              disabled={resettingId === auto.id}
              className="inline-flex cursor-pointer items-center justify-center rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
            >
              <RotateCcw className={`h-3 w-3 ${resettingId === auto.id ? 'animate-spin' : ''}`} />
            </button>
            <button
              type="button"
              onClick={() => onDelete(auto.id)}
              className="inline-flex cursor-pointer items-center justify-center rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-red-300 bg-white text-destructive shadow-sm hover:-translate-y-0.5 hover:border-red-400 hover:shadow-md"
            >
              <Trash2 className="h-3 w-3" />
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
