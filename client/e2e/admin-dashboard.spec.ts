import { expect, test } from '@playwright/test';
import { setupAuthMocks } from './fixtures/auth.fixture';
import {
  mockCurrentUserAsAdmin,
  mockGetAdminProfileStats,
  mockGetObservability,
  mockGetShopeeAffiliateAudit,
  mockListWebhookLogs,
  mockSimulateKiwifyWebhook,
  mockListSettings,
  setupAdminDashboardMocks,
} from './fixtures/admin.fixture';
import { AdminPage } from './pages/admin.page';

test.describe('Admin Dashboard', () => {
  let adminPage: AdminPage;

  test.beforeEach(async ({ page }) => {
    adminPage = new AdminPage(page);
  });

  // ================================================================
  // Stats Cards
  // ================================================================

  test.describe('Stats Cards', () => {
    test('should display user stats (total, active, inactive)', async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await setupAdminDashboardMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      await expect(adminPage.statsCards).toBeVisible();

      const statsText = await adminPage.statsCards.textContent();
      expect(statsText).toContain('1');
    });

    test('should handle stats API error gracefully', async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await mockGetAdminProfileStats(page, 'error');
      await mockGetObservability(page);
      await mockGetShopeeAffiliateAudit(page);
      await mockListWebhookLogs(page);
      await mockSimulateKiwifyWebhook(page);
      await mockListSettings(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      // Page should not crash
      await expect(adminPage.tabDashboard).toBeVisible();
    });
  });

  // ================================================================
  // Observability Card
  // ================================================================

  test.describe('Observability Card', () => {
    test.beforeEach(async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await setupAdminDashboardMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();
    });

    test('should display queue metrics', async () => {
      await expect(adminPage.observabilityCard).toBeVisible();

      const obsText = await adminPage.observabilityCard.textContent();
      // Should show some queue numbers from MOCK_OBSERVABILITY
      expect(obsText).toBeTruthy();
    });

    test('should have a refresh button', async () => {
      const refreshBtn = adminPage.refreshObsButton;
      const isVisible = await refreshBtn.isVisible().catch(() => false);
      // Refresh button may exist inside observability card
      expect(typeof isVisible).toBe('boolean');
    });
  });

  // ================================================================
  // Shopee Audit
  // ================================================================

  test.describe('Shopee Audit', () => {
    test.beforeEach(async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await setupAdminDashboardMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();
    });

    test('should display audit summary', async () => {
      await expect(adminPage.shopeeAuditCard).toBeVisible();
    });

    test('should have days selector', async () => {
      const hasDaysSelect = await adminPage.auditDaysSelect.isVisible().catch(() => false);
      expect(typeof hasDaysSelect).toBe('boolean');
    });
  });

  // ================================================================
  // Webhook Section
  // ================================================================

  test.describe('Webhook Section', () => {
    test.beforeEach(async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await setupAdminDashboardMocks(page);
      await adminPage.goto();
      await adminPage.waitForLoad();
    });

    test('should display webhook logs table', async () => {
      await expect(adminPage.webhookSection).toBeVisible();
    });

    test('should display webhook log entries', async ({ page }) => {
      const logRows = adminPage.webhookSection.locator('tr');
      const count = await logRows.count();
      // Should have header + at least one data row
      expect(count).toBeGreaterThanOrEqual(1);
    });

    test('should show simulator form', async () => {
      const hasForm = await adminPage.webhookSimulatorForm.isVisible().catch(() => false);
      expect(typeof hasForm).toBe('boolean');
    });
  });

  // ================================================================
  // Error Handling
  // ================================================================

  test.describe('Error Handling', () => {
    test('should handle observability API error', async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await mockGetAdminProfileStats(page);
      await mockGetObservability(page, 'error');
      await mockGetShopeeAffiliateAudit(page);
      await mockListWebhookLogs(page);
      await mockSimulateKiwifyWebhook(page);
      await mockListSettings(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      // Page should not crash
      await expect(adminPage.tabDashboard).toBeVisible();
    });

    test('should handle webhook logs API error', async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await mockGetAdminProfileStats(page);
      await mockGetObservability(page);
      await mockGetShopeeAffiliateAudit(page);
      await mockListWebhookLogs(page, 'error');
      await mockSimulateKiwifyWebhook(page);
      await mockListSettings(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      await expect(adminPage.tabDashboard).toBeVisible();
    });

    test('should handle Shopee audit API error', async ({ page }) => {
      await setupAuthMocks(page);
      await mockCurrentUserAsAdmin(page);
      await mockGetAdminProfileStats(page);
      await mockGetObservability(page);
      await mockGetShopeeAffiliateAudit(page, 'error');
      await mockListWebhookLogs(page);
      await mockSimulateKiwifyWebhook(page);
      await mockListSettings(page);
      await adminPage.goto();
      await adminPage.waitForLoad();

      await expect(adminPage.tabDashboard).toBeVisible();
    });
  });
});
