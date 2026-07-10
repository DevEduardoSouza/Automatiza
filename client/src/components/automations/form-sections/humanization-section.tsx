import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { Automation } from '@/types/automation';
import { useTranslation } from 'react-i18next';

interface HumanizationSectionProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
}

type HumanizeMode = 'off' | 'low' | 'medium' | 'high';

export function HumanizationSection({ formData, setFormData }: HumanizationSectionProps) {
  const { t } = useTranslation();

  const options: { value: HumanizeMode; label: string }[] = [
    { value: 'off', label: t('pages.offerAutomations.form.humanizeModeOff') },
    { value: 'low', label: t('pages.offerAutomations.form.humanizeModeLow') },
    { value: 'medium', label: t('pages.offerAutomations.form.humanizeModeMedium') },
    { value: 'high', label: t('pages.offerAutomations.form.humanizeModeHigh') },
  ];

  return (
    <div className="space-y-1.5 xs:space-y-2">
      <Label htmlFor="humanize-mode" className="cursor-pointer">
        {t('pages.offerAutomations.form.humanizeMode')}
      </Label>
      <Select
        value={formData.humanize_mode ?? 'high'}
        onValueChange={(v) =>
          setFormData((prev) => ({ ...prev, humanize_mode: v as HumanizeMode }))
        }
      >
        <SelectTrigger id="humanize-mode" className="cursor-pointer">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((o) => (
            <SelectItem key={o.value} value={o.value} className="cursor-pointer">
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <p className="text-xs text-muted-foreground">
        {t('pages.offerAutomations.form.humanizeModeHint')}
      </p>
    </div>
  );
}
