import { Button } from '@/components/ui/button';
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
          <FileEdit className="mr-1 md:mr-2 h-3 w-3 md:h-4 md:w-4" aria-hidden="true" />
          <span className="hidden sm:inline">
            {t('pages.groups.bulkActions.changeDescription')}
          </span>
          <span className="sm:hidden">{t('pages.groups.bulkActions.changeDescriptionShort')}</span>
        </Button>
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
          <Button className="w-full" onClick={onSubmit}>
            {t('pages.groups.dialogs.changeDescription.submit')}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
