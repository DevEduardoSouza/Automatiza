import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
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
      <div
        className="rounded-xl border px-4 py-3 text-sm"
        style={{
          backgroundColor: 'rgba(141, 45, 83, 0.06)',
          borderColor: 'rgba(141, 45, 83, 0.15)',
        }}
      >
        <div className="flex items-start gap-3">
          <Info className="h-4 w-4 mt-0.5 shrink-0" style={{ color: 'rgba(141, 45, 83, 0.6)' }} />
          <p style={{ color: 'rgba(141, 45, 83, 0.8)' }}>
            {t('pages.offerAutomations.form.listModeInfo')}
          </p>
        </div>
      </div>

      <div className="space-y-1.5 xs:space-y-2">
        <Label className="cursor-pointer">{t('pages.offerAutomations.form.linkList')} *</Label>
        <Select
          value={String(formData.ml_link_list_id || '')}
          onValueChange={(v) => setFormData((prev) => ({ ...prev, ml_link_list_id: v || null }))}
        >
          <SelectTrigger className="cursor-pointer">
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
              <SelectItem value="__empty__" disabled className="cursor-pointer">
                {t('pages.offerAutomations.form.noListsFound')}
              </SelectItem>
            ) : (
              mlLinkLists.map((l) => (
                <SelectItem key={l.id} value={l.id} className="cursor-pointer">
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

      <div className="flex items-center justify-between gap-3 rounded-lg border px-4 py-3">
        <div className="space-y-1">
          <Label htmlFor="list-send-as-image" className="cursor-pointer">
            {t('pages.offerAutomations.form.sendAsImage')}
          </Label>
          <p className="text-xs text-muted-foreground">
            {t('pages.offerAutomations.form.sendAsImageHint')}
          </p>
        </div>
        <Switch
          id="list-send-as-image"
          checked={Boolean(formData.list_send_as_image ?? false)}
          onCheckedChange={(v) =>
            setFormData((prev) => ({ ...prev, list_send_as_image: Boolean(v) }))
          }
        />
      </div>

      <div className="flex items-center justify-between gap-3 rounded-lg border px-4 py-3">
        <div className="space-y-1">
          <Label htmlFor="repeat-when-finished" className="cursor-pointer">
            {t('pages.offerAutomations.form.repeatWhenFinished')}
          </Label>
          <p className="text-xs text-muted-foreground">
            {t('pages.offerAutomations.form.repeatWhenFinishedHint')}
          </p>
        </div>
        <Switch
          id="repeat-when-finished"
          checked={Boolean(formData.repeat_when_finished ?? true)}
          onCheckedChange={(v) =>
            setFormData((prev) => ({ ...prev, repeat_when_finished: Boolean(v) }))
          }
        />
      </div>
    </div>
  );
}
