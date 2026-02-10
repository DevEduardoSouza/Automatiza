import { expect, test } from '@playwright/test';
import {
  MOCK_ML_LINK_LISTS,
  MOCK_ML_LINK_LIST_ITEMS,
  TEST_INVALID_URL,
  TEST_ML_LINK_LIST,
  TEST_VALID_ML_URL,
  mockBulkCreateItemsApi,
  mockCreateItemApi,
  mockCreateListApi,
  mockDeleteItemApi,
  mockDeleteListApi,
  mockPreviewApi,
  mockReorderItemsApi,
  mockResetConsumedApi,
  mockUpdateItemApi,
  mockUpdateListApi,
  setupEmptyMlLinkListsMocks,
  setupMlLinkListsMocks,
} from './fixtures/ml-link-lists.fixture';
import { MlLinkListsPage } from './pages/ml-link-lists.page';

test.describe('ML Link Lists - Page Load', () => {
  let mlLinkListsPage: MlLinkListsPage;

  test.beforeEach(async ({ page }) => {
    mlLinkListsPage = new MlLinkListsPage(page);
  });

  test('should display page title and components', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    await expect(mlLinkListsPage.pageTitle).toBeVisible();
    await expect(mlLinkListsPage.createListButton).toBeVisible();
    await expect(mlLinkListsPage.refreshButton).toBeVisible();
  });

  test('should display lists in dropdown', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    const listCount = await mlLinkListsPage.getListCount();
    expect(listCount).toBe(MOCK_ML_LINK_LISTS.length);
  });

  test('should auto-select first list', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    // Wait for items to load
    await page.waitForTimeout(500);

    // Should have items displayed
    const itemCount = await mlLinkListsPage.getItemCount();
    expect(itemCount).toBeGreaterThan(0);
  });
});

test.describe('ML Link Lists - Empty State', () => {
  let mlLinkListsPage: MlLinkListsPage;

  test.beforeEach(async ({ page }) => {
    mlLinkListsPage = new MlLinkListsPage(page);
  });

  test('should display empty state when no lists', async ({ page }) => {
    await setupEmptyMlLinkListsMocks(page);
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    // Create button should still be visible
    await expect(mlLinkListsPage.createListButton).toBeVisible();
  });

  test('should show "select list" message when no list selected', async ({ page }) => {
    await setupEmptyMlLinkListsMocks(page);
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    const selectListAlert = page.locator('text=/selecione uma lista|select a list/i');
    const hasAlert = await selectListAlert.isVisible().catch(() => false);
    expect(hasAlert || (await mlLinkListsPage.isEmpty())).toBeTruthy();
  });
});

test.describe('ML Link Lists - CRUD Lists', () => {
  let mlLinkListsPage: MlLinkListsPage;

  test.beforeEach(async ({ page }) => {
    mlLinkListsPage = new MlLinkListsPage(page);
  });

  test('should open create list dialog', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    await mlLinkListsPage.openCreateDialog();

    await expect(mlLinkListsPage.dialogContent).toBeVisible();
    await expect(mlLinkListsPage.listNameInput).toBeVisible();
  });

  test('should create a new list', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mockCreateListApi(page, 'success');
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    await mlLinkListsPage.openCreateDialog();
    await mlLinkListsPage.fillListForm({
      name: TEST_ML_LINK_LIST.name,
      description: TEST_ML_LINK_LIST.description,
      isActive: TEST_ML_LINK_LIST.is_active,
    });
    await mlLinkListsPage.saveList();

    // Dialog should close
    await expect(mlLinkListsPage.dialogContent).not.toBeVisible();
  });

  test('should edit existing list', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mockUpdateListApi(page, 'success');
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    await mlLinkListsPage.openEditDialog();
    await mlLinkListsPage.fillListForm({
      name: 'Updated List Name',
    });
    await mlLinkListsPage.saveList();

    await expect(mlLinkListsPage.dialogContent).not.toBeVisible();
  });

  test('should delete list with confirmation', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mockDeleteListApi(page, 'success');
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    await mlLinkListsPage.requestDeleteList();

    // Confirm dialog should appear
    const alertDialog = page.locator('[role="alertdialog"]');
    await expect(alertDialog).toBeVisible();

    await mlLinkListsPage.confirmDelete();

    // Alert dialog should close
    await expect(alertDialog).not.toBeVisible();
  });

  test('should cancel delete action', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    await mlLinkListsPage.requestDeleteList();

    const alertDialog = page.locator('[role="alertdialog"]');
    await expect(alertDialog).toBeVisible();

    // Click cancel
    await page
      .locator('[role="alertdialog"]')
      .getByRole('button', { name: /cancelar|cancel/i })
      .click();

    await expect(alertDialog).not.toBeVisible();
  });
});

