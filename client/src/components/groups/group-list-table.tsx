import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { VirtualTable } from '@/components/ui/virtual-table';
import type { LocalInstance } from '@/hooks/use-instances-connect';
import { cn } from '@/lib/utils';
import type { Group } from '@/types/group';
import {
  AlertCircle,
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  Check,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  List,
  Star,
  Users,
} from 'lucide-react';
import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

// Wine/burgundy color palette based on #8D2D53
const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

type SortKey = 'name' | 'members' | 'limit' | 'status' | null;
type SortDirection = 'asc' | 'desc';

/** Threshold for enabling virtualization */
const VIRTUALIZATION_THRESHOLD = 50;

const chipDotColors: Record<LocalInstance['status'], string> = {
  connected: '#22c55e',
  disconnected: '#ef4444',
  pending: '#f59e0b',
};

function formatPhoneShort(raw: string | undefined): string | null {
  if (!raw) return null;
  const digits = raw.replace(/\D/g, '');
  if (digits.length === 0) return null;
  if (digits.startsWith('55') && (digits.length === 12 || digits.length === 13)) {
    const ddd = digits.slice(2, 4);
    const rest = digits.slice(4);
    if (rest.length === 9) return `(${ddd}) ${rest.slice(0, 5)}-${rest.slice(5)}`;
    if (rest.length === 8) return `(${ddd}) ${rest.slice(0, 4)}-${rest.slice(4)}`;
  }
  return digits;
}

interface ChipOwnerBadgeProps {
  instance: LocalInstance;
}

/**
 * Compact pill shown next to a group's name to identify which WhatsApp
 * chip "owns" it (groups.instance_id). Only rendered when the user has
 * more than one chip — single-chip accounts get no clutter.
 *
 * Label fallback: friendly_name → formatted phone → truncated session id.
 * The raw session id reads as garbage to users, so we only use it as a
 * last resort when the chip has no phone yet.
 */
function ChipOwnerBadge({ instance }: ChipOwnerBadgeProps) {
  const friendly = instance.friendly_name?.trim();
  const phone = formatPhoneShort(instance.phone_number);
  const label =
    friendly ||
    phone ||
    (instance.instance_id.length > 16
      ? `${instance.instance_id.slice(0, 8)}…${instance.instance_id.slice(-4)}`
      : instance.instance_id);
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-[10px] font-medium"
      style={{
        backgroundColor: 'rgba(141, 45, 83, 0.06)',
        borderColor: 'rgba(141, 45, 83, 0.2)',
        color: 'rgba(141, 45, 83, 0.85)',
      }}
      title={label}
    >
      <span
        className="inline-block h-1.5 w-1.5 rounded-full shrink-0"
        style={{ backgroundColor: chipDotColors[instance.status] }}
        aria-hidden="true"
      />
      <span className="truncate max-w-[140px]">{label}</span>
    </span>
  );
}

interface GroupListTableProps {
  groups: Group[];
  filteredGroups: Group[];
  selectedGroups: string[];
  searchQuery: string;
  onToggleGroup: (id: string) => void;
  onToggleAll: () => void;
  onToggleFavorite: (groupId: string, currentFavorite: boolean) => void;
  onEditInviteCode: (group: Group) => void;
  onManageMembers: (group: { wa_group_id: string; name: string }) => void;
  // Multi-chip ownership: when supplied with >1 entries, each row gets a
  // ChipOwnerBadge so the user can tell which number a group came from
  // (especially useful when "Todos os números" is selected in the filter).
  instances?: LocalInstance[];
  // Pagination props (optional for backwards compatibility)
  usePagination?: boolean;
  currentPage?: number;
  pageSize?: number;
  totalCount?: number;
  totalPages?: number;
  hasMore?: boolean;
  onPageChange?: (page: number) => void;
  onPageSizeChange?: (size: number) => void;
}

/**
 * Tabela de listagem de grupos com versões desktop (virtualizada) e mobile.
 * Supports optional server-side pagination.
 */
