import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
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
  onSelectAll: () => void;
  onDeselectAll: () => void;
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
  onSelectAll,
  onDeselectAll,
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
          <input
            placeholder={t('pages.redirectLink.dialog.searchPlaceholder')}
            value={searchTerm}
            onChange={(e) => onSearchChange(e.target.value)}
            className="mb-2 w-full rounded-2xl !border-2 !border-gray-300 bg-gray-50 px-4 py-2 text-xs xs:py-2.5 xs:text-sm lg:py-3 lg:text-base text-gray-900 placeholder-gray-400 outline-none transition-all hover:!border-[#520B27] focus:!border-[#520B27] focus:ring-1 focus:ring-[#3a0718]/30"
          />
          <div className="flex items-center justify-between text-xs px-1">
            <span className="text-muted-foreground">
              {selectedIds.length} {t('pages.redirectLink.dialog.selectedCount')}
            </span>
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={onSelectAll}
                className="text-[#520B27] hover:underline cursor-pointer font-medium"
              >
                {t('pages.redirectLink.dialog.selectAll')}
              </button>
              <span className="text-muted-foreground">|</span>
              <button
                type="button"
                onClick={onDeselectAll}
                className="text-[#520B27] hover:underline cursor-pointer font-medium"
              >
                {t('pages.redirectLink.dialog.deselectAll')}
              </button>
            </div>
          </div>
          <div className="border rounded-lg p-4 space-y-2 max-h-80 sm:max-h-96 overflow-y-auto">
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
          <button
            type="button"
            onClick={onApply}
            className="auth-btn-glow relative inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
          >
            <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
              <div className="auth-btn-glow__glow absolute inset-0" />
            </div>
            <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
              {t('pages.redirectLink.dialog.apply')}
            </span>
          </button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
