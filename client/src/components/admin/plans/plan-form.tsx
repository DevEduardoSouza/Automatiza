import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Save } from 'lucide-react';
import type { PlanFormData } from '../types/plans.types';

interface PlanFormProps {
  formData: PlanFormData;
  setFormData: (data: PlanFormData) => void;
  onSave: () => void;
  t: (key: string, options?: Record<string, unknown>) => string;
}

export function PlanForm({ formData, setFormData, onSave, t }: PlanFormProps) {
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label htmlFor="name">{t('admin.plans.dialog.planName')}</Label>
          <Input
            id="name"
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="slug">{t('admin.plans.dialog.slug')}</Label>
          <Input
            id="slug"
            value={formData.slug}
            onChange={(e) => setFormData({ ...formData, slug: e.target.value })}
          />
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="description">{t('common.description')}</Label>
        <Textarea
          id="description"
          value={formData.description}
          onChange={(e) => setFormData({ ...formData, description: e.target.value })}
        />
      </div>

      <PriceFields formData={formData} setFormData={setFormData} t={t} />
      <LimitFields formData={formData} setFormData={setFormData} t={t} />

      <div className="space-y-2">
        <Label htmlFor="features">{t('admin.plans.dialog.features')}</Label>
        <Textarea
          id="features"
          rows={4}
          value={formData.features}
          onChange={(e) => setFormData({ ...formData, features: e.target.value })}
          placeholder={t('admin.plans.dialog.featuresPlaceholder')}
        />
      </div>

      <KiwifyLinksSection formData={formData} setFormData={setFormData} t={t} />

      <Button onClick={onSave} className="w-full">
        <Save className="h-4 w-4 mr-2" />
        {t('admin.plans.dialog.savePlan')}
      </Button>
    </div>
  );
}

function PriceFields({ formData, setFormData, t }: Omit<PlanFormProps, 'onSave'>) {
  return (
    <div className="grid grid-cols-3 gap-4">
      <div className="space-y-2">
        <Label htmlFor="price_monthly">{t('admin.plans.dialog.monthlyPrice')}</Label>
        <Input
          id="price_monthly"
          type="number"
          step="0.01"
          placeholder="0.00"
          value={formData.price_monthly}
          onChange={(e) => setFormData({ ...formData, price_monthly: e.target.value })}
        />
        <p className="text-xs text-muted-foreground">
          {t('admin.plans.dialog.priceHint', {
            period: t('admin.plans.periods.monthly'),
          })}
        </p>
      </div>
      <div className="space-y-2">
        <Label htmlFor="price_semester">{t('admin.plans.dialog.semesterPrice')}</Label>
        <Input
          id="price_semester"
          type="number"
          step="0.01"
          placeholder="0.00"
          value={formData.price_semester}
          onChange={(e) => setFormData({ ...formData, price_semester: e.target.value })}
        />
        <p className="text-xs text-muted-foreground">
          {t('admin.plans.dialog.priceHint', {
            period: t('admin.plans.periods.semester'),
          })}
        </p>
      </div>
      <div className="space-y-2">
        <Label htmlFor="price_annual">{t('admin.plans.dialog.annualPrice')}</Label>
        <Input
          id="price_annual"
          type="number"
          step="0.01"
          placeholder="0.00"
          value={formData.price_annual}
          onChange={(e) => setFormData({ ...formData, price_annual: e.target.value })}
        />
        <p className="text-xs text-muted-foreground">
          {t('admin.plans.dialog.priceHint', {
            period: t('admin.plans.periods.annual'),
          })}
        </p>
      </div>
    </div>
  );
}

function LimitFields({ formData, setFormData, t }: Omit<PlanFormProps, 'onSave'>) {
  return (
    <div className="grid grid-cols-2 gap-4">
      <div className="space-y-2">
        <Label htmlFor="group_limit">{t('admin.plans.dialog.groupLimit')}</Label>
        <Input
          id="group_limit"
          type="number"
          value={formData.group_limit}
          onChange={(e) => setFormData({ ...formData, group_limit: e.target.value })}
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor="storage_limit">{t('admin.plans.dialog.storageLimit')}</Label>
        <Input
          id="storage_limit"
          type="number"
          value={formData.storage_limit}
          onChange={(e) => setFormData({ ...formData, storage_limit: e.target.value })}
        />
      </div>
    </div>
  );
}

