import { Button } from '@/components/ui/button';
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
    <div className="flex flex-wrap items-center gap-2 mb-4">
      <Select value={filters.eventType ?? 'all'} onValueChange={handleEventTypeChange}>
        <SelectTrigger className="w-[180px]">
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

      <Select value={filters.status ?? 'all'} onValueChange={handleStatusChange}>
        <SelectTrigger className="w-[140px]">
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

      {hasFilters && (
        <Button variant="ghost" size="sm" onClick={onClearFilters}>
          <X className="h-4 w-4 mr-1" />
          {t('pages.history.filters.clear')}
        </Button>
      )}
    </div>
  );
}
