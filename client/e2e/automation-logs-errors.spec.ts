import { expect, test } from '@playwright/test';
import {
  MOCK_DISPATCH_LOGS,
  mockAppShellApis,
  mockAuthApi,
  mockDispatchLogsApi,
  mockDispatchLogsApiSlow,
  mockListJobsApi,
  mockLogAutomationsApi,
  mockLogGroupsApi,
  mockQueueStatusApi,
  setupAutomationLogsMocks,
  setupAutomationLogsMocksWithApiError,
  setupAutomationLogsMocksWithAutomationsError,
} from './fixtures/automation-logs.fixture';
import { AutomationLogsPage } from './pages/automation-logs.page';

test.describe('Automation Logs - Error Scenarios', () => {
  let logsPage: AutomationLogsPage;

  test.beforeEach(async ({ page }) => {
    logsPage = new AutomationLogsPage(page);
  });

  // ============================================================
  // API Failures
  // ============================================================

  test.describe('API Failures', () => {
    test('should handle ListDispatchLogs API error gracefully', async ({ page }) => {
      const errors: Error[] = [];
      page.on('pageerror', (error) => errors.push(error));

      await setupAutomationLogsMocksWithApiError(page);
      await logsPage.page.goto('/automation-logs');

      // Wait for the page to settle after the error response
      await page.waitForTimeout(2000);

      // Page should not crash (no uncaught exceptions)
      expect(errors).toHaveLength(0);

      // Page title should still be visible (page rendered)
      await expect(logsPage.pageTitle).toBeVisible();
    });

    test('should handle ListAutomations API error gracefully', async ({ page }) => {
      const errors: Error[] = [];
      page.on('pageerror', (error) => errors.push(error));

      await setupAutomationLogsMocksWithAutomationsError(page);
      await logsPage.page.goto('/automation-logs');

      // Wait for the page to settle
      await page.waitForTimeout(2000);

      // Page should not crash
      expect(errors).toHaveLength(0);

      // Page title should still be visible
      await expect(logsPage.pageTitle).toBeVisible();

      // Logs should still render even if automations filter failed
      const badges = logsPage.getStatusBadges();
      await expect(badges.first()).toBeVisible();
    });

    test('should handle groups API failure gracefully', async ({ page }) => {
      const errors: Error[] = [];
      page.on('pageerror', (error) => errors.push(error));

      // Set up mocks with groups API returning 500
      await mockAppShellApis(page);
      await mockAuthApi(page);
      await mockDispatchLogsApi(page);
      await mockLogAutomationsApi(page);
      await page.route('**/groups.v1.GroupsService/ListGroups', async (route) => {
        await route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ code: 'internal', message: 'Internal server error' }),
        });
      });
      await mockQueueStatusApi(page);
      await mockListJobsApi(page);

      await logsPage.page.goto('/automation-logs');
      await page.waitForTimeout(2000);

      // Page should not crash
      expect(errors).toHaveLength(0);

      // Page title visible
      await expect(logsPage.pageTitle).toBeVisible();

      // Logs should still render
      const badges = logsPage.getStatusBadges();
      await expect(badges.first()).toBeVisible();
    });

    test('should handle queue status API failure gracefully', async ({ page }) => {
      const errors: Error[] = [];
      page.on('pageerror', (error) => errors.push(error));

      // Set up mocks with queue status API returning 500
      await mockAppShellApis(page);
      await mockAuthApi(page);
      await mockDispatchLogsApi(page);
      await mockLogAutomationsApi(page);
      await mockLogGroupsApi(page);
      await page.route('**/jobs.v1.JobsService/GetQueueStatus', async (route) => {
        await route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ code: 'internal', message: 'Internal server error' }),
        });
      });
      await mockListJobsApi(page);

      await logsPage.page.goto('/automation-logs');
      await page.waitForTimeout(2000);

      // Page should not crash
      expect(errors).toHaveLength(0);

      // Page title visible
      await expect(logsPage.pageTitle).toBeVisible();

      // Logs should still render regardless of queue status failure
      const badges = logsPage.getStatusBadges();
      await expect(badges.first()).toBeVisible();
    });
  });

  // ============================================================
  // Network / Edge Cases
  // ============================================================

  test.describe('Network / Edge Cases', () => {
    test('should handle slow API response', async ({ page }) => {
      // Set up mocks with slow dispatch logs response (3s delay)
      await mockAppShellApis(page);
      await mockAuthApi(page);
      await mockDispatchLogsApiSlow(page, 3000);
      await mockLogAutomationsApi(page);
      await mockLogGroupsApi(page);
      await mockQueueStatusApi(page);
      await mockListJobsApi(page);

      await logsPage.page.goto('/automation-logs');

      // Loading state should be visible while waiting for slow response
      const loading = page.getByText(/carregando|loading/i);
      await expect(loading).toBeVisible({ timeout: 2000 });

      // After delay, loading should eventually disappear
      await expect(loading).not.toBeVisible({ timeout: 10000 });
    });

    test('should handle special characters in search', async ({ page }) => {
      const errors: Error[] = [];
      page.on('pageerror', (error) => errors.push(error));

      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      // Type XSS-like input into search
      await logsPage.searchInput.fill('<script>alert(1)</script>');

      // Wait for debounce
      await page.waitForTimeout(1000);

      // Page should not crash
      expect(errors).toHaveLength(0);

      // Search input should contain the literal text (not executed as script)
      await expect(logsPage.searchInput).toHaveValue('<script>alert(1)</script>');

      // Page title still visible (page didn't break)
      await expect(logsPage.pageTitle).toBeVisible();
    });

    test('should handle rapid filter changes (debounce)', async ({ page }) => {
      const errors: Error[] = [];
      page.on('pageerror', (error) => errors.push(error));

      const requestCount: string[] = [];
      page.on('request', (request) => {
        if (request.url().includes('AutomationsService/ListDispatchLogs')) {
          requestCount.push(request.url());
        }
      });

      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      // Rapidly type characters into search (simulates fast typing)
      await logsPage.searchInput.pressSequentially('shopee', { delay: 50 });

      // Wait for debounce to settle
      await page.waitForTimeout(1500);

      // Page should not crash
      expect(errors).toHaveLength(0);

      // Page title still visible
      await expect(logsPage.pageTitle).toBeVisible();
    });

    test('should handle page navigation after error', async ({ page }) => {
      const errors: Error[] = [];
      page.on('pageerror', (error) => errors.push(error));

      // First, load with API error
      await setupAutomationLogsMocksWithApiError(page);
      await logsPage.page.goto('/automation-logs');
      await page.waitForTimeout(2000);

      // Navigate away
      await page.goto('/');
      await page.waitForTimeout(500);

      // Now set up working mocks and navigate back
      await page.unrouteAll();
      await setupAutomationLogsMocks(page);
      await logsPage.page.goto('/automation-logs');
      await logsPage.waitForLoad();

      // Page should work normally after returning
      expect(errors).toHaveLength(0);
      await expect(logsPage.pageTitle).toBeVisible();

      // Logs should render on second visit
      const badges = logsPage.getStatusBadges();
      await expect(badges.first()).toBeVisible();
    });
  });

  // ============================================================
  // Data Edge Cases
  // ============================================================

  test.describe('Data Edge Cases', () => {
    test('should handle log with no product URLs', async ({ page }) => {
      const errors: Error[] = [];
      page.on('pageerror', (error) => errors.push(error));

      // Create a log entry with no productUrl or affiliateLink
      const logsWithoutUrls = [
        {
          id: '100',
          automationId: 'auto-1',
          groupId: 'group-1',
          groupName: 'Promo Tech',
          canonicalKey: 'shopee:999:888',
          offerTitle: 'Product Without Links',
          status: 'sent',
          createdAt: new Date().toISOString(),
          store: 'shopee',
          // No productUrl, no affiliateLink, no productPageUrl
        },
      ];

      await mockAppShellApis(page);
      await mockAuthApi(page);
      await mockDispatchLogsApi(page, logsWithoutUrls);
      await mockLogAutomationsApi(page);
      await mockLogGroupsApi(page);
      await mockQueueStatusApi(page);
      await mockListJobsApi(page);

      await logsPage.goto();
      await logsPage.waitForLoad();

      // Page should not crash
      expect(errors).toHaveLength(0);

      // Sent badge should be visible (log rendered)
      const sentBadge = logsPage.getStatusBadgeByText('Enviado');
      await expect(sentBadge.first()).toBeVisible();

      // No broken external links should appear for this log
      const brokenLinks = page.locator('a[href="undefined"], a[href="null"], a[href=""]');
      await expect(brokenLinks).toHaveCount(0);
    });

    test('should handle long error messages', async ({ page }) => {
      const errors: Error[] = [];
      page.on('pageerror', (error) => errors.push(error));

      const longErrorMessage =
        `Este e um erro muito longo que pode potencialmente quebrar o layout da tabela ${'se nao houver tratamento adequado de overflow. '.repeat(5)}`;

      const logsWithLongError = [
        {
          id: '200',
          automationId: 'auto-1',
          groupId: 'group-1',
          groupName: 'Promo Tech',
          canonicalKey: 'shopee:777:666',
          offerTitle: 'Product With Long Error',
          status: 'failed',
          errorMessage: longErrorMessage,
          createdAt: new Date().toISOString(),
          store: 'shopee',
          productUrl: 'https://shopee.com.br/product/777/666',
        },
      ];

      await mockAppShellApis(page);
      await mockAuthApi(page);
      await mockDispatchLogsApi(page, logsWithLongError);
      await mockLogAutomationsApi(page);
      await mockLogGroupsApi(page);
      await mockQueueStatusApi(page);
      await mockListJobsApi(page);

      await logsPage.goto();
      await logsPage.waitForLoad();

      // Page should not crash
      expect(errors).toHaveLength(0);

      // Error badge should be visible
      const errorBadge = logsPage.getStatusBadgeByText('Erro');
      await expect(errorBadge.first()).toBeVisible();

      // The page should not have horizontal scrollbar caused by the long error
      const hasHorizontalOverflow = await page.evaluate(() => {
        return document.documentElement.scrollWidth > document.documentElement.clientWidth;
      });
      expect(hasHorizontalOverflow).toBe(false);
    });

    test('should handle totalCount > visible logs', async ({ page }) => {
      const errors: Error[] = [];
      page.on('pageerror', (error) => errors.push(error));

      // Return only 2 logs but claim totalCount is 50
      const partialLogs = MOCK_DISPATCH_LOGS.slice(0, 2);

      await mockAppShellApis(page);
      await mockAuthApi(page);
      await mockDispatchLogsApi(page, partialLogs, 50);
      await mockLogAutomationsApi(page);
      await mockLogGroupsApi(page);
      await mockQueueStatusApi(page);
      await mockListJobsApi(page);

      await logsPage.goto();
      await logsPage.waitForLoad();

      // Page should not crash
      expect(errors).toHaveLength(0);

      // Page title should be visible
      await expect(logsPage.pageTitle).toBeVisible();

      // Status badges from the 2 rendered logs should be visible
      const badges = logsPage.getStatusBadges();
      await expect(badges.first()).toBeVisible();
    });

    test('should handle refetch after initial load', async ({ page }) => {
      const errors: Error[] = [];
      page.on('pageerror', (error) => errors.push(error));

      let requestCount = 0;
      page.on('request', (request) => {
        if (request.url().includes('AutomationsService/ListDispatchLogs')) {
          requestCount++;
        }
      });

      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      const initialCount = requestCount;

      // Trigger a refetch by changing a filter and resetting it
      await logsPage.selectFilter(logsPage.storeFilter, 'Shopee');
      await page.waitForTimeout(500);
      await logsPage.selectFilter(logsPage.storeFilter, 'Todas');
      await page.waitForTimeout(1000);

      // Page should not crash
      expect(errors).toHaveLength(0);

      // At least one additional request should have been made after filter change
      expect(requestCount).toBeGreaterThan(initialCount);

      // Page still renders correctly
      await expect(logsPage.pageTitle).toBeVisible();
      const badges = logsPage.getStatusBadges();
      await expect(badges.first()).toBeVisible();
    });
  });
});
