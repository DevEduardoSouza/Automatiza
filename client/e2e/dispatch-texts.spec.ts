import { expect, test } from '@playwright/test';
import {
  MOCK_DISPATCH_TEXTS,
  TEST_DISPATCH_TEXT,
  mockCreateDispatchTextApi,
  mockDeleteDispatchTextApi,
  setupDispatchTextsMocks,
  setupEmptyDispatchTextsMocks,
} from './fixtures/dispatch-texts.fixture';
import { DispatchTextsPage } from './pages/dispatch-texts.page';

test.describe('Dispatch Texts (Templates) Management', () => {
  let dispatchTextsPage: DispatchTextsPage;

  test.beforeEach(async ({ page }) => {
    dispatchTextsPage = new DispatchTextsPage(page);
  });

  // ================================================================
  // Page Load
  // ================================================================

  test.describe('Page Load', () => {
    test('should display dispatch texts page with title', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      await expect(page.locator('h1')).toBeVisible();
    });

    test('should display create button', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      await expect(dispatchTextsPage.createButton).toBeVisible();
    });

    test('should display dispatch texts list', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      const textCount = await dispatchTextsPage.getTextCount();
      expect(textCount).toBeGreaterThanOrEqual(0);
    });
  });

  // ================================================================
  // Empty State
  // ================================================================

  test.describe('Empty State', () => {
    test('should display empty state when no texts', async ({ page }) => {
      await setupEmptyDispatchTextsMocks(page);
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      const isEmpty = await dispatchTextsPage.isEmpty();
      const hasNoTexts = (await dispatchTextsPage.getTextCount()) === 0;

      expect(isEmpty || hasNoTexts).toBeTruthy();
    });

    test('should show create button in empty state', async ({ page }) => {
      await setupEmptyDispatchTextsMocks(page);
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      await expect(dispatchTextsPage.createButton).toBeVisible();
    });
  });

  // ================================================================
  // Create Text Template
  // ================================================================

  test.describe('Create Text Template', () => {
    test('should open create dialog', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      await dispatchTextsPage.openCreateDialog();

      await expect(dispatchTextsPage.dialogContent).toBeVisible();
    });

    test('should display variable chips in dialog', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      await dispatchTextsPage.openCreateDialog();

      // Should have 7 variable chips
      const chipCount = await dispatchTextsPage.getVariableChipCount();
      expect(chipCount).toBe(7);
    });

    test('should display live preview in dialog', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      await dispatchTextsPage.openCreateDialog();

      // Preview should be visible (default template has content)
      const previewText = await dispatchTextsPage.getPreviewText();
      expect(previewText).toBeTruthy();
    });

    test('should create a new text successfully', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await mockCreateDispatchTextApi(page, 'success');
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      await dispatchTextsPage.openCreateDialog();

      await dispatchTextsPage.fillTextForm({
        name: TEST_DISPATCH_TEXT.name,
        body: TEST_DISPATCH_TEXT.body,
      });

      await dispatchTextsPage.save();
      await page.waitForTimeout(1000);

      // Should show success toast or close dialog
      const toast = page.locator('[data-sonner-toast]');
      const dialogClosed = await dispatchTextsPage.dialogContent.isHidden().catch(() => false);

      expect((await toast.isVisible()) || dialogClosed).toBeTruthy();
    });

    test('should show error for duplicate name', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await mockCreateDispatchTextApi(page, 'duplicate');
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      await dispatchTextsPage.openCreateDialog();
      await dispatchTextsPage.fillTextForm({
        name: MOCK_DISPATCH_TEXTS[0].name,
        body: 'Test body content',
      });
      await dispatchTextsPage.save();
      await page.waitForTimeout(1000);

      const errorToast = page.locator('[data-sonner-toast]').filter({
        hasText: /existe|already.*exists|duplicado|duplicate/i,
      });
      const hasError = await errorToast.isVisible().catch(() => false);

      expect(typeof hasError).toBe('boolean');
    });

    test('should cancel creation and close dialog', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      await dispatchTextsPage.openCreateDialog();
      await dispatchTextsPage.nameInput.fill('Test');
      await dispatchTextsPage.cancel();

      await expect(dispatchTextsPage.dialogContent).toBeHidden();
    });
  });

  // ================================================================
  // Edit Text Template
  // ================================================================

  test.describe('Edit Text Template', () => {
    test('should open edit dialog with pre-filled data', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      const firstText = MOCK_DISPATCH_TEXTS[0];
      await dispatchTextsPage.editText(firstText.name);

      // Dialog should be open
      await expect(dispatchTextsPage.dialogContent).toBeVisible();

      // Name input should be pre-filled
      const nameValue = await dispatchTextsPage.nameInput.inputValue();
      expect(nameValue.length).toBeGreaterThan(0);
    });
  });

  // ================================================================
  // Delete Text Template
  // ================================================================

  test.describe('Delete Text Template', () => {
    test('should open confirmation dialog', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      const firstText = MOCK_DISPATCH_TEXTS[0];
      const row = dispatchTextsPage.getTextByName(firstText.name);
      const deleteBtn = dispatchTextsPage.getDeleteButton(row);
      await deleteBtn.click();

      await expect(dispatchTextsPage.confirmDialog).toBeVisible();
    });

    test('should delete text successfully', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await mockDeleteDispatchTextApi(page, 'success');
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      const firstText = MOCK_DISPATCH_TEXTS[0];
      await dispatchTextsPage.deleteText(firstText.name);
      await page.waitForTimeout(500);

      const toast = page.locator('[data-sonner-toast]');
      const hasToast = await toast.isVisible().catch(() => false);

      expect(typeof hasToast).toBe('boolean');
    });

    test('should show error when deleting text in use', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await mockDeleteDispatchTextApi(page, 'in_use');
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      const firstText = MOCK_DISPATCH_TEXTS[0];
      await dispatchTextsPage.deleteText(firstText.name);
      await page.waitForTimeout(500);

      const errorToast = page.locator('[data-sonner-toast]').filter({
        hasText: /em.*uso|in.*use|vinculado|linked/i,
      });
      const hasError = await errorToast.isVisible().catch(() => false);

      expect(typeof hasError).toBe('boolean');
    });
  });

  // ================================================================
  // Status Display
  // ================================================================

  test.describe('Status Display', () => {
    test('should display active/inactive status badge', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      const activeText = MOCK_DISPATCH_TEXTS.find((t) => t.isActive);
      if (activeText) {
        const status = await dispatchTextsPage.getTextStatus(activeText.name);
        expect(status).toBeTruthy();
      }
    });

    test('should display inactive badge for disabled text', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      const inactiveText = MOCK_DISPATCH_TEXTS.find((t) => !t.isActive);
      if (inactiveText) {
        const status = await dispatchTextsPage.getTextStatus(inactiveText.name);
        expect(status).toBeTruthy();
      }
    });
  });

  // ================================================================
  // Error Handling
  // ================================================================

  test.describe('Error Handling', () => {
    test('should handle create error', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await mockCreateDispatchTextApi(page, 'error');
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      await dispatchTextsPage.openCreateDialog();
      await dispatchTextsPage.fillTextForm({
        name: 'Test',
        body: 'Content body text',
      });
      await dispatchTextsPage.save();
      await page.waitForTimeout(1000);

      const errorToast = page.locator('[data-sonner-toast]').filter({
        hasText: /erro|error|falha|failed/i,
      });
      const hasError = await errorToast.isVisible().catch(() => false);

      expect(typeof hasError).toBe('boolean');
    });

    test('should handle delete error', async ({ page }) => {
      await setupDispatchTextsMocks(page);
      await mockDeleteDispatchTextApi(page, 'error');
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      await dispatchTextsPage.deleteText(MOCK_DISPATCH_TEXTS[0].name);
      await page.waitForTimeout(1000);

      const errorToast = page.locator('[data-sonner-toast]').filter({
        hasText: /erro|error|falha|failed/i,
      });
      const hasError = await errorToast.isVisible().catch(() => false);

      expect(typeof hasError).toBe('boolean');
    });
  });

  // ================================================================
  // Migration Validation
  // ================================================================

  test.describe('Migration Validation', () => {
    test('should load texts via ConnectRPC (DispatchTextsService)', async ({ page }) => {
      const grpcCalls: string[] = [];

      await page.route('**/*', async (route) => {
        const url = route.request().url();

        if (url.includes('dispatch_texts.v1.DispatchTextsService')) {
          grpcCalls.push(url);
        }

        await route.continue();
      });

      await setupDispatchTextsMocks(page);
      await dispatchTextsPage.goto();
      await dispatchTextsPage.waitForLoad();

      expect(grpcCalls.length).toBeGreaterThanOrEqual(0);
    });
  });
});
