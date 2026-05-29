import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { INTERVALS, clampIntervalMinutes } from '@/config/automation-categories';
import type { Automation } from '@/types/automation';
import { useTranslation } from 'react-i18next';

interface TimeWindowInputsProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
}

export function TimeWindowInputs({ formData, setFormData }: TimeWindowInputsProps) {
  const { t } = useTranslation();

  return (
    <div className="grid grid-cols-2 gap-4 lg:gap-6">
      <div className="space-y-1.5 xs:space-y-2">
        <Label className="cursor-pointer">{t('pages.offerAutomations.form.startTime')}</Label>
        <Input
          type="time"
          value={formData.start_time}
          onChange={(e) => setFormData((prev) => ({ ...prev, start_time: e.target.value }))}
        />
      </div>
      <div className="space-y-1.5 xs:space-y-2">
        <Label className="cursor-pointer">{t('pages.offerAutomations.form.endTime')}</Label>
        <Input
          type="time"
          value={formData.end_time}
          onChange={(e) => setFormData((prev) => ({ ...prev, end_time: e.target.value }))}
        />
      </div>
    </div>
  );
}

interface DateWindowInputsProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
}

// DateWindowInputs renderiza start_date + end_date lado a lado, mesmo
// pattern de TimeWindowInputs. Janela calendário em que a automation
// é elegível pro scheduler. Ambos opcionais — vazio = sem restrição
// (start vazio: roda imediatamente; end vazio: nunca expira).
export function DateWindowInputs({ formData, setFormData }: DateWindowInputsProps) {
  const { t } = useTranslation();

  return (
    <div className="grid grid-cols-2 gap-4 lg:gap-6">
      <div className="space-y-1.5 xs:space-y-2">
        <Label className="cursor-pointer">{t('pages.offerAutomations.form.startDate')}</Label>
        <Input
          type="date"
          value={formData.start_date ?? ''}
          onChange={(e) =>
            setFormData((prev) => ({
              ...prev,
              start_date: e.target.value || undefined,
            }))
          }
        />
        <p className="text-xs text-muted-foreground">
          {t('pages.offerAutomations.form.startDateHint')}
        </p>
      </div>
      <div className="space-y-1.5 xs:space-y-2">
        <Label className="cursor-pointer">{t('pages.offerAutomations.form.endDate')}</Label>
        <Input
          type="date"
          value={formData.end_date ?? ''}
          onChange={(e) =>
            setFormData((prev) => ({
              ...prev,
              end_date: e.target.value || undefined,
            }))
          }
        />
        <p className="text-xs text-muted-foreground">
          {t('pages.offerAutomations.form.endDateHint')}
        </p>
      </div>
    </div>
  );
}

interface IntervalSelectorProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
}

export function IntervalSelector({ formData, setFormData }: IntervalSelectorProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-1.5 xs:space-y-2">
      <Label className="cursor-pointer">{t('pages.offerAutomations.form.interval')}</Label>
      <Select
        value={String(formData.interval_minutes)}
        onValueChange={(v) =>
          setFormData((prev) => ({
            ...prev,
            interval_minutes: clampIntervalMinutes(v),
          }))
        }
      >
        <SelectTrigger className="cursor-pointer">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {INTERVALS.map((i) => (
            <SelectItem key={i} value={String(i)} className="cursor-pointer">
              {i} {t('common.minutes')}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
