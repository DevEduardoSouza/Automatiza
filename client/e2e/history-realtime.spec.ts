import { expect, test } from './fixtures/index';
import { MOCK_HISTORY_EVENTS, MOCK_HISTORY_STATS, setupHistoryMocks } from './fixtures/history.fixture';
import { setupAuthMocks } from './fixtures/auth.fixture';
import { HistoryPage } from './pages/history.page';

test.describe('History — Realtime SSE', () => {
  let historyPage: HistoryPage;

  test.beforeEach(async ({ page }) => {
    historyPage = new HistoryPage(page);
  });

  test('should refetch ListHistory after SSE history.change event', async ({ page }) => {
    await setupAuthMocks(page);

    let listHistoryCallCount = 0;

    await page.route('**/history.v1.HistoryService/ListHistory', async (route) => {
      listHistoryCallCount++;
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

    await page.route('**/history.v1.HistoryService/ClearHistory', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({ deletedCount: 0 }),
      });
    });

    await historyPage.goto();
    await historyPage.waitForLoad();

    const initialCalls = listHistoryCallCount;

    // Dispatch SSE-like event on window to trigger useHistoryRealtime refetch
    await page.evaluate(() => {
      window.dispatchEvent(
        new CustomEvent('sse:history-event', {
          detail: { type: 'history.change', data: {} },
        })
      );
    });

    // Wait for the refetch to happen (debounce time + network)
    await page.waitForTimeout(1000);

    // Should have made at least one more call after the SSE event
    expect(listHistoryCallCount).toBeGreaterThan(initialCalls);
  });

  test('should debounce multiple rapid SSE events into fewer requests', async ({ page }) => {
    await setupAuthMocks(page);

    let listHistoryCallCount = 0;

    await page.route('**/history.v1.HistoryService/ListHistory', async (route) => {
      listHistoryCallCount++;
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

    await page.route('**/history.v1.HistoryService/ClearHistory', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({ deletedCount: 0 }),
      });
    });

    await historyPage.goto();
    await historyPage.waitForLoad();

    const callsBeforeEvents = listHistoryCallCount;

    // Dispatch 5 rapid events
    await page.evaluate(() => {
      for (let i = 0; i < 5; i++) {
        window.dispatchEvent(
          new CustomEvent('sse:history-event', {
            detail: { type: 'history.change', data: { iteration: i } },
          })
        );
      }
    });

    // Wait for debounce to settle
    await page.waitForTimeout(1500);

    const callsAfterEvents = listHistoryCallCount - callsBeforeEvents;

    // Should have fewer calls than events fired (debounce coalescence)
    // At most 2-3 calls for 5 events (depends on exact debounce implementation)
    expect(callsAfterEvents).toBeLessThan(5);
  });

  test('should not refetch on irrelevant SSE event types', async ({ page }) => {
    await setupHistoryMocks(page);

    let listHistoryCallCount = 0;

    // Override the ListHistory route to count calls
    await page.route('**/history.v1.HistoryService/ListHistory', async (route) => {
      listHistoryCallCount++;
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

    await historyPage.goto();
    await historyPage.waitForLoad();

    const callsAfterLoad = listHistoryCallCount;

    // Dispatch irrelevant event
    await page.evaluate(() => {
      window.dispatchEvent(
        new CustomEvent('sse:history-event', {
          detail: { type: 'groups.update', data: {} },
        })
      );
    });

    await page.waitForTimeout(1000);

    // Should not have triggered any additional calls
    expect(listHistoryCallCount).toBe(callsAfterLoad);
  });

  test('should handle SSE event dispatched before page fully loaded', async ({ page }) => {
    await setupHistoryMocks(page);

    await page.goto('/history');

    // Dispatch event immediately (before waitForLoad)
    await page.evaluate(() => {
      window.dispatchEvent(
        new CustomEvent('sse:history-event', {
          detail: { type: 'history.change', data: {} },
        })
      );
    });

    // Page should not crash
    await historyPage.waitForLoad();
    await expect(historyPage.pageTitle).toBeVisible();
  });
});
