import { Card, CardContent } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import {
  ALL_BELEZA_SUBCATEGORY_KEYS,
  ALL_CASA_SUBCATEGORY_KEYS,
  ALL_ELETRODOMESTICOS_SUBCATEGORY_KEYS,
  ALL_ELETRONICOS_SUBCATEGORY_KEYS,
  ALL_ESPORTES_SUBCATEGORY_KEYS,
  ALL_HOMENS_SUBCATEGORY_KEYS,
  ALL_MODA_SUBCATEGORY_KEYS,
  BELEZA_CATEGORY,
  BELEZA_GROUPS,
  CASA_CATEGORY,
  CASA_GROUPS,
  ELETRODOMESTICOS_CATEGORY,
  ELETRODOMESTICOS_GROUPS,
  ELETRONICOS_CATEGORY,
  ELETRONICOS_GROUPS,
  ESPORTES_CATEGORY,
  ESPORTES_GROUPS,
  CATEGORIES as FALLBACK_CATEGORIES,
  GENERAL_CATEGORY,
  HOMENS_CATEGORY,
  HOMENS_GROUPS,
  MODA_CATEGORY,
  MODA_GROUPS,
  isBelezaSubcategoryKey,
  isCasaSubcategoryKey,
  isEletrodomesticosSubcategoryKey,
  isEletronicosSubcategoryKey,
  isEsportesSubcategoryKey,
  isHomensSubcategoryKey,
  isModaSubcategoryKey,
} from '@/config/automation-categories';
import type { RefinementState } from '@/hooks/use-automations';
import { type LocalCategory, useCategories } from '@/hooks/use-categories';
import type { Automation } from '@/types/automation';
import { ChevronDown, ChevronUp } from 'lucide-react';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { CategoryRefinementSection } from '../category-refinement-section';

interface CategoriesSectionProps {
  formData: Partial<Automation>;
  setFormData: React.Dispatch<React.SetStateAction<Partial<Automation>>>;
  refinementOpen: RefinementState;
  toggleRefinement: (key: keyof RefinementState) => void;
  closeAllRefinements: () => void;
}

/**
 * Maps category slug to refinement key.
 * Used to map dynamic categories to the refinement state keys.
 */
const SLUG_TO_REFINEMENT_KEY: Record<string, keyof RefinementState> = {
  eletronicos: 'eletronicos',
  eletrodomesticos: 'eletrodomesticos',
  moda: 'moda',
  homens: 'homens',
  esportes: 'esportes',
  casaDecoracao: 'casa',
  beleza: 'beleza',
};

/**
 * Maps category name to refinement key for legacy support.
 */
const NAME_TO_REFINEMENT_KEY: Record<string, keyof RefinementState> = {
  Eletrônicos: 'eletronicos',
  Eletrodomésticos: 'eletrodomesticos',
  Moda: 'moda',
  Homens: 'homens',
  Esportes: 'esportes',
  'Casa e Decoração': 'casa',
  Beleza: 'beleza',
};

/**
 * Fallback groups and keys for when API data is unavailable.
 * Maps refinement key to its hardcoded data.
 */
const FALLBACK_REFINEMENT_DATA: Record<
  keyof RefinementState,
  {
    category: string;
    groups: typeof ELETRONICOS_GROUPS;
    allKeys: readonly string[];
    isSubcategoryKey: (c: string) => boolean;
    translationKey: string;
  }
