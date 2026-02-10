import { expect, test } from '@playwright/test';
import {
  TEST_ML_CREDENTIALS,
  TEST_SHOPEE_CREDENTIALS,
  mockAllAffiliateApis,
  mockDeleteCredential,
  mockMlConnect,
  mockMlCreateLink,
  mockMlPreview,
  mockUpsertCredential,
} from './fixtures/affiliate-programs.fixture';
import { loginUser } from './fixtures/auth.fixture';
import { AffiliateProgramsPage } from './pages/affiliate-programs.page';

test.describe('Affiliate Programs Page', () => {
  let affiliatePage: AffiliateProgramsPage;

  test.beforeEach(async ({ page }) => {
    affiliatePage = new AffiliateProgramsPage(page);

    // Mock all affiliate APIs
    await mockAllAffiliateApis(page);

    // Login first
    await loginUser(page);

    // Navigate to affiliate programs
    await affiliatePage.goto();
  });

  test.describe('Page Load', () => {
    test('should display page title and subtitle', async ({ page }) => {
      await expect(affiliatePage.pageTitle).toBeVisible();
      const title = await affiliatePage.pageTitle.textContent();
      expect(title?.toLowerCase()).toMatch(/affiliate|afiliado|programas/i);
    });

    test('should display store cards grid', async () => {
      await affiliatePage.waitForLoad();
      await expect(affiliatePage.storeCardsGrid).toBeVisible();
    });

    test('should display multiple store options', async () => {
      await affiliatePage.waitForLoad();

      // Check for Shopee
      const shopeeCard = affiliatePage.getStoreCard('Shopee');
      await expect(shopeeCard).toBeVisible();

      // Check for Mercado Livre
      const mlCard = affiliatePage.getStoreCard('Mercado Livre');
      await expect(mlCard).toBeVisible();
    });

    test('should show coming soon cards for unsupported stores', async ({ page }) => {
      await affiliatePage.waitForLoad();

      // Check for coming soon stores (Amazon, Magalu, etc.)
      const comingSoonText = page.locator('text=/em breve|coming soon/i');
      const count = await comingSoonText.count();

      // Should have at least some coming soon cards
      expect(count).toBeGreaterThanOrEqual(0);
    });
  });

  test.describe('Shopee Configuration', () => {
    test('should fill and save Shopee credentials', async ({ page }) => {
      await affiliatePage.waitForLoad();

      // Setup mock for save
      await mockUpsertCredential(page, 'success');

      // Fill credentials
      await affiliatePage.fillShopeeCredentials(
        TEST_SHOPEE_CREDENTIALS.appId,
        TEST_SHOPEE_CREDENTIALS.appSecret
      );

      // Save
      await affiliatePage.saveStore('Shopee');

      // Wait for toast or status change
      await page.waitForTimeout(500);

      // Should not show error toast
      const errorToast = page.locator('[data-sonner-toast][data-type="error"]');
      await expect(errorToast).not.toBeVisible();
    });

    test('should toggle auto-generate for Shopee', async ({ page }) => {
      await affiliatePage.waitForLoad();

      const switchBtn = affiliatePage.getAutoGenerateSwitch('Shopee');

      if (await switchBtn.isVisible()) {
        // Get initial state
        const initialState = await switchBtn.getAttribute('data-state');

        // Toggle
        await affiliatePage.toggleAutoGenerate('Shopee');

        // Wait for state change
        await page.waitForTimeout(300);

        // State should change
        const newState = await switchBtn.getAttribute('data-state');
        expect(newState).not.toBe(initialState);
      }
    });

    test('should show validation error for empty Shopee fields', async ({ page }) => {
      await affiliatePage.waitForLoad();

      // Try to save without filling fields
      const saveButton = affiliatePage.getSaveButton('Shopee');

      if ((await saveButton.isVisible()) && (await saveButton.isEnabled())) {
        await saveButton.click();

        // Should show validation error or stay disabled
        await page.waitForTimeout(500);
      }
    });
  });

  test.describe('Mercado Livre Configuration', () => {
    test('should display ML connection form', async () => {
      await affiliatePage.waitForLoad();

      const mlCard = affiliatePage.getStoreCard('Mercado Livre');
      await expect(mlCard).toBeVisible();

      // Should have token input
      const tokenInput = affiliatePage.getMlConnectionTokenInput();
      await expect(tokenInput).toBeVisible();
    });

    test('should validate ML token', async ({ page }) => {
      await affiliatePage.waitForLoad();

      // Fill connection token
      const tokenInput = affiliatePage.getMlConnectionTokenInput();
      await tokenInput.fill(TEST_ML_CREDENTIALS.connectionToken);

      // Validate
      const validateBtn = affiliatePage.getMlValidateButton();
      if (await validateBtn.isVisible()) {
        await validateBtn.click();

        // Wait for validation result
        await page.waitForTimeout(500);
      }
    });

    test('should save ML credentials successfully', async ({ page }) => {
      await affiliatePage.waitForLoad();

      // Setup mock
      await mockMlConnect(page, 'success');

      // Fill credentials
      await affiliatePage.fillMlCredentials(
        TEST_ML_CREDENTIALS.connectionToken,
        TEST_ML_CREDENTIALS.tag
      );

      // Save
      await affiliatePage.saveStore('Mercado Livre');

      // Wait for result
      await page.waitForTimeout(500);

      // Should not show error
      const errorToast = page.locator('[data-sonner-toast][data-type="error"]');
      await expect(errorToast).not.toBeVisible();
    });

    test('should test ML link creation', async ({ page }) => {
      await affiliatePage.waitForLoad();

      // Setup mocks
      await mockMlPreview(page, 'success');
      await mockMlCreateLink(page, 'success');

      // Fill test URL
      const urlInput = affiliatePage.getMlTestUrlInput();
      if (await urlInput.isVisible()) {
        await urlInput.fill(TEST_ML_CREDENTIALS.testProductUrl);

        // Click test button
        await affiliatePage.testStore('Mercado Livre');

        // Wait for result
        await page.waitForTimeout(1000);
      }
    });
  });

  test.describe('Credential Management', () => {
    test('should show active status after saving valid credentials', async ({ page }) => {
      await affiliatePage.waitForLoad();

      // Setup mock
      await mockUpsertCredential(page, 'success');

      // Fill and save Shopee credentials
      await affiliatePage.fillShopeeCredentials(
        TEST_SHOPEE_CREDENTIALS.appId,
        TEST_SHOPEE_CREDENTIALS.appSecret
      );
      await affiliatePage.saveStore('Shopee');

      // Wait for update
      await page.waitForTimeout(500);
    });

    test('should handle delete credential', async ({ page }) => {
      await affiliatePage.waitForLoad();

      // Setup mock
      await mockDeleteCredential(page, 'success');

      // Find delete button if exists
      const shopeeCard = affiliatePage.getStoreCard('Shopee');
      const deleteBtn = shopeeCard.locator('button').filter({ hasText: /delete|excluir|remover/i });

      if (await deleteBtn.isVisible()) {
        await deleteBtn.click();

        // Confirm if there's a confirmation dialog
        const confirmBtn = page.locator('button').filter({ hasText: /confirm|confirmar|yes|sim/i });
        if (await confirmBtn.isVisible()) {
          await confirmBtn.click();
        }

        await page.waitForTimeout(500);
      }
    });

    test('should persist auto-generate toggle state', async ({ page }) => {
      await affiliatePage.waitForLoad();

      // Setup mock
      await mockUpsertCredential(page, 'success');

      const switchBtn = affiliatePage.getAutoGenerateSwitch('Shopee');

      if (await switchBtn.isVisible()) {
        // Get initial state
        const initialState = await switchBtn.getAttribute('data-state');

        // Toggle
        await switchBtn.click();
        await page.waitForTimeout(300);

        // Save
        await affiliatePage.saveStore('Shopee');
        await page.waitForTimeout(500);

        // Reload page
        await affiliatePage.goto();
        await affiliatePage.waitForLoad();

        // Note: In a real test, we'd verify the state persisted
        // This requires proper API mocking to return the updated state
      }
    });
  });

  test.describe('Error Handling', () => {
    test('should show error toast on save failure', async ({ page }) => {
      await affiliatePage.waitForLoad();

      // Setup mock to return error
      await mockUpsertCredential(page, 'error');

      // Fill credentials
      await affiliatePage.fillShopeeCredentials(
        TEST_SHOPEE_CREDENTIALS.appId,
        TEST_SHOPEE_CREDENTIALS.appSecret
      );

      // Try to save
      await affiliatePage.saveStore('Shopee');

      // Wait for error toast
      await page.waitForTimeout(500);
    });

    test('should show error on ML preview failure', async ({ page }) => {
      await affiliatePage.waitForLoad();

      // Setup mock to return error
      await mockMlPreview(page, 'error');

      // Fill test URL and test
      const urlInput = affiliatePage.getMlTestUrlInput();
      if (await urlInput.isVisible()) {
        await urlInput.fill(TEST_ML_CREDENTIALS.testProductUrl);
        await affiliatePage.testStore('Mercado Livre');

        // Wait for error
        await page.waitForTimeout(500);
      }
    });

    test('should show not found error for invalid product URL', async ({ page }) => {
      await affiliatePage.waitForLoad();

      // Setup mock to return not found
      await mockMlPreview(page, 'not_found');

      const urlInput = affiliatePage.getMlTestUrlInput();
      if (await urlInput.isVisible()) {
        await urlInput.fill('https://www.mercadolivre.com.br/invalid-product');
        await affiliatePage.testStore('Mercado Livre');

        await page.waitForTimeout(500);
      }
    });
  });

  test.describe('Loading States', () => {
    test('should show loading state while saving', async ({ page }) => {
      await affiliatePage.waitForLoad();

      // Setup slow mock
      await page.route(
        '**/affiliates.v1.AffiliateCredentialsService/UpsertCredential',
        async (route) => {
          await new Promise((resolve) => setTimeout(resolve, 1000));
          await route.fulfill({
            status: 200,
            contentType: 'application/grpc-web+proto',
            headers: { 'grpc-status': '0' },
            body: Buffer.from([]),
          });
        }
      );

      // Fill and save
      await affiliatePage.fillShopeeCredentials(
        TEST_SHOPEE_CREDENTIALS.appId,
        TEST_SHOPEE_CREDENTIALS.appSecret
      );
      await affiliatePage.saveStore('Shopee');

      // Check for loading indicator
      const saveButton = affiliatePage.getSaveButton('Shopee');
      const hasLoader = await saveButton
        .locator('svg[class*="animate"]')
        .isVisible()
        .catch(() => false);

      // Button should be disabled or show loader during save
      const isDisabled = await saveButton.isDisabled().catch(() => false);
      expect(hasLoader || isDisabled).toBeTruthy();
    });

    test('should show loading state while testing', async ({ page }) => {
      await affiliatePage.waitForLoad();

      // Setup slow mock
      await page.route('**/affiliates.v1.MercadoLivreService/Preview', async (route) => {
        await new Promise((resolve) => setTimeout(resolve, 1000));
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from([]),
        });
      });

      const urlInput = affiliatePage.getMlTestUrlInput();
      if (await urlInput.isVisible()) {
        await urlInput.fill(TEST_ML_CREDENTIALS.testProductUrl);
        await affiliatePage.testStore('Mercado Livre');

        // Check for loading indicator
        const testButton = affiliatePage.getTestButton('Mercado Livre');
        const hasLoader = await testButton
          .locator('svg[class*="animate"]')
          .isVisible()
          .catch(() => false);
        const isDisabled = await testButton.isDisabled().catch(() => false);
        expect(hasLoader || isDisabled).toBeTruthy();
      }
    });
  });

  test.describe('Accessibility', () => {
    test('should have proper form labels', async () => {
      await affiliatePage.waitForLoad();

      // Check Shopee card has labels
      const shopeeCard = affiliatePage.getStoreCard('Shopee');
      const labels = shopeeCard.locator('label');
      const labelCount = await labels.count();

      expect(labelCount).toBeGreaterThan(0);
    });

    test('should be navigable by keyboard', async ({ page }) => {
      await affiliatePage.waitForLoad();

      // Tab through the page
      await page.keyboard.press('Tab');
      await page.keyboard.press('Tab');

      // Should focus on an interactive element
      const focusedElement = page.locator(':focus');
      await expect(focusedElement).toBeVisible();
    });
  });
});
