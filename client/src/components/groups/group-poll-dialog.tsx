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
import { Textarea } from '@/components/ui/textarea';
import { BarChart3, Plus, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface GroupPollDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  question: string;
  onQuestionChange: (value: string) => void;
  options: string[];
  onAddOption: () => void;
  onRemoveOption: (index: number) => void;
  onUpdateOption: (index: number, value: string) => void;
  sendingPoll: boolean;
  selectedGroupsCount: number;
  onSubmit: () => void;
  onNoGroupsSelected: () => void;
}

/**
 * Dialog para criar e enviar enquetes em massa para grupos.
 */
export function GroupPollDialog({
  open,
  onOpenChange,
  question,
  onQuestionChange,
  options,
  onAddOption,
  onRemoveOption,
  onUpdateOption,
  sendingPoll,
  selectedGroupsCount,
  onSubmit,
  onNoGroupsSelected,
}: GroupPollDialogProps) {
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
          <BarChart3 className="mr-1 md:mr-2 h-3 w-3 md:h-4 md:w-4" aria-hidden="true" />
          <span className="hidden sm:inline">{t('pages.groups.bulkActions.sendPoll')}</span>
          <span className="sm:hidden">{t('pages.groups.bulkActions.sendPollShort')}</span>
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t('pages.groups.dialogs.poll.title')}</DialogTitle>
          <DialogDescription>{t('pages.groups.dialogs.poll.description')}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="poll-question">{t('pages.groups.dialogs.poll.questionLabel')}</Label>
            <Textarea
              id="poll-question"
              placeholder={t('pages.groups.dialogs.poll.questionPlaceholder')}
              value={question}
              onChange={(e) => onQuestionChange(e.target.value)}
              rows={3}
            />
          </div>

          <div className="space-y-2">
            <Label>{t('pages.groups.dialogs.poll.optionsLabel')}</Label>
            {options.map((option, index) => (
              <div key={index} className="flex gap-2">
                <Input
                  placeholder={t('pages.groups.dialogs.poll.optionPlaceholder', {
                    number: index + 1,
                  })}
                  value={option}
                  onChange={(e) => onUpdateOption(index, e.target.value)}
                />
                {options.length > 2 && (
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={() => onRemoveOption(index)}
                    aria-label={t('pages.groups.dialogs.poll.removeOption')}
                  >
                    <Trash2 className="h-4 w-4" aria-hidden="true" />
                  </Button>
                )}
              </div>
            ))}
            {options.length < 12 && (
              <Button variant="outline" onClick={onAddOption} className="w-full">
                <Plus className="mr-2 h-4 w-4" aria-hidden="true" />
                {t('pages.groups.dialogs.poll.addOption')}
              </Button>
            )}
          </div>

          <Button
            className="w-full"
            onClick={onSubmit}
            disabled={sendingPoll || selectedGroupsCount === 0}
          >
            {sendingPoll
              ? t('pages.groups.dialogs.poll.sending')
              : t('pages.groups.dialogs.poll.submit', {
                  count: selectedGroupsCount,
                })}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
