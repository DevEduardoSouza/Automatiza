import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules
// ============================================================

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

const mockCreateCategory = vi.fn();
const mockUpdateCategory = vi.fn();
const mockDeleteCategory = vi.fn();
const mockCreateSubcategory = vi.fn();
const mockUpdateSubcategory = vi.fn();
const mockDeleteSubcategory = vi.fn();

const mockCategories = [
  {
    id: 'cat-1',
    slug: 'electronics',
    name: 'Electronics',
    icon: 'cpu',
    displayOrder: 1,
    isActive: true,
    isSystem: false,
    hasRefinements: false,
    subcategories: [
      {
        id: 'sub-1',
        slug: 'phones',
        name: 'Phones',
        displayKey: 'phones',
        groupName: 'devices',
        displayOrder: 0,
        isActive: true,
        isSystem: false,
        categoryId: 'cat-1',
      },
    ],
  },
  {
    id: 'cat-2',
    slug: 'system-cat',
    name: 'System Category',
    icon: 'shield',
    displayOrder: 0,
    isActive: true,
    isSystem: true,
    hasRefinements: true,
    subcategories: [
      {
        id: 'sub-2',
        slug: 'system-sub',
        name: 'System Sub',
        displayKey: 'system_sub',
        groupName: null,
        displayOrder: 0,
        isActive: true,
        isSystem: true,
        categoryId: 'cat-2',
      },
    ],
  },
];

vi.mock('@/hooks/use-categories', () => ({
  useCategories: () => ({
    categories: mockCategories,
    isLoading: false,
    createCategory: mockCreateCategory,
    updateCategory: mockUpdateCategory,
    deleteCategory: mockDeleteCategory,
    createSubcategory: mockCreateSubcategory,
    updateSubcategory: mockUpdateSubcategory,
    deleteSubcategory: mockDeleteSubcategory,
    isCreatingCategory: false,
    isUpdatingCategory: false,
    isDeletingCategory: false,
    isCreatingSubcategory: false,
    isUpdatingSubcategory: false,
    isDeletingSubcategory: false,
  }),
}));

// ============================================================
// Import after mocks
// ============================================================

import { toast } from 'sonner';
import {
  INITIAL_CATEGORY_FORM_DATA,
  INITIAL_SUBCATEGORY_FORM_DATA,
} from '../../types/categories.types';
import { useCategoriesTab } from '../use-categories-tab';

// ============================================================
// Test Helpers
// ============================================================

function renderAndGet() {
  const { result } = renderHook(() => useCategoriesTab());
  return result;
}

// ============================================================
// Tests
// ============================================================