test.describe('ML Link Lists - Add Items', () => {
  let mlLinkListsPage: MlLinkListsPage;

  test.beforeEach(async ({ page }) => {
    mlLinkListsPage = new MlLinkListsPage(page);
  });

  test('should add single valid ML URL', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mockCreateItemApi(page, 'success');
    await mockPreviewApi(page, 'success');
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    await mlLinkListsPage.addSingleUrl(TEST_VALID_ML_URL);

    // Wait for API call
    await page.waitForTimeout(500);

    // Input should be cleared on success
    const inputValue = await mlLinkListsPage.singleUrlInput.inputValue();
    expect(inputValue).toBe('');
  });

  test('should show error for invalid URL', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    await mlLinkListsPage.addSingleUrl(TEST_INVALID_URL);

    // Error toast should appear
    const toast = page.locator('[role="status"], [data-sonner-toast]').first();
    const hasToast = await toast.isVisible().catch(() => false);

    // Either toast or the URL stays in input (validation failure)
    const inputValue = await mlLinkListsPage.singleUrlInput.inputValue();
    expect(hasToast || inputValue === TEST_INVALID_URL).toBeTruthy();
  });

  test('should import bulk URLs', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mockBulkCreateItemsApi(page, 'success');
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    const urls = [
      'https://www.mercadolivre.com.br/p/MLB111111',
      'https://www.mercadolivre.com.br/p/MLB222222',
      'https://www.mercadolivre.com.br/p/MLB333333',
    ];

    await mlLinkListsPage.importBulkUrls(urls);

    // Wait for API call
    await page.waitForTimeout(500);

    // Textarea should be cleared on success
    const textareaValue = await mlLinkListsPage.bulkTextarea.inputValue();
    expect(textareaValue).toBe('');
  });

  test('should handle partial bulk import (some duplicates)', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mockBulkCreateItemsApi(page, 'partial');
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    const urls = [
      'https://www.mercadolivre.com.br/p/MLB111111',
      MOCK_ML_LINK_LIST_ITEMS[0].originUrl, // Duplicate
    ];

    await mlLinkListsPage.importBulkUrls(urls);

    // Wait for API response
    await page.waitForTimeout(500);

    // Should still clear and show success (partial)
    const textareaValue = await mlLinkListsPage.bulkTextarea.inputValue();
    expect(textareaValue).toBe('');
  });
});

test.describe('ML Link Lists - Item Actions', () => {
  let mlLinkListsPage: MlLinkListsPage;

  test.beforeEach(async ({ page }) => {
    mlLinkListsPage = new MlLinkListsPage(page);
  });

  test('should toggle item active status', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mockUpdateItemApi(page, 'success');
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    // Wait for items to load
    await page.waitForTimeout(500);

    // Toggle first item
    await mlLinkListsPage.toggleItemActive(0);

    // Wait for API call
    await page.waitForTimeout(300);

    // No error should occur
    const errorToast = page.locator('[data-sonner-toast][data-type="error"]');
    const hasError = await errorToast.isVisible().catch(() => false);
    expect(hasError).toBeFalsy();
  });

  test('should reorder items (move up/down)', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mockReorderItemsApi(page, 'success');
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    // Wait for items to load
    await page.waitForTimeout(500);

    // Move second item up
    await mlLinkListsPage.moveItemUp(1);

    // Wait for API call
    await page.waitForTimeout(300);

    // No error should occur
    const errorToast = page.locator('[data-sonner-toast][data-type="error"]');
    const hasError = await errorToast.isVisible().catch(() => false);
    expect(hasError).toBeFalsy();
  });

  test('should delete item with confirmation', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mockDeleteItemApi(page, 'success');
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    // Wait for items to load
    await page.waitForTimeout(500);

    await mlLinkListsPage.requestDeleteItem(0);

    const alertDialog = page.locator('[role="alertdialog"]');
    await expect(alertDialog).toBeVisible();

    await mlLinkListsPage.confirmDelete();

    await expect(alertDialog).not.toBeVisible();
  });
});

