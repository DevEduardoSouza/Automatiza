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
        className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8"
        style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
      >
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 mb-3 xs:mb-4 sm:mb-5">
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
          <button
            type="button"
            onClick={() => handleOpenCategoryDialog()}
            className="auth-btn-glow relative inline-flex w-full md:w-auto cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold text-white transition-all"
          >
            <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
              <div className="auth-btn-glow__glow absolute inset-0" />
            </div>
            <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
              <Plus className="h-4 w-4" />
              {t('admin.categories.newCategory')}
            </span>
          </button>
        </div>
        <div className="space-y-3 xs:space-y-4">
          {loading ? (
            <div className="flex items-center justify-center py-6 xs:py-8" role="status" aria-live="polite">
              <div className="text-center space-y-4">
                <div
                  className="h-8 w-8 xs:h-10 xs:w-10 animate-spin rounded-full border-[3px] xs:border-4 border-t-transparent mx-auto"
                  style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
                />
                <p className="text-xs xs:text-sm" style={{ color: colors.wineTextMuted }}>
                  {t('common.loading')}
                </p>
              </div>
            </div>
          ) : categories.length === 0 ? (
            <div className="text-center py-8 text-sm" style={{ color: colors.wineTextMuted }}>
              {t('admin.categories.noCategories')}
            </div>
          ) : (
            <div className="space-y-2 xs:space-y-3">
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
