import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { AlertCircle, ArrowDown, ArrowUp, Check } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { DistributionStrategy, Group } from '../types';

const colors = {
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
};

interface PriorityMobileCardsProps {
  groups: Group[];
  distributionStrategy: DistributionStrategy;
  onMoveUp: (index: number) => void;
  onMoveDown: (index: number) => void;
  onClickLimitChange: (groupId: string, limit: number) => void;
}

export function PriorityMobileCards({
  groups,
  distributionStrategy,
  onMoveUp,
  onMoveDown,
  onClickLimitChange,
}: PriorityMobileCardsProps) {
  const { t } = useTranslation();

  return (
    <div className="md:hidden space-y-3">
      <SortableContext items={groups.map((g) => g.id)} strategy={verticalListSortingStrategy}>
        {groups.map((group, index) => (
          <div
            key={group.id}
            className="rounded-xl xs:rounded-2xl border p-3 xs:p-4 space-y-3"
            style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
          >
            <div className="flex items-start justify-between">
              <div className="flex-1 space-y-2">
                <div className="flex items-center gap-2">
                  <Badge variant="secondary" className="text-sm">
                    #{group.priority || index + 1}
                  </Badge>
                  <span className="font-semibold text-sm">{group.name}</span>
                </div>

                <div className="flex items-center gap-2">
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
                </div>

                {distributionStrategy === 'member_limit' && (
                  <p className="text-sm" style={{ color: colors.wineTextMuted }}>
                    {t('pages.redirectLink.priority.members')}: {group.members_count}/
                    {group.member_limit}
                  </p>
                )}

                {distributionStrategy === 'click_limit' && (
                  <div className="space-y-1">
                    <Label htmlFor={`limit-${group.id}`} className="text-xs">
                      {t('pages.redirectLink.priority.clickLimit')}
                    </Label>
                    <input
                      id={`limit-${group.id}`}
                      type="number"
                      min="1"
                      placeholder={t('pages.redirectLink.priority.clickLimitPlaceholder')}
                      value={group.click_limit || ''}
                      onChange={(e) => {
                        const newLimit = Number.parseInt(e.target.value, 10) || 0;
                        onClickLimitChange(group.id, newLimit);
                      }}
                      className="w-full rounded-2xl !border-2 !border-gray-300 bg-gray-50 px-4 py-2 text-xs xs:py-2.5 xs:text-sm lg:py-3 lg:text-base outline-none transition-all hover:!border-[#520B27] focus:!border-[#520B27] focus:ring-1 focus:ring-[#3a0718]/30"
                    />
                  </div>
                )}
              </div>
            </div>

            <div className="flex gap-2 pt-2 border-t" style={{ borderColor: colors.wineBorder }}>
              <Button
                variant="outline"
                size="sm"
                onClick={() => onMoveUp(index)}
                disabled={index === 0}
                className="flex-1"
              >
                <ArrowUp className="h-4 w-4 mr-1" />
                {t('pages.redirectLink.priority.up')}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => onMoveDown(index)}
                disabled={index === groups.length - 1}
                className="flex-1"
              >
                <ArrowDown className="h-4 w-4 mr-1" />
                {t('pages.redirectLink.priority.down')}
              </Button>
            </div>
          </div>
        ))}
      </SortableContext>
    </div>
  );
}
