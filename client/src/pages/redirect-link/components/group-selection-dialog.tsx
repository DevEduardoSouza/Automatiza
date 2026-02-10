import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useTranslation } from 'react-i18next';
import type { Group } from '../types';

interface GroupSelectionDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  groups: Group[];
  selectedIds: string[];
  searchTerm: string;
  onSearchChange: (value: string) => void;
  onToggleSelection: (groupId: string) => void;
  onApply: () => void;
}

export function GroupSelectionDialog({
  open,
  onOpenChange,
  groups,
  selectedIds,
  searchTerm,
  onSearchChange,
  onToggleSelection,
  onApply,
}: GroupSelectionDialogProps) {
  const { t } = useTranslation();

  const favoriteGroups = groups.filter((g) => g.is_favorite);
  const nonFavoriteGroups = groups.filter((g) => !g.is_favorite);

  const filteredNonFavorites = nonFavoriteGroups.filter(
    (g) => searchTerm === '' || g.name.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const filteredFavorites =
    searchTerm === ''
      ? favoriteGroups
      : favoriteGroups.filter((g) => g.name.toLowerCase().includes(searchTerm.toLowerCase()));

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t('pages.redirectLink.dialog.title')}</DialogTitle>
          <DialogDescription>{t('pages.redirectLink.dialog.description')}</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <Input
            placeholder={t('pages.redirectLink.dialog.searchPlaceholder')}
            value={searchTerm}
            onChange={(e) => onSearchChange(e.target.value)}
            className="mb-2"
          />
          <div className="border rounded-lg p-4 space-y-2 max-h-64 overflow-y-auto">
            {searchTerm === '' && groups.length > 0 && (
              <>
                {filteredFavorites.length > 0 && (
                  <>
                    <p className="text-xs font-semibold text-muted-foreground mb-2">
                      {t('pages.redirectLink.dialog.favorites')}
                    </p>
                    {filteredFavorites.map((group) => (
                      <div key={group.id} className="flex items-center space-x-2">
                        <Checkbox
                          id={`link-group-${group.id}`}
                          checked={selectedIds.includes(group.id)}
                          onCheckedChange={() => onToggleSelection(group.id)}
                        />
                        <Label
                          htmlFor={`link-group-${group.id}`}
                          className="text-sm font-normal cursor-pointer flex-1"
                        >
                          {group.name}
                          <span className="text-xs text-muted-foreground ml-2">
                            ({group.members_count}/{group.member_limit})
                          </span>
                        </Label>
                      </div>
                    ))}
                    <div className="border-t my-2" />
                  </>
                )}
                <p className="text-xs font-semibold text-muted-foreground mb-2">
                  {t('pages.redirectLink.dialog.allGroups')}
                </p>
              </>
            )}
            {searchTerm !== '' &&
              filteredFavorites.length > 0 &&
              filteredFavorites.map((group) => (
                <div key={group.id} className="flex items-center space-x-2">
                  <Checkbox
                    id={`link-group-search-${group.id}`}
                    checked={selectedIds.includes(group.id)}
                    onCheckedChange={() => onToggleSelection(group.id)}
                  />
                  <Label
                    htmlFor={`link-group-search-${group.id}`}
                    className="text-sm font-normal cursor-pointer flex-1"
                  >
                    {group.name}
                    <span className="text-xs text-muted-foreground ml-2">
                      ({group.members_count}/{group.member_limit})
                    </span>
                  </Label>
                </div>
              ))}
            {filteredNonFavorites.map((group) => (
              <div key={group.id} className="flex items-center space-x-2">
                <Checkbox
                  id={`link-group-${group.id}`}
                  checked={selectedIds.includes(group.id)}
                  onCheckedChange={() => onToggleSelection(group.id)}
                />
                <Label
                  htmlFor={`link-group-${group.id}`}
                  className="text-sm font-normal cursor-pointer flex-1"
                >
                  {group.name}
                  <span className="text-xs text-muted-foreground ml-2">
                    ({group.members_count}/{group.member_limit})
                  </span>
                </Label>
              </div>
            ))}
          </div>
          <Button onClick={onApply} className="w-full">
            {t('pages.redirectLink.dialog.apply')}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
