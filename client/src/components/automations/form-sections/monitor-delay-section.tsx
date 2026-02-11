import { Input } from '@/components/ui/input';
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

interface MonitorDelayInputProps {
  monitorDelayUnit: 'seconds' | 'minutes';
  monitorDelayInput: string;
  setMonitorDelayUnit: (unit: 'seconds' | 'minutes') => void;
  setMonitorDelayInput: (value: string) => void;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
}

export function MonitorDelayInput({
  monitorDelayUnit,
  monitorDelayInput,
  setMonitorDelayUnit,
  setMonitorDelayInput,
  setFormData,
}: MonitorDelayInputProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-1.5 xs:space-y-2">
      <Label className="cursor-pointer">{t('pages.offerAutomations.form.delay')}</Label>
      <div className="flex gap-2">
        <Input
          type="number"
          min={monitorDelayUnit === 'minutes' ? '1' : '10'}
          value={monitorDelayInput}
          onChange={(e) => setMonitorDelayInput(e.target.value)}
          onBlur={() => {
            const raw = monitorDelayInput.trim();
            const n = Number(raw);
            if (!raw || !Number.isFinite(n)) {
              const fallback = monitorDelayUnit === 'minutes' ? 1 : 10;
              setMonitorDelayInput(String(fallback));
              setFormData((prev) => ({
                ...prev,
                delay_seconds: monitorDelayUnit === 'minutes' ? fallback * 60 : fallback,
              }));
              return;
            }
            if (monitorDelayUnit === 'minutes') {
              const minutes = Math.max(1, Math.floor(n));
              setMonitorDelayInput(String(minutes));
              setFormData((prev) => ({ ...prev, delay_seconds: minutes * 60 }));
            } else {
              const seconds = Math.max(10, Math.floor(n));
              setMonitorDelayInput(String(seconds));
              setFormData((prev) => ({ ...prev, delay_seconds: seconds }));
            }
          }}
        />
        <Select
          value={monitorDelayUnit}
          onValueChange={(v) => {
            const next = v === 'minutes' ? 'minutes' : 'seconds';
            setMonitorDelayUnit(next);
            setFormData((prev) => {
              const current = Number(prev.delay_seconds ?? 10);
              if (next === 'minutes') {
                const mins = Math.max(1, Math.round(current / 60));
                setMonitorDelayInput(String(mins));
                return { ...prev, delay_seconds: mins * 60 };
              }
              const secs = Math.max(10, current);
              setMonitorDelayInput(String(secs));
              return { ...prev, delay_seconds: secs };
            });
          }}
        >
          <SelectTrigger className="w-[140px] cursor-pointer">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="seconds" className="cursor-pointer">
              {t('common.seconds')}
            </SelectItem>
            <SelectItem value="minutes" className="cursor-pointer">
              {t('common.minutes')}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
      <p className="text-xs text-muted-foreground">{t('pages.offerAutomations.form.delayHint')}</p>
    </div>
  );
}
