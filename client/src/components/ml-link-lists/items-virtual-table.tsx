import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { VirtualTable } from '@/components/ui/virtual-table';
import type { MlLinkListItem } from '@/entities/ml-link-list';
import { formatBRL } from '@/lib/formatters';
import { ArrowDown, ArrowUp, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const colors = {
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

interface ItemsVirtualTableProps {
  items: MlLinkListItem[];
  onMoveUp: (item: MlLinkListItem) => void;
  onMoveDown: (item: MlLinkListItem) => void;
  onToggleActive: (item: MlLinkListItem) => void;
  onRemove: (item: MlLinkListItem) => void;
  onCopyLink: (url: string) => void;
  /** Called when scroll reaches near the end of the list */
  onLoadMore?: () => void;
  /** Whether there are more items to load */
  hasMore?: boolean;
  /** Whether more items are currently being loaded */
  isLoadingMore?: boolean;
}

/**
 * Virtualized table for ML link list items.
 * Supports infinite scroll via onLoadMore callback.
 */
export function ItemsVirtualTable({
  items,
  onMoveUp,
  onMoveDown,
  onToggleActive,
  onRemove,
  onCopyLink,
  onLoadMore,
  hasMore = false,
  isLoadingMore = false,
}: ItemsVirtualTableProps) {
  const { t } = useTranslation();

  const columns = [
    {
      key: 'order',
      header: t('pages.mlLinkLists.items.table.order'),
      width: '56px',
      render: (item: MlLinkListItem) => (
        <div className="flex flex-col items-center gap-1">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => onMoveUp(item)}
            aria-label={t('pages.mlLinkLists.items.moveUp')}
          >
            <ArrowUp className="h-4 w-4" aria-hidden="true" />
          </Button>
          <div className="text-xs" style={{ color: colors.wineTextMuted }}>
            {item.order_index}
          </div>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => onMoveDown(item)}
            aria-label={t('pages.mlLinkLists.items.moveDown')}
          >
            <ArrowDown className="h-4 w-4" aria-hidden="true" />
          </Button>
        </div>
      ),
    },
    {
      key: 'product',
      header: t('pages.mlLinkLists.items.table.product'),
      width: '300px',
      render: (item: MlLinkListItem) => (
        <div className="flex gap-3">
          {item.product_image_url ? (
            <img src={item.product_image_url} alt="" className="h-10 w-10 xs:h-12 xs:w-12 rounded object-cover" />
          ) : (
            <div className="h-10 w-10 xs:h-12 xs:w-12 rounded" style={{ backgroundColor: colors.iconBg }} />
          )}
          <div className="min-w-0">
            <div className="text-xs xs:text-sm font-medium line-clamp-2">
              {item.product_title || item.origin_url}
            </div>
            <button
              type="button"
              className="text-xs underline"
              style={{ color: colors.wineTextMuted }}
              onClick={() => onCopyLink(item.origin_url)}
            >
              {t('pages.mlLinkLists.items.copyLink')}
            </button>
          </div>
        </div>
      ),
    },
    {
      key: 'price',
      header: t('pages.mlLinkLists.items.table.price'),
      width: '140px',
      render: (item: MlLinkListItem) => (
        <div className="text-xs xs:text-sm">
          {item.product_price != null && item.product_price > 0
            ? formatBRL(item.product_price)
            : ''}
        </div>
      ),
    },
    {
      key: 'status',
      header: t('pages.mlLinkLists.items.table.status'),
      width: '120px',
      render: (item: MlLinkListItem) => (
        <div className="flex flex-col gap-1">
          <Badge variant={item.is_active ? 'secondary' : 'outline'}>
            {item.is_active
              ? t('pages.mlLinkLists.items.statusActive')
              : t('pages.mlLinkLists.items.statusInactive')}
          </Badge>
          {item.consumed_at ? (
            <Badge variant="outline">{t('pages.mlLinkLists.items.statusSent')}</Badge>
          ) : (
            <Badge variant="secondary">{t('pages.mlLinkLists.items.statusAvailable')}</Badge>
          )}
        </div>
      ),
    },
    {
      key: 'actions',
      header: t('pages.mlLinkLists.items.table.actions'),
      width: '140px',
      render: (item: MlLinkListItem) => (
        <div className="flex justify-end gap-2">
          <Button variant="outline" size="sm" onClick={() => onToggleActive(item)}>
            {item.is_active
              ? t('pages.mlLinkLists.items.deactivate')
              : t('pages.mlLinkLists.items.activate')}
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={() => onRemove(item)}
            aria-label={t('common.delete')}
          >
            <Trash2 className="h-4 w-4" aria-hidden="true" />
          </Button>
        </div>
      ),
    },
  ];

  if (items.length === 0) {
    return null;
  }

  return (
    <VirtualTable
      data={items}
      columns={columns}
      getRowKey={(item) => item.id}
      estimateSize={80}
      className="h-[350px] xs:h-[400px] sm:h-[450px] lg:h-[500px] xl:h-[600px]"
      onEndReached={onLoadMore}
      hasMore={hasMore}
      isLoadingMore={isLoadingMore}
      loadingMoreText={t('common.loadingMore')}
    />
  );
}
