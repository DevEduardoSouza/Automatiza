import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { HistoryFilters as HistoryFiltersState } from '@/hooks/use-history';
import type { ActionHistoryStatus } from '@/types/action-history';
import { X } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface HistoryFiltersProps {
  filters: HistoryFiltersState;
  onFiltersChange: (filters: HistoryFiltersState) => void;
  onClearFilters: () => void;
}

const EVENT_TYPES = [
  'send_message',
  'close_groups',
  'open_groups',
  'change_name',
  'change_photo',
  'change_description',
  'import_groups',
  'automation_run',
  'send_campaign',
  'generate_image',
] as const;

const STATUSES: ActionHistoryStatus[] = ['pending', 'running', 'completed', 'failed'];

export function HistoryFilters({ filters, onFiltersChange, onClearFilters }: HistoryFiltersProps) {
  const { t } = useTranslation();

  const hasFilters = filters.eventType !== null || filters.status !== null;

  const handleEventTypeChange = (value: string) => {
    onFiltersChange({
      ...filters,
      eventType: value === 'all' ? null : value,
    });
  };

  const handleStatusChange = (value: string) => {
    onFiltersChange({
      ...filters,
      status: value === 'all' ? null : (value as ActionHistoryStatus),
    });
  };

  return (
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 xs:gap-4">
      <div className="space-y-2">
        <Label>{t('pages.history.filters.eventType')}</Label>
        <Select value={filters.eventType ?? 'all'} onValueChange={handleEventTypeChange}>
          <SelectTrigger>
            <SelectValue placeholder={t('pages.history.filters.eventType')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t('pages.history.filters.allTypes')}</SelectItem>
            {EVENT_TYPES.map((type) => (
              <SelectItem key={type} value={type}>
                {t(
                  `pages.history.actions.${type.replace(/_([a-z])/g, (_, c) => c.toUpperCase())}`,
                  type
                )}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="space-y-2">
        <Label>{t('pages.history.filters.status')}</Label>
        <Select value={filters.status ?? 'all'} onValueChange={handleStatusChange}>
          <SelectTrigger>
            <SelectValue placeholder={t('pages.history.filters.status')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t('pages.history.filters.allStatuses')}</SelectItem>
            {STATUSES.map((status) => (
              <SelectItem key={status} value={status}>
                {t(`pages.history.status.${status}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {hasFilters && (
        <div className="flex items-end">
          <button
            type="button"
            onClick={onClearFilters}
            className="inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:text-sm font-semibold transition-all border border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
          >
            <X className="h-3.5 w-3.5" />
            {t('pages.history.filters.clear')}
          </button>
        </div>
      )}
    </div>
  );
}