describe('useCategoriesTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCreateCategory.mockResolvedValue(undefined);
    mockUpdateCategory.mockResolvedValue(undefined);
    mockDeleteCategory.mockResolvedValue(undefined);
    mockCreateSubcategory.mockResolvedValue(undefined);
    mockUpdateSubcategory.mockResolvedValue(undefined);
    mockDeleteSubcategory.mockResolvedValue(undefined);
    vi.spyOn(globalThis, 'confirm').mockReturnValue(true);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── Initial state ───────────────────────────────────────────

  describe('initial state', () => {
    it('should have expandedCategories as empty Set', () => {
      const result = renderAndGet();
      expect(result.current.expandedCategories.size).toBe(0);
    });

    it('should have category dialog closed', () => {
      const result = renderAndGet();
      expect(result.current.isCategoryDialogOpen).toBe(false);
      expect(result.current.editingCategory).toBeNull();
    });

    it('should have subcategory dialog closed', () => {
      const result = renderAndGet();
      expect(result.current.isSubcategoryDialogOpen).toBe(false);
      expect(result.current.editingSubcategory).toBeNull();
      expect(result.current.parentCategoryForSubcategory).toBeNull();
    });

    it('should have form data as initial values', () => {
      const result = renderAndGet();
      expect(result.current.categoryFormData).toEqual(INITIAL_CATEGORY_FORM_DATA);
      expect(result.current.subcategoryFormData).toEqual(INITIAL_SUBCATEGORY_FORM_DATA);
    });

    it('should return categories from hook', () => {
      const result = renderAndGet();
      expect(result.current.categories).toHaveLength(2);
      expect(result.current.categories[0].name).toBe('Electronics');
    });
  });

  // ── generateSlug (tested via handleSaveCategory) ────────────

  describe('generateSlug via handleSaveCategory', () => {
    it('should convert "Cafe & Comida" to "cafe-comida"', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenCategoryDialog());
      act(() =>
        result.current.setCategoryFormData({
          ...INITIAL_CATEGORY_FORM_DATA,
          name: 'Café & Comida',
        })
      );

      await act(async () => {
        await result.current.handleSaveCategory();
      });

      expect(mockCreateCategory).toHaveBeenCalledWith(
        expect.objectContaining({ slug: 'cafe-comida' })
      );
    });

    it('should convert "Hello World" to "hello-world"', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenCategoryDialog());
      act(() =>
        result.current.setCategoryFormData({
          ...INITIAL_CATEGORY_FORM_DATA,
          name: 'Hello World',
        })
      );

      await act(async () => {
        await result.current.handleSaveCategory();
      });

      expect(mockCreateCategory).toHaveBeenCalledWith(
        expect.objectContaining({ slug: 'hello-world' })
      );
    });
  });

  // ── handleOpenCategoryDialog ────────────────────────────────

  describe('handleOpenCategoryDialog', () => {
    it('should populate form when category provided', () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenCategoryDialog(mockCategories[0]));

      expect(result.current.editingCategory).toEqual(mockCategories[0]);
      expect(result.current.categoryFormData.name).toBe('Electronics');
      expect(result.current.categoryFormData.slug).toBe('electronics');
      expect(result.current.categoryFormData.icon).toBe('cpu');
      expect(result.current.isCategoryDialogOpen).toBe(true);
    });

    it('should reset form when no category provided', () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenCategoryDialog());

      expect(result.current.editingCategory).toBeNull();
      expect(result.current.categoryFormData).toEqual(INITIAL_CATEGORY_FORM_DATA);
      expect(result.current.isCategoryDialogOpen).toBe(true);
    });
  });

  // ── handleSaveCategory ──────────────────────────────────────

  describe('handleSaveCategory', () => {
    it('should toast error when name is empty', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenCategoryDialog());
      act(() =>
        result.current.setCategoryFormData({
          ...INITIAL_CATEGORY_FORM_DATA,
          name: '  ',
        })
      );

      await act(async () => {
        await result.current.handleSaveCategory();
      });

      expect(toast.error).toHaveBeenCalledWith('admin.categories.errors.nameRequired');
      expect(mockCreateCategory).not.toHaveBeenCalled();
    });

    it('should call createCategory with generated slug for new category', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenCategoryDialog());
      act(() =>
        result.current.setCategoryFormData({
          ...INITIAL_CATEGORY_FORM_DATA,
          name: 'New Category',
          icon: 'star',
          displayOrder: '5',
          hasRefinements: true,
        })
      );

      await act(async () => {
        await result.current.handleSaveCategory();
      });

      expect(mockCreateCategory).toHaveBeenCalledWith({
        slug: 'new-category',
        name: 'New Category',
        icon: 'star',
        displayOrder: 5,
        hasRefinements: true,
      });
    });

    it('should call updateCategory with existing slug when editing', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenCategoryDialog(mockCategories[0]));

      await act(async () => {
        await result.current.handleSaveCategory();
      });

      expect(mockUpdateCategory).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'cat-1',
          name: 'Electronics',
        })
      );
      expect(mockCreateCategory).not.toHaveBeenCalled();
    });
  });

  // ── handleDeleteCategory ────────────────────────────────────

  describe('handleDeleteCategory', () => {
    it('should toast error for system category', async () => {
      const result = renderAndGet();

      await act(async () => {
        await result.current.handleDeleteCategory('cat-2', true);
      });

      expect(toast.error).toHaveBeenCalledWith('admin.categories.errors.cannotDeleteSystem');
      expect(mockDeleteCategory).not.toHaveBeenCalled();
    });

    it('should call deleteCategory when confirmed', async () => {
      const result = renderAndGet();

      await act(async () => {
        await result.current.handleDeleteCategory('cat-1', false);
      });

      expect(mockDeleteCategory).toHaveBeenCalledWith('cat-1');
    });

    it('should not delete when user cancels confirm', async () => {
      vi.spyOn(globalThis, 'confirm').mockReturnValue(false);

      const result = renderAndGet();

      await act(async () => {
        await result.current.handleDeleteCategory('cat-1', false);
      });

      expect(mockDeleteCategory).not.toHaveBeenCalled();
    });
  });

  // ── handleToggleCategoryActive ──────────────────────────────

  describe('handleToggleCategoryActive', () => {
    it('should invert isActive', async () => {
      const result = renderAndGet();

      await act(async () => {
        await result.current.handleToggleCategoryActive(mockCategories[0]);
      });

      expect(mockUpdateCategory).toHaveBeenCalledWith({
        id: 'cat-1',
        isActive: false,
      });
    });
  });

  // ── toggleCategoryExpanded ──────────────────────────────────

  describe('toggleCategoryExpanded', () => {
    it('should add category to expanded set', () => {
      const result = renderAndGet();

      act(() => result.current.toggleCategoryExpanded('cat-1'));

      expect(result.current.expandedCategories.has('cat-1')).toBe(true);
    });

    it('should remove category from expanded set on second toggle', () => {
      const result = renderAndGet();

      act(() => result.current.toggleCategoryExpanded('cat-1'));
      act(() => result.current.toggleCategoryExpanded('cat-1'));

      expect(result.current.expandedCategories.has('cat-1')).toBe(false);
    });
  });

  // ── Subcategory actions ─────────────────────────────────────

  describe('subcategory actions', () => {
    it('should open subcategory dialog with parent category', () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenSubcategoryDialog(mockCategories[0]));

      expect(result.current.parentCategoryForSubcategory).toEqual(mockCategories[0]);
      expect(result.current.editingSubcategory).toBeNull();
      expect(result.current.subcategoryFormData).toEqual(INITIAL_SUBCATEGORY_FORM_DATA);
      expect(result.current.isSubcategoryDialogOpen).toBe(true);
    });

    it('should populate form when editing existing subcategory', () => {
      const result = renderAndGet();
      const sub = mockCategories[0].subcategories[0];

      act(() => result.current.handleOpenSubcategoryDialog(mockCategories[0], sub));

      expect(result.current.editingSubcategory).toEqual(sub);
      expect(result.current.subcategoryFormData.name).toBe('Phones');
      expect(result.current.subcategoryFormData.displayKey).toBe('phones');
      expect(result.current.subcategoryFormData.groupName).toBe('devices');
    });

    it('should toast error when subcategory name is empty', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenSubcategoryDialog(mockCategories[0]));
      act(() =>
        result.current.setSubcategoryFormData({
          ...INITIAL_SUBCATEGORY_FORM_DATA,
          name: '',
          displayKey: 'test',
        })
      );

      await act(async () => {
        await result.current.handleSaveSubcategory();
      });

      expect(toast.error).toHaveBeenCalledWith('admin.categories.errors.nameRequired');
      expect(mockCreateSubcategory).not.toHaveBeenCalled();
    });

    it('should toast error when displayKey is empty', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenSubcategoryDialog(mockCategories[0]));
      act(() =>
        result.current.setSubcategoryFormData({
          ...INITIAL_SUBCATEGORY_FORM_DATA,
          name: 'Test',
          displayKey: '',
        })
      );

      await act(async () => {
        await result.current.handleSaveSubcategory();
      });

      expect(toast.error).toHaveBeenCalledWith('admin.categories.errors.displayKeyRequired');
      expect(mockCreateSubcategory).not.toHaveBeenCalled();
    });

    it('should create subcategory with generated slug and parent ID', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenSubcategoryDialog(mockCategories[0]));
      act(() =>
        result.current.setSubcategoryFormData({
          ...INITIAL_SUBCATEGORY_FORM_DATA,
          name: 'Tablets',
          displayKey: 'tablets',
          groupName: 'devices',
          displayOrder: '2',
          slug: '',
        })
      );

      await act(async () => {
        await result.current.handleSaveSubcategory();
      });

      expect(mockCreateSubcategory).toHaveBeenCalledWith({
        categoryId: 'cat-1',
        slug: 'tablets',
        name: 'Tablets',
        displayKey: 'tablets',
        groupName: 'devices',
        displayOrder: 2,
      });
    });

    it('should update subcategory when editing', async () => {
      const result = renderAndGet();
      const sub = mockCategories[0].subcategories[0];

      act(() => result.current.handleOpenSubcategoryDialog(mockCategories[0], sub));

      await act(async () => {
        await result.current.handleSaveSubcategory();
      });

      expect(mockUpdateSubcategory).toHaveBeenCalledWith(
        expect.objectContaining({ id: 'sub-1', name: 'Phones' })
      );
      expect(mockCreateSubcategory).not.toHaveBeenCalled();
    });

    it('should toast error when deleting system subcategory', async () => {
      const result = renderAndGet();

      await act(async () => {
        await result.current.handleDeleteSubcategory('sub-2', true);
      });

      expect(toast.error).toHaveBeenCalledWith(
        'admin.categories.errors.cannotDeleteSystemSubcategory'
      );
      expect(mockDeleteSubcategory).not.toHaveBeenCalled();
    });

    it('should delete subcategory when confirmed', async () => {
      const result = renderAndGet();

      await act(async () => {
        await result.current.handleDeleteSubcategory('sub-1', false);
      });

      expect(mockDeleteSubcategory).toHaveBeenCalledWith('sub-1');
    });

    it('should toggle subcategory active', async () => {
      const result = renderAndGet();
      const sub = mockCategories[0].subcategories[0];

      await act(async () => {
        await result.current.handleToggleSubcategoryActive(sub);
      });

      expect(mockUpdateSubcategory).toHaveBeenCalledWith({
        id: 'sub-1',
        isActive: false,
      });
    });
  });
});