> = {
  eletronicos: {
    category: ELETRONICOS_CATEGORY,
    groups: ELETRONICOS_GROUPS,
    allKeys: ALL_ELETRONICOS_SUBCATEGORY_KEYS,
    isSubcategoryKey: isEletronicosSubcategoryKey,
    translationKey: 'automations.refinement.eletronicos',
  },
  eletrodomesticos: {
    category: ELETRODOMESTICOS_CATEGORY,
    groups: ELETRODOMESTICOS_GROUPS,
    allKeys: ALL_ELETRODOMESTICOS_SUBCATEGORY_KEYS,
    isSubcategoryKey: isEletrodomesticosSubcategoryKey,
    translationKey: 'automations.refinement.eletrodomesticos',
  },
  moda: {
    category: MODA_CATEGORY,
    groups: MODA_GROUPS,
    allKeys: ALL_MODA_SUBCATEGORY_KEYS,
    isSubcategoryKey: isModaSubcategoryKey,
    translationKey: 'automations.refinement.moda',
  },
  homens: {
    category: HOMENS_CATEGORY,
    groups: HOMENS_GROUPS,
    allKeys: ALL_HOMENS_SUBCATEGORY_KEYS,
    isSubcategoryKey: isHomensSubcategoryKey,
    translationKey: 'automations.refinement.homens',
  },
  esportes: {
    category: ESPORTES_CATEGORY,
    groups: ESPORTES_GROUPS,
    allKeys: ALL_ESPORTES_SUBCATEGORY_KEYS,
    isSubcategoryKey: isEsportesSubcategoryKey,
    translationKey: 'automations.refinement.esportes',
  },
  casa: {
    category: CASA_CATEGORY,
    groups: CASA_GROUPS,
    allKeys: ALL_CASA_SUBCATEGORY_KEYS,
    isSubcategoryKey: isCasaSubcategoryKey,
    translationKey: 'automations.refinement.casaDecoracao',
  },
  beleza: {
    category: BELEZA_CATEGORY,
    groups: BELEZA_GROUPS,
    allKeys: ALL_BELEZA_SUBCATEGORY_KEYS,
    isSubcategoryKey: isBelezaSubcategoryKey,
    translationKey: 'automations.refinement.beleza',
  },
};

/**
 * Transform API category data to the format expected by refinement sections.
 */
function buildRefinementDataFromApi(category: LocalCategory) {
  // Group subcategories by groupName
  const groupedSubs: Record<string, { key: string; i18nKey: string }[]> = {};

  for (const sub of category.subcategories) {
    const groupName = sub.groupName || 'Geral';
    if (!groupedSubs[groupName]) {
      groupedSubs[groupName] = [];
    }
    groupedSubs[groupName].push({
      key: sub.displayKey,
      i18nKey: `config.subcategories.dynamic.${sub.slug}`, // Fallback i18n key
    });
  }

  const groups = Object.entries(groupedSubs).map(([name, items]) => ({
    name,
    i18nKey: `config.subcategoryGroups.${name.toLowerCase().replace(/\s+/g, '')}`,
    items: items.sort((a, b) => a.key.localeCompare(b.key)),
  }));

  const allKeys = category.subcategories.map((s) => s.displayKey);

  return { groups, allKeys };
}

