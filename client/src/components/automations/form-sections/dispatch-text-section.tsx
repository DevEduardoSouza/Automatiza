import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { Automation, DispatchText } from '@/types/automation';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';

interface DispatchTextSelectorProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  dispatchTexts: DispatchText[];
}

export function DispatchTextSelector({
  formData,
  setFormData,
  dispatchTexts,
}: DispatchTextSelectorProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-1.5 xs:space-y-2">
      <Label className="cursor-pointer">{t('pages.offerAutomations.form.dispatchText')}</Label>
      <Select
        value={formData.dispatch_text_id || '__default__'}
        onValueChange={(v) =>
          setFormData((prev) => ({
            ...prev,
            dispatch_text_id: v === '__default__' ? null : v,
          }))
        }
      >
        <SelectTrigger className="cursor-pointer">
          <SelectValue placeholder={t('pages.offerAutomations.form.useDefault')} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__default__" className="cursor-pointer">
            {t('pages.offerAutomations.form.systemDefault')}
          </SelectItem>
          {dispatchTexts.map((dt) => (
            <SelectItem key={dt.id} value={dt.id} className="cursor-pointer">
              {dt.name}
              {dt.copies?.length > 1 &&
                ` (${dt.copies.length} ${t('pages.dispatchTexts.copies.variations')})`}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <p className="text-xs text-muted-foreground">
        {t('pages.offerAutomations.form.dispatchTextHint')}
      </p>
      {!dispatchTexts.length && (
        <p className="text-xs text-muted-foreground">
          {t('pages.offerAutomations.form.noDispatchTexts')}{' '}
          <Link to="/dispatch-texts" className="underline">
            {t('pages.offerAutomations.form.createNow')}
          </Link>
        </p>
      )}
    </div>
  );
}
