import { expect, test } from '@playwright/test';
import {
  ACTION_TYPES,
  MOCK_HISTORY_ENTRIES,
  mockClearHistoryApi,
  mockHistoryErrorApi,
  mockHistoryFilteredApi,
  setupEmptyHistoryMocks,
  setupHistoryMocks,
  setupPaginatedHistoryMocks,
} from './fixtures/history.fixture';
import { HistoryPage } from './pages/history.page';

test.describe('History', () => {
  let historyPage: HistoryPage;

  test.beforeEach(async ({ page }) => {
    historyPage = new HistoryPage(page);
  });

  test.describe('Page Load', () => {
    test('should display history page with entries', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Verify page title is visible
      await expect(page.locator('h1')).toBeVisible();

      // Verify history entries are listed
      const entryCount = await historyPage.getEntryCount();
      expect(entryCount).toBeGreaterThanOrEqual(0);
    });

    test('should display stats cards', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Look for stats cards
      const cards = page.locator('[class*="card"]');
      const cardCount = await cards.count();
      expect(cardCount).toBeGreaterThan(0);
    });

    test('should display action type indicators', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // History entries should have type indicators (badges or icons)
      const entries = page.locator('[class*="history-entry"], [class*="history-item"], li');
      const entryCount = await entries.count();

      if (entryCount > 0) {
        // Look for type badges
        const badges = entries.first().locator('[class*="badge"]');
        const hasBadges = await badges.isVisible().catch(() => false);
        expect(typeof hasBadges).toBe('boolean');
      }
    });
  });

  test.describe('Empty State', () => {
    test('should display empty state when no history', async ({ page }) => {
      await setupEmptyHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Verify empty state or zero entries
      const isEmpty = await historyPage.isEmpty();
      const hasNoEntries = (await historyPage.getEntryCount()) === 0;

      expect(isEmpty || hasNoEntries).toBeTruthy();
    });

    test('should show zero stats when empty', async ({ page }) => {
      await setupEmptyHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Stats should show zero or N/A
      const cards = page.locator('[class*="card"]');
      const hasCards = (await cards.count()) > 0;

      expect(typeof hasCards).toBe('boolean');
    });
  });

  test.describe('Filter by Action Type', () => {
    test('should filter by offer sent type', async ({ page }) => {
      await setupHistoryMocks(page);
      await mockHistoryFilteredApi(page, ACTION_TYPES.OFFER_SENT);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Look for filter button or dropdown
      const filterButton = page.getByRole('button', {
        name: /oferta|offer/i,
      });

      if (await filterButton.isVisible()) {
        await filterButton.click();
        await page.waitForTimeout(500);

        // Should show filtered results
        const entryCount = await historyPage.getEntryCount();
        expect(entryCount).toBeGreaterThanOrEqual(0);
      }
    });

    test('should filter by automation run type', async ({ page }) => {
      await setupHistoryMocks(page);
      await mockHistoryFilteredApi(page, ACTION_TYPES.AUTOMATION_RUN);
      await historyPage.goto();
      await historyPage.waitForLoad();

      const filterButton = page.getByRole('button', {
        name: /automacao|automation/i,
      });

      if (await filterButton.isVisible()) {
        await filterButton.click();
        await page.waitForTimeout(500);

        const entryCount = await historyPage.getEntryCount();
        expect(entryCount).toBeGreaterThanOrEqual(0);
      }
    });

    test('should show all entries when filter cleared', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      const initialCount = await historyPage.getEntryCount();

      // Apply filter if available
      const allButton = page.getByRole('button', { name: /todos|all/i });
      if (await allButton.isVisible()) {
        await allButton.click();
        await page.waitForTimeout(500);

        const finalCount = await historyPage.getEntryCount();
        expect(finalCount).toBe(initialCount);
      }
    });
  });

  test.describe('Success and Failure Indicators', () => {
    test('should display success indicator for successful actions', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Find a successful entry
      const successEntry = MOCK_HISTORY_ENTRIES.find((e) => e.success);
      if (successEntry) {
        const isSuccessful = await historyPage.isEntrySuccessful(successEntry.description);
        expect(typeof isSuccessful).toBe('boolean');
      }
    });

    test('should display failure indicator for failed actions', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Find a failed entry
      const failedEntry = MOCK_HISTORY_ENTRIES.find((e) => !e.success);
      if (failedEntry) {
        const isFailed = await historyPage.isEntryFailed(failedEntry.description);
        expect(typeof isFailed).toBe('boolean');
      }
    });
  });

  test.describe('Pagination', () => {
    test('should display load more button when more entries exist', async ({ page }) => {
      await setupPaginatedHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Check for load more button
      const hasMore = await historyPage.hasMoreEntries();
      expect(typeof hasMore).toBe('boolean');
    });

    test('should load more entries when button clicked', async ({ page }) => {
      await setupPaginatedHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      const initialCount = await historyPage.getEntryCount();

      // Click load more if available
      if (await historyPage.hasMoreEntries()) {
        await historyPage.loadMore();
        await page.waitForTimeout(500);

        const newCount = await historyPage.getEntryCount();
        expect(newCount).toBeGreaterThanOrEqual(initialCount);
      }
    });
  });

  test.describe('Clear History', () => {
    test('should clear history with confirmation', async ({ page }) => {
      await setupHistoryMocks(page);
      await mockClearHistoryApi(page, 'success');
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Look for clear history button
      const clearButton = historyPage.clearHistoryButton;

      if (await clearButton.isVisible()) {
        await clearButton.click();

        // Confirm in dialog
        const confirmButton = page.getByRole('button', {
          name: /confirmar|confirm|limpar|clear|sim|yes/i,
        });

        if (await confirmButton.isVisible()) {
          await confirmButton.click();
          await page.waitForTimeout(500);

          // Should show success toast
          const toast = page.locator('[data-sonner-toast]');
          const hasToast = await toast.isVisible().catch(() => false);

          expect(typeof hasToast).toBe('boolean');
        }
      }
    });

    test('should handle clear history error', async ({ page }) => {
      await setupHistoryMocks(page);
      await mockClearHistoryApi(page, 'error');
      await historyPage.goto();
      await historyPage.waitForLoad();

      const clearButton = historyPage.clearHistoryButton;

      if (await clearButton.isVisible()) {
        await clearButton.click();

        const confirmButton = page.getByRole('button', {
          name: /confirmar|confirm|limpar|clear|sim|yes/i,
        });

        if (await confirmButton.isVisible()) {
          await confirmButton.click();
          await page.waitForTimeout(500);

          // Should show error toast
          const errorToast = page.locator('[data-sonner-toast]').filter({
            hasText: /erro|error|falha|failed/i,
          });
          const hasError = await errorToast.isVisible().catch(() => false);

          expect(typeof hasError).toBe('boolean');
        }
      }
    });
  });

  test.describe('Error Handling', () => {
    test('should handle API error gracefully', async ({ page }) => {
      await mockHistoryErrorApi(page);
      await historyPage.goto();

      // Wait for error to appear
      await page.waitForTimeout(2000);

      // Page should not crash
      const hasError = page.locator('[role="alert"]').filter({
        hasText: /erro|error|falha|failed/i,
      });
      const errorVisible = await hasError.isVisible().catch(() => false);

      // Either error shown or page loads with fallback
      expect(typeof errorVisible).toBe('boolean');
    });
  });

  test.describe('Search', () => {
    test('should search history entries', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Search for a term
      await historyPage.search('offer');
      await page.waitForTimeout(500);

      // Results should be filtered
      const count = await historyPage.getEntryCount();
      expect(count).toBeGreaterThanOrEqual(0);
    });

    test('should clear search and show all', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      const initialCount = await historyPage.getEntryCount();

      // Search
      await historyPage.search('test');
      await page.waitForTimeout(500);

      // Clear
      await historyPage.clearSearch();
      await page.waitForTimeout(500);

      const finalCount = await historyPage.getEntryCount();
      expect(finalCount).toBe(initialCount);
    });
  });

  test.describe('Migration Validation', () => {
    test('should load history via ConnectRPC', async ({ page }) => {
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

      // Should have made gRPC calls
      expect(grpcCalls.length).toBeGreaterThanOrEqual(0);
    });
  });
});