export function CategoriesSection({
  formData,
  setFormData,
  refinementOpen,
  toggleRefinement,
  closeAllRefinements,
}: CategoriesSectionProps) {
  const { t } = useTranslation();

  // Fetch categories from API with fallback
  const {
    categories: apiCategories,
    isLoading,
    isError,
  } = useCategories({
    includeInactive: false,
    includeSubcategories: true,
  });

  // Use API categories or fallback to hardcoded
  const useFallback = isLoading || isError || apiCategories.length === 0;

  // Build category list from API or fallback
  const categoryList = useMemo(() => {
    if (useFallback) {
      return FALLBACK_CATEGORIES;
    }
    return apiCategories
      .filter((c) => c.isActive)
      .sort((a, b) => a.displayOrder - b.displayOrder)
      .map((c) => c.name);
  }, [useFallback, apiCategories]);

  // Build category map for quick lookup
  const categoryByName = useMemo(() => {
    const map = new Map<string, LocalCategory>();
    for (const cat of apiCategories) {
      map.set(cat.name, cat);
    }
    return map;
  }, [apiCategories]);

  // Get refinement key for a category name
  const getRefinementKey = (categoryName: string): keyof RefinementState | null => {
    // First check direct name mapping (legacy)
    if (NAME_TO_REFINEMENT_KEY[categoryName]) {
      return NAME_TO_REFINEMENT_KEY[categoryName];
    }
    // Then check if we have API category data
    const apiCat = categoryByName.get(categoryName);
    if (apiCat && SLUG_TO_REFINEMENT_KEY[apiCat.slug]) {
      return SLUG_TO_REFINEMENT_KEY[apiCat.slug];
    }
    return null;
  };

  // Check if a category has refinements (only if it actually has subcategories)
  const hasRefinements = (categoryName: string): boolean => {
    // Don't show chevrons while loading - wait for actual data
    if (isLoading) {
      return false;
    }
    // When using fallback (error or no data), use hardcoded mapping
    if (useFallback) {
      return NAME_TO_REFINEMENT_KEY[categoryName] !== undefined;
    }
    // With API data, only show chevrons if category has subcategories registered
    const apiCat = categoryByName.get(categoryName);
    return (apiCat?.hasRefinements && apiCat.subcategories.length > 0) ?? false;
  };

  // Check if a value is a subcategory key (for any category)
  const isAnySubcategoryKey = (value: string): boolean => {
    // Check fallback data
    for (const data of Object.values(FALLBACK_REFINEMENT_DATA)) {
      if (data.isSubcategoryKey(value)) {
        return true;
      }
    }
    // Check API data
    for (const cat of apiCategories) {
      for (const sub of cat.subcategories) {
        if (sub.displayKey === value) {
          return true;
        }
      }
    }
    return false;
  };

  // Get subcategory check function for a category
  const getSubcategoryChecker = (categoryName: string): ((c: string) => boolean) => {
    const refinementKey = getRefinementKey(categoryName);
    if (refinementKey && FALLBACK_REFINEMENT_DATA[refinementKey]) {
      return FALLBACK_REFINEMENT_DATA[refinementKey].isSubcategoryKey;
    }
    // For API categories, create a dynamic checker
    const apiCat = categoryByName.get(categoryName);
    if (apiCat) {
      const keys = new Set(apiCat.subcategories.map((s) => s.displayKey));
      return (c: string) => keys.has(c);
    }
    return () => false;
  };

  const handleCategoryChange = (cat: string, checked: boolean) => {
    const isGeneral = cat === GENERAL_CATEGORY || cat === 'Ofertas gerais';
    setFormData((prev) => {
      const current = prev.categories || [];
      const isSelected = current.includes(cat);

      if (checked && isGeneral) {
        return { ...prev, categories: [cat] };
      }

      if (
        checked &&
        !isGeneral &&
        (current.includes(GENERAL_CATEGORY) || current.includes('Ofertas gerais'))
      ) {
        const withoutGeneral = current.filter(
          (c) => c !== GENERAL_CATEGORY && c !== 'Ofertas gerais'
        );
        return {
          ...prev,
          categories: isSelected ? withoutGeneral : [...withoutGeneral, cat],
        };
      }

      // Handle unchecking parent categories - remove their subcategories
      if (!checked) {
        const checker = getSubcategoryChecker(cat);
        const refinementKey = getRefinementKey(cat);
        if (refinementKey) {
          closeAllRefinements();
        }
        return {
          ...prev,
          categories: current.filter((c) => c !== cat && !checker(c)),
        };
      }

      return {
        ...prev,
        categories: checked ? [...current, cat] : current.filter((c) => c !== cat),
      };
    });
  };

  const renderCategoryButton = (cat: string, refinementKey: keyof RefinementState) => {
    const isChecked = formData.categories?.includes(cat);

    const handleClick = () => {
      // If category is not checked, check it first
      if (!isChecked) {
        handleCategoryChange(cat, true);
      }
      // Then toggle the refinement panel
      toggleRefinement(refinementKey);
    };

    return (
      <button
        type="button"
        className="flex items-center gap-1 text-left"
        onClick={handleClick}
        title={t('pages.offerAutomations.form.toggleRefinement', { category: cat })}
      >
        <Label className="cursor-pointer">{cat}</Label>
        {refinementOpen[refinementKey] ? (
          <ChevronUp className="h-4 w-4 text-muted-foreground" />
        ) : (
          <ChevronDown className="h-4 w-4 text-muted-foreground" />
        )}
      </button>
    );
  };

  // Build refinement sections based on available data
  const refinementSections = useMemo(() => {
    const sections: Array<{
      key: keyof RefinementState;
      parentCategory: string;
      label: string;
      groups: Array<{ i18nKey: string; items: readonly { key: string; i18nKey: string }[] }>;
      allKeys: string[];
      isSubcategoryKey: (c: string) => boolean;
    }> = [];

    // For each refinement key, build section data
    const refinementKeys: (keyof RefinementState)[] = [
      'eletronicos',
      'eletrodomesticos',
      'esportes',
      'homens',
      'moda',
      'casa',
      'beleza',
    ];

    for (const key of refinementKeys) {
      const fallback = FALLBACK_REFINEMENT_DATA[key];

      // Check if we have API data for this category
      const apiCat = apiCategories.find(
        (c) => SLUG_TO_REFINEMENT_KEY[c.slug] === key || c.name === fallback.category
      );

      if (apiCat && apiCat.hasRefinements && apiCat.subcategories.length > 0 && !useFallback) {
        // Use API data
        const { groups, allKeys } = buildRefinementDataFromApi(apiCat);
        sections.push({
          key,
          parentCategory: apiCat.name,
          label: t(fallback.translationKey),
          groups: groups.map((g) => ({
            i18nKey: g.i18nKey,
            items: g.items,
          })),
          allKeys,
          isSubcategoryKey: (c: string) => allKeys.includes(c),
        });
      } else {
        // Use fallback data
        sections.push({
          key,
          parentCategory: fallback.category,
          label: t(fallback.translationKey),
          groups: fallback.groups,
          allKeys: [...fallback.allKeys],
          isSubcategoryKey: fallback.isSubcategoryKey,
        });
      }
    }

    return sections;
  }, [apiCategories, useFallback, t]);

  return (
    <div className="space-y-2">
      <Label>{t('pages.offerAutomations.form.categories')}</Label>
      <p className="text-xs text-muted-foreground">
        {t('pages.offerAutomations.form.categoriesHint', { general: GENERAL_CATEGORY })}
      </p>
      <Card>
        <CardContent className="pt-4 grid grid-cols-2 xl:grid-cols-3 gap-2 lg:gap-3 max-h-40 lg:max-h-52 xl:max-h-60 overflow-y-auto">
          {categoryList.map((cat) => {
            const refinementKey = getRefinementKey(cat);
            const showRefinementToggle = hasRefinements(cat) && refinementKey;

            return (
              <div key={cat} className="flex items-center space-x-2">
                <Checkbox
                  checked={formData.categories?.includes(cat)}
                  onCheckedChange={(checked) => handleCategoryChange(cat, Boolean(checked))}
                />
                {showRefinementToggle ? (
                  renderCategoryButton(cat, refinementKey)
                ) : (
                  <Label className="cursor-pointer">{cat}</Label>
                )}
              </div>
            );
          })}
        </CardContent>
      </Card>

      {/* Render refinement sections */}
      {refinementSections.map((section) => (
        <CategoryRefinementSection
          key={section.key}
          parentCategory={section.parentCategory}
          label={section.label}
          description={t('automations.refinement.description', {
            category: section.label,
          })}
          subcategoryGroups={section.groups}
          allSubcategoryKeys={section.allKeys}
          isOpen={refinementOpen[section.key]}
          isSubcategoryKey={section.isSubcategoryKey}
          formData={formData}
          setFormData={setFormData}
        />
      ))}
    </div>
  );
}
