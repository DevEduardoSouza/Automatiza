import { Button } from '@/components/ui/button';
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
        <Button variant="outline" className="w-full text-xs md:text-sm">
          <Type className="mr-1 md:mr-2 h-3 w-3 md:h-4 md:w-4" aria-hidden="true" />
          <span className="hidden sm:inline">{t('pages.groups.bulkActions.changeName')}</span>
          <span className="sm:hidden">{t('pages.groups.bulkActions.changeNameShort')}</span>
        </Button>
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
          <Button className="w-full" onClick={onSubmit} disabled={!autoNumber && !groupName.trim()}>
            {t('pages.groups.dialogs.changeName.submit')}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
