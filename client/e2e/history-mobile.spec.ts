import { expect, test } from './fixtures/index';
import { MOCK_HISTORY_EVENTS, setupHistoryMocks } from './fixtures/history.fixture';
import { HistoryPage } from './pages/history.page';

test.describe('History — Mobile Viewport', () => {
  let historyPage: HistoryPage;

  test.beforeEach(async ({ page }) => {
    historyPage = new HistoryPage(page);
  });

  test.describe('Mobile (375x667)', () => {
    test.beforeEach(async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 667 });
    });

    test('should hide desktop table on mobile', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Desktop table has "hidden md:block" — hidden on mobile
      const desktopTable = page.locator('.hidden.md\\:block');
      await expect(desktopTable).toBeHidden();
    });

    test('should show mobile cards on mobile viewport', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Mobile cards container is md:hidden — visible on mobile
      const mobileContainer = page.locator('.md\\:hidden');
      await expect(mobileContainer).toBeVisible();

      // Should have cards matching the events count
      const cards = mobileContainer.locator(':scope > div');
      const cardCount = await cards.count();
      expect(cardCount).toBe(MOCK_HISTORY_EVENTS.length);
    });

    test('should display action type and date on mobile cards', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      const mobileContainer = page.locator('.md\\:hidden');
      const firstCard = mobileContainer.locator(':scope > div').first();

      // Action type should be visible
      const actionText = await firstCard.locator('.font-medium').textContent();
      expect(actionText).toBeTruthy();

      // Date should be visible (dd/MM/yyyy HH:mm format)
      const dateText = await firstCard.textContent();
      expect(dateText).toMatch(/\d{2}\/\d{2}\/\d{4}/);
    });

    test('should display status badge on mobile cards', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      const mobileContainer = page.locator('.md\\:hidden');
      const firstCard = mobileContainer.locator(':scope > div').first();

      // Status badge should be present
      const badge = firstCard.locator('span.inline-flex');
      await expect(badge).toBeVisible();
    });

    test('should display error message on mobile cards when present', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // history-4 has errorMessage "Group not accessible"
      const mobileContainer = page.locator('.md\\:hidden');
      const errorText = mobileContainer.getByText('Group not accessible');
      await expect(errorText).toBeVisible();
    });

    test('should display results on mobile cards when target_count > 0', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      const mobileContainer = page.locator('.md\\:hidden');
      // First event has targetCount: 5
      const totalText = mobileContainer.getByText('Total: 5');
      await expect(totalText).toBeVisible();
    });

    test('should display pagination controls on mobile', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      await expect(historyPage.firstPageButton).toBeVisible();
      await expect(historyPage.nextPageButton).toBeVisible();
    });

    test('should display stat cards in 2-column grid on mobile', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      await expect(historyPage.statCards).toHaveCount(4);

      // Grid container should have grid-cols-2 class
      const grid = page.locator('.grid.grid-cols-2');
      await expect(grid).toBeVisible();
    });
  });

  test.describe('Tablet (768x1024)', () => {
    test.beforeEach(async ({ page }) => {
      await page.setViewportSize({ width: 768, height: 1024 });
    });

    test('should show desktop table on tablet viewport', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Desktop table should be visible at md breakpoint (768px)
      const desktopTable = page.locator('.hidden.md\\:block');
      await expect(desktopTable).toBeVisible();
    });

    test('should hide mobile cards on tablet viewport', async ({ page }) => {
      await setupHistoryMocks(page);
      await historyPage.goto();
      await historyPage.waitForLoad();

      // Mobile cards should be hidden at md breakpoint
      const mobileContainer = page.locator('.md\\:hidden');
      await expect(mobileContainer).toBeHidden();
    });
  });
});
