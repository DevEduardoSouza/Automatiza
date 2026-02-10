import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { Automation, FilterType, PriorityType } from '@/types/automation';
import { useTranslation } from 'react-i18next';

interface PriorityFilterSectionProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
}

export function PriorityFilterSection({ formData, setFormData }: PriorityFilterSectionProps) {
  const { t } = useTranslation();

  return (
    <div className="grid grid-cols-2 gap-4">
      <div className="space-y-2">
        <Label>{t('pages.offerAutomations.form.priority')}</Label>
        <Select
          value={formData.priority}
          onValueChange={(v) => setFormData((prev) => ({ ...prev, priority: v as PriorityType }))}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="discount">
              {t('pages.offerAutomations.priorities.discount')}
            </SelectItem>
            <SelectItem value="price">{t('pages.offerAutomations.priorities.price')}</SelectItem>
            <SelectItem value="commission">
              {t('pages.offerAutomations.priorities.commission')}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <Label>{t('pages.offerAutomations.form.filterType')}</Label>
        <Select
          value={formData.filter_type}
          onValueChange={(v) => setFormData((prev) => ({ ...prev, filter_type: v as FilterType }))}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="light">{t('pages.offerAutomations.filterTypes.light')}</SelectItem>
            <SelectItem value="heavy">{t('pages.offerAutomations.filterTypes.heavy')}</SelectItem>
          </SelectContent>
        </Select>
      </div>
    </div>
  );
}

interface FiltersSectionProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
}

export function FiltersSection({ formData, setFormData }: FiltersSectionProps) {
  const { t } = useTranslation();

  return (
    <div className="grid grid-cols-3 gap-4">
      <div className="space-y-2">
        <Label>{t('pages.offerAutomations.form.minDiscount')}</Label>
        <Input
          type="number"
          min="0"
          max="100"
          value={formData.min_discount ?? ''}
          onChange={(e) =>
            setFormData((prev) => ({
              ...prev,
              min_discount: e.target.value === '' ? null : Number(e.target.value),
            }))
          }
        />
      </div>
      <div className="space-y-2">
        <Label>{t('pages.offerAutomations.form.minPrice')}</Label>
        <Input
          type="number"
          min="0"
          step="0.01"
          value={formData.min_price ?? ''}
          onChange={(e) =>
            setFormData((prev) => ({
              ...prev,
              min_price: e.target.value === '' ? null : Number(e.target.value),
            }))
          }
        />
      </div>
      <div className="space-y-2">
        <Label>{t('pages.offerAutomations.form.maxPrice')}</Label>
        <Input
          type="number"
          min="0"
          step="0.01"
          value={formData.max_price ?? ''}
          onChange={(e) =>
            setFormData((prev) => ({
              ...prev,
              max_price: e.target.value === '' ? null : Number(e.target.value),
            }))
          }
        />
      </div>
    </div>
  );
}
