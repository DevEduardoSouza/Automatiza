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
import { Image as ImageIcon } from 'lucide-react';
import type { ChangeEvent } from 'react';
import { useTranslation } from 'react-i18next';

interface GroupPhotoDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  photo: File | null;
  onPhotoUpload: (e: ChangeEvent<HTMLInputElement>) => void;
  onSubmit: () => void;
  selectedGroupsCount: number;
  onNoGroupsSelected: () => void;
}

/**
 * Dialog para alterar foto dos grupos em massa.
 */
export function GroupPhotoDialog({
  open,
  onOpenChange,
  photo,
  onPhotoUpload,
  onSubmit,
  selectedGroupsCount,
  onNoGroupsSelected,
}: GroupPhotoDialogProps) {
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
          <ImageIcon className="mr-1 md:mr-2 h-3 w-3 md:h-4 md:w-4" aria-hidden="true" />
          <span className="hidden sm:inline">{t('pages.groups.bulkActions.changePhoto')}</span>
          <span className="sm:hidden">{t('pages.groups.bulkActions.changePhotoShort')}</span>
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('pages.groups.dialogs.changePhoto.title')}</DialogTitle>
          <DialogDescription>{t('pages.groups.dialogs.changePhoto.description')}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="groupPhoto">{t('pages.groups.dialogs.changePhoto.label')}</Label>
            <div className="flex items-center gap-3">
              <Input
                id="groupPhoto"
                type="file"
                accept="image/*"
                onChange={onPhotoUpload}
                className="flex-1"
              />
            </div>
            {photo && (
              <p className="text-sm text-muted-foreground">
                {t('pages.groups.dialogs.changePhoto.selectedFile', {
                  name: photo.name,
                })}
              </p>
            )}
          </div>
          <Button className="w-full" onClick={onSubmit} disabled={!photo}>
            {t('pages.groups.dialogs.changePhoto.submit')}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
