import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useDebouncedValue } from '@/hooks/use-debounced-value';
import { useVirtualizer } from '@tanstack/react-virtual';
import { Search, Star } from 'lucide-react';
import { useMemo, useRef } from 'react';
import { useTranslation } from 'react-i18next';

interface Group {
  id: string;
  name: string;
  wa_group_id: string;
  is_favorite?: boolean;
}

interface GroupSelectorProps {
  groups: Group[];
  selectedGroups: string[];
  searchTerm: string;
  loading?: boolean;
  onSearchChange: (value: string) => void;
  onToggleGroup: (groupId: string) => void;
}

const VIRTUALIZATION_THRESHOLD = 50;
const ITEM_HEIGHT = 36;

/**
 * Seletor de grupos com busca debounced e virtualização condicional.
 */
export function GroupSelector({
  groups,
  selectedGroups,
  searchTerm,
  loading = false,
  onSearchChange,
  onToggleGroup,
}: GroupSelectorProps) {
  const { t } = useTranslation();
  const parentRef = useRef<HTMLDivElement>(null);
  const debouncedSearch = useDebouncedValue(searchTerm, 300);

  const filteredGroups = useMemo(() => {
    const searchLower = debouncedSearch.toLowerCase();
    return groups.filter((group) => {
      const matchesSearch = group.name.toLowerCase().includes(searchLower);
      const isSelected = selectedGroups.includes(group.id);
      // Always show selected groups, even if they don't match the search
      return matchesSearch || isSelected;
    });
  }, [groups, debouncedSearch, selectedGroups]);

  const shouldVirtualize = filteredGroups.length > VIRTUALIZATION_THRESHOLD;

  const virtualizer = useVirtualizer({
    count: filteredGroups.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => ITEM_HEIGHT,
    overscan: 5,
    enabled: shouldVirtualize,
  });

  const renderGroupItem = (group: Group) => {
    const isSelected = selectedGroups.includes(group.id);
    const matchesSearch =
      !debouncedSearch || group.name.toLowerCase().includes(debouncedSearch.toLowerCase());
    const isSelectedButFiltered = isSelected && !matchesSearch;

    return (
      <div
        className={`flex items-center space-x-2 cursor-pointer rounded-md py-1 px-2 -mx-2 transition-colors hover:bg-accent ${isSelectedButFiltered ? 'bg-primary/5' : ''}`}
        onClick={() => onToggleGroup(group.id)}
      >
        <Checkbox
          id={`group-${group.id}`}
          checked={isSelected}
          className="cursor-pointer"
        />
        <Label
          className="text-xs xs:text-sm font-normal cursor-pointer flex items-center gap-1.5 xs:gap-2 flex-1"
        >
          {group.is_favorite && <Star className="h-3 w-3 fill-[#8D2D53] text-[#8D2D53]" />}
          <span className={isSelectedButFiltered ? 'text-muted-foreground' : ''}>{group.name}</span>
          {isSelectedButFiltered && (
            <span className="text-xs text-primary ml-auto">
              ({t('pages.groups.bulkActions.selected')})
            </span>
          )}
        </Label>
      </div>
    );
  };

  return (
    <div className="space-y-2">
      <Label className="cursor-default">{t('pages.jobs.form.selectGroups')}</Label>
      <div className="relative mb-2">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input
          placeholder={t('pages.jobs.form.searchGroups')}
          value={searchTerm}
          onChange={(e) => onSearchChange(e.target.value)}
          className="pl-9 cursor-text"
        />
      </div>
      <div ref={parentRef} className="border rounded-lg p-2 xs:p-3 sm:p-4 space-y-1.5 xs:space-y-2 max-h-36 xs:max-h-44 sm:max-h-48 lg:max-h-64 xl:max-h-72 2xl:max-h-80 overflow-y-auto">
        {loading ? (
          <div className="flex items-center justify-center py-4">
            <div className="h-5 w-5 animate-spin rounded-full border-2 border-primary border-t-transparent" />
            <span className="ml-2 text-sm text-muted-foreground">{t('common.loading')}</span>
          </div>
        ) : groups.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-4">
            {t('pages.jobs.form.noGroups')}
          </p>
        ) : filteredGroups.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-4">{t('common.noResults')}</p>
        ) : shouldVirtualize ? (
          <div
            style={{
              height: `${virtualizer.getTotalSize()}px`,
              width: '100%',
              position: 'relative',
            }}
          >
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const group = filteredGroups[virtualRow.index];
              return (
                <div
                  key={group.id}
                  style={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    width: '100%',
                    height: `${virtualRow.size}px`,
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                >
                  {renderGroupItem(group)}
                </div>
              );
            })}
          </div>
        ) : (
          filteredGroups.map((group) => <div key={group.id}>{renderGroupItem(group)}</div>)
        )}
      </div>
      <p className="text-xs text-muted-foreground">
        {selectedGroups.length} {t('pages.jobs.form.groupsSelected')}
      </p>
    </div>
  );
}
