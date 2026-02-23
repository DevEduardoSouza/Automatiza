import type { Page } from '@playwright/test';

/**
 * Mock coupons data matching proto Coupon message fields.
 * Uses camelCase field names as ConnectRPC JSON serialization.
 */
export const MOCK_COUPONS = [
  {
    id: 'coupon-1',
    userId: 'user-123',
    code: 'PROMO10',
    title: '10% off em compras acima de R$100',
    store: 1, // Store.SHOPEE
    isActive: true,
    expiresAt: new Date(Date.now() + 7 * 24 * 3600000).toISOString(),
    discountType: 1, // DiscountType.FIXED
    discountValue: 10,
    maxDiscount: 50,
    minProductPrice: 100,
    affiliateLink: 'https://shopee.com.br/promo',
    category: 'generalOffers',
    couponText: 'Use este cupom!',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  },
  {
    id: 'coupon-2',
    userId: 'user-123',
    code: 'FRETE15',
    title: 'R$15 de desconto no frete',
    store: 4, // Store.MERCADO_LIVRE
    isActive: true,
    expiresAt: new Date(Date.now() + 3 * 24 * 3600000).toISOString(),
    discountType: 1, // FIXED
    discountValue: 15,
    createdAt: new Date(Date.now() - 86400000).toISOString(),
    updatedAt: new Date(Date.now() - 86400000).toISOString(),
  },
  {
    id: 'coupon-3',
    userId: 'user-123',
    code: 'OFERTAEXP',
    title: 'Cupom expirado de teste',
    store: 2, // Store.AMAZON
    isActive: true, // active but expired
    expiresAt: new Date(Date.now() - 86400000).toISOString(),
    discountType: 2, // PERCENT
    discountValue: 20,
    maxDiscount: 100,
    createdAt: new Date(Date.now() - 7 * 24 * 3600000).toISOString(),
    updatedAt: new Date(Date.now() - 7 * 24 * 3600000).toISOString(),
  },
  {
    id: 'coupon-4',
    userId: 'user-123',
    code: 'DESATIVADO',
    title: 'Cupom desativado manualmente',
    store: 3, // Store.MAGALU
    isActive: false,
    expiresAt: new Date(Date.now() + 30 * 24 * 3600000).toISOString(),
    discountType: 1,
    discountValue: 5,
    minProductPrice: 30,
    createdAt: new Date(Date.now() - 3 * 24 * 3600000).toISOString(),
    updatedAt: new Date(Date.now() - 3 * 24 * 3600000).toISOString(),
  },
];

/**
 * Test coupon data for creation.
 */
export const TEST_COUPON = {
  code: 'TESTE20',
  title: 'Cupom de teste E2E',
  store: 1, // Store.SHOPEE
  discountType: 2, // PERCENT
  discountValue: 20,
  maxDiscount: 40,
  minProductPrice: 80,
  expiresAt: new Date(Date.now() + 14 * 24 * 3600000).toISOString(),
};

/**
 * Mocks the coupons list API.
 */
export async function mockCouponsListApi(page: Page, coupons = MOCK_COUPONS): Promise<void> {
  await page.route('**/coupons.v1.CouponsService/ListCoupons', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ coupons }),
    });
  });
}

/**
 * Mocks the create coupon API.
 */
export async function mockCreateCouponApi(
  page: Page,
  scenario: 'success' | 'duplicate' | 'error' = 'success'
): Promise<void> {
  await page.route('**/coupons.v1.CouponsService/CreateCoupon', async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            coupon: {
              id: 'new-coupon-id',
              userId: 'user-123',
              ...TEST_COUPON,
              isActive: true,
              createdAt: new Date().toISOString(),
              updatedAt: new Date().toISOString(),
            },
          }),
        });
        break;
      case 'duplicate':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: {
            'grpc-status': '6', // ALREADY_EXISTS
            'grpc-message': 'Coupon code already exists',
          },
          body: JSON.stringify({}),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: {
            'grpc-status': '13', // INTERNAL
            'grpc-message': 'Failed to create coupon',
          },
          body: JSON.stringify({}),
        });
        break;
    }
  });
}

/**
 * Mocks the update coupon API.
 */
export async function mockUpdateCouponApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/coupons.v1.CouponsService/UpdateCoupon', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ coupon: MOCK_COUPONS[0] }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to update coupon',
        },
        body: JSON.stringify({}),
      });
    }
  });
}

/**
 * Mocks the delete coupon API.
 */
export async function mockDeleteCouponApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/coupons.v1.CouponsService/DeleteCoupon', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({}),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to delete coupon',
        },
        body: JSON.stringify({}),
      });
    }
  });
}

/**
 * Sets up all common mocks for coupons tests.
 */
export async function setupCouponsMocks(page: Page): Promise<void> {
  await mockCouponsListApi(page);
  await mockCreateCouponApi(page);
  await mockUpdateCouponApi(page);
  await mockDeleteCouponApi(page);
}

/**
 * Sets up mocks for empty coupons state.
 */
export async function setupEmptyCouponsMocks(page: Page): Promise<void> {
  await mockCouponsListApi(page, []);
  await mockCreateCouponApi(page);
}

/**
 * Sets up mocks for filtered coupons (by store).
 */
export async function setupFilteredCouponsMocks(page: Page, store: number): Promise<void> {
  const filtered = MOCK_COUPONS.filter((c) => c.store === store);
  await mockCouponsListApi(page, filtered);
}
