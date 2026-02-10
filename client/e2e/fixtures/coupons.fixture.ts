import type { Page } from '@playwright/test';

/**
 * Coupon status enum.
 */
export const COUPON_STATUS = {
  ACTIVE: 1,
  EXPIRED: 2,
  DISABLED: 3,
} as const;

/**
 * Store types enum.
 */
export const STORE_TYPES = {
  SHOPEE: 1,
  MERCADO_LIVRE: 2,
  AMAZON: 3,
  MAGAZINE_LUIZA: 4,
} as const;

/**
 * Mock coupons data.
 */
export const MOCK_COUPONS = [
  {
    id: 'coupon-1',
    code: 'PROMO10',
    description: '10% off em compras acima de R$100',
    discountValue: 10,
    discountType: 'PERCENTAGE',
    minOrderValue: 100,
    maxDiscount: 50,
    store: STORE_TYPES.SHOPEE,
    status: COUPON_STATUS.ACTIVE,
    expiresAt: new Date(Date.now() + 7 * 24 * 3600000).toISOString(),
    usageCount: 45,
    createdAt: new Date().toISOString(),
  },
  {
    id: 'coupon-2',
    code: 'FRETE15',
    description: 'R$15 de desconto no frete',
    discountValue: 15,
    discountType: 'FIXED',
    minOrderValue: 50,
    maxDiscount: null,
    store: STORE_TYPES.MERCADO_LIVRE,
    status: COUPON_STATUS.ACTIVE,
    expiresAt: new Date(Date.now() + 3 * 24 * 3600000).toISOString(),
    usageCount: 120,
    createdAt: new Date(Date.now() - 86400000).toISOString(),
  },
  {
    id: 'coupon-3',
    code: 'OFERTAEXP',
    description: 'Cupom expirado de teste',
    discountValue: 20,
    discountType: 'PERCENTAGE',
    minOrderValue: 0,
    maxDiscount: 100,
    store: STORE_TYPES.AMAZON,
    status: COUPON_STATUS.EXPIRED,
    expiresAt: new Date(Date.now() - 86400000).toISOString(),
    usageCount: 200,
    createdAt: new Date(Date.now() - 7 * 24 * 3600000).toISOString(),
  },
  {
    id: 'coupon-4',
    code: 'DESATIVADO',
    description: 'Cupom desativado manualmente',
    discountValue: 5,
    discountType: 'FIXED',
    minOrderValue: 30,
    maxDiscount: null,
    store: STORE_TYPES.MAGAZINE_LUIZA,
    status: COUPON_STATUS.DISABLED,
    expiresAt: new Date(Date.now() + 30 * 24 * 3600000).toISOString(),
    usageCount: 15,
    createdAt: new Date(Date.now() - 3 * 24 * 3600000).toISOString(),
  },
];

/**
 * Test coupon data for creation.
 */
export const TEST_COUPON = {
  code: 'TESTE20',
  description: 'Cupom de teste E2E',
  discountValue: 20,
  discountType: 'PERCENTAGE',
  minOrderValue: 80,
  maxDiscount: 40,
  store: STORE_TYPES.SHOPEE,
  expiresAt: new Date(Date.now() + 14 * 24 * 3600000).toISOString(),
};

/**
 * Mocks the coupons list API.
 */
export async function mockCouponsListApi(page: Page, coupons = MOCK_COUPONS): Promise<void> {
  await page.route('**/coupons.v1.CouponsService/ListCoupons', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify({ coupons })),
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
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(
            JSON.stringify({
              coupon: {
                id: 'new-coupon-id',
                ...TEST_COUPON,
                status: COUPON_STATUS.ACTIVE,
                usageCount: 0,
                createdAt: new Date().toISOString(),
              },
            })
          ),
        });
        break;
      case 'duplicate':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '6', // ALREADY_EXISTS
            'grpc-message': 'Coupon code already exists',
          },
          body: Buffer.from([]),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '13', // INTERNAL
            'grpc-message': 'Failed to create coupon',
          },
          body: Buffer.from([]),
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
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify({ coupon: MOCK_COUPONS[0] })),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to update coupon',
        },
        body: Buffer.from([]),
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
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from([]),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to delete coupon',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the toggle coupon status API.
 */
export async function mockToggleCouponStatusApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/coupons.v1.CouponsService/ToggleCouponStatus', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(
          JSON.stringify({
            coupon: { ...MOCK_COUPONS[0], status: COUPON_STATUS.DISABLED },
          })
        ),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to toggle coupon status',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the copy coupon to clipboard API (track usage).
 */
export async function mockCopyCouponApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/coupons.v1.CouponsService/TrackCouponCopy', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify({ success: true })),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to track copy',
        },
        body: Buffer.from([]),
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
  await mockToggleCouponStatusApi(page);
  await mockCopyCouponApi(page);
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
