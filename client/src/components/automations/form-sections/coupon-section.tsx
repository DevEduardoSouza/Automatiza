import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { Automation, AutomationCoupon, StoreKey } from '@/types/automation';
import { useTranslation } from 'react-i18next';

interface CouponSelectorProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  availableCoupons: AutomationCoupon[];
}

export function CouponSelector({ formData, setFormData, availableCoupons }: CouponSelectorProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-2">
      <Label>{t('pages.offerAutomations.form.coupon')}</Label>
      <p className="text-xs" style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
        {t('pages.offerAutomations.form.couponHint')}
      </p>
      <Select
        value={formData.coupon_id || undefined}
        onValueChange={(value) => setFormData((prev) => ({ ...prev, coupon_id: value || null }))}
      >
        <SelectTrigger>
          <SelectValue placeholder={t('pages.offerAutomations.form.noCoupon')} />
        </SelectTrigger>
        <SelectContent>
          {availableCoupons
            .filter((coupon) => {
              if (!formData.stores || formData.stores.length === 0) return true;
              return formData.stores.includes(coupon.store as StoreKey);
            })
            .map((coupon) => (
              <SelectItem key={coupon.id} value={coupon.id}>
                {coupon.code} - {coupon.title}
              </SelectItem>
            ))}
        </SelectContent>
      </Select>
      {formData.coupon_id && (
        <p className="text-xs" style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
          {t('pages.offerAutomations.form.couponActive')}
        </p>
      )}
    </div>
  );
}
