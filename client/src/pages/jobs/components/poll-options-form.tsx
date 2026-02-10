import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Plus, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface PollOptionsFormProps {
  pollQuestion: string;
  pollOptions: string[];
  onQuestionChange: (value: string) => void;
  onAddOption: () => void;
  onRemoveOption: (index: number) => void;
  onUpdateOption: (index: number, value: string) => void;
}

/**
 * Formulário para criar/editar enquetes com pergunta e opções.
 */
export function PollOptionsForm({
  pollQuestion,
  pollOptions,
  onQuestionChange,
  onAddOption,
  onRemoveOption,
  onUpdateOption,
}: PollOptionsFormProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-3 xs:space-y-4">
      <div className="space-y-2">
        <Label htmlFor="poll-question" className="cursor-pointer">
          {t('pages.jobs.form.pollQuestion')}
        </Label>
        <Textarea
          id="poll-question"
          placeholder={t('pages.jobs.form.pollQuestionPlaceholder')}
          value={pollQuestion}
          onChange={(e) => onQuestionChange(e.target.value)}
          rows={2}
          className="cursor-text min-h-[56px] xs:min-h-[70px] lg:min-h-[90px]"
        />
      </div>

      <div className="space-y-2">
        <Label className="cursor-default">{t('pages.jobs.form.pollOptions')}</Label>
        {pollOptions.map((option, index) => (
          <div key={index} className="flex gap-1.5 xs:gap-2">
            <Input
              placeholder={t('pages.jobs.form.pollOptionPlaceholder', {
                index: index + 1,
              })}
              value={option}
              onChange={(e) => onUpdateOption(index, e.target.value)}
              className="cursor-text"
            />
            {pollOptions.length > 2 && (
              <Button
                type="button"
                variant="outline"
                size="icon"
                onClick={() => onRemoveOption(index)}
                className="cursor-pointer"
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            )}
          </div>
        ))}
        {pollOptions.length < 12 && (
          <Button
            type="button"
            variant="outline"
            onClick={onAddOption}
            className="w-full cursor-pointer text-xs xs:text-sm h-8 xs:h-10"
          >
            <Plus className="mr-2 h-4 w-4" />
            {t('pages.jobs.form.addOption')}
          </Button>
        )}
      </div>
    </div>
  );
}