function KiwifyLinksSection({ formData, setFormData, t }: Omit<PlanFormProps, 'onSave'>) {
  return (
    <div className="space-y-4 border-t pt-4">
      <div>
        <Label className="text-base font-semibold">
          {t('admin.plans.dialog.kiwifyLinks.title')}
        </Label>
        <p className="text-sm text-muted-foreground mt-1">
          {t('admin.plans.dialog.kiwifyLinks.description')}
        </p>
      </div>

      <div className="bg-blue-50 dark:bg-blue-950 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
        <p className="text-sm font-medium text-blue-900 dark:text-blue-100 mb-2">
          {t('admin.plans.dialog.kiwifyLinks.howToFind.title')}
        </p>
        <ol className="text-sm text-blue-800 dark:text-blue-200 space-y-1 list-decimal list-inside">
          <li>{t('admin.plans.dialog.kiwifyLinks.howToFind.step1')}</li>
          <li>{t('admin.plans.dialog.kiwifyLinks.howToFind.step2')}</li>
          <li>{t('admin.plans.dialog.kiwifyLinks.howToFind.step3')}</li>
        </ol>
      </div>

      <div className="grid grid-cols-1 gap-4">
        <KiwifyIdInput
          id="kiwify_monthly"
          label={t('admin.plans.dialog.kiwifyLinks.monthlyId')}
          price={formData.price_monthly}
          value={formData.kiwify_product_id_monthly}
          onChange={(value) => setFormData({ ...formData, kiwify_product_id_monthly: value })}
          placeholder={t('admin.plans.dialog.kiwifyLinks.placeholder', {
            example: 'SeNH87h',
          })}
          hint={t('admin.plans.dialog.kiwifyLinks.hint')}
        />
        <KiwifyIdInput
          id="kiwify_semester"
          label={t('admin.plans.dialog.kiwifyLinks.semesterId')}
          price={formData.price_semester}
          value={formData.kiwify_product_id_semester}
          onChange={(value) => setFormData({ ...formData, kiwify_product_id_semester: value })}
          placeholder={t('admin.plans.dialog.kiwifyLinks.placeholder', {
            example: 'o1oHx40',
          })}
          hint={t('admin.plans.dialog.kiwifyLinks.hint')}
        />
        <KiwifyIdInput
          id="kiwify_annual"
          label={t('admin.plans.dialog.kiwifyLinks.annualId')}
          price={formData.price_annual}
          value={formData.kiwify_product_id_annual}
          onChange={(value) => setFormData({ ...formData, kiwify_product_id_annual: value })}
          placeholder={t('admin.plans.dialog.kiwifyLinks.placeholder', {
            example: 'M15R197',
          })}
          hint={t('admin.plans.dialog.kiwifyLinks.hint')}
        />
      </div>

      <div className="bg-amber-50 dark:bg-amber-950 border border-amber-200 dark:border-amber-800 rounded-lg p-3">
        <p className="text-xs text-amber-900 dark:text-amber-100">
          <strong>{t('common.tip')}:</strong> {t('admin.plans.dialog.kiwifyLinks.tip')}
        </p>
      </div>
    </div>
  );
}

interface KiwifyIdInputProps {
  id: string;
  label: string;
  price: string;
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
  hint: string;
}

function KiwifyIdInput({
  id,
  label,
  price,
  value,
  onChange,
  placeholder,
  hint,
}: KiwifyIdInputProps) {
  const showPrice = price && Number.parseFloat(price) > 0;

  return (
    <div className="space-y-2">
      <Label htmlFor={id} className="flex items-center gap-2">
        {label}
        {showPrice && (
          <span className="text-xs font-normal text-muted-foreground">
            (R$ {Number.parseFloat(price).toFixed(2)})
          </span>
        )}
      </Label>
      <Input
        id={id}
        value={value}
        onChange={(e) => onChange(e.target.value.trim())}
        placeholder={placeholder}
      />
      <p className="text-xs text-muted-foreground">{hint}</p>
    </div>
  );
}
