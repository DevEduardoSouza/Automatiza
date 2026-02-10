import { type LocalCategory, type LocalSubcategory, useCategories } from '@/hooks/use-categories';
import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import {
  type CategoryFormData,
  INITIAL_CATEGORY_FORM_DATA,
  INITIAL_SUBCATEGORY_FORM_DATA,
  type SubcategoryFormData,
  type UseCategoriesTabReturn,
} from '../types/categories.types';

/**
 * Generates a slug from a name string.
 * Converts to lowercase, removes accents, and replaces spaces with hyphens.
 */
function generateSlug(name: string): string {
  return name
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '') // Remove accents
    .replace(/[^a-z0-9\s-]/g, '') // Remove special characters
    .replace(/\s+/g, '-') // Replace spaces with hyphens
    .replace(/-+/g, '-') // Replace multiple hyphens with single
    .trim();
}

/**
 * Hook that manages categories tab logic.
 * Uses the shared useCategories hook for data and mutations.
 */
export function useCategoriesTab(): UseCategoriesTabReturn {
  const { t } = useTranslation();

  // Get categories data and mutations from shared hook
  const {
    categories,
    isLoading: loading,
    createCategory,
    updateCategory,
    deleteCategory,
    createSubcategory,
    updateSubcategory,
    deleteSubcategory,
    isCreatingCategory,
    isUpdatingCategory,
    isDeletingCategory,
    isCreatingSubcategory,
    isUpdatingSubcategory,
    isDeletingSubcategory,
  } = useCategories({ includeInactive: true, includeSubcategories: true });

  const saving =
    isCreatingCategory ||
    isUpdatingCategory ||
    isDeletingCategory ||
    isCreatingSubcategory ||
    isUpdatingSubcategory ||
    isDeletingSubcategory;

  // Category dialog state
  const [editingCategory, setEditingCategory] = useState<LocalCategory | null>(null);
  const [isCategoryDialogOpen, setIsCategoryDialogOpen] = useState(false);
  const [categoryFormData, setCategoryFormData] = useState<CategoryFormData>(
    INITIAL_CATEGORY_FORM_DATA
  );

  // Subcategory dialog state
  const [editingSubcategory, setEditingSubcategory] = useState<LocalSubcategory | null>(null);
  const [parentCategoryForSubcategory, setParentCategoryForSubcategory] =
    useState<LocalCategory | null>(null);
  const [isSubcategoryDialogOpen, setIsSubcategoryDialogOpen] = useState(false);
  const [subcategoryFormData, setSubcategoryFormData] = useState<SubcategoryFormData>(
    INITIAL_SUBCATEGORY_FORM_DATA
  );

  // Expanded categories state (for accordion-like behavior)
  const [expandedCategories, setExpandedCategories] = useState<Set<string>>(new Set());

  const toggleCategoryExpanded = useCallback((categoryId: string) => {
    setExpandedCategories((prev) => {
      const next = new Set(prev);
      if (next.has(categoryId)) {
        next.delete(categoryId);
      } else {
        next.add(categoryId);
      }
      return next;
    });
  }, []);

  // ============================================================
  // Category Actions
  // ============================================================

  const handleOpenCategoryDialog = useCallback((category?: LocalCategory) => {
    if (category) {
      setEditingCategory(category);
      setCategoryFormData({
        slug: category.slug,
        name: category.name,
        icon: category.icon ?? '',
        displayOrder: category.displayOrder.toString(),
        hasRefinements: category.hasRefinements,
      });
    } else {
      setEditingCategory(null);
      setCategoryFormData(INITIAL_CATEGORY_FORM_DATA);
    }
    setIsCategoryDialogOpen(true);
  }, []);

  const handleSaveCategory = useCallback(async () => {
    if (!categoryFormData.name.trim()) {
      toast.error(t('admin.categories.errors.nameRequired', 'Nome é obrigatório'));
      return;
    }

    const slug = editingCategory?.slug || generateSlug(categoryFormData.name);

    try {
      if (editingCategory) {
        await updateCategory({
          id: editingCategory.id,
          name: categoryFormData.name,
          icon: categoryFormData.icon || undefined,
          displayOrder: Number.parseInt(categoryFormData.displayOrder, 10) || 0,
          hasRefinements: categoryFormData.hasRefinements,
        });
      } else {
        await createCategory({
          slug,
          name: categoryFormData.name,
          icon: categoryFormData.icon || undefined,
          displayOrder: Number.parseInt(categoryFormData.displayOrder, 10) || 0,
          hasRefinements: categoryFormData.hasRefinements,
        });
      }
      setIsCategoryDialogOpen(false);
      setCategoryFormData(INITIAL_CATEGORY_FORM_DATA);
      setEditingCategory(null);
    } catch {
      // Error is handled by the hook's toast
    }
  }, [categoryFormData, editingCategory, createCategory, updateCategory, t]);

  const handleDeleteCategory = useCallback(
    async (id: string, isSystem: boolean) => {
      if (isSystem) {
        toast.error(
          t(
            'admin.categories.errors.cannotDeleteSystem',
            'Categorias do sistema não podem ser excluídas'
          )
        );
        return;
      }

      if (
        !confirm(
          t('admin.categories.confirm.delete', 'Tem certeza que deseja excluir esta categoria?')
        )
      ) {
        return;
      }

      try {
        await deleteCategory(id);
      } catch {
        // Error is handled by the hook's toast
      }
    },
    [deleteCategory, t]
  );

  const handleToggleCategoryActive = useCallback(
    async (category: LocalCategory) => {
      try {
        await updateCategory({
          id: category.id,
          isActive: !category.isActive,
        });
      } catch {
        // Error is handled by the hook's toast
      }
    },
    [updateCategory]
  );

  // ============================================================
  // Subcategory Actions
  // ============================================================

  const handleOpenSubcategoryDialog = useCallback(
    (parentCategory: LocalCategory, subcategory?: LocalSubcategory) => {
      setParentCategoryForSubcategory(parentCategory);

      if (subcategory) {
        setEditingSubcategory(subcategory);
        setSubcategoryFormData({
          slug: subcategory.slug,
          name: subcategory.name,
          displayKey: subcategory.displayKey,
          groupName: subcategory.groupName ?? '',
          displayOrder: subcategory.displayOrder.toString(),
        });
      } else {
        setEditingSubcategory(null);
        setSubcategoryFormData(INITIAL_SUBCATEGORY_FORM_DATA);
      }
      setIsSubcategoryDialogOpen(true);
    },
    []
  );

  const handleSaveSubcategory = useCallback(async () => {
    if (!subcategoryFormData.name.trim()) {
      toast.error(t('admin.categories.errors.nameRequired', 'Nome é obrigatório'));
      return;
    }

    if (!subcategoryFormData.displayKey.trim()) {
      toast.error(t('admin.categories.errors.displayKeyRequired', 'Display Key é obrigatório'));
      return;
    }

    if (!parentCategoryForSubcategory) {
      toast.error(t('admin.categories.errors.parentRequired', 'Categoria pai é obrigatória'));
      return;
    }

    const slug = editingSubcategory?.slug || generateSlug(subcategoryFormData.name);

    try {
      if (editingSubcategory) {
        await updateSubcategory({
          id: editingSubcategory.id,
          name: subcategoryFormData.name,
          displayKey: subcategoryFormData.displayKey,
          groupName: subcategoryFormData.groupName || undefined,
          displayOrder: Number.parseInt(subcategoryFormData.displayOrder, 10) || 0,
        });
      } else {
        await createSubcategory({
          categoryId: parentCategoryForSubcategory.id,
          slug,
          name: subcategoryFormData.name,
          displayKey: subcategoryFormData.displayKey,
          groupName: subcategoryFormData.groupName || undefined,
          displayOrder: Number.parseInt(subcategoryFormData.displayOrder, 10) || 0,
        });
      }
      setIsSubcategoryDialogOpen(false);
      setSubcategoryFormData(INITIAL_SUBCATEGORY_FORM_DATA);
      setEditingSubcategory(null);
      setParentCategoryForSubcategory(null);
    } catch {
      // Error is handled by the hook's toast
    }
  }, [
    subcategoryFormData,
    editingSubcategory,
    parentCategoryForSubcategory,
    createSubcategory,
    updateSubcategory,
    t,
  ]);

  const handleDeleteSubcategory = useCallback(
    async (id: string, isSystem: boolean) => {
      if (isSystem) {
        toast.error(
          t(
            'admin.categories.errors.cannotDeleteSystemSubcategory',
            'Subcategorias do sistema não podem ser excluídas'
          )
        );
        return;
      }

      if (
        !confirm(
          t(
            'admin.categories.confirm.deleteSubcategory',
            'Tem certeza que deseja excluir esta subcategoria?'
          )
        )
      ) {
        return;
      }

      try {
        await deleteSubcategory(id);
      } catch {
        // Error is handled by the hook's toast
      }
    },
    [deleteSubcategory, t]
  );

  const handleToggleSubcategoryActive = useCallback(
    async (subcategory: LocalSubcategory) => {
      try {
        await updateSubcategory({
          id: subcategory.id,
          isActive: !subcategory.isActive,
        });
      } catch {
        // Error is handled by the hook's toast
      }
    },
    [updateSubcategory]
  );

  return {
    categories,
    loading,
    saving,
    // Category dialog state
    editingCategory,
    isCategoryDialogOpen,
    setIsCategoryDialogOpen,
    categoryFormData,
    setCategoryFormData,
    // Subcategory dialog state
    editingSubcategory,
    parentCategoryForSubcategory,
    isSubcategoryDialogOpen,
    setIsSubcategoryDialogOpen,
    subcategoryFormData,
    setSubcategoryFormData,
    // Category actions
    handleOpenCategoryDialog,
    handleSaveCategory,
    handleDeleteCategory,
    handleToggleCategoryActive,
    // Subcategory actions
    handleOpenSubcategoryDialog,
    handleSaveSubcategory,
    handleDeleteSubcategory,
    handleToggleSubcategoryActive,
    // Expanded state
    expandedCategories,
    toggleCategoryExpanded,
  };
}
