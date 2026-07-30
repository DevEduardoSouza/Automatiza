import { Card, CardContent } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { GENERAL_CATEGORY } from '@/config/automation-categories';
import type { Automation } from '@/types/automation';
import { useTranslation } from 'react-i18next';

interface SubcategoryItem {
  key: string;
  i18nKey: string;
  /** Nome vindo do banco, usado quando a chave i18n nao esta cadastrada */
  name?: string;
}

interface SubcategoryGroup {
  i18nKey: string;
  /** Nome do grupo vindo do banco, usado como fallback do i18n */
  name?: string;
  items: readonly SubcategoryItem[];
}

interface CategoryRefinementSectionProps {
  /** Parent category constant (e.g., "Eletrônicos") */
  parentCategory: string;
  /** Display label for the section */
  label: string;
  /** Description text shown below the label */
  description: string;
  /** Grouped subcategories to render */
  subcategoryGroups: SubcategoryGroup[];
  /** All subcategory keys for the "select all" toggle */
  allSubcategoryKeys: string[];
  /** Whether the refinement panel is open */
  isOpen: boolean;
  /** Function to check if a value is a subcategory key */
  isSubcategoryKey: (value: string) => boolean;
  /** Current form data */
  formData: Partial<Automation>;
  /** Form data setter */
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
}

/**
 * Reusable component for category refinement sections.
 * Renders a collapsible panel with subcategory checkboxes.
 */
export function CategoryRefinementSection({
  parentCategory,
  label,
  description,
  subcategoryGroups,
  allSubcategoryKeys,
  isOpen,
  isSubcategoryKey,
  formData,
  setFormData,
}: CategoryRefinementSectionProps) {
  const { t } = useTranslation();
  const selected = formData.categories || [];
  const hasParent = selected.includes(parentCategory);
  const hasSubcats = selected.some(isSubcategoryKey);
  const refineEnabled = hasSubcats;

  if (!hasParent || !isOpen) {
    return null;
  }

  const handleToggleRefine = (checked: boolean) => {
    setFormData((prev) => {
      const current = prev.categories || [];
      const withoutGeneral = current.filter((c) => c !== GENERAL_CATEGORY);
      const withoutSubcats = withoutGeneral.filter((c) => !isSubcategoryKey(c));
      const ensureParent = withoutSubcats.includes(parentCategory)
        ? withoutSubcats
        : [...withoutSubcats, parentCategory];

      if (checked) {
        return {
          ...prev,
          categories: [...ensureParent, ...allSubcategoryKeys],
        };
      }
      return { ...prev, categories: ensureParent };
    });
  };

  const handleSubcategoryChange = (key: string, checked: boolean) => {
    setFormData((prev) => {
      const current = prev.categories || [];
      const base = current.filter((c) => c !== GENERAL_CATEGORY);
      const ensureParent = base.includes(parentCategory) ? base : [...base, parentCategory];
      const already = ensureParent.includes(key);
      const next = checked
        ? already
          ? ensureParent
          : [...ensureParent, key]
        : ensureParent.filter((c) => c !== key);
      return { ...prev, categories: next };
    });
  };

  return (
    <Card className="border-dashed">
      <CardContent className="pt-4 space-y-3">
        <div className="space-y-1">
          <Label>{label}</Label>
          <p className="text-xs text-muted-foreground">{description}</p>
        </div>

        <div className="flex items-center gap-2">
          <Switch checked={refineEnabled} onCheckedChange={handleToggleRefine} />
          <Label className="cursor-pointer">{t('automations.refine', { category: label })}</Label>
        </div>

        {refineEnabled && (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4 xl:gap-5">
            {subcategoryGroups.map((group) => (
              <div key={group.i18nKey} className="space-y-2">
                <Label className="text-sm">{t(group.i18nKey, { defaultValue: group.name })}</Label>
                <div className="space-y-2">
                  {group.items.map((item) => (
                    <div key={item.key} className="flex items-center space-x-2">
                      <Checkbox
                        checked={formData.categories?.includes(item.key)}
                        onCheckedChange={(checked) =>
                          handleSubcategoryChange(item.key, Boolean(checked))
                        }
                      />
                      <Label className="cursor-pointer">
                        {t(item.i18nKey, { defaultValue: item.name })}
                      </Label>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}

        {refineEnabled && (
          <p className="text-xs text-muted-foreground">
            {t('automations.refineTip', { category: label })}
          </p>
        )}
      </CardContent>
    </Card>
  );
}
