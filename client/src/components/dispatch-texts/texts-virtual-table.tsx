import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { VirtualTable } from '@/components/ui/virtual-table';
import type { DispatchText } from '@/entities/dispatch-text';
import { Edit, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface TextsVirtualTableProps {
  items: DispatchText[];
  onEdit: (item: DispatchText) => void;
  onRemove: (item: DispatchText) => void;
}

/**
 * Virtualized table for dispatch texts.
 */
export function TextsVirtualTable({ items, onEdit, onRemove }: TextsVirtualTableProps) {
  const { t } = useTranslation();

  const columns = [
    {
      key: 'name',
      header: t('pages.dispatchTexts.table.name'),
      width: '200px',
      render: (item: DispatchText) => (
        <div className="flex items-center gap-2">
          <span className="font-medium">{item.name}</span>
          {item.copies?.length > 1 && (
            <span className="inline-flex items-center rounded-full bg-[#8D2D53]/10 px-2 py-0.5 text-[10px] font-medium text-[#8D2D53]">
              {t('pages.dispatchTexts.copies.count', { count: item.copies.length })}
            </span>
          )}
        </div>
      ),
    },
    {
      key: 'status',
      header: t('pages.dispatchTexts.table.status'),
      width: '100px',
      render: (item: DispatchText) => (
        <Badge
          variant={item.is_active ? 'secondary' : 'outline'}
          className={item.is_active ? 'bg-[#8D2D53] text-white hover:bg-[#8D2D53]/90' : ''}
        >
          {item.is_active ? t('common.active') : t('common.inactive')}
        </Badge>
      ),
    },
    {
      key: 'actions',
      header: t('pages.dispatchTexts.table.actions'),
      width: '200px',
      render: (item: DispatchText) => (
        <div className="flex justify-end gap-1.5 xs:gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => onEdit(item)}
            aria-label={t('pages.dispatchTexts.aria.editItem', {
              name: item.name,
            })}
          >
            <Edit className="h-4 w-4 xs:mr-1" aria-hidden="true" />
            <span className="hidden xs:inline">{t('common.edit')}</span>
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={() => onRemove(item)}
            aria-label={t('pages.dispatchTexts.aria.deleteItem', {
              name: item.name,
            })}
          >
            <Trash2 className="h-4 w-4 xs:mr-1" aria-hidden="true" />
            <span className="hidden xs:inline">{t('common.delete')}</span>
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
      estimateSize={56}
      maxHeight={500}
    />
  );
}
