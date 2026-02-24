import { expect, test } from '@playwright/test';
import { setupAuthMocks } from './fixtures/auth.fixture';
import {
  mockAdminAccessDenied,
  setupFullAdminMocks,
} from './fixtures/admin.fixture';
import { AdminPage } from './pages/admin.page';

test.describe('Admin Panel - Access & Navigation', () => {
  let adminPage: AdminPage;

  test.beforeEach(async ({ page }) => {
    adminPage = new AdminPage(page);
  });

  // ================================================================
  // Access Control
  // ================================================================

  test.describe('Access Control', () => {
    test('should display admin panel for admin users', async ({ page }) => {
      await setupAuthMocks(page);
      await setupFullAdminMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      const isDenied = await adminPage.isAccessDenied();
      expect(isDenied).toBeFalsy();

      await expect(adminPage.tabDashboard).toBeVisible();
    });

    test('should deny access for non-admin users', async ({ page }) => {
      await setupAuthMocks(page);
      await mockAdminAccessDenied(page);
      await adminPage.goto();
      await page.waitForTimeout(2000);

      const isDenied = await adminPage.isAccessDenied();
      const isRedirected =
        page.url().includes('/dashboard') || page.url().includes('/auth');

      expect(isDenied || isRedirected).toBeTruthy();
    });
  });

  // ================================================================
  // Tab Navigation
  // ================================================================

  test.describe('Tab Navigation', () => {
    test.beforeEach(async ({ page }) => {
      await setupAuthMocks(page);
      await setupFullAdminMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();
    });

    test('should display all 7 tabs', async () => {
      await expect(adminPage.tabDashboard).toBeVisible();
      await expect(adminPage.tabUsers).toBeVisible();
      await expect(adminPage.tabPlans).toBeVisible();
      await expect(adminPage.tabCategories).toBeVisible();
      await expect(adminPage.tabTeam).toBeVisible();
      await expect(adminPage.tabEmails).toBeVisible();
      await expect(adminPage.tabSettings).toBeVisible();
    });

    test('should default to Dashboard tab', async () => {
      await expect(adminPage.tabDashboard).toHaveAttribute('data-state', 'active');
    });

    test('should switch to Users tab on click', async ({ page }) => {
      await adminPage.switchTab('users');
      await page.waitForTimeout(300);

      await expect(adminPage.tabUsers).toHaveAttribute('data-state', 'active');
    });

    test('should switch to Plans tab on click', async ({ page }) => {
      await adminPage.switchTab('plans');
      await page.waitForTimeout(300);

      await expect(adminPage.tabPlans).toHaveAttribute('data-state', 'active');
    });

    test('should switch to Categories tab on click', async ({ page }) => {
      await adminPage.switchTab('categories');
      await page.waitForTimeout(300);

      await expect(adminPage.tabCategories).toHaveAttribute('data-state', 'active');
    });

    test('should switch to Team tab on click', async ({ page }) => {
      await adminPage.switchTab('team');
      await page.waitForTimeout(300);

      await expect(adminPage.tabTeam).toHaveAttribute('data-state', 'active');
    });

    test('should switch to Emails tab on click', async ({ page }) => {
      await adminPage.switchTab('emails');
      await page.waitForTimeout(300);

      await expect(adminPage.tabEmails).toHaveAttribute('data-state', 'active');
    });

    test('should switch to Settings tab on click', async ({ page }) => {
      await adminPage.switchTab('settings');
      await page.waitForTimeout(300);

      await expect(adminPage.tabSettings).toHaveAttribute('data-state', 'active');
    });
  });

  // ================================================================
  // Migration Validation
  // ================================================================

  test.describe('Migration Validation', () => {
    test('should load admin data via ConnectRPC', async ({ page }) => {
      const grpcCalls: string[] = [];

      await page.route('**/*', async (route) => {
        const url = route.request().url();
        if (url.includes('admin.v1.AdminService') || url.includes('profiles.v1.ProfilesService')) {
          grpcCalls.push(url);
        }
        await route.continue();
      });

      await setupAuthMocks(page);
      await setupFullAdminMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      expect(grpcCalls.length).toBeGreaterThan(0);
    });
  });
});
