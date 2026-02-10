import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { useTranslation } from 'react-i18next';

interface MlOfferFormProps {
  mlProductUrl: string;
  mlIncludeImage: boolean;
  payload: string;
  onUrlChange: (value: string) => void;
  onIncludeImageChange: (value: boolean) => void;
  onPayloadChange: (value: string) => void;
}

/**
 * Formulário para ofertas do Mercado Livre (afiliado).
 */
export function MlOfferForm({
  mlProductUrl,
  mlIncludeImage,
  payload,
  onUrlChange,
  onIncludeImageChange,
  onPayloadChange,
}: MlOfferFormProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="ml-url" className="cursor-pointer">
          {t('pages.jobs.form.mlUrl')}
        </Label>
        <Input
          id="ml-url"
          placeholder={t('pages.jobs.form.mlUrlPlaceholder')}
          value={mlProductUrl}
          onChange={(e) => onUrlChange(e.target.value)}
          className="cursor-text"
        />
        <p className="text-xs text-muted-foreground">{t('pages.jobs.form.mlUrlHint')}</p>
      </div>

      <label
        htmlFor="ml-include-image"
        className="flex items-center justify-between space-x-2 rounded-lg border p-4 cursor-pointer"
      >
        <div className="space-y-0.5">
          <span className="font-medium text-sm">{t('pages.jobs.form.sendImage')}</span>
          <p className="text-sm text-muted-foreground">{t('pages.jobs.form.sendImageHint')}</p>
        </div>
        <Switch
          id="ml-include-image"
          checked={mlIncludeImage}
          onCheckedChange={onIncludeImageChange}
          className="cursor-pointer"
        />
      </label>

      <div className="space-y-2">
        <Label htmlFor="ml-text" className="cursor-pointer">
          {t('pages.jobs.form.dispatchText')}
        </Label>
        <Textarea
          id="ml-text"
          placeholder={t('pages.jobs.form.dispatchTextPlaceholder')}
          value={payload}
          onChange={(e) => onPayloadChange(e.target.value)}
          rows={5}
          className="cursor-text"
        />
        <p className="text-xs text-muted-foreground">
          {t('pages.jobs.form.dispatchTextVariables')}
        </p>
      </div>
    </div>
  );
}