test.describe('ML Link Lists - Reset Consumed', () => {
  let mlLinkListsPage: MlLinkListsPage;

  test.beforeEach(async ({ page }) => {
    mlLinkListsPage = new MlLinkListsPage(page);
  });

  test('should reset consumed items with confirmation', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mockResetConsumedApi(page, 'success');
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    await mlLinkListsPage.requestResetConsumed();

    const alertDialog = page.locator('[role="alertdialog"]');
    await expect(alertDialog).toBeVisible();

    await mlLinkListsPage.confirmDelete(); // Same confirm button

    await expect(alertDialog).not.toBeVisible();
  });

  test('should cancel reset action', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    await mlLinkListsPage.requestResetConsumed();

    const alertDialog = page.locator('[role="alertdialog"]');
    await expect(alertDialog).toBeVisible();

    await page
      .locator('[role="alertdialog"]')
      .getByRole('button', { name: /cancelar|cancel/i })
      .click();

    await expect(alertDialog).not.toBeVisible();
  });
});

test.describe('ML Link Lists - Stats Display', () => {
  let mlLinkListsPage: MlLinkListsPage;

  test.beforeEach(async ({ page }) => {
    mlLinkListsPage = new MlLinkListsPage(page);
  });

  test('should display correct stats badges', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    // Wait for items to load
    await page.waitForTimeout(500);

    // Available badge should be visible
    await expect(mlLinkListsPage.availableBadge).toBeVisible();

    // Sent badge should be visible
    await expect(mlLinkListsPage.sentBadge).toBeVisible();
  });

  test('should show correct available count', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    // Wait for items to load
    await page.waitForTimeout(500);

    const availableCount = await mlLinkListsPage.getAvailableCount();
    const expectedAvailable = MOCK_ML_LINK_LIST_ITEMS.filter(
      (i) => i.listId === MOCK_ML_LINK_LISTS[0].id && !i.consumedAt && i.isActive
    ).length;

    expect(availableCount).toBe(expectedAvailable);
  });
});

test.describe('ML Link Lists - Preview', () => {
  let mlLinkListsPage: MlLinkListsPage;

  test.beforeEach(async ({ page }) => {
    mlLinkListsPage = new MlLinkListsPage(page);
  });

  test('should show preview card after adding URL', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mockCreateItemApi(page, 'success');
    await mockPreviewApi(page, 'success');
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    await mlLinkListsPage.addSingleUrl(TEST_VALID_ML_URL);

    // Wait for preview to load
    await page.waitForTimeout(1000);

    const hasPreview = await mlLinkListsPage.hasPreview();
    expect(hasPreview).toBeTruthy();
  });

  test('should handle preview error gracefully', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mockCreateItemApi(page, 'success');
    await mockPreviewApi(page, 'error');
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    await mlLinkListsPage.addSingleUrl(TEST_VALID_ML_URL);

    // Wait for API calls
    await page.waitForTimeout(500);

    // Item should still be created even if preview fails
    const inputValue = await mlLinkListsPage.singleUrlInput.inputValue();
    expect(inputValue).toBe('');
  });
});

test.describe('ML Link Lists - Pagination', () => {
  let mlLinkListsPage: MlLinkListsPage;

  test.beforeEach(async ({ page }) => {
    mlLinkListsPage = new MlLinkListsPage(page);
  });

  test('should load items on page load', async ({ page }) => {
    await setupMlLinkListsMocks(page);
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    // Wait for items to load
    await page.waitForTimeout(500);

    const itemCount = await mlLinkListsPage.getItemCount();
    expect(itemCount).toBeGreaterThan(0);
  });
});

test.describe('ML Link Lists - ConnectRPC Integration', () => {
  test('should use ConnectRPC service endpoints', async ({ page }) => {
    const connectRpcCalls: string[] = [];

    page.on('request', (request) => {
      const url = request.url();
      if (url.includes('ml_links.v1.MlLinksService')) {
        connectRpcCalls.push(url);
      }
    });

    await setupMlLinkListsMocks(page);

    const mlLinkListsPage = new MlLinkListsPage(page);
    await mlLinkListsPage.goto();
    await mlLinkListsPage.waitForLoad();

    // Wait for initial load
    await page.waitForTimeout(500);

    // Should have made at least ListMlLinkLists call
    expect(connectRpcCalls.length).toBeGreaterThan(0);
    expect(connectRpcCalls.some((url) => url.includes('ListMlLinkLists'))).toBeTruthy();
  });
});
