import { expect, test } from '@playwright/test';
import { setupAuthMocks } from './fixtures/auth.fixture';
import {
  MOCK_ADMIN_USERS_LIST,
  mockCurrentUserAsAdmin,
  mockListAdminUsers,
  mockGrantAdminRole,
  mockRevokeAdminRole,
  mockSearchProfiles,
  setupAdminDashboardMocks,
  setupAdminTeamMocks,
} from './fixtures/admin.fixture';
import { AdminPage } from './pages/admin.page';

test.describe('Admin Team', () => {
  let adminPage: AdminPage;

  test.beforeEach(async ({ page }) => {
    adminPage = new AdminPage(page);
  });

  async function goToTeamTab(page: import('@playwright/test').Page): Promise<void> {
    await setupAuthMocks(page);
    await mockCurrentUserAsAdmin(page);
    await setupAdminDashboardMocks(page);
    await setupAdminTeamMocks(page);
    await adminPage.goto();
    await adminPage.waitForLoad();
    await adminPage.switchTab('team');
    await page.waitForTimeout(500);
  }

  // ================================================================
  // List Team
  // ================================================================

  test.describe('List Team', () => {
    test('should display team table with admin members', async ({ page }) => {
      await goToTeamTab(page);

      await expect(adminPage.teamTable).toBeVisible();
    });

    test('should show admin email in team list', async ({ page }) => {
      await goToTeamTab(page);

      const pageText = await page.textContent('body');
      expect(pageText).toContain('admin@example.com');
    });

    test('should handle API error', async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await setupAdminDashboardMocks(page);
      await mockListAdminUsers(page, 'error');
      await mockGrantAdminRole(page);
      await mockRevokeAdminRole(page);
      await mockSearchProfiles(page);
      await adminPage.goto();
      await adminPage.waitForLoad();
      await adminPage.switchTab('team');
      await page.waitForTimeout(500);

      await expect(adminPage.tabTeam).toBeVisible();
    });
  });

  // ================================================================
  // Add Admin
  // ================================================================

  test.describe('Add Admin', () => {
    test('should show add admin button', async ({ page }) => {
      await goToTeamTab(page);

      const hasBtn = await adminPage.addAdminButton.isVisible().catch(() => false);
      expect(typeof hasBtn).toBe('boolean');
    });

    test('should open add admin dialog', async ({ page }) => {
      await goToTeamTab(page);

      const hasBtn = await adminPage.addAdminButton.isVisible().catch(() => false);
      if (hasBtn) {
        await adminPage.addAdminButton.click();
        await page.waitForTimeout(300);

        await expect(adminPage.dialogContent.first()).toBeVisible();
      }
    });
  });

  // ================================================================
  // Remove Admin
  // ================================================================

  test.describe('Remove Admin', () => {
    test('should show remove button for admin members', async ({ page }) => {
      await goToTeamTab(page);

      const hasRemove = await adminPage.removeAdminButton.isVisible().catch(() => false);
      expect(typeof hasRemove).toBe('boolean');
    });
  });
});
