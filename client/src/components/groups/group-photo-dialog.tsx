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
import { useToast } from '@/hooks/use-toast';
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
  const { toast } = useToast();

  /** WF19/FR9: Validate photo type (image/*) and size (max 5MB) */
  const handlePhotoUploadWithValidation = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!file.type.startsWith('image/')) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.errors.invalidPhotoType'),
        description: t('pages.groups.errors.invalidPhotoTypeMessage'),
      });
      e.target.value = '';
      return;
    }

    if (file.size > 5 * 1024 * 1024) {
      toast({
        variant: 'destructive',
        title: t('pages.groups.errors.photoTooLarge'),
        description: t('pages.groups.errors.photoTooLargeMessage'),
      });
      e.target.value = '';
      return;
    }

    onPhotoUpload(e);
  };

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
          <ImageIcon className="shrink-0 h-3 w-3 md:h-4 md:w-4" aria-hidden="true" />
          <span className="hidden sm:inline truncate">
            {t('pages.groups.bulkActions.changePhoto')}
          </span>
          <span className="sm:hidden truncate">
            {t('pages.groups.bulkActions.changePhotoShort')}
          </span>
        </button>
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
                onChange={handlePhotoUploadWithValidation}
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
          <button
            type="button"
            onClick={onSubmit}
            disabled={!photo}
            className="auth-btn-glow relative inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
          >
            <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
              <div className="auth-btn-glow__glow absolute inset-0" />
            </div>
            <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
              {t('pages.groups.dialogs.changePhoto.submit')}
            </span>
          </button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
