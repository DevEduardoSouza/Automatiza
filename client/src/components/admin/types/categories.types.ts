import type { LocalCategory, LocalSubcategory } from '@/hooks/use-categories';

// Re-export for convenience
export type { LocalCategory, LocalSubcategory };

export interface CategoryFormData {
  slug: string;
  name: string;
  icon: string;
  displayOrder: string;
  hasRefinements: boolean;
}

export interface SubcategoryTranslations {
  en?: string;
  es?: string;
  [key: string]: string | undefined;
}

export interface SubcategoryFormData {
  slug: string;
  name: string;
  displayKey: string;
  groupName: string;
  displayOrder: string;
  translations?: SubcategoryTranslations;
}

export const INITIAL_CATEGORY_FORM_DATA: CategoryFormData = {
  slug: '',
  name: '',
  icon: '',
  displayOrder: '0',
  hasRefinements: false,
};

export const INITIAL_SUBCATEGORY_FORM_DATA: SubcategoryFormData = {
  slug: '',
  name: '',
  displayKey: '',
  groupName: '',
  displayOrder: '0',
  translations: {},
};

export interface UseCategoriesTabReturn {
  categories: LocalCategory[];
  loading: boolean;
  saving: boolean;
  // Category dialog state
  editingCategory: LocalCategory | null;
  isCategoryDialogOpen: boolean;
  setIsCategoryDialogOpen: (open: boolean) => void;
  categoryFormData: CategoryFormData;
  setCategoryFormData: React.Dispatch<React.SetStateAction<CategoryFormData>>;
  // Subcategory dialog state
  editingSubcategory: LocalSubcategory | null;
  parentCategoryForSubcategory: LocalCategory | null;
  isSubcategoryDialogOpen: boolean;
  setIsSubcategoryDialogOpen: (open: boolean) => void;
  subcategoryFormData: SubcategoryFormData;
  setSubcategoryFormData: React.Dispatch<React.SetStateAction<SubcategoryFormData>>;
  // Category actions
  handleOpenCategoryDialog: (category?: LocalCategory) => void;
  handleSaveCategory: () => Promise<void>;
  handleDeleteCategory: (id: string, isSystem: boolean) => Promise<void>;
  handleToggleCategoryActive: (category: LocalCategory) => Promise<void>;
  // Subcategory actions
  handleOpenSubcategoryDialog: (
    parentCategory: LocalCategory,
    subcategory?: LocalSubcategory
  ) => void;
  handleSaveSubcategory: () => Promise<void>;
  handleDeleteSubcategory: (id: string, isSystem: boolean) => Promise<void>;
  handleToggleSubcategoryActive: (subcategory: LocalSubcategory) => Promise<void>;
  // Expanded categories state
  expandedCategories: Set<string>;
  toggleCategoryExpanded: (categoryId: string) => void;
}
