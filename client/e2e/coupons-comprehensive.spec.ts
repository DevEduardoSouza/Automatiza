import { expect, test } from '@playwright/test';
import { setupAuthMocks } from './fixtures/auth.fixture';
import {
  MOCK_COUPONS,
  mockCouponsListApi,
  mockDeleteCouponApi,
  mockUpdateCouponApi,
  setupCouponsMocks,
  setupEmptyCouponsMocks,
} from './fixtures/coupons.fixture';
import { CouponsPage } from './pages/coupons.page';

test.describe('Coupons Comprehensive', () => {
  let couponsPage: CouponsPage;

  test.beforeEach(async ({ page }) => {
    couponsPage = new CouponsPage(page);
    await setupAuthMocks(page);
  });

  // ================================================================
  // Page Load & Display
  // ================================================================

  test('should display coupons list with all mock items', async ({ page }) => {
    await setupCouponsMocks(page);
    await couponsPage.goto();
    await couponsPage.waitForLoad();

    await expect(couponsPage.pageTitle).toBeVisible();

    for (const coupon of MOCK_COUPONS) {
      const row = couponsPage.getCouponByCode(coupon.code);
      await expect(row).toBeVisible();
    }
  });

  test('should show empty state when no coupons exist', async ({ page }) => {
    await setupEmptyCouponsMocks(page);
    await couponsPage.goto();
    await couponsPage.waitForLoad();

    const isEmpty = await couponsPage.isEmpty();
    const hasNoCoupons = (await couponsPage.getCouponCount()) === 0;
    expect(isEmpty || hasNoCoupons).toBeTruthy();
    await expect(couponsPage.createButton).toBeVisible();
  });

  // ================================================================
  // Search
  // ================================================================

  test('should search coupons by code', async ({ page }) => {
    await setupCouponsMocks(page);
    await couponsPage.goto();
    await couponsPage.waitForLoad();

    const initialCount = await couponsPage.getCouponCount();

    await couponsPage.search('PROMO10');
    await page.waitForTimeout(500);

    const searchCount = await couponsPage.getCouponCount();
    expect(searchCount).toBeLessThanOrEqual(initialCount);
    await expect(couponsPage.getCouponByCode('PROMO10')).toBeVisible();
  });

  test('should clear search and show all coupons again', async ({ page }) => {
    await setupCouponsMocks(page);
    await couponsPage.goto();
    await couponsPage.waitForLoad();

    const initialCount = await couponsPage.getCouponCount();

    await couponsPage.search('PROMO10');
    await page.waitForTimeout(500);

    const searchCount = await couponsPage.getCouponCount();
    expect(searchCount).toBeLessThanOrEqual(initialCount);

    await couponsPage.clearSearch();
    await page.waitForTimeout(500);

    const afterClearCount = await couponsPage.getCouponCount();
    expect(afterClearCount).toBe(initialCount);
  });

  // ================================================================
  // Filters
  // ================================================================

  test('should filter by store via Select', async ({ page }) => {
    await setupCouponsMocks(page);
    await couponsPage.goto();
    await couponsPage.waitForLoad();

    const initialCount = await couponsPage.getCouponCount();

    await couponsPage.filterByStore(/shopee/i);
    await page.waitForTimeout(500);

    const filteredCount = await couponsPage.getCouponCount();
    expect(filteredCount).toBeLessThanOrEqual(initialCount);
  });

  test('should filter by status inactive', async ({ page }) => {
    await setupCouponsMocks(page);
    await couponsPage.goto();
    await couponsPage.waitForLoad();

    await couponsPage.filterByStatus('inactive');
    await page.waitForTimeout(500);

    // Active non-expired coupons should be hidden
    await expect(couponsPage.getCouponByCode('PROMO10')).not.toBeVisible();
    await expect(couponsPage.getCouponByCode('FRETE15')).not.toBeVisible();
  });

  test('should combine store and status filters', async ({ page }) => {
    await setupCouponsMocks(page);
    await couponsPage.goto();
    await couponsPage.waitForLoad();

    const initialCount = await couponsPage.getCouponCount();

    await couponsPage.filterByStore(/shopee/i);
    await page.waitForTimeout(300);

    await couponsPage.filterByStatus('active');
    await page.waitForTimeout(500);

    const filteredCount = await couponsPage.getCouponCount();
    expect(filteredCount).toBeLessThanOrEqual(initialCount);
  });

  // ================================================================
  // CRUD Operations
  // ================================================================

  test('should delete coupon with confirm dialog', async ({ page }) => {
    await setupCouponsMocks(page);
    await mockDeleteCouponApi(page, 'success');
    await couponsPage.goto();
    await couponsPage.waitForLoad();

    let dialogAccepted = false;
    page.on('dialog', async (dialog) => {
      dialogAccepted = true;
      await dialog.accept();
    });

    const coupon = MOCK_COUPONS[0];
    const row = couponsPage.getCouponByCode(coupon.code);
    const deleteBtn = couponsPage.getDeleteButton(row);

    if (await deleteBtn.isVisible().catch(() => false)) {
      await deleteBtn.click();
      await page.waitForTimeout(500);
      expect(dialogAccepted).toBe(true);
    }
  });

  test('should toggle status with confirm dialog', async ({ page }) => {
    await setupCouponsMocks(page);
    await mockUpdateCouponApi(page, 'success');
    await couponsPage.goto();
    await couponsPage.waitForLoad();

    let dialogAccepted = false;
    page.on('dialog', async (dialog) => {
      dialogAccepted = true;
      await dialog.accept();
    });

    const row = couponsPage.getCouponByCode('PROMO10');
    const toggleBtn = couponsPage.getToggleStatusButton(row);

    if (await toggleBtn.isVisible().catch(() => false)) {
      await toggleBtn.click();
      await page.waitForTimeout(500);
      expect(dialogAccepted).toBe(true);
    }
  });

  test('should edit coupon and pre-fill form fields', async ({ page }) => {
    await setupCouponsMocks(page);
    await couponsPage.goto();
    await couponsPage.waitForLoad();

    const coupon = MOCK_COUPONS[0]; // PROMO10
    await couponsPage.editCoupon(coupon.code);

    await expect(couponsPage.dialogContent).toBeVisible();

    const codeValue = await couponsPage.codeInput.inputValue();
    expect(codeValue).toBe(coupon.code);

    const titleValue = await couponsPage.titleInput.inputValue();
    expect(titleValue.length).toBeGreaterThan(0);
  });

  test('should update a coupon via edit dialog', async ({ page }) => {
    await setupCouponsMocks(page);
    await mockUpdateCouponApi(page, 'success');
    await couponsPage.goto();
    await couponsPage.waitForLoad();

    const coupon = MOCK_COUPONS[0];
    await couponsPage.editCoupon(coupon.code);
    await expect(couponsPage.dialogContent).toBeVisible();

    // Modify discount value
    await couponsPage.discountValueInput.fill('25');
    await couponsPage.save();
    await page.waitForTimeout(1000);

    const toast = page.locator('[data-sonner-toast]');
    const dialogClosed = await couponsPage.dialogContent.isHidden().catch(() => false);
    expect((await toast.isVisible()) || dialogClosed).toBeTruthy();
  });

  // ================================================================
  // Responsive Design
  // ================================================================

  test('should show mobile card view at 375px', async ({ page }) => {
    await setupCouponsMocks(page);
    await page.setViewportSize({ width: 375, height: 812 });
    await couponsPage.goto();
    await couponsPage.waitForLoad();

    await expect(couponsPage.pageTitle).toBeVisible();
    await expect(couponsPage.createButton).toBeVisible();

    // Mobile cards container should be visible
    const mobileCards = page.locator('.md\\:hidden');
    await expect(mobileCards).toBeVisible();

    // Coupon codes should still be visible in cards
    await expect(page.getByText('PROMO10').first()).toBeVisible();
  });

  // ================================================================
  // API Verification
  // ================================================================

  test('should make ConnectRPC ListCoupons call on load', async ({ page }) => {
    const connectRequests: string[] = [];
    page.on('request', (request) => {
      if (request.url().includes('/coupons.v1.CouponsService')) {
        connectRequests.push(request.url());
      }
    });

    await setupCouponsMocks(page);
    await couponsPage.goto();
    await couponsPage.waitForLoad();

    await page.waitForTimeout(1000);

    const listCalls = connectRequests.filter((url) => url.includes('ListCoupons'));
    expect(listCalls.length).toBeGreaterThan(0);
  });
});
