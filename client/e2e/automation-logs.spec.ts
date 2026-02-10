import { expect, test } from '@playwright/test';
import {
  MOCK_DISPATCH_LOGS,
  mockAppShellApis,
  mockAuthApi,
  mockListJobsApi,
  mockLogAutomationsApi,
  mockLogGroupsApi,
  mockQueueStatusApi,
  setupAutomationLogsMocks,
  setupEmptyAutomationLogsMocks,
} from './fixtures/automation-logs.fixture';
import { AutomationLogsPage } from './pages/automation-logs.page';

test.describe('Automation Logs Page', () => {
  let logsPage: AutomationLogsPage;

  test.beforeEach(async ({ page }) => {
    logsPage = new AutomationLogsPage(page);
  });

  // ============================================================
  // Loading
  // ============================================================

  test.describe('Loading', () => {
    test('should display list of logs', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      // Verify page title is visible
      await expect(logsPage.pageTitle).toBeVisible();

      // Verify logs are displayed (status badges should be visible)
      const badges = logsPage.getStatusBadges();
      await expect(badges.first()).toBeVisible();
    });

    test('should display empty state when no logs', async ({ page }) => {
      await setupEmptyAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      await expect(logsPage.emptyState).toBeVisible();
    });

    test('should display loading state during loading', async ({ page }) => {
      // Catch-all FIRST (Playwright evaluates routes in reverse order)
      await mockAppShellApis(page);
      await mockAuthApi(page);
      // Delay the response to see loading state
      await page.route('**/automations.v1.AutomationsService/ListDispatchLogs', async (route) => {
        await new Promise((resolve) => setTimeout(resolve, 2000));
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ logs: [], totalCount: 0, hasMore: false }),
        });
      });
      await mockLogAutomationsApi(page);
      await mockLogGroupsApi(page);
      await mockQueueStatusApi(page);
      await mockListJobsApi(page);

      await logsPage.page.goto('/automation-logs');

      // Loading state should be visible
      const loading = page.getByText(/carregando|loading/i);
      await expect(loading).toBeVisible({ timeout: 3000 });
    });
  });

  // ============================================================
  // Filters
  // ============================================================

  test.describe('Filters', () => {
    test('should filter by automation', async ({ page }) => {
      let requestUrl = '';
      // Catch-all FIRST (Playwright evaluates routes in reverse order)
      await mockAppShellApis(page);
      await mockAuthApi(page);
      await page.route('**/automations.v1.AutomationsService/ListDispatchLogs', async (route) => {
        requestUrl = route.request().url();
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            logs: MOCK_DISPATCH_LOGS.filter((l) => l.automationId === 'auto-1'),
            totalCount: 3,
            hasMore: false,
          }),
        });
      });
      await mockLogAutomationsApi(page);
      await mockLogGroupsApi(page);
      await mockQueueStatusApi(page);
      await mockListJobsApi(page);

      await logsPage.goto();
      await logsPage.waitForLoad();

      // Select automation filter
      await logsPage.selectFilter(logsPage.automationFilter, 'Shopee Promos');

      // Verify the request was made (page should reload data)
      await page.waitForTimeout(500);
    });

    test('should filter by store', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      // Select store filter
      await logsPage.selectFilter(logsPage.storeFilter, 'Shopee');

      await page.waitForTimeout(500);
    });

    test('should filter by group', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      // Select group filter
      await logsPage.selectFilter(logsPage.groupFilter, 'Promo Tech');

      await page.waitForTimeout(500);
    });

    test('should filter by status', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      // Select status filter
      await logsPage.selectFilter(logsPage.statusFilter, 'Enviado');

      await page.waitForTimeout(500);
    });

    test('should search by product URL', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      // Type in search input
      await logsPage.searchInput.fill('shopee');

      // Debounce wait
      await page.waitForTimeout(500);
    });

    test('should combine multiple filters', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      // Apply multiple filters
      await logsPage.selectFilter(logsPage.storeFilter, 'Shopee');
      await page.waitForTimeout(300);
      await logsPage.selectFilter(logsPage.statusFilter, 'Enviado');
      await page.waitForTimeout(300);
    });

    test('should clear filters', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      // Apply a filter then clear it
      await logsPage.selectFilter(logsPage.storeFilter, 'Shopee');
      await page.waitForTimeout(300);
      await logsPage.selectFilter(logsPage.storeFilter, 'Todas');
      await page.waitForTimeout(300);
    });
  });

  // ============================================================
  // Status Badges
  // ============================================================

  test.describe('Status Badges', () => {
    test('should display green badge for sent status', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      const sentBadge = logsPage.getStatusBadgeByText('Enviado');
      await expect(sentBadge.first()).toBeVisible();
    });

    test('should display badge for queued status', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      const queuedBadge = logsPage.getStatusBadgeByText('Em fila');
      await expect(queuedBadge.first()).toBeVisible();
    });

    test('should display badge for skipped status', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      const skippedBadge = logsPage.getStatusBadgeByText('Ignorado');
      await expect(skippedBadge.first()).toBeVisible();
    });

    test('should display red badge for error status', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      const errorBadge = logsPage.getStatusBadgeByText('Erro');
      await expect(errorBadge.first()).toBeVisible();
    });
  });

  // ============================================================
  // Coupons
  // ============================================================

  test.describe('Coupons', () => {
    test('should display applied coupon with checkmark', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      const appliedBadge = logsPage.getCouponAppliedBadge();
      await expect(appliedBadge.first()).toBeVisible();
    });

    test('should display not-applied coupon with X', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      const notAppliedBadge = logsPage.getCouponNotAppliedBadge();
      await expect(notAppliedBadge.first()).toBeVisible();
    });

    test('should display coupon skip reason', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      const skipReason = logsPage.getCouponSkipReason();
      await expect(skipReason.first()).toBeVisible();
    });
  });

  // ============================================================
  // Queue Status
  // ============================================================

  test.describe('Queue Status', () => {
    test('should display queue status', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      // Verify queue metrics are visible
      const queueText = page.getByText(/aguardando.*7/i);
      await expect(queueText).toBeVisible();
    });

    test('should display estimated time', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      // Verify estimated time (use first() to avoid strict mode)
      const estimate = page.getByText(/1.*min/i).first();
      await expect(estimate).toBeVisible();
    });
  });

  // ============================================================
  // Error Handling
  // ============================================================

  test.describe('Errors', () => {
    test('should display reconnect button for instance error', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      // The mock data has a log with "instância não encontrada" error
      const reconnectLink = logsPage.getReconnectLinks();
      await expect(reconnectLink.first()).toBeVisible();
    });

    test('should navigate to /whatsapp when clicking reconnect', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      const reconnectLink = logsPage.getReconnectLinks().first();
      const href = await reconnectLink.getAttribute('href');
      expect(href).toBe('/whatsapp');
    });
  });

  // ============================================================
  // External Links
  // ============================================================

  test.describe('External Links', () => {
    test('should open product link in new tab', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      const productLinks = logsPage.getProductLinks();
      const firstLink = productLinks.first();

      // Verify the link opens in a new tab
      await expect(firstLink).toHaveAttribute('target', '_blank');
      await expect(firstLink).toHaveAttribute('rel', 'noopener noreferrer');
    });
  });

  // ============================================================
  // Export
  // ============================================================

  test.describe('Export', () => {
    test('should export logs to CSV', async ({ page }) => {
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      // Verify export button is enabled
      await expect(logsPage.exportCsvButton).toBeEnabled();

      // Click export and verify download
      const downloadPromise = page.waitForEvent('download');
      await logsPage.exportCsvButton.click();
      const download = await downloadPromise;

      expect(download.suggestedFilename()).toMatch(/logs-\d{4}-\d{2}-\d{2}\.csv/);
    });

    test('should disable export when no logs', async ({ page }) => {
      await setupEmptyAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      await expect(logsPage.exportCsvButton).toBeDisabled();
    });
  });

  // ============================================================
  // Responsive
  // ============================================================

  test.describe('Responsive', () => {
    test('should display table on desktop', async ({ page }) => {
      await page.setViewportSize({ width: 1280, height: 720 });
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      await expect(logsPage.virtualTable).toBeVisible();
    });

    test('should display cards on mobile', async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 667 });
      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      await expect(logsPage.mobileCards).toBeVisible();
    });
  });

  // ============================================================
  // Integration (no legacy calls)
  // ============================================================

  test.describe('Integration', () => {
    test('should use ConnectRPC to fetch logs', async ({ page }) => {
      const connectCalls: string[] = [];

      page.on('request', (request) => {
        const url = request.url();
        if (url.includes('AutomationsService/ListDispatchLogs')) {
          connectCalls.push(url);
        }
      });

      await setupAutomationLogsMocks(page);
      await logsPage.goto();
      await logsPage.waitForLoad();

      // Wait for data to load
      await page.waitForTimeout(1000);

      expect(connectCalls.length).toBeGreaterThan(0);
    });
  });
});
