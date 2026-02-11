import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Hash } from 'lucide-react';
import { Type } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface GroupNameDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  groupName: string;
  onGroupNameChange: (value: string) => void;
  autoNumber: boolean;
  onAutoNumberChange: (value: boolean) => void;
  onSubmit: () => void;
  selectedGroupsCount: number;
  onNoGroupsSelected: () => void;
}

/**
 * Dialog para alterar nome dos grupos em massa.
 */
export function GroupNameDialog({
  open,
  onOpenChange,
  groupName,
  onGroupNameChange,
  autoNumber,
  onAutoNumberChange,
  onSubmit,
  selectedGroupsCount,
  onNoGroupsSelected,
}: GroupNameDialogProps) {
  const { t } = useTranslation();

  const handleOpenChange = (newOpen: boolean) => {
    if (newOpen && selectedGroupsCount === 0) {
      onNoGroupsSelected();
      return;
    }
    onOpenChange(newOpen);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <button
          type="button"
          className="inline-flex w-full min-w-0 cursor-pointer items-center justify-center gap-1.5 rounded-full border border-gray-300 bg-white px-3 py-1.5 text-xs xs:px-4 xs:py-2 xs:text-[13px] md:text-sm font-medium text-[#8D2D53] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
        >
          <Type className="shrink-0 h-3 w-3 md:h-4 md:w-4" aria-hidden="true" />
          <span className="hidden sm:inline truncate">
            {t('pages.groups.bulkActions.changeName')}
          </span>
          <span className="sm:hidden truncate">
            {t('pages.groups.bulkActions.changeNameShort')}
          </span>
        </button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('pages.groups.dialogs.changeName.title')}</DialogTitle>
          <DialogDescription>{t('pages.groups.dialogs.changeName.description')}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="flex items-center justify-between space-x-2 rounded-lg border p-4">
            <div className="flex items-start space-x-3 flex-1">
              <Hash className="h-5 w-5 mt-0.5 text-primary" aria-hidden="true" />
              <div className="space-y-0.5">
                <Label htmlFor="auto-number" className="font-medium">
                  {t('pages.groups.dialogs.changeName.autoNumber')}
                </Label>
                <p className="text-sm text-muted-foreground">
                  {t('pages.groups.dialogs.changeName.autoNumberDescription')}
                </p>
              </div>
            </div>
            <Switch id="auto-number" checked={autoNumber} onCheckedChange={onAutoNumberChange} />
          </div>

          <div className="space-y-2">
            <Label htmlFor="groupName">
              {autoNumber
                ? t('pages.groups.dialogs.changeName.baseNameOptional')
                : t('pages.groups.dialogs.changeName.newName')}
            </Label>
            <Input
              id="groupName"
              placeholder={
                autoNumber
                  ? t('pages.groups.dialogs.changeName.baseNamePlaceholder')
                  : t('pages.groups.dialogs.changeName.newNamePlaceholder')
              }
              value={groupName}
              onChange={(e) => onGroupNameChange(e.target.value)}
            />
            {autoNumber && (
              <p className="text-xs text-muted-foreground">
                {t('pages.groups.dialogs.changeName.preview', {
                  name: groupName || t('pages.groups.dialogs.changeName.defaultName'),
                })}
              </p>
            )}
          </div>
          <button
            type="button"
            onClick={onSubmit}
            disabled={!autoNumber && !groupName.trim()}
            className="auth-btn-glow relative inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
          >
            <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
              <div className="auth-btn-glow__glow absolute inset-0" />
            </div>
            <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
              {t('pages.groups.dialogs.changeName.submit')}
            </span>
          </button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
