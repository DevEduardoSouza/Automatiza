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
import * as Sentry from '@sentry/react';
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

  /** WF38: Defensive validation + Sentry error capture on poll submit */
  const handleSubmitWithValidation = () => {
    try {
      if (!question.trim()) return;
      const filledOptions = options.filter((o) => o.trim());
      if (filledOptions.length < 2) return;
      if (selectedGroupsCount === 0) return;
      onSubmit();
    } catch (error) {
      Sentry.captureException(error, {
        tags: { component: 'GroupPollDialog', action: 'submit' },
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
          <BarChart3 className="shrink-0 h-3 w-3 md:h-4 md:w-4" aria-hidden="true" />
          <span className="hidden sm:inline truncate">
            {t('pages.groups.bulkActions.sendPoll')}
          </span>
          <span className="sm:hidden truncate">{t('pages.groups.bulkActions.sendPollShort')}</span>
        </button>
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
              <button
                type="button"
                onClick={onAddOption}
                className="inline-flex w-full cursor-pointer items-center justify-center gap-2 rounded-full border border-gray-300 bg-white px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-[#8D2D53] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                {t('pages.groups.dialogs.poll.addOption')}
              </button>
            )}
          </div>

          <button
            type="button"
            onClick={handleSubmitWithValidation}
            disabled={sendingPoll || selectedGroupsCount === 0 || !question.trim()}
            className="auth-btn-glow relative inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
          >
            <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
              <div className="auth-btn-glow__glow absolute inset-0" />
            </div>
            <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
              {sendingPoll
                ? t('pages.groups.dialogs.poll.sending')
                : t('pages.groups.dialogs.poll.submit', {
                    count: selectedGroupsCount,
                  })}
            </span>
          </button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
