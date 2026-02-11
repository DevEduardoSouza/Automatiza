import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import type { Automation } from '@/types/automation';
import { Plus, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface TextsSectionProps {
  formData: Partial<Automation>;
  suggestingText: boolean;
  onAddText: () => void;
  onUpdateText: (index: number, value: string) => void;
  onRemoveText: (index: number) => void;
  onSuggestTexts: () => Promise<void>;
}

export function TextsSection({
  formData,
  suggestingText,
  onAddText,
  onUpdateText,
  onRemoveText,
  onSuggestTexts,
}: TextsSectionProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-1.5 xs:space-y-2">
      <div className="flex justify-between items-center">
        <Label className="cursor-pointer">{t('pages.offerAutomations.form.texts')}</Label>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={onSuggestTexts} disabled={suggestingText}>
            {suggestingText
              ? t('pages.offerAutomations.form.generating')
              : t('pages.offerAutomations.form.suggestWithAI')}
          </Button>
          {(formData.texts?.length || 0) < 3 && (
            <Button variant="outline" size="sm" onClick={onAddText}>
              <Plus className="h-3 w-3 mr-1" /> {t('common.add')}
            </Button>
          )}
        </div>
      </div>
      {formData.texts?.map((text, index) => (
        <div key={index} className="flex gap-2">
          <Textarea
            value={text}
            onChange={(e) => onUpdateText(index, e.target.value)}
            placeholder={t('pages.offerAutomations.form.textPlaceholder')}
            rows={2}
          />
          {formData.texts && formData.texts.length > 1 && (
            <Button variant="ghost" size="icon" onClick={() => onRemoveText(index)}>
              <X className="h-4 w-4" />
            </Button>
          )}
        </div>
      ))}
    </div>
  );
}
