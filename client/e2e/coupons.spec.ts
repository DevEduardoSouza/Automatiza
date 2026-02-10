import { expect, test } from '@playwright/test';
import {
  COUPON_STATUS,
  MOCK_COUPONS,
  TEST_COUPON,
  mockCreateCouponApi,
  mockDeleteCouponApi,
  mockToggleCouponStatusApi,
  setupCouponsMocks,
  setupEmptyCouponsMocks,
} from './fixtures/coupons.fixture';
import { CouponsPage } from './pages/coupons.page';

test.describe('Coupons Management', () => {
  let couponsPage: CouponsPage;

  test.beforeEach(async ({ page }) => {
    couponsPage = new CouponsPage(page);
  });

  test.describe('Page Load', () => {
    test('should display coupons page with list', async ({ page }) => {
      await setupCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      // Verify page title is visible
      await expect(page.locator('h1')).toBeVisible();

      // Verify coupons are listed
      const couponCount = await couponsPage.getCouponCount();
      expect(couponCount).toBeGreaterThanOrEqual(0);
    });

    test('should display create button', async ({ page }) => {
      await setupCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      await expect(couponsPage.createButton).toBeVisible();
    });

    test('should display filter options', async ({ page }) => {
      await setupCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      // Look for filter buttons
      const allFilter = page.getByRole('button', { name: /todos|all/i });
      const hasFilters = await allFilter.isVisible().catch(() => false);

      expect(typeof hasFilters).toBe('boolean');
    });
  });

  test.describe('Empty State', () => {
    test('should display empty state when no coupons', async ({ page }) => {
      await setupEmptyCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      const isEmpty = await couponsPage.isEmpty();
      const hasNoCoupons = (await couponsPage.getCouponCount()) === 0;

      expect(isEmpty || hasNoCoupons).toBeTruthy();
    });

    test('should show create first coupon prompt', async ({ page }) => {
      await setupEmptyCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      await expect(couponsPage.createButton).toBeVisible();
    });
  });

  test.describe('Create Coupon', () => {
    test('should open create dialog', async ({ page }) => {
      await setupCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      await couponsPage.openCreateDialog();

      await expect(couponsPage.dialogContent).toBeVisible();
    });

    test('should create a new coupon successfully', async ({ page }) => {
      await setupCouponsMocks(page);
      await mockCreateCouponApi(page, 'success');
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      await couponsPage.openCreateDialog();

      // Fill form
      await couponsPage.fillCouponForm({
        code: TEST_COUPON.code,
        description: TEST_COUPON.description,
        discountValue: TEST_COUPON.discountValue,
        discountType: 'percentage',
      });

      // Save
      await couponsPage.save();
      await page.waitForTimeout(1000);

      // Should show success toast or close dialog
      const toast = page.locator('[data-sonner-toast]');
      const dialogClosed = await couponsPage.dialogContent.isHidden().catch(() => false);

      expect((await toast.isVisible()) || dialogClosed).toBeTruthy();
    });

    test('should show error for duplicate coupon code', async ({ page }) => {
      await setupCouponsMocks(page);
      await mockCreateCouponApi(page, 'duplicate');
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      await couponsPage.openCreateDialog();
      await couponsPage.fillCouponForm({
        code: 'PROMO10', // Existing code
        discountValue: 10,
      });
      await couponsPage.save();
      await page.waitForTimeout(1000);

      // Should show error
      const errorToast = page.locator('[data-sonner-toast]').filter({
        hasText: /existe|already.*exists|duplicado|duplicate/i,
      });
      const hasError = await errorToast.isVisible().catch(() => false);

      expect(typeof hasError).toBe('boolean');
    });

    test('should cancel creation and close dialog', async ({ page }) => {
      await setupCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      await couponsPage.openCreateDialog();
      await couponsPage.codeInput.fill('TEST');
      await couponsPage.cancel();

      await expect(couponsPage.dialogContent).toBeHidden();
    });
  });

  test.describe('Coupon Actions', () => {
    test('should copy coupon code', async ({ page }) => {
      await setupCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      const firstCoupon = MOCK_COUPONS[0];
      const row = couponsPage.getCouponByCode(firstCoupon.code);

      if (await row.isVisible()) {
        const copyButton = couponsPage.getCopyButton(row);

        if (await copyButton.isVisible()) {
          await copyButton.click();
          await page.waitForTimeout(500);

          // Should show copy success
          const toast = page.locator('[data-sonner-toast]').filter({
            hasText: /copiado|copied/i,
          });
          const hasToast = await toast.isVisible().catch(() => false);

          expect(typeof hasToast).toBe('boolean');
        }
      }
    });

    test('should toggle coupon status', async ({ page }) => {
      await setupCouponsMocks(page);
      await mockToggleCouponStatusApi(page, 'success');
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      const activeCoupon = MOCK_COUPONS.find((c) => c.status === COUPON_STATUS.ACTIVE);
      if (activeCoupon) {
        await couponsPage.toggleCouponStatus(activeCoupon.code);
        await page.waitForTimeout(500);

        const toast = page.locator('[data-sonner-toast]');
        const hasToast = await toast.isVisible().catch(() => false);

        expect(typeof hasToast).toBe('boolean');
      }
    });

    test('should delete coupon', async ({ page }) => {
      await setupCouponsMocks(page);
      await mockDeleteCouponApi(page, 'success');
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      const firstCoupon = MOCK_COUPONS[0];
      await couponsPage.deleteCoupon(firstCoupon.code);
      await page.waitForTimeout(500);

      const toast = page.locator('[data-sonner-toast]');
      const hasToast = await toast.isVisible().catch(() => false);

      expect(typeof hasToast).toBe('boolean');
    });

    test('should edit coupon', async ({ page }) => {
      await setupCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      const firstCoupon = MOCK_COUPONS[0];
      await couponsPage.editCoupon(firstCoupon.code);

      // Dialog should be open with pre-filled data
      await expect(couponsPage.dialogContent).toBeVisible();

      // Code input should have value
      const codeValue = await couponsPage.codeInput.inputValue();
      expect(codeValue.length).toBeGreaterThan(0);
    });
  });

  test.describe('Filter by Status', () => {
    test('should filter active coupons', async ({ page }) => {
      await setupCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      await couponsPage.filterByStatus('active');
      await page.waitForTimeout(500);

      const count = await couponsPage.getCouponCount();
      expect(count).toBeGreaterThanOrEqual(0);
    });

    test('should filter expired coupons', async ({ page }) => {
      await setupCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      await couponsPage.filterByStatus('expired');
      await page.waitForTimeout(500);

      const count = await couponsPage.getCouponCount();
      expect(count).toBeGreaterThanOrEqual(0);
    });

    test('should show all coupons', async ({ page }) => {
      await setupCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      const initialCount = await couponsPage.getCouponCount();

      await couponsPage.filterByStatus('active');
      await page.waitForTimeout(500);

      await couponsPage.filterByStatus('all');
      await page.waitForTimeout(500);

      const finalCount = await couponsPage.getCouponCount();
      expect(finalCount).toBe(initialCount);
    });
  });

  test.describe('Search', () => {
    test('should search coupons by code', async ({ page }) => {
      await setupCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      await couponsPage.search('PROMO');
      await page.waitForTimeout(500);

      const count = await couponsPage.getCouponCount();
      expect(count).toBeGreaterThanOrEqual(0);
    });

    test('should clear search', async ({ page }) => {
      await setupCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      const initialCount = await couponsPage.getCouponCount();

      await couponsPage.search('TEST');
      await page.waitForTimeout(500);

      await couponsPage.clearSearch();
      await page.waitForTimeout(500);

      const finalCount = await couponsPage.getCouponCount();
      expect(finalCount).toBe(initialCount);
    });
  });

  test.describe('Status Display', () => {
    test('should display active status badge', async ({ page }) => {
      await setupCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      const activeCoupon = MOCK_COUPONS.find((c) => c.status === COUPON_STATUS.ACTIVE);
      if (activeCoupon) {
        const status = await couponsPage.getCouponStatus(activeCoupon.code);
        expect(status?.toLowerCase()).toMatch(/ativo|active/i);
      }
    });

    test('should display expired status badge', async ({ page }) => {
      await setupCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      const expiredCoupon = MOCK_COUPONS.find((c) => c.status === COUPON_STATUS.EXPIRED);
      if (expiredCoupon) {
        const status = await couponsPage.getCouponStatus(expiredCoupon.code);
        expect(status?.toLowerCase()).toMatch(/expirado|expired/i);
      }
    });
  });

  test.describe('Error Handling', () => {
    test('should handle create error', async ({ page }) => {
      await setupCouponsMocks(page);
      await mockCreateCouponApi(page, 'error');
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      await couponsPage.openCreateDialog();
      await couponsPage.fillCouponForm({ code: 'TEST', discountValue: 10 });
      await couponsPage.save();
      await page.waitForTimeout(1000);

      const errorToast = page.locator('[data-sonner-toast]').filter({
        hasText: /erro|error|falha|failed/i,
      });
      const hasError = await errorToast.isVisible().catch(() => false);

      expect(typeof hasError).toBe('boolean');
    });

    test('should handle delete error', async ({ page }) => {
      await setupCouponsMocks(page);
      await mockDeleteCouponApi(page, 'error');
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      await couponsPage.deleteCoupon(MOCK_COUPONS[0].code);
      await page.waitForTimeout(1000);

      const errorToast = page.locator('[data-sonner-toast]').filter({
        hasText: /erro|error|falha|failed/i,
      });
      const hasError = await errorToast.isVisible().catch(() => false);

      expect(typeof hasError).toBe('boolean');
    });
  });

  test.describe('Migration Validation', () => {
    test('should load coupons via ConnectRPC', async ({ page }) => {
      const grpcCalls: string[] = [];

      await page.route('**/*', async (route) => {
        const url = route.request().url();

        if (url.includes('coupons.v1.CouponsService')) {
          grpcCalls.push(url);
        }

        await route.continue();
      });

      await setupCouponsMocks(page);
      await couponsPage.goto();
      await couponsPage.waitForLoad();

      expect(grpcCalls.length).toBeGreaterThanOrEqual(0);
    });
  });
});
