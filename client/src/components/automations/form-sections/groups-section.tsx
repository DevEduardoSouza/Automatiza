import { Card, CardContent } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { MAX_SEND_GROUPS } from '@/config/automation-categories';
import { useToast } from '@/hooks/use-toast';
import type { Automation, AutomationGroup } from '@/types/automation';
import { Search, Star } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

interface GroupsListCheckboxProps {
  groups: AutomationGroup[];
  selectedIds: string[];
  onToggle: (waGroupId: string, checked: boolean) => void;
  onToggleFavorite: (groupId: string) => void;
  maxGroups?: number;
  disabled?: boolean;
  showSearch?: boolean;
}

export function GroupsListCheckbox({
  groups,
  selectedIds,
  onToggle,
  onToggleFavorite,
  maxGroups,
  disabled = false,
  showSearch = false,
}: GroupsListCheckboxProps) {
  const { t } = useTranslation();
  const [searchTerm, setSearchTerm] = useState('');

  const filteredGroups = useMemo(() => {
    if (!searchTerm.trim()) return groups;
    const lowerSearch = searchTerm.toLowerCase();
    return groups.filter((g) => g.name.toLowerCase().includes(lowerSearch));
  }, [groups, searchTerm]);

  return (
    <Card>
      <CardContent className="pt-4">
        {showSearch && (
          <div className="relative mb-3">
            <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder={t('pages.offerAutomations.form.searchGroups')}
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="pl-8"
            />
          </div>
        )}
        <div className="max-h-40 overflow-y-auto">
          {filteredGroups.map((group) => {
            const isSelected = selectedIds.includes(group.wa_group_id);
            const isDisabled =
              disabled ||
              (!isSelected && maxGroups !== undefined && selectedIds.length >= maxGroups);

            return (
              <div key={group.id} className="flex items-center space-x-2 mb-2 group/item">
                <Checkbox
                  checked={isSelected}
                  onCheckedChange={(checked) => onToggle(group.wa_group_id, Boolean(checked))}
                  disabled={isDisabled}
                />
                <Label className="cursor-pointer flex-1">{group.name}</Label>
                <button
                  type="button"
                  onClick={() => onToggleFavorite(group.id)}
                  className="opacity-50 hover:opacity-100 transition-opacity"
                  title={
                    group.is_favorite ? t('common.removeFromFavorites') : t('common.addToFavorites')
                  }
                >
                  <Star
                    className={`h-4 w-4 ${
                      group.is_favorite
                        ? 'fill-yellow-400 text-yellow-400'
                        : 'text-muted-foreground'
                    }`}
                  />
                </button>
              </div>
            );
          })}
          {filteredGroups.length === 0 && searchTerm && (
            <p className="text-sm text-muted-foreground text-center py-2">
              {t('common.noResults')}
            </p>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

interface SendGroupsSelectorProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  groups: AutomationGroup[];
  onToggleFavorite: (groupId: string) => Promise<void>;
}

export function SendGroupsSelector({
  formData,
  setFormData,
  groups,
  onToggleFavorite,
}: SendGroupsSelectorProps) {
  const { t } = useTranslation();
  const { toast } = useToast();

  const handleToggle = (waGroupId: string, checked: boolean) => {
    setFormData((prev) => {
      const current = prev.send_groups || [];
      const isSelected = current.includes(waGroupId);
      if (checked && !isSelected && current.length >= MAX_SEND_GROUPS) {
        toast({
          title: t('pages.offerAutomations.validation.groupsLimit'),
          description: t('pages.offerAutomations.validation.groupsLimitMessage', {
            max: MAX_SEND_GROUPS,
          }),
          variant: 'destructive',
        });
        return prev;
      }
      return {
        ...prev,
        send_groups: checked ? [...current, waGroupId] : current.filter((g) => g !== waGroupId),
      };
    });
  };

  return (
    <div className="space-y-2">
      <Label>
        {t('pages.offerAutomations.form.sendGroups')} *{' '}
        <span className="text-xs text-muted-foreground">
          ({formData.send_groups?.length || 0}/{MAX_SEND_GROUPS})
        </span>
      </Label>
      <p className="text-xs text-muted-foreground">
        {t('pages.offerAutomations.form.sendGroupsHint', { max: MAX_SEND_GROUPS })}
      </p>
      <GroupsListCheckbox
        groups={groups}
        selectedIds={formData.send_groups || []}
        onToggle={handleToggle}
        onToggleFavorite={onToggleFavorite}
        maxGroups={MAX_SEND_GROUPS}
        showSearch
      />
    </div>
  );
}

const MAX_MONITOR_GROUPS = 3;

interface MonitorGroupsSelectorProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  allGroups: AutomationGroup[];
  onToggleFavorite: (groupId: string) => Promise<void>;
}

export function MonitorGroupsSelector({
  formData,
  setFormData,
  allGroups,
  onToggleFavorite,
}: MonitorGroupsSelectorProps) {
  const { t } = useTranslation();
  const { toast } = useToast();

  const handleToggle = (waGroupId: string, checked: boolean) => {
    setFormData((prev) => {
      const current = prev.monitor_groups || [];
      const isSelected = current.includes(waGroupId);
      if (checked && !isSelected && current.length >= MAX_MONITOR_GROUPS) {
        toast({
          title: t('pages.offerAutomations.validation.maxMonitorGroups'),
          description: t('pages.offerAutomations.validation.maxMonitorGroupsMessage'),
          variant: 'destructive',
        });
        return prev;
      }
      const next = checked ? [...current, waGroupId] : current.filter((g) => g !== waGroupId);
      return { ...prev, monitor_groups: next };
    });
  };

  const selectedCount = formData.monitor_groups?.length || 0;

  return (
    <div className="space-y-2">
      <Label>
        {t('pages.offerAutomations.form.monitorGroups')}{' '}
        <span className="text-xs text-muted-foreground">
          ({selectedCount}/{MAX_MONITOR_GROUPS} {t('pages.offerAutomations.form.groupsSelected')})
        </span>
      </Label>
      <GroupsListCheckbox
        groups={allGroups}
        selectedIds={formData.monitor_groups || []}
        onToggle={handleToggle}
        onToggleFavorite={onToggleFavorite}
        maxGroups={MAX_MONITOR_GROUPS}
        showSearch
      />
    </div>
  );
}
