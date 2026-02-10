import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Hash } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface GroupNameFormProps {
  payload: string;
  autoNumberGroups: boolean;
  onPayloadChange: (value: string) => void;
  onAutoNumberChange: (value: boolean) => void;
}

/**
 * Formulário para alterar nome de grupos com numeração automática opcional.
 */
export function GroupNameForm({
  payload,
  autoNumberGroups,
  onPayloadChange,
  onAutoNumberChange,
}: GroupNameFormProps) {
  const { t } = useTranslation();
  const defaultGroupName = t('pages.jobs.form.newNamePreviewDefault');

  return (
    <div className="space-y-4">
      <label
        htmlFor="auto-number-job"
        className="flex items-center justify-between space-x-2 rounded-lg border p-4 cursor-pointer"
      >
        <div className="flex items-start space-x-3 flex-1">
          <Hash className="h-5 w-5 mt-0.5 text-primary" />
          <div className="space-y-0.5">
            <span className="font-medium text-sm">{t('pages.jobs.form.autoNumber')}</span>
            <p className="text-sm text-muted-foreground">{t('pages.jobs.form.autoNumberHint')}</p>
          </div>
        </div>
        <Switch
          id="auto-number-job"
          checked={autoNumberGroups}
          onCheckedChange={onAutoNumberChange}
          className="cursor-pointer"
        />
      </label>

      <div className="space-y-2">
        <Label htmlFor="payload" className="cursor-pointer">
          {autoNumberGroups ? t('pages.jobs.form.newNameBase') : t('pages.jobs.form.newName')}
        </Label>
        <Input
          id="payload"
          placeholder={
            autoNumberGroups
              ? t('pages.jobs.form.newNameBasePlaceholder')
              : t('pages.jobs.form.newNamePlaceholder')
          }
          value={payload}
          onChange={(e) => onPayloadChange(e.target.value)}
          className="cursor-text"
        />
        {autoNumberGroups && (
          <p className="text-xs text-muted-foreground">
            {t('pages.jobs.form.newNamePreview')} #1 {payload || defaultGroupName}, #2{' '}
            {payload || defaultGroupName}, #3 {payload || defaultGroupName}
            ...
          </p>
        )}
      </div>
    </div>
  );
}
