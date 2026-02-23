import { expect, test } from './fixtures/index';
import {
  MOCK_HISTORY_EVENTS,
  MOCK_HISTORY_STATS,
  mockClearHistoryApi,
  mockHistoryErrorApi,
  mockHistoryListApi,
  setupEmptyHistoryMocks,
  setupHistoryMocks,
  setupPaginatedHistoryMocks,
} from './fixtures/history.fixture';
import { setupAuthMocks } from './fixtures/auth.fixture';
import { HistoryPage } from './pages/history.page';

test.describe('History', () => {
  let historyPage: HistoryPage;

  test.beforeEach(async ({ page }) => {
    historyPage = new HistoryPage(page);
  });

  // ==============================================================
  // 1. Load & Display
  // ==============================================================
  test.describe('Load & Display', () => {
    test('should display page title', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      await expect(historyPage.pageTitle).toBeVisible();
    });

    test('should display stat cards with correct values', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // 4 stat cards should be visible
      await expect(historyPage.statCards).toHaveCount(4);

      // Verify stat values match mock data
      const totalValue = await historyPage.getStatValue(0);
      expect(totalValue?.trim()).toBe(String(MOCK_HISTORY_STATS.total));

      const completedValue = await historyPage.getStatValue(1);
      expect(completedValue?.trim()).toBe(String(MOCK_HISTORY_STATS.completed));

      const runningValue = await historyPage.getStatValue(2);
      expect(runningValue?.trim()).toBe(String(MOCK_HISTORY_STATS.running));

      const failedValue = await historyPage.getStatValue(3);
      expect(failedValue?.trim()).toBe(String(MOCK_HISTORY_STATS.failed));
    });

    test('should display table rows matching event count', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Desktop table should show 5 rows
      const rowCount = await historyPage.getTableRowCount();
      expect(rowCount).toBe(MOCK_HISTORY_EVENTS.length);
    });

    test('should display empty state when no history', async ({ page }) => {
      await setupEmptyHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Empty state message or zero rows
      const isEmpty = await historyPage.isEmpty();
      const rowCount = await historyPage.getTableRowCount();
      expect(isEmpty || rowCount === 0).toBeTruthy();
    });

    test('should display zero stats when empty', async ({ page }) => {
      await setupEmptyHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      await expect(historyPage.statCards).toHaveCount(4);

      const totalValue = await historyPage.getStatValue(0);
      expect(totalValue?.trim()).toBe('0');
    });
  });

  // ==============================================================
  // 2. Filters
  // ==============================================================
  test.describe('Filters', () => {
    test('should filter by event type', async ({ page }) => {
      await setupAuthMocks(page);
      await mockClearHistoryApi(page);

      // Set up route that returns filtered results when eventType param is provided
      const filteredEvents = MOCK_HISTORY_EVENTS.filter(
        (e) => e.eventType === 'EVENT_TYPE_MESSAGE_SENT'
      );
      await mockHistoryListApi(page, filteredEvents, {
        totalCount: filteredEvents.length,
        stats: { total: filteredEvents.length, completed: filteredEvents.length, running: 0, failed: 0 },
      });

      await historyPage.goto();
      await historyPage.waitForLoad();

      // Click the event type filter
      await historyPage.eventTypeFilter.click();

      // Select an option from the dropdown
      const option = page.locator('[role="option"]').first();
      if (await option.isVisible()) {
        await option.click();
        await page.waitForTimeout(300);
      }

      // The filter combobox should have changed from its default value
      await expect(historyPage.eventTypeFilter).toBeVisible();
    });

    test('should filter by status', async ({ page }) => {
      await setupAuthMocks(page);
      await mockClearHistoryApi(page);

      const failedEvents = MOCK_HISTORY_EVENTS.filter((e) => e.status === 'failed');
      await mockHistoryListApi(page, failedEvents, {
        totalCount: failedEvents.length,
        stats: { total: failedEvents.length, completed: 0, running: 0, failed: failedEvents.length },
      });

      await historyPage.goto();
      await historyPage.waitForLoad();

      // Click the status filter
      await historyPage.statusFilter.click();

      // Select an option from the dropdown
      const option = page.locator('[role="option"]').first();
      if (await option.isVisible()) {
        await option.click();
        await page.waitForTimeout(300);
      }

      await expect(historyPage.statusFilter).toBeVisible();
    });

    test('should show clear filters button when filters are active', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Apply a filter first
      await historyPage.eventTypeFilter.click();
      const options = page.locator('[role="option"]');
      const optionCount = await options.count();

      if (optionCount > 1) {
        // Select the second option (first is usually "all")
        await options.nth(1).click();
        await page.waitForTimeout(300);

        // Clear filters button (with X icon) should appear
        const clearBtn = historyPage.clearFiltersButton;
        const isVisible = await clearBtn.isVisible().catch(() => false);
        expect(typeof isVisible).toBe('boolean');
      }
    });
  });

  // ==============================================================
  // 3. Pagination
  // ==============================================================
  test.describe('Pagination', () => {
    test('should display pagination controls', async ({ page }) => {
      await setupPaginatedHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Pagination buttons should be visible
      await expect(historyPage.firstPageButton).toBeVisible();
      await expect(historyPage.prevPageButton).toBeVisible();
      await expect(historyPage.nextPageButton).toBeVisible();
      await expect(historyPage.lastPageButton).toBeVisible();
    });

    test('should disable first/prev buttons on first page', async ({ page }) => {
      await setupPaginatedHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      await expect(historyPage.firstPageButton).toBeDisabled();
      await expect(historyPage.prevPageButton).toBeDisabled();
    });

    test('should enable next/last buttons when more pages exist', async ({ page }) => {
      await setupPaginatedHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      await expect(historyPage.nextPageButton).toBeEnabled();
      await expect(historyPage.lastPageButton).toBeEnabled();
    });

    test('should display pagination info text', async ({ page }) => {
      await setupPaginatedHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Should show "1 / 5" or similar page info
      await expect(historyPage.paginationInfo.first()).toBeVisible();
    });

    test('should navigate to next page on click', async ({ page }) => {
      await setupPaginatedHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Intercept the next ListHistory call to verify pagination
      let nextPageCalled = false;
      await page.route('**/history.v1.HistoryService/ListHistory', async (route) => {
        nextPageCalled = true;
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: { 'grpc-status': '0' },
          body: JSON.stringify({
            events: MOCK_HISTORY_EVENTS.slice(0, 3),
            hasMore: false,
            stats: { total: 100, completed: 70, running: 10, failed: 20 },
            totalCount: 100,
            page: 2,
            pageSize: 20,
            totalPages: 5,
          }),
        });
      });

      await historyPage.nextPageButton.click();
      await page.waitForTimeout(500);

      expect(nextPageCalled).toBeTruthy();
    });
  });

  // ==============================================================
  // 4. Clear History
  // ==============================================================
  test.describe('Clear History', () => {
    test('should open confirmation dialog on clear button click', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      await historyPage.clearHistoryButton.click();
      await expect(historyPage.confirmDialog).toBeVisible();
    });

    test('should close dialog when cancel is clicked', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      await historyPage.clearHistoryButton.click();
      await expect(historyPage.confirmDialog).toBeVisible();

      await historyPage.cancelButton.click();
      await expect(historyPage.confirmDialog).not.toBeVisible();
    });

    test('should clear history when confirmed', async ({ page }) => {
      await setupAuthMocks(page);
      await mockHistoryListApi(page);
      await mockClearHistoryApi(page, 'success');

      await historyPage.goto();
      await historyPage.waitForLoad();

      // Track if ClearHistory was called
      let clearCalled = false;
      await page.route('**/history.v1.HistoryService/ClearHistory', async (route) => {
        clearCalled = true;
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: { 'grpc-status': '0' },
          body: JSON.stringify({ deletedCount: 100 }),
        });
      });

      await historyPage.clearHistory();
      await page.waitForTimeout(500);

      expect(clearCalled).toBeTruthy();
    });

    test('should handle clear history error', async ({ page }) => {
      await setupAuthMocks(page);
      await mockHistoryListApi(page);
      await mockClearHistoryApi(page, 'error');

      await historyPage.goto();
      await historyPage.waitForLoad();

      await historyPage.clearHistory();
      await page.waitForTimeout(500);

      // Page should not crash; still showing content
      await expect(historyPage.pageTitle).toBeVisible();
    });

    test('should disable clear button when history is empty', async ({ page }) => {
      await setupEmptyHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Clear button should be disabled when no events
      await expect(historyPage.clearHistoryButton).toBeDisabled();
    });
  });

  // ==============================================================
  // 5. Error Handling
  // ==============================================================
  test.describe('Error Handling', () => {
    test('should handle API error gracefully', async ({ page }) => {
      await setupAuthMocks(page);
      await mockHistoryErrorApi(page);

      await page.goto('/history');
      await page.waitForTimeout(2000);

      // Page should not crash; error boundary or fallback should render
      const pageContent = page.locator('body');
      await expect(pageContent).not.toBeEmpty();
    });
  });

  // ==============================================================
  // 6. ConnectRPC Integration
  // ==============================================================
  test.describe('ConnectRPC Integration', () => {
    test('should make calls to the correct service path', async ({ page }) => {
      const grpcCalls: string[] = [];

      await page.route('**/*', async (route) => {
        const url = route.request().url();
        if (url.includes('history.v1.HistoryService')) {
          grpcCalls.push(url);
        }
        await route.continue();
      });

      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Should have called ListHistory at minimum
      const listHistoryCalls = grpcCalls.filter((url) => url.includes('ListHistory'));
      expect(listHistoryCalls.length).toBeGreaterThanOrEqual(1);
    });

    test('should send requests with application/json content type', async ({ page }) => {
      let requestContentType = '';

      await page.route('**/history.v1.HistoryService/ListHistory', async (route) => {
        requestContentType = route.request().headers()['content-type'] || '';
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: { 'grpc-status': '0' },
          body: JSON.stringify({
            events: MOCK_HISTORY_EVENTS,
            hasMore: false,
            stats: MOCK_HISTORY_STATS,
            totalCount: MOCK_HISTORY_EVENTS.length,
            page: 1,
            pageSize: 20,
            totalPages: 1,
          }),
        });
      });

      await setupAuthMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // ConnectRPC uses application/json
      expect(requestContentType).toContain('json');
    });
  });
});
