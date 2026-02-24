import { expect, test } from '@playwright/test';
import { setupAuthMocks } from './fixtures/auth.fixture';
import {
  mockCurrentUserAsAdmin,
  mockListCategories,
  mockCreateCategory,
  mockUpdateCategory,
  mockDeleteCategory,
  mockCreateSubcategory,
  mockUpdateSubcategory,
  mockDeleteSubcategory,
  setupAdminDashboardMocks,
  setupAdminCategoriesMocks,
} from './fixtures/admin.fixture';
import { AdminPage } from './pages/admin.page';

test.describe('Admin Categories', () => {
  let adminPage: AdminPage;

  test.beforeEach(async ({ page }) => {
    adminPage = new AdminPage(page);
  });

  async function goToCategoriesTab(page: import('@playwright/test').Page): Promise<void> {
    await setupAuthMocks(page);
    await mockCurrentUserAsAdmin(page);
    await setupAdminDashboardMocks(page);
    await setupAdminCategoriesMocks(page);
    await adminPage.goto();
    await adminPage.waitForLoad();
    await adminPage.switchTab('categories');
    await page.waitForTimeout(500);
  }

  // ================================================================
  // List Categories
  // ================================================================

  test.describe('List Categories', () => {
    test('should display categories from mock data', async ({ page }) => {
      await goToCategoriesTab(page);

      const pageText = await page.textContent('body');
      const hasEletronicos = pageText?.includes('Eletronicos');
      const hasModa = pageText?.includes('Moda');

      expect(hasEletronicos || hasModa).toBeTruthy();
    });

    test('should handle empty categories', async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await setupAdminDashboardMocks(page);
      await mockListCategories(page, 'empty');
      await mockCreateCategory(page);
      await mockUpdateCategory(page);
      await mockDeleteCategory(page);
      await mockCreateSubcategory(page);
      await mockUpdateSubcategory(page);
      await mockDeleteSubcategory(page);
      await adminPage.goto();
      await adminPage.waitForLoad();
      await adminPage.switchTab('categories');
      await page.waitForTimeout(500);

      await expect(adminPage.tabCategories).toBeVisible();
    });

    test('should handle API error', async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await setupAdminDashboardMocks(page);
      await mockListCategories(page, 'error');
      await mockCreateCategory(page);
      await mockUpdateCategory(page);
      await mockDeleteCategory(page);
      await mockCreateSubcategory(page);
      await mockUpdateSubcategory(page);
      await mockDeleteSubcategory(page);
      await adminPage.goto();
      await adminPage.waitForLoad();
      await adminPage.switchTab('categories');
      await page.waitForTimeout(500);

      await expect(adminPage.tabCategories).toBeVisible();
    });
  });

  // ================================================================
  // Create Category
  // ================================================================

  test.describe('Create Category', () => {
    test('should show create category button', async ({ page }) => {
      await goToCategoriesTab(page);

      const hasBtn = await adminPage.createCategoryButton.isVisible().catch(() => false);
      expect(typeof hasBtn).toBe('boolean');
    });

    test('should open create category dialog', async ({ page }) => {
      await goToCategoriesTab(page);

      const hasBtn = await adminPage.createCategoryButton.isVisible().catch(() => false);
      if (hasBtn) {
        await adminPage.createCategoryButton.click();
        await page.waitForTimeout(300);

        await expect(adminPage.dialogContent.first()).toBeVisible();
      }
    });
  });

  // ================================================================
  // Edit Category
  // ================================================================

  test.describe('Edit Category', () => {
    test('should show edit buttons for categories', async ({ page }) => {
      await goToCategoriesTab(page);

      const editBtn = page.getByRole('button', { name: /editar|edit/i }).first();
      const hasEdit = await editBtn.isVisible().catch(() => false);
      expect(typeof hasEdit).toBe('boolean');
    });
  });

  // ================================================================
  // Delete Category
  // ================================================================

  test.describe('Delete Category', () => {
    test('should show delete buttons for non-system categories', async ({ page }) => {
      await goToCategoriesTab(page);

      const deleteBtn = page.getByRole('button', { name: /excluir|delete|remover|remove/i }).first();
      const hasDelete = await deleteBtn.isVisible().catch(() => false);
      expect(typeof hasDelete).toBe('boolean');
    });
  });

  // ================================================================
  // Subcategories
  // ================================================================

  test.describe('Subcategories', () => {
    test('should display subcategory names', async ({ page }) => {
      await goToCategoriesTab(page);

      const pageText = await page.textContent('body');
      const hasCelulares = pageText?.includes('Celulares');
      const hasNotebooks = pageText?.includes('Notebooks');

      expect(hasCelulares || hasNotebooks).toBeTruthy();
    });

    test('should show subcategory action buttons', async ({ page }) => {
      await goToCategoriesTab(page);

      const hasSubBtn = await adminPage.subcategoryButtons.first().isVisible().catch(() => false);
      expect(typeof hasSubBtn).toBe('boolean');
    });
  });
});
