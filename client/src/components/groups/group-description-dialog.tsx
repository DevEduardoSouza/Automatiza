import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import * as Sentry from '@sentry/react';
import { FileEdit } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface GroupDescriptionDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  description: string;
  onDescriptionChange: (value: string) => void;
  onSubmit: () => void;
  selectedGroupsCount: number;
  onNoGroupsSelected: () => void;
}

/**
 * Dialog para alterar descrição dos grupos em massa.
 */
export function GroupDescriptionDialog({
  open,
  onOpenChange,
  description,
  onDescriptionChange,
  onSubmit,
  selectedGroupsCount,
  onNoGroupsSelected,
}: GroupDescriptionDialogProps) {
  const { t } = useTranslation();

  /** WF26: Defensive validation + Sentry error capture on submit */
  const handleSubmitWithValidation = () => {
    try {
      if (!description.trim()) return;
      if (selectedGroupsCount === 0) return;
      onSubmit();
    } catch (error) {
      Sentry.captureException(error, {
        tags: { component: 'GroupDescriptionDialog', action: 'submit' },
      });
    }
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
          <FileEdit className="shrink-0 h-3 w-3 md:h-4 md:w-4" aria-hidden="true" />
          <span className="hidden sm:inline truncate">
            {t('pages.groups.bulkActions.changeDescription')}
          </span>
          <span className="sm:hidden truncate">
            {t('pages.groups.bulkActions.changeDescriptionShort')}
          </span>
        </button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('pages.groups.dialogs.changeDescription.title')}</DialogTitle>
          <DialogDescription>
            {t('pages.groups.dialogs.changeDescription.description')}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="description">{t('pages.groups.dialogs.changeDescription.label')}</Label>
            <Textarea
              id="description"
              placeholder={t('pages.groups.dialogs.changeDescription.placeholder')}
              value={description}
              onChange={(e) => onDescriptionChange(e.target.value)}
              rows={4}
            />
          </div>
          <button
            type="button"
            onClick={handleSubmitWithValidation}
            disabled={!description.trim() || selectedGroupsCount === 0}
            className="auth-btn-glow relative inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
          >
            <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
              <div className="auth-btn-glow__glow absolute inset-0" />
            </div>
            <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
              {t('pages.groups.dialogs.changeDescription.submit')}
            </span>
          </button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
