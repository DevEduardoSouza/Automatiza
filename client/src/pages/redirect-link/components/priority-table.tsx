import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { AlertCircle, ArrowDown, ArrowUp, Check } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { DistributionStrategy, Group } from '../types';

interface PriorityTableProps {
  groups: Group[];
  distributionStrategy: DistributionStrategy;
  onMoveUp: (index: number) => void;
  onMoveDown: (index: number) => void;
  onClickLimitChange: (groupId: string, limit: number) => void;
}

export function PriorityTable({
  groups,
  distributionStrategy,
  onMoveUp,
  onMoveDown,
  onClickLimitChange,
}: PriorityTableProps) {
  const { t } = useTranslation();

  return (
    <div className="hidden md:block">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-[100px]">{t('pages.redirectLink.priority.order')}</TableHead>
            <TableHead>{t('pages.redirectLink.priority.group')}</TableHead>
            <TableHead>{t('pages.redirectLink.priority.whatsappLink')}</TableHead>
            {distributionStrategy === 'click_limit' && (
              <TableHead>{t('pages.redirectLink.priority.clickLimit')}</TableHead>
            )}
            {distributionStrategy === 'member_limit' && (
              <TableHead>{t('pages.redirectLink.priority.members')}</TableHead>
            )}
            <TableHead className="text-right">{t('pages.redirectLink.priority.actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <SortableContext items={groups.map((g) => g.id)} strategy={verticalListSortingStrategy}>
            {groups.map((group, index) => (
              <TableRow key={group.id}>
                <TableCell className="font-medium">#{group.priority || index + 1}</TableCell>
                <TableCell>{group.name}</TableCell>
                <TableCell>
                  {group.invite_code ? (
                    <Badge variant="outline" className="text-xs">
                      <Check className="mr-1 h-3 w-3" />
                      {t('pages.redirectLink.priority.configured')}
                    </Badge>
                  ) : (
                    <Badge variant="outline" className="text-xs text-orange-600">
                      <AlertCircle className="mr-1 h-3 w-3" />
                      {t('pages.redirectLink.priority.noLink')}
                    </Badge>
                  )}
                </TableCell>
                {distributionStrategy === 'click_limit' && (
                  <TableCell>
                    <Input
                      type="number"
                      min="1"
                      placeholder={t('pages.redirectLink.priority.clickLimitPlaceholder')}
                      value={group.click_limit || ''}
                      onChange={(e) => {
                        const newLimit = Number.parseInt(e.target.value, 10) || 0;
                        onClickLimitChange(group.id, newLimit);
                      }}
                      className="w-24"
                    />
                  </TableCell>
                )}
                {distributionStrategy === 'member_limit' && (
                  <TableCell>
                    {group.members_count}/{group.member_limit}
                  </TableCell>
                )}
                <TableCell className="text-right">
                  <div className="flex justify-end gap-1">
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => onMoveUp(index)}
                      disabled={index === 0}
                    >
                      <ArrowUp className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => onMoveDown(index)}
                      disabled={index === groups.length - 1}
                    >
                      <ArrowDown className="h-4 w-4" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </SortableContext>
        </TableBody>
      </Table>
    </div>
  );
}
