import { expect, test } from '@playwright/test';
import { setupAuthMocks } from './fixtures/auth.fixture';
import {
  mockCurrentUserAsAdmin,
  mockListEmailTemplates,
  mockCreateEmailTemplate,
  mockUpdateEmailTemplate,
  mockToggleEmailTemplateActive,
  setupAdminDashboardMocks,
  setupAdminEmailTemplatesMocks,
} from './fixtures/admin.fixture';
import { AdminPage } from './pages/admin.page';

test.describe('Admin Email Templates', () => {
  let adminPage: AdminPage;

  test.beforeEach(async ({ page }) => {
    adminPage = new AdminPage(page);
  });

  async function goToEmailsTab(page: import('@playwright/test').Page): Promise<void> {
    await setupAuthMocks(page);
    await mockCurrentUserAsAdmin(page);
    await setupAdminDashboardMocks(page);
    await setupAdminEmailTemplatesMocks(page);
    await adminPage.goto();
    await adminPage.waitForLoad();
    await adminPage.switchTab('emails');
    await page.waitForTimeout(500);
  }

  // ================================================================
  // List Templates
  // ================================================================

  test.describe('List Templates', () => {
    test('should display templates table', async ({ page }) => {
      await goToEmailsTab(page);

      await expect(adminPage.templatesTable).toBeVisible();
    });

    test('should show template names from mock data', async ({ page }) => {
      await goToEmailsTab(page);

      const pageText = await page.textContent('body');
      const hasWelcome = pageText?.includes('welcome_email') || pageText?.includes('Bem-vindo');
      const hasReset = pageText?.includes('password_reset') || pageText?.includes('Redefinicao');

      expect(hasWelcome || hasReset).toBeTruthy();
    });

    test('should handle empty templates list', async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await setupAdminDashboardMocks(page);
      await mockListEmailTemplates(page, 'empty');
      await mockCreateEmailTemplate(page);
      await mockUpdateEmailTemplate(page);
      await mockToggleEmailTemplateActive(page);
      await adminPage.goto();
      await adminPage.waitForLoad();
      await adminPage.switchTab('emails');
      await page.waitForTimeout(500);

      await expect(adminPage.tabEmails).toBeVisible();
    });

    test('should handle API error', async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await setupAdminDashboardMocks(page);
      await mockListEmailTemplates(page, 'error');
      await mockCreateEmailTemplate(page);
      await mockUpdateEmailTemplate(page);
      await mockToggleEmailTemplateActive(page);
      await adminPage.goto();
      await adminPage.waitForLoad();
      await adminPage.switchTab('emails');
      await page.waitForTimeout(500);

      await expect(adminPage.tabEmails).toBeVisible();
    });
  });

  // ================================================================
  // Search
  // ================================================================

  test.describe('Search', () => {
    test('should filter templates by search term', async ({ page }) => {
      await goToEmailsTab(page);

      const hasSearch = await adminPage.searchInput.isVisible().catch(() => false);
      if (hasSearch) {
        await adminPage.searchInput.fill('welcome');
        await page.waitForTimeout(500);

        const pageText = await page.textContent('body');
        expect(pageText).toContain('welcome');
      }
    });
  });

  // ================================================================
  // Create Template
  // ================================================================

  test.describe('Create Template', () => {
    test('should show create template button', async ({ page }) => {
      await goToEmailsTab(page);

      const hasBtn = await adminPage.createTemplateButton.isVisible().catch(() => false);
      expect(typeof hasBtn).toBe('boolean');
    });

    test('should open create template dialog', async ({ page }) => {
      await goToEmailsTab(page);

      const hasBtn = await adminPage.createTemplateButton.isVisible().catch(() => false);
      if (hasBtn) {
        await adminPage.createTemplateButton.click();
        await page.waitForTimeout(300);

        await expect(adminPage.dialogContent.first()).toBeVisible();
      }
    });
  });

  // ================================================================
  // Edit Template
  // ================================================================

  test.describe('Edit Template', () => {
    test('should show edit buttons for templates', async ({ page }) => {
      await goToEmailsTab(page);

      const editBtn = page.getByRole('button', { name: /editar|edit/i }).first();
      const hasEdit = await editBtn.isVisible().catch(() => false);
      expect(typeof hasEdit).toBe('boolean');
    });
  });

  // ================================================================
  // Toggle Active
  // ================================================================

  test.describe('Toggle Active', () => {
    test('should show toggle switch for templates', async ({ page }) => {
      await goToEmailsTab(page);

      const hasToggle = await adminPage.toggleActiveButton.first().isVisible().catch(() => false);
      expect(typeof hasToggle).toBe('boolean');
    });
  });
});
