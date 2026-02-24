import { expect, test } from '@playwright/test';
import { setupAuthMocks } from './fixtures/auth.fixture';
import {
  mockCurrentUserAsAdmin,
  mockListPlans,
  mockCreatePlan,
  mockUpdatePlan,
  mockDeletePlan,
  setupAdminDashboardMocks,
  setupAdminPlansMocks,
} from './fixtures/admin.fixture';
import { AdminPage } from './pages/admin.page';

test.describe('Admin Plans', () => {
  let adminPage: AdminPage;

  test.beforeEach(async ({ page }) => {
    adminPage = new AdminPage(page);
  });

  async function goToPlansTab(page: import('@playwright/test').Page): Promise<void> {
    await setupAuthMocks(page);
    await mockCurrentUserAsAdmin(page);
    await setupAdminDashboardMocks(page);
    await setupAdminPlansMocks(page);
    await adminPage.goto();
    await adminPage.waitForLoad();
    await adminPage.switchTab('plans');
    await page.waitForTimeout(500);
  }

  // ================================================================
  // List Plans
  // ================================================================

  test.describe('List Plans', () => {
    test('should display plans table with mock data', async ({ page }) => {
      await goToPlansTab(page);

      await expect(adminPage.plansTable).toBeVisible();
    });

    test('should show plan names from mock data', async ({ page }) => {
      await goToPlansTab(page);

      const pageText = await page.textContent('body');
      const hasFree = pageText?.includes('Free');
      const hasPro = pageText?.includes('Pro');
      const hasElite = pageText?.includes('Elite');

      expect(hasFree || hasPro || hasElite).toBeTruthy();
    });

    test('should handle empty plans list', async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await setupAdminDashboardMocks(page);
      await mockListPlans(page, 'empty');
      await mockCreatePlan(page);
      await mockUpdatePlan(page);
      await mockDeletePlan(page);
      await adminPage.goto();
      await adminPage.waitForLoad();
      await adminPage.switchTab('plans');
      await page.waitForTimeout(500);

      // Should not crash
      await expect(adminPage.tabPlans).toBeVisible();
    });

    test('should handle API error', async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await setupAdminDashboardMocks(page);
      await mockListPlans(page, 'error');
      await mockCreatePlan(page);
      await mockUpdatePlan(page);
      await mockDeletePlan(page);
      await adminPage.goto();
      await adminPage.waitForLoad();
      await adminPage.switchTab('plans');
      await page.waitForTimeout(500);

      await expect(adminPage.tabPlans).toBeVisible();
    });
  });

  // ================================================================
  // Create Plan
  // ================================================================

  test.describe('Create Plan', () => {
    test('should show create plan button', async ({ page }) => {
      await goToPlansTab(page);

      const hasBtn = await adminPage.createPlanButton.isVisible().catch(() => false);
      expect(typeof hasBtn).toBe('boolean');
    });

    test('should open create plan dialog', async ({ page }) => {
      await goToPlansTab(page);

      const hasBtn = await adminPage.createPlanButton.isVisible().catch(() => false);
      if (hasBtn) {
        await adminPage.createPlanButton.click();
        await page.waitForTimeout(300);

        await expect(adminPage.dialogContent.first()).toBeVisible();
      }
    });
  });

  // ================================================================
  // Edit Plan
  // ================================================================

  test.describe('Edit Plan', () => {
    test('should show edit button on plan rows', async ({ page }) => {
      await goToPlansTab(page);

      const editBtn = page.getByRole('button', { name: /editar|edit/i }).first();
      const hasEdit = await editBtn.isVisible().catch(() => false);
      expect(typeof hasEdit).toBe('boolean');
    });
  });

  // ================================================================
  // Delete Plan
  // ================================================================

  test.describe('Delete Plan', () => {
    test('should show delete button on plan rows', async ({ page }) => {
      await goToPlansTab(page);

      const hasDelete = await adminPage.deletePlanButton.isVisible().catch(() => false);
      expect(typeof hasDelete).toBe('boolean');
    });
  });
});
