import { useVirtualizer } from '@tanstack/react-virtual';
import { useEffect, useRef } from 'react';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './table';

interface Column<T> {
  key: string;
  header: React.ReactNode;
  width?: string;
  render: (item: T, index: number) => React.ReactNode;
}

interface VirtualTableProps<T> {
  data: T[];
  columns: Column<T>[];
  getRowKey: (item: T, index: number) => string;
  estimateSize?: number;
  maxHeight?: number;
  className?: string;
  /** Called when scroll reaches near the end of the list */
  onEndReached?: () => void;
  /** Whether there are more items to load */
  hasMore?: boolean;
  /** Whether more items are currently being loaded */
  isLoadingMore?: boolean;
  /** Text to show when loading more items */
  loadingMoreText?: string;
}

/**
 * Tabela virtualizada usando @tanstack/react-virtual.
 * Renderiza apenas as linhas visíveis para melhor performance em listas grandes.
 * Suporta infinite scroll via onEndReached callback.
 */
export function VirtualTable<T>({
  data,
  columns,
  getRowKey,
  estimateSize = 48,
  maxHeight = 400,
  className,
  onEndReached,
  hasMore = false,
  isLoadingMore = false,
  loadingMoreText = 'Loading...',
}: VirtualTableProps<T>) {
  const parentRef = useRef<HTMLDivElement>(null);
  const lastTriggeredLength = useRef(0);

  const rowVirtualizer = useVirtualizer({
    count: data.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => estimateSize,
    overscan: 5,
  });

  const virtualItems = rowVirtualizer.getVirtualItems();

  console.log('[DEBUG VirtualTable] data.length:', data.length);
  console.log('[DEBUG VirtualTable] parentRef.current:', parentRef.current);
  console.log('[DEBUG VirtualTable] virtualItems.length:', virtualItems.length);
  console.log('[DEBUG VirtualTable] virtualItems:', virtualItems);

  // Infinite scroll: trigger onEndReached when near the bottom
  useEffect(() => {
    if (!onEndReached || !hasMore || isLoadingMore) return;

    const lastItem = virtualItems[virtualItems.length - 1];
    if (!lastItem) return;

    // Prevent multiple triggers for the same data length
    if (lastTriggeredLength.current >= data.length) return;

    // Trigger when last visible item is within 5 items of the end
    if (lastItem.index >= data.length - 5) {
      lastTriggeredLength.current = data.length;
      onEndReached();
    }
  }, [virtualItems, data.length, onEndReached, hasMore, isLoadingMore]);

  if (data.length === 0) {
    return null;
  }

  return (
    <div
      ref={parentRef}
      className={`overflow-auto border rounded-lg ${className ?? ''}`}
      style={{ height: maxHeight }}
    >
      <Table>
        <TableHeader className="sticky top-0 bg-background z-10">
          <TableRow>
            {columns.map((col) => (
              <TableHead key={col.key} style={{ width: col.width }}>
                {col.header}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {virtualItems.length > 0 && (
            <>
              {virtualItems[0].start > 0 && (
                <TableRow>
                  <TableCell colSpan={columns.length} style={{ height: virtualItems[0].start }} />
                </TableRow>
              )}
              {virtualItems.map((virtualRow) => {
                const item = data[virtualRow.index];
                return (
                  <TableRow
                    key={getRowKey(item, virtualRow.index)}
                    data-index={virtualRow.index}
                    ref={rowVirtualizer.measureElement}
                  >
                    {columns.map((col) => (
                      <TableCell key={col.key}>{col.render(item, virtualRow.index)}</TableCell>
                    ))}
                  </TableRow>
                );
              })}
              {virtualItems[virtualItems.length - 1].end < rowVirtualizer.getTotalSize() && (
                <TableRow>
                  <TableCell
                    colSpan={columns.length}
                    style={{
                      height:
                        rowVirtualizer.getTotalSize() - virtualItems[virtualItems.length - 1].end,
                    }}
                  />
                </TableRow>
              )}
              {isLoadingMore && (
                <TableRow>
                  <TableCell colSpan={columns.length} className="text-center py-4">
                    <span className="text-muted-foreground text-sm">{loadingMoreText}</span>
                  </TableCell>
                </TableRow>
              )}
            </>
          )}
        </TableBody>
      </Table>
    </div>
  );
}
