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
              <button
                type="button"
                onClick={() => onRemoveOption(index)}
                className="inline-flex shrink-0 cursor-pointer items-center justify-center rounded-full h-9 w-9 xs:h-10 xs:w-10 border border-gray-300 bg-white text-[#520B27] shadow-sm transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
              >
                <Trash2 className="h-4 w-4" />
              </button>
            )}
          </div>
        ))}
        {pollOptions.length < 12 && (
          <button
            type="button"
            onClick={onAddOption}
            className="inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold transition-all border border-gray-300 bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-md"
          >
            <Plus className="h-4 w-4" />
            {t('pages.jobs.form.addOption')}
          </button>
        )}
      </div>
    </div>
  );
}
