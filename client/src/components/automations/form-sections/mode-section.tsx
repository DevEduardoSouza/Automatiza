import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { Automation, AutomationMode } from '@/types/automation';
import { useTranslation } from 'react-i18next';

interface ModeSectionProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
}

export function ModeSection({ formData, setFormData }: ModeSectionProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-1.5 xs:space-y-2">
      <Label className="cursor-pointer">{t('pages.offerAutomations.form.mode')}</Label>
      <Select
        value={formData.mode}
        onValueChange={(v) =>
          setFormData((prev) => {
            const nextMode = v as AutomationMode;
            if (nextMode === 'monitor') {
              const currentStores = prev.stores || [];
              const hasShopee = currentStores.includes('shopee');
              return {
                ...prev,
                mode: nextMode,
                stores: hasShopee ? currentStores : ['shopee'],
              };
            }
            if (nextMode === 'list') {
              return {
                ...prev,
                mode: nextMode,
                stores: ['ml'],
                categories: [],
                ml_link_list_id: prev.ml_link_list_id || null,
                list_items_per_run:
                  prev.list_items_per_run != null ? Number(prev.list_items_per_run) : 1,
                list_include_image: prev.list_include_image ?? true,
                list_send_as_image: prev.list_send_as_image ?? true,
              };
            }
            return { ...prev, mode: nextMode };
          })
        }
      >
        <SelectTrigger className="cursor-pointer">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="search" className="cursor-pointer">
            {t('pages.offerAutomations.modes.search')}
          </SelectItem>
          <SelectItem value="monitor" className="cursor-pointer">
            {t('pages.offerAutomations.modes.monitor')}
          </SelectItem>
          <SelectItem value="list" className="cursor-pointer">
            {t('pages.offerAutomations.modes.list')}
          </SelectItem>
        </SelectContent>
      </Select>
    </div>
  );
}
