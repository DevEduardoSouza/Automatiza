import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import type { CTA } from '@/entities/cta';
import type { Automation } from '@/types/automation';
import { MessageSquare } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface CTASelectorProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  availableCTAs: CTA[];
}

export function CTASelector({ formData, setFormData, availableCTAs }: CTASelectorProps) {
  const { t } = useTranslation();
  const selectedIds = formData.ctas || [];

  const handleToggle = (ctaId: string, checked: boolean) => {
    setFormData((prev) => {
      const current = prev.ctas || [];
      const next = checked ? [...current, ctaId] : current.filter((id) => id !== ctaId);
      return { ...prev, ctas: next };
    });
  };

  if (availableCTAs.length === 0) return null;

  return (
    <div className="space-y-1.5 xs:space-y-2">
      <Label className="cursor-pointer">{t('pages.offerAutomations.form.ctas')}</Label>
      <p className="text-xs" style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
        {t('pages.offerAutomations.form.ctasHint')}
      </p>
      <div
        className="rounded-xl border p-3 max-h-[200px] overflow-y-auto space-y-2"
        style={{ borderColor: 'rgba(141, 45, 83, 0.15)' }}
      >
        {availableCTAs.map((cta) => (
          // biome-ignore lint/a11y/noLabelWithoutControl: Radix Checkbox is wrapped inside the label
          <label
            key={cta.id}
            className="flex items-start gap-3 p-2 rounded-lg cursor-pointer hover:bg-gray-50 transition-colors"
          >
            <Checkbox
              checked={selectedIds.includes(cta.id)}
              onCheckedChange={(checked) => handleToggle(cta.id, !!checked)}
              className="mt-0.5"
            />
            <div className="flex-1 min-w-0">
              <p className="text-sm line-clamp-2">{cta.message}</p>
              <div className="flex gap-1.5 mt-1">
                <Badge
                  variant="outline"
                  className="text-[10px]"
                  style={{ borderColor: 'rgba(141, 45, 83, 0.2)', color: '#8D2D53' }}
                >
                  {t(`pages.ctas.platforms.${cta.platform}`)}
                </Badge>
                <Badge
                  variant="outline"
                  className="text-[10px]"
                  style={{ borderColor: 'rgba(141, 45, 83, 0.2)', color: '#8D2D53' }}
                >
                  {t(`pages.ctas.types.${cta.cta_type}`)}
                </Badge>
              </div>
            </div>
          </label>
        ))}
      </div>
      {selectedIds.length > 0 && (
        <p className="text-xs flex items-center gap-1" style={{ color: 'rgba(141, 45, 83, 0.6)' }}>
          <MessageSquare className="h-3 w-3" />
          {t('pages.offerAutomations.form.ctasSelected', { count: selectedIds.length })}
        </p>
      )}
    </div>
  );
}
