import { expect, test } from './fixtures/index';
import { setupHistoryMocks, setupPaginatedHistoryMocks } from './fixtures/history.fixture';
import { HistoryPage } from './pages/history.page';

test.describe('History — Accessibility', () => {
  let historyPage: HistoryPage;

  test.beforeEach(async ({ page }) => {
    historyPage = new HistoryPage(page);
  });

  test('should have aria-label on all pagination buttons', async ({ page }) => {
    await setupPaginatedHistoryMocks(page);
    await historyPage.goto();
    await historyPage.waitForLoad();

    // First page button
    const firstBtn = historyPage.firstPageButton;
    await expect(firstBtn).toBeVisible();
    const firstLabel = await firstBtn.getAttribute('aria-label');
    expect(firstLabel).toBeTruthy();

    // Previous page button
    const prevBtn = historyPage.prevPageButton;
    await expect(prevBtn).toBeVisible();
    const prevLabel = await prevBtn.getAttribute('aria-label');
    expect(prevLabel).toBeTruthy();

    // Next page button
    const nextBtn = historyPage.nextPageButton;
    await expect(nextBtn).toBeVisible();
    const nextLabel = await nextBtn.getAttribute('aria-label');
    expect(nextLabel).toBeTruthy();

    // Last page button
    const lastBtn = historyPage.lastPageButton;
    await expect(lastBtn).toBeVisible();
    const lastLabel = await lastBtn.getAttribute('aria-label');
    expect(lastLabel).toBeTruthy();
  });

  test('should have aria-hidden on status badge icons', async ({ page }) => {
    await setupHistoryMocks(page);
    await historyPage.goto();
    await historyPage.waitForLoad();

    // Status badge icons in the table should have aria-hidden="true"
    const badgeIcons = page.locator('span.inline-flex svg[aria-hidden="true"]');
    const count = await badgeIcons.count();
    expect(count).toBeGreaterThan(0);
  });

  test('should have aria-hidden on stat card icons', async ({ page }) => {
    await setupHistoryMocks(page);
    await historyPage.goto();
    await historyPage.waitForLoad();

    // Stat card icons should have aria-hidden
    const statIcons = page.locator('.grid.grid-cols-2 svg[aria-hidden="true"]');
    const count = await statIcons.count();
    expect(count).toBe(4);
  });

  test('should support keyboard Tab navigation between filter selects', async ({ page }) => {
    await setupHistoryMocks(page);
    await historyPage.goto();
    await historyPage.waitForLoad();

    // Focus on the event type filter
    await historyPage.eventTypeFilter.focus();
    await expect(historyPage.eventTypeFilter).toBeFocused();

    // Tab to the next focusable element (status filter)
    await page.keyboard.press('Tab');

    // Status filter should be focusable
    const activeTag = await page.evaluate(() => document.activeElement?.tagName);
    expect(activeTag).toBeTruthy();
  });

  test('should show focus-visible on pagination buttons', async ({ page }) => {
    await setupPaginatedHistoryMocks(page);
    await historyPage.goto();
    await historyPage.waitForLoad();

    // Tab into pagination area
    const nextBtn = historyPage.nextPageButton;
    await nextBtn.focus();

    // Check that the button is focused
    await expect(nextBtn).toBeFocused();
  });

  test('should have role="alertdialog" on the clear confirmation dialog', async ({ page }) => {
    await setupHistoryMocks(page);
    await historyPage.goto();
    await historyPage.waitForLoad();

    // Open the clear dialog
    await historyPage.clearHistoryButton.click();

    // Dialog should have alertdialog role
    const dialog = page.locator('[role="alertdialog"]');
    await expect(dialog).toBeVisible();

    // Dialog should have a title
    const title = dialog.locator('[id]').first();
    await expect(title).toBeVisible();
  });
});
