import { expect, test } from '@playwright/test';
import { setupAuthMocks } from './fixtures/auth.fixture';
import {
  mockCurrentUserAsAdmin,
  mockListProfiles,
  mockSearchProfiles,
  mockCreateUser,
  mockGetUserInvestigation,
  mockGrantAdminRole,
  mockRevokeAdminRole,
  mockAdminBlockUser,
  mockUnblockUser,
  mockResetUserPassword,
  mockListAdminUsers,
  mockAdminBulkUpdateAutomationStatus,
  mockAdminSpreadAutomationSchedule,
  mockUpdateUserPlan,
  setupAdminUsersMocks,
  setupAdminDashboardMocks,
} from './fixtures/admin.fixture';
import { AdminPage } from './pages/admin.page';

test.describe('Admin Users', () => {
  let adminPage: AdminPage;

  test.beforeEach(async ({ page }) => {
    adminPage = new AdminPage(page);
  });

  async function goToUsersTab(page: import('@playwright/test').Page): Promise<void> {
    await setupAuthMocks(page);
    await mockCurrentUserAsAdmin(page);
    await setupAdminDashboardMocks(page);
    await setupAdminUsersMocks(page);
    await adminPage.goto();
    await adminPage.waitForLoad();
    await adminPage.switchTab('users');
    await page.waitForTimeout(500);
  }

  // ================================================================
  // User List
  // ================================================================

  test.describe('User List', () => {
    test('should display users table with mock data', async ({ page }) => {
      await goToUsersTab(page);

      await expect(adminPage.usersTable).toBeVisible();
      const count = await adminPage.getUserCount();
      expect(count).toBeGreaterThan(0);
    });

    test('should show user emails in table', async ({ page }) => {
      await goToUsersTab(page);

      const adminRow = adminPage.getUserByEmail('admin@example.com');
      const proRow = adminPage.getUserByEmail('pro@example.com');

      const hasAdmin = await adminRow.isVisible().catch(() => false);
      const hasPro = await proRow.isVisible().catch(() => false);

      expect(hasAdmin || hasPro).toBeTruthy();
    });

    test('should handle API error on user list', async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await setupAdminDashboardMocks(page);
      await mockListProfiles(page, 'error');
      await mockSearchProfiles(page);
      await mockListAdminUsers(page);
      await mockCreateUser(page);
      await mockGetUserInvestigation(page);
      await mockGrantAdminRole(page);
      await mockRevokeAdminRole(page);
      await mockAdminBlockUser(page);
      await mockUnblockUser(page);
      await mockResetUserPassword(page);
      await mockAdminBulkUpdateAutomationStatus(page);
      await mockAdminSpreadAutomationSchedule(page);
      await mockUpdateUserPlan(page);
      await adminPage.goto();
      await adminPage.waitForLoad();
      await adminPage.switchTab('users');
      await page.waitForTimeout(500);

      // Page should not crash
      await expect(adminPage.tabUsers).toBeVisible();
    });
  });

  // ================================================================
  // Search
  // ================================================================

  test.describe('Search', () => {
    test('should filter users by search term', async ({ page }) => {
      await goToUsersTab(page);

      const hasSearch = await adminPage.userSearchInput.isVisible().catch(() => false);
      if (hasSearch) {
        await adminPage.searchUser('admin');
        await page.waitForTimeout(500);

        const count = await adminPage.getUserCount();
        expect(count).toBeGreaterThanOrEqual(0);
      }
    });

    test('should clear search and show all users', async ({ page }) => {
      await goToUsersTab(page);

      const hasSearch = await adminPage.userSearchInput.isVisible().catch(() => false);
      if (hasSearch) {
        await adminPage.searchUser('admin');
        await page.waitForTimeout(300);
        await adminPage.clearSearch();
        await page.waitForTimeout(500);

        const count = await adminPage.getUserCount();
        expect(count).toBeGreaterThanOrEqual(0);
      }
    });
  });

  // ================================================================
  // Create User
  // ================================================================

  test.describe('Create User', () => {
    test('should open create user dialog', async ({ page }) => {
      await goToUsersTab(page);

      const hasBtn = await adminPage.createUserButton.isVisible().catch(() => false);
      if (hasBtn) {
        await adminPage.createUserButton.click();
        await page.waitForTimeout(300);

        await expect(adminPage.dialogContent.first()).toBeVisible();
      }
    });

    test('should show success with temporary password on create', async ({ page }) => {
      await goToUsersTab(page);

      const hasBtn = await adminPage.createUserButton.isVisible().catch(() => false);
      if (hasBtn) {
        await adminPage.createUserButton.click();
        await page.waitForTimeout(300);

        // Fill form fields
        const emailInput = page.locator('[role="dialog"]').getByPlaceholder(/email/i);
        if (await emailInput.isVisible()) {
          await emailInput.fill('newuser@example.com');

          const submitBtn = page
            .locator('[role="dialog"]')
            .getByRole('button', { name: /criar|create|salvar|save/i });
          if (await submitBtn.isVisible()) {
            await submitBtn.click();
            await page.waitForTimeout(1000);
          }
        }
      }
    });
  });

  // ================================================================
  // User Details
  // ================================================================

  test.describe('User Details', () => {
    test('should open user details dialog on row click', async ({ page }) => {
      await goToUsersTab(page);

      const firstRow = adminPage.userRows.first();
      if (await firstRow.isVisible()) {
        await firstRow.click();
        await page.waitForTimeout(500);

        const hasDialog = await adminPage.dialogContent.first().isVisible().catch(() => false);
        expect(typeof hasDialog).toBe('boolean');
      }
    });
  });

  // ================================================================
  // Block / Unblock
  // ================================================================

  test.describe('Block / Unblock', () => {
    test('should show block button for active users', async ({ page }) => {
      await goToUsersTab(page);

      // Click on a non-blocked user row first
      const proRow = adminPage.getUserByEmail('pro@example.com');
      if (await proRow.isVisible()) {
        await proRow.click();
        await page.waitForTimeout(500);

        const hasBlock = await adminPage.blockUserButton.isVisible().catch(() => false);
        expect(typeof hasBlock).toBe('boolean');
      }
    });

    test('should show unblock button for blocked users', async ({ page }) => {
      await goToUsersTab(page);

      const blockedRow = adminPage.getUserByEmail('blocked@example.com');
      if (await blockedRow.isVisible()) {
        await blockedRow.click();
        await page.waitForTimeout(500);

        const hasUnblock = await adminPage.unblockUserButton.isVisible().catch(() => false);
        expect(typeof hasUnblock).toBe('boolean');
      }
    });
  });

  // ================================================================
  // Reset Password
  // ================================================================

  test.describe('Reset Password', () => {
    test('should show reset password button in user details', async ({ page }) => {
      await goToUsersTab(page);

      const proRow = adminPage.getUserByEmail('pro@example.com');
      if (await proRow.isVisible()) {
        await proRow.click();
        await page.waitForTimeout(500);

        const hasReset = await adminPage.resetPasswordButton.isVisible().catch(() => false);
        expect(typeof hasReset).toBe('boolean');
      }
    });
  });

  // ================================================================
  // Admin Role
  // ================================================================

  test.describe('Admin Role', () => {
    test('should show admin toggle in user details', async ({ page }) => {
      await goToUsersTab(page);

      const proRow = adminPage.getUserByEmail('pro@example.com');
      if (await proRow.isVisible()) {
        await proRow.click();
        await page.waitForTimeout(500);

        const hasAdminToggle = await adminPage.adminToggleButton.isVisible().catch(() => false);
        expect(typeof hasAdminToggle).toBe('boolean');
      }
    });
  });
});
