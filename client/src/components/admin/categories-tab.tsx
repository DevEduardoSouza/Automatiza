import { Button } from '@/components/ui/button';
import { Plus } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { CategoryCard, CategoryDialog, SubcategoryDialog } from './categories';
import { useCategoriesTab } from './hooks';

const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
};

/**
 * Tab for managing product categories and subcategories.
 * Allows creating, editing, and deleting categories and their subcategories.
 * System categories (marked with lock icon) cannot be deleted but can be edited.
 */
export default function CategoriesTab() {
  const { t } = useTranslation();
  const {
    categories,
    loading,
    saving,
    editingCategory,
    isCategoryDialogOpen,
    setIsCategoryDialogOpen,
    categoryFormData,
    setCategoryFormData,
    editingSubcategory,
    parentCategoryForSubcategory,
    isSubcategoryDialogOpen,
    setIsSubcategoryDialogOpen,
    subcategoryFormData,
    setSubcategoryFormData,
    handleOpenCategoryDialog,
    handleSaveCategory,
    handleDeleteCategory,
    handleToggleCategoryActive,
    handleOpenSubcategoryDialog,
    handleSaveSubcategory,
    handleDeleteSubcategory,
    handleToggleSubcategoryActive,
    expandedCategories,
    toggleCategoryExpanded,
  } = useCategoriesTab();

  return (
    <div className="space-y-4">
      <div
        className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 mb-3 xs:mb-4">
          <div>
            <span
              className="text-xs xs:text-sm font-semibold uppercase tracking-[0.1em]"
              style={{ color: colors.wineText }}
            >
              {t('admin.categories.title')}
            </span>
            <p className="text-xs md:text-sm mt-0.5" style={{ color: colors.wineTextMuted }}>
              {t('admin.categories.description')}
            </p>
          </div>
          <Button onClick={() => handleOpenCategoryDialog()} className="w-full md:w-auto">
            <Plus className="h-4 w-4 mr-2" />
            {t('admin.categories.newCategory')}
          </Button>
        </div>
        <div className="space-y-3 xs:space-y-4">
          {loading ? (
            <div className="text-center py-8 text-sm" role="status" aria-live="polite">
              {t('common.loading')}
            </div>
          ) : categories.length === 0 ? (
            <div className="text-center py-8 text-sm" style={{ color: colors.wineTextMuted }}>
              {t('admin.categories.noCategories')}
            </div>
          ) : (
            <div className="space-y-3">
              {categories
                .sort((a, b) => a.displayOrder - b.displayOrder)
                .map((category) => (
                  <CategoryCard
                    key={category.id}
                    category={category}
                    isExpanded={expandedCategories.has(category.id)}
                    onToggleExpanded={() => toggleCategoryExpanded(category.id)}
                    onEditCategory={handleOpenCategoryDialog}
                    onDeleteCategory={handleDeleteCategory}
                    onToggleActive={handleToggleCategoryActive}
                    onAddSubcategory={(cat) => handleOpenSubcategoryDialog(cat)}
                    onEditSubcategory={handleOpenSubcategoryDialog}
                    onDeleteSubcategory={handleDeleteSubcategory}
                    onToggleSubcategoryActive={handleToggleSubcategoryActive}
                    t={t}
                  />
                ))}
            </div>
          )}
        </div>
      </div>

      <CategoryDialog
        open={isCategoryDialogOpen}
        onOpenChange={setIsCategoryDialogOpen}
        editingCategory={editingCategory}
        formData={categoryFormData}
        setFormData={setCategoryFormData}
        onSave={handleSaveCategory}
        saving={saving}
        t={t}
      />

      <SubcategoryDialog
        open={isSubcategoryDialogOpen}
        onOpenChange={setIsSubcategoryDialogOpen}
        parentCategory={parentCategoryForSubcategory}
        editingSubcategory={editingSubcategory}
        formData={subcategoryFormData}
        setFormData={setSubcategoryFormData}
        onSave={handleSaveSubcategory}
        saving={saving}
        t={t}
      />
    </div>
  );
}
