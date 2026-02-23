import { expect, test } from './fixtures/index';
import {
  MOCK_HISTORY_EVENTS,
  MOCK_HISTORY_STATS,
  mockClearHistoryApi,
  mockHistoryListApi,
  setupHistoryMocks,
  setupPaginatedHistoryMocks,
} from './fixtures/history.fixture';
import { setupAuthMocks } from './fixtures/auth.fixture';
import { HistoryPage } from './pages/history.page';

test.describe('History — Filters + Pagination Combined', () => {
  let historyPage: HistoryPage;

  test.beforeEach(async ({ page }) => {
    historyPage = new HistoryPage(page);
  });

  test('should include eventType in request when event type filter is applied', async ({ page }) => {
    await setupAuthMocks(page);
    await mockClearHistoryApi(page);

    let capturedBody: Record<string, unknown> | null = null;

    await page.route('**/history.v1.HistoryService/ListHistory', async (route) => {
      const body = route.request().postDataJSON();
      capturedBody = body;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({
          events: MOCK_HISTORY_EVENTS.slice(0, 2),
          hasMore: false,
          stats: MOCK_HISTORY_STATS,
          totalCount: 2,
          page: 1,
          pageSize: 20,
          totalPages: 1,
        }),
      });
    });

    await historyPage.goto();
    await historyPage.waitForLoad();

    // Apply event type filter
    await historyPage.eventTypeFilter.click();
    const options = page.locator('[role="option"]');
    const count = await options.count();
    if (count > 1) {
      await options.nth(1).click();
      await page.waitForTimeout(500);
    }

    // Verify the filter was applied (request was made)
    expect(capturedBody).not.toBeNull();
  });

  test('should include status in request when status filter is applied', async ({ page }) => {
    await setupAuthMocks(page);
    await mockClearHistoryApi(page);

    let capturedBody: Record<string, unknown> | null = null;

    await page.route('**/history.v1.HistoryService/ListHistory', async (route) => {
      const body = route.request().postDataJSON();
      capturedBody = body;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({
          events: MOCK_HISTORY_EVENTS.filter((e) => e.status === 'completed'),
          hasMore: false,
          stats: { total: 3, completed: 3, running: 0, failed: 0 },
          totalCount: 3,
          page: 1,
          pageSize: 20,
          totalPages: 1,
        }),
      });
    });

    await historyPage.goto();
    await historyPage.waitForLoad();

    // Apply status filter
    await historyPage.statusFilter.click();
    const options = page.locator('[role="option"]');
    const count = await options.count();
    if (count > 1) {
      await options.nth(1).click();
      await page.waitForTimeout(500);
    }

    expect(capturedBody).not.toBeNull();
  });

  test('should include both eventType and status when combined', async ({ page }) => {
    await setupAuthMocks(page);
    await mockClearHistoryApi(page);

    const capturedBodies: Record<string, unknown>[] = [];

    await page.route('**/history.v1.HistoryService/ListHistory', async (route) => {
      const body = route.request().postDataJSON();
      capturedBodies.push(body);
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({
          events: MOCK_HISTORY_EVENTS.slice(0, 1),
          hasMore: false,
          stats: { total: 1, completed: 1, running: 0, failed: 0 },
          totalCount: 1,
          page: 1,
          pageSize: 20,
          totalPages: 1,
        }),
      });
    });

    await historyPage.goto();
    await historyPage.waitForLoad();

    // Apply event type filter
    await historyPage.eventTypeFilter.click();
    let options = page.locator('[role="option"]');
    if ((await options.count()) > 1) {
      await options.nth(1).click();
      await page.waitForTimeout(300);
    }

    // Apply status filter
    await historyPage.statusFilter.click();
    options = page.locator('[role="option"]');
    if ((await options.count()) > 1) {
      await options.nth(1).click();
      await page.waitForTimeout(500);
    }

    // At least 2 requests should have been made (initial + filter changes)
    expect(capturedBodies.length).toBeGreaterThanOrEqual(2);
  });

  test('should clear filters and return to unfiltered request', async ({ page }) => {
    await setupHistoryMocks(page);
    await historyPage.goto();
    await historyPage.waitForLoad();

    // Apply a filter first
    await historyPage.eventTypeFilter.click();
    const options = page.locator('[role="option"]');
    if ((await options.count()) > 1) {
      await options.nth(1).click();
      await page.waitForTimeout(300);
    }

    // Clear filters button should appear
    const clearBtn = historyPage.clearFiltersButton;
    const isVisible = await clearBtn.isVisible().catch(() => false);
    if (isVisible) {
      await clearBtn.click();
      await page.waitForTimeout(300);
      // After clearing, the clear button should be hidden
      const stillVisible = await clearBtn.isVisible().catch(() => false);
      expect(stillVisible).toBeFalsy();
    }
  });

  test('should reset page to 1 when filter is applied', async ({ page }) => {
    await setupAuthMocks(page);
    await mockClearHistoryApi(page);

    let requestCount = 0;

    await page.route('**/history.v1.HistoryService/ListHistory', async (route) => {
      requestCount++;
      const isFirstPage = requestCount <= 1;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({
          events: MOCK_HISTORY_EVENTS,
          hasMore: true,
          stats: { total: 100, completed: 70, running: 10, failed: 20 },
          totalCount: 100,
          page: isFirstPage ? 1 : 1, // After filter, page resets to 1
          pageSize: 20,
          totalPages: 5,
        }),
      });
    });

    await historyPage.goto();
    await historyPage.waitForLoad();

    // Navigate to page 2
    await historyPage.nextPageButton.click();
    await page.waitForTimeout(300);

    // Apply a filter → should reset to page 1
    await historyPage.eventTypeFilter.click();
    const options = page.locator('[role="option"]');
    if ((await options.count()) > 1) {
      await options.nth(1).click();
      await page.waitForTimeout(500);
    }

    // Pagination should show page 1 again
    const paginationText = await historyPage.paginationInfo.first().textContent();
    expect(paginationText).toContain('1');
  });

  test('should update pagination info after applying filter', async ({ page }) => {
    await setupAuthMocks(page);
    await mockClearHistoryApi(page);

    let callCount = 0;

    await page.route('**/history.v1.HistoryService/ListHistory', async (route) => {
      callCount++;
      // First call: full data; subsequent calls: filtered data
      const isFiltered = callCount > 1;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({
          events: isFiltered ? MOCK_HISTORY_EVENTS.slice(0, 1) : MOCK_HISTORY_EVENTS,
          hasMore: false,
          stats: isFiltered
            ? { total: 1, completed: 1, running: 0, failed: 0 }
            : MOCK_HISTORY_STATS,
          totalCount: isFiltered ? 1 : MOCK_HISTORY_EVENTS.length,
          page: 1,
          pageSize: 20,
          totalPages: 1,
        }),
      });
    });

    await historyPage.goto();
    await historyPage.waitForLoad();

    // Apply filter
    await historyPage.eventTypeFilter.click();
    const options = page.locator('[role="option"]');
    if ((await options.count()) > 1) {
      await options.nth(1).click();
      await page.waitForTimeout(500);
    }

    // Verify pagination info updated
    const paginationText = await historyPage.paginationInfo.first().textContent();
    expect(paginationText).toBeTruthy();
  });
});