export function GroupListTable({
  groups,
  filteredGroups,
  selectedGroups,
  searchQuery,
  onToggleGroup,
  onToggleAll,
  onToggleFavorite,
  onEditInviteCode,
  onManageMembers,
  instances,
  // Pagination props
  usePagination = false,
  currentPage = 1,
  pageSize = 20,
  totalCount,
  totalPages = 1,
  onPageChange,
  onPageSizeChange,
}: GroupListTableProps) {
  const { t } = useTranslation();
  const [sortKey, setSortKey] = useState<SortKey>(null);
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc');

  const handleSort = useCallback(
    (key: SortKey) => {
      if (sortKey === key) {
        setSortDirection((prev) => (prev === 'asc' ? 'desc' : 'asc'));
      } else {
        setSortKey(key);
        setSortDirection('asc');
      }
    },
    [sortKey]
  );

  const sortedGroups = useMemo(() => {
    // Always sort favorites first, then apply secondary sort
    return [...filteredGroups].sort((a, b) => {
      // Favorites always come first (pinned)
      if (a.is_favorite !== b.is_favorite) {
        return a.is_favorite ? -1 : 1;
      }

      // If no secondary sort key, sort by name
      if (!sortKey) {
        return a.name.localeCompare(b.name, 'pt-BR', { sensitivity: 'base' });
      }

      // Apply secondary sort
      let comparison = 0;
      switch (sortKey) {
        case 'name':
          comparison = a.name.localeCompare(b.name, 'pt-BR', { sensitivity: 'base' });
          break;
        case 'members':
          comparison = a.members_count - b.members_count;
          break;
        case 'limit':
          comparison = a.member_limit - b.member_limit;
          break;
        case 'status':
          comparison = a.status.localeCompare(b.status);
          break;
      }

      return sortDirection === 'asc' ? comparison : -comparison;
    });
  }, [filteredGroups, sortKey, sortDirection]);

  const SortIcon = useCallback(
    ({ columnKey }: { columnKey: SortKey }) => {
      if (sortKey !== columnKey) {
        return <ArrowUpDown className="ml-1 h-4 w-4 text-muted-foreground/50" />;
      }
      return sortDirection === 'asc' ? (
        <ArrowUp className="ml-1 h-4 w-4" />
      ) : (
        <ArrowDown className="ml-1 h-4 w-4" />
      );
    },
    [sortKey, sortDirection]
  );

  const SortableHeader = useCallback(
    ({ columnKey, label }: { columnKey: SortKey; label: string }) => (
      <button
        type="button"
        className="flex items-center hover:text-foreground transition-colors cursor-pointer"
        onClick={() => handleSort(columnKey)}
      >
        {label}
        <SortIcon columnKey={columnKey} />
      </button>
    ),
    [handleSort, SortIcon]
  );

  const getStatusBadge = useCallback(
    (status: string) => {
      const variants: Record<string, { label: string; className: string }> = {
        open: {
          label: t('pages.groups.status.open'),
          className: 'bg-gradient-success',
        },
        closed: {
          label: t('pages.groups.status.closed'),
          className: 'bg-destructive',
        },
        full: {
          label: t('pages.groups.status.full'),
          className: 'bg-primary',
        },
      };

      const variant = variants[status] || variants.open;
      return <Badge className={variant.className}>{variant.label}</Badge>;
    },
    [t]
  );

  const showChipBadges = (instances?.length ?? 0) > 1;
  const instanceById = useMemo(() => {
    const map = new Map<string, LocalInstance>();
    for (const inst of instances ?? []) map.set(inst.id, inst);
    return map;
  }, [instances]);

  const shouldVirtualize = sortedGroups.length > VIRTUALIZATION_THRESHOLD;

  // Calculate display range for pagination info
  const displayTotalCount = usePagination ? (totalCount ?? 0) : groups.length;
  const startItem = usePagination ? (currentPage - 1) * pageSize + 1 : 1;
  const endItem = usePagination
    ? Math.min(currentPage * pageSize, displayTotalCount)
    : filteredGroups.length;

  const columns = useMemo(
    () => [
      {
        key: 'checkbox',
        header: (
          <Checkbox
            checked={selectedGroups.length === filteredGroups.length && filteredGroups.length > 0}
            onCheckedChange={onToggleAll}
            aria-label={t('pages.groups.list.selectAll')}
            className="h-5 w-5"
          />
        ),
        width: '48px',
        render: (group: Group) => (
          <Checkbox
            checked={selectedGroups.includes(group.id)}
            onCheckedChange={() => onToggleGroup(group.id)}
            aria-label={t('pages.groups.list.selectGroup', {
              name: group.name,
            })}
            className="h-5 w-5"
          />
        ),
      },
      {
        key: 'favorite',
        header: '',
        width: '48px',
        render: (group: Group) => (
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            onClick={() => onToggleFavorite(group.id, group.is_favorite)}
            aria-label={
              group.is_favorite ? t('common.removeFromFavorites') : t('common.addToFavorites')
            }
          >
            <Star
              className={cn(
                'h-4 w-4',
                group.is_favorite ? 'fill-yellow-400 text-yellow-400' : 'text-muted-foreground'
              )}
              aria-hidden="true"
            />
          </Button>
        ),
      },
      {
        key: 'name',
        header: <SortableHeader columnKey="name" label={t('pages.groups.list.columns.name')} />,
        render: (group: Group) => {
          const owner =
            showChipBadges && group.instance_id ? instanceById.get(group.instance_id) : undefined;
          return (
            <div className="flex items-center gap-2 flex-wrap">
              <span className="font-medium">{group.name}</span>
              {owner && <ChipOwnerBadge instance={owner} />}
            </div>
          );
        },
      },
      {
        key: 'members',
        header: (
          <SortableHeader columnKey="members" label={t('pages.groups.list.columns.members')} />
        ),
        width: '120px',
        render: (group: Group) => group.members_count,
      },
      {
        key: 'limit',
        header: <SortableHeader columnKey="limit" label={t('pages.groups.list.columns.limit')} />,
        width: '120px',
        render: (group: Group) => group.member_limit,
      },
      {
        key: 'status',
        header: <SortableHeader columnKey="status" label={t('pages.groups.list.columns.status')} />,
        width: '120px',
        render: (group: Group) => getStatusBadge(group.status),
      },
      {
        key: 'inviteLink',
        header: t('pages.groups.list.columns.inviteLink'),
        width: '150px',
        render: (group: Group) =>
          group.invite_code ? (
            <Button
              variant="outline"
              size="sm"
              className="rounded-full text-[#8D2D53]"
              onClick={() => onEditInviteCode(group)}
            >
              <Check className="mr-2 h-4 w-4 text-green-600" aria-hidden="true" />
              {t('pages.groups.inviteCode.configured')}
            </Button>
          ) : (
            <Button
              variant="outline"
              size="sm"
              className="rounded-full text-orange-600 border-orange-300"
              onClick={() => onEditInviteCode(group)}
            >
              <AlertCircle className="mr-2 h-4 w-4" aria-hidden="true" />
              {t('pages.groups.inviteCode.configure')}
            </Button>
          ),
      },
      {
        key: 'manageMembers',
        header: t('pages.groups.list.columns.actions'),
        width: '120px',
        render: (group: Group) => (
          <Button
            variant="outline"
            size="sm"
            className="rounded-full text-[#8D2D53]"
            onClick={() =>
              onManageMembers({
                wa_group_id: group.wa_group_id,
                name: group.name,
              })
            }
          >
            <Users className="mr-2 h-4 w-4" aria-hidden="true" />
            {t('pages.groups.members.manage')}
          </Button>
        ),
      },
    ],
    [
      selectedGroups,
      filteredGroups.length,
      onToggleAll,
      onToggleGroup,
      onToggleFavorite,
      onEditInviteCode,
      onManageMembers,
      getStatusBadge,
      SortableHeader,
      showChipBadges,
      instanceById,
      t,
    ]
  );

  return (
    <div
      className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 lg:p-8 xl:p-10"
      style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
    >
      {/* Header */}
      <div className="mb-3 xs:mb-4">
        <div className="flex items-center gap-2 xs:gap-3 mb-1 xs:mb-2">
          <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
            <List className="w-4 h-4 xs:w-5 xs:h-5" style={{ color: colors.wine }} />
          </div>
          <span
            className="uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em]"
            style={{ color: colors.wineTextLight }}
          >
            {usePagination
              ? t('pages.groups.list.titlePaginated', {
                  start: startItem,
                  end: endItem,
                  total: displayTotalCount,
                  defaultValue: `Grupos (${startItem}-${endItem} de ${displayTotalCount})`,
                })
              : t('pages.groups.list.title', {
                  filtered: filteredGroups.length,
                  total: groups.length,
                })}
          </span>
        </div>
        <p className="text-xs xs:text-sm" style={{ color: colors.wineTextMuted }}>
          {groups.length === 0 && !usePagination
            ? t('pages.groups.list.emptyState')
            : searchQuery
              ? t('pages.groups.list.searchResults', {
                  count: usePagination ? displayTotalCount : filteredGroups.length,
                })
              : t('pages.groups.list.allGroups')}
        </p>
      </div>

      {/* Desktop Table - Virtualized for large lists */}
      <div className="hidden md:block">
        {sortedGroups.length === 0 ? (
          <div className="text-center py-8 lg:py-12" style={{ color: colors.wineTextMuted }}>
            {searchQuery ? t('pages.groups.list.noSearchResults') : t('pages.groups.list.noGroups')}
          </div>
        ) : shouldVirtualize && !usePagination ? (
          <VirtualTable
            data={sortedGroups}
            columns={columns}
            getRowKey={(group) => group.id}
            estimateSize={48}
            maxHeight={600}
          />
        ) : (
          <table className="w-full">
            <thead>
              <tr style={{ borderBottomColor: colors.wineBorder }} className="border-b">
                {columns.map((col) => (
                  <th
                    key={col.key}
                    className="h-12 px-4 lg:h-14 lg:px-5 xl:px-6 text-left align-middle font-medium"
                    style={{ width: col.width, color: colors.wineTextMuted }}
                  >
                    {col.header}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {sortedGroups.map((group) => (
                <tr
                  key={group.id}
                  className="border-b transition-colors hover:bg-[rgba(141,45,83,0.04)]"
                  style={{ borderBottomColor: colors.wineBorder }}
                >
                  {columns.map((col) => (
                    <td key={col.key} className="p-4 lg:p-5 xl:p-6 align-middle">
                      {col.render(group)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Mobile Cards */}
      <div className="md:hidden space-y-3 xs:space-y-4">
        {sortedGroups.length === 0 ? (
          <div className="text-center py-8 text-sm" style={{ color: colors.wineTextMuted }}>
            {searchQuery ? t('pages.groups.list.noSearchResults') : t('pages.groups.list.noGroups')}
          </div>
        ) : (
          sortedGroups.map((group) => {
            const owner =
              showChipBadges && group.instance_id
                ? instanceById.get(group.instance_id)
                : undefined;
            return (
            <div
              key={group.id}
              className="rounded-xl xs:rounded-2xl border p-3 xs:p-4 sm:p-5"
              style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
            >
              <div className="space-y-3 xs:space-y-4">
                <div className="flex items-start justify-between gap-2">
                  <div className="flex items-start gap-2 flex-1">
                    <Checkbox
                      checked={selectedGroups.includes(group.id)}
                      onCheckedChange={() => onToggleGroup(group.id)}
                      className="mt-1 h-5 w-5"
                      aria-label={t('pages.groups.list.selectGroup', {
                        name: group.name,
                      })}
                    />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-1 flex-wrap">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-6 w-6"
                          onClick={() => onToggleFavorite(group.id, group.is_favorite)}
                          aria-label={
                            group.is_favorite
                              ? t('common.removeFromFavorites')
                              : t('common.addToFavorites')
                          }
                        >
                          <Star
                            className={cn(
                              'h-4 w-4',
                              group.is_favorite
                                ? 'fill-yellow-400 text-yellow-400'
                                : 'text-muted-foreground'
                            )}
                            aria-hidden="true"
                          />
                        </Button>
                        <h3 className="font-semibold text-sm xs:text-base break-words">
                          {group.name}
                        </h3>
                        {owner && <ChipOwnerBadge instance={owner} />}
                      </div>
                      <div
                        className="flex flex-wrap gap-2 text-xs"
                        style={{ color: colors.wineTextMuted }}
                      >
                        <span>
                          {group.members_count}/{group.member_limit}
                        </span>
                        {getStatusBadge(group.status)}
                      </div>
                    </div>
                  </div>
                </div>

                <div
                  className="pt-2 xs:pt-3 flex flex-col xs:flex-row gap-2"
                  style={{ borderTopColor: colors.wineBorder, borderTopWidth: '1px' }}
                >
                  {group.invite_code ? (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => onEditInviteCode(group)}
                      className="w-full xs:w-auto flex-1 min-w-0 text-xs rounded-full text-[#8D2D53]"
                    >
                      <Check className="mr-2 h-3 w-3 text-green-600" aria-hidden="true" />
                      {t('pages.groups.inviteCode.configured')}
                    </Button>
                  ) : (
                    <Button
                      variant="outline"
                      size="sm"
                      className="w-full xs:w-auto flex-1 min-w-0 text-xs rounded-full text-orange-600 border-orange-300"
                      onClick={() => onEditInviteCode(group)}
                    >
                      <AlertCircle className="mr-2 h-3 w-3" aria-hidden="true" />
                      {t('pages.groups.inviteCode.configure')}
                    </Button>
                  )}
                  <Button
                    variant="outline"
                    size="sm"
                    className="w-full xs:w-auto flex-1 min-w-0 text-xs rounded-full text-[#8D2D53]"
                    onClick={() =>
                      onManageMembers({
                        wa_group_id: group.wa_group_id,
                        name: group.name,
                      })
                    }
                  >
                    <Users className="mr-2 h-3 w-3" aria-hidden="true" />
                    {t('pages.groups.members.manage')}
                  </Button>
                </div>
              </div>
            </div>
            );
          })
        )}
      </div>

      {/* Pagination Controls */}
      {usePagination && (
        <div className="flex flex-col xs:flex-row items-center justify-between gap-3 xs:gap-4 mt-4 lg:mt-6">
          {/* Page info - left side */}
          <div className="text-xs xs:text-sm" style={{ color: colors.wineTextMuted }}>
            {t('pages.groups.pagination.showing', {
              start: startItem,
              end: endItem,
              total: displayTotalCount,
              defaultValue: `Mostrando ${startItem}-${endItem} de ${displayTotalCount} grupos`,
            })}
          </div>

          {/* Page navigation - right side */}
          <div className="flex items-center gap-1 xs:gap-1.5">
            {/* First page */}
            <Button
              variant="outline"
              size="icon"
              className="h-8 w-8"
              onClick={() => onPageChange?.(1)}
              disabled={currentPage === 1}
              aria-label={t('pages.groups.pagination.first', { defaultValue: 'Primeira página' })}
            >
              <ChevronsLeft className="h-4 w-4" />
            </Button>

            {/* Previous page */}
            <Button
              variant="outline"
              size="icon"
              className="h-8 w-8"
              onClick={() => onPageChange?.(currentPage - 1)}
              disabled={currentPage === 1}
              aria-label={t('pages.groups.pagination.previous', {
                defaultValue: 'Página anterior',
              })}
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>

            {/* Next page */}
            <Button
              variant="outline"
              size="icon"
              className="h-8 w-8"
              onClick={() => onPageChange?.(currentPage + 1)}
              disabled={currentPage === totalPages || totalPages === 0}
              aria-label={t('pages.groups.pagination.next', { defaultValue: 'Próxima página' })}
            >
              <ChevronRight className="h-4 w-4" />
            </Button>

            {/* Last page */}
            <Button
              variant="outline"
              size="icon"
              className="h-8 w-8"
              onClick={() => onPageChange?.(totalPages)}
              disabled={currentPage === totalPages || totalPages === 0}
              aria-label={t('pages.groups.pagination.last', { defaultValue: 'Última página' })}
            >
              <ChevronsRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
