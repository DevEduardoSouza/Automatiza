import { Alert, AlertDescription } from '@/components/ui/alert';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { Automation, MlLinkList } from '@/types/automation';
import { Info } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface ListModeConfigProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  mlLinkLists: MlLinkList[];
  loadingMlLinkLists: boolean;
}

export function ListModeConfig({
  formData,
  setFormData,
  mlLinkLists,
  loadingMlLinkLists,
}: ListModeConfigProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-4">
      <Alert>
        <Info className="h-4 w-4" />
        <AlertDescription>{t('pages.offerAutomations.form.listModeInfo')}</AlertDescription>
      </Alert>

      <div className="space-y-2">
        <Label>{t('pages.offerAutomations.form.linkList')} *</Label>
        <Select
          value={String(formData.ml_link_list_id || '')}
          onValueChange={(v) => setFormData((prev) => ({ ...prev, ml_link_list_id: v || null }))}
        >
          <SelectTrigger>
            <SelectValue
              placeholder={
                loadingMlLinkLists
                  ? t('common.loading')
                  : t('pages.offerAutomations.form.selectList')
              }
            />
          </SelectTrigger>
          <SelectContent>
            {mlLinkLists.length === 0 ? (
              <SelectItem value="__empty__" disabled>
                {t('pages.offerAutomations.form.noListsFound')}
              </SelectItem>
            ) : (
              mlLinkLists.map((l) => (
                <SelectItem key={l.id} value={l.id}>
                  {l.name}
                </SelectItem>
              ))
            )}
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-foreground">
          {t('pages.offerAutomations.form.linkListHint')}
        </p>
      </div>
    </div>
  );
}
