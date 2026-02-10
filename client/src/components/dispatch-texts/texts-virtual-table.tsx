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
      render: (item: DispatchText) => <span className="font-medium">{item.name}</span>,
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
        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => onEdit(item)}
            aria-label={t('pages.dispatchTexts.aria.editItem', {
              name: item.name,
            })}
          >
            <Edit className="h-4 w-4 mr-1" aria-hidden="true" />
            {t('common.edit')}
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={() => onRemove(item)}
            aria-label={t('pages.dispatchTexts.aria.deleteItem', {
              name: item.name,
            })}
          >
            <Trash2 className="h-4 w-4 mr-1" aria-hidden="true" />
            {t('common.delete')}
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
