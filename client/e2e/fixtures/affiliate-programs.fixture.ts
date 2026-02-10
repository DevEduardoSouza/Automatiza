import type { Page } from '@playwright/test';
import { test as base } from './auth.fixture';

/**
 * Test credentials for affiliate programs.
 */
export const TEST_SHOPEE_CREDENTIALS = {
  appId: 'test-app-id-12345',
  appSecret: 'test-secret-key-abcdef',
};

export const TEST_ML_CREDENTIALS = {
  connectionToken: 'MLCRED_V1:eyJ0YWciOiJ0ZXN0LXRhZyIsInNpdGVfaWQiOiJNTEIifQ==',
  tag: 'test-tag',
  testProductUrl: 'https://www.mercadolivre.com.br/test-product',
};

/**
 * Mock affiliate credentials list response.
 */
export const MOCK_CREDENTIALS_LIST = {
  credentials: [
    {
      id: 'cred-1',
      userId: 'user-1',
      store: 1, // SHOPEE
      credentials: JSON.stringify({
        app_id: TEST_SHOPEE_CREDENTIALS.appId,
        app_secret: TEST_SHOPEE_CREDENTIALS.appSecret,
      }),
      autoGenerate: true,
      isActive: true,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
  ],
};

/**
 * Mock ML preview response.
 */
export const MOCK_ML_PREVIEW = {
  preview: {
    title: 'Test Product',
    price: '199.90',
    imageUrl: 'https://example.com/image.jpg',
    currency: 'BRL',
  },
};

/**
 * Mock ML create link response.
 */
export const MOCK_ML_CREATE_LINK = {
  urls: [
    {
      longUrl: 'https://www.mercadolivre.com.br/test-product',
      shortUrl: 'https://afl.ml/abc123',
    },
  ],
};

/**
 * Mock Shopee search response.
 */
export const MOCK_SHOPEE_SEARCH = {
  offers: [
    {
      offerId: 'offer-1',
      title: 'Test Shopee Product',
      price: '99.90',
      imageUrl: 'https://example.com/shopee.jpg',
      affiliateUrl: 'https://shope.ee/test123',
    },
  ],
};

/**
 * Extended test fixture with affiliate programs helpers.
 */
export const test = base;

/**
 * Mocks the AffiliateCredentials ListCredentials RPC.
 */
export async function mockListCredentials(
  page: Page,
  credentials: typeof MOCK_CREDENTIALS_LIST = MOCK_CREDENTIALS_LIST
): Promise<void> {
  await page.route(
    '**/affiliates.v1.AffiliateCredentialsService/ListCredentials',
    async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from([]),
      });
    }
  );
}

/**
 * Mocks the AffiliateCredentials UpsertCredential RPC.
 */
export async function mockUpsertCredential(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route(
    '**/affiliates.v1.AffiliateCredentialsService/UpsertCredential',
    async (route) => {
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
            'grpc-message': 'Failed to save credentials',
          },
          body: Buffer.from([]),
        });
      }
    }
  );
}

/**
 * Mocks the AffiliateCredentials DeleteCredential RPC.
 */
export async function mockDeleteCredential(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route(
    '**/affiliates.v1.AffiliateCredentialsService/DeleteCredential',
    async (route) => {
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
            'grpc-status': '13',
            'grpc-message': 'Failed to delete credentials',
          },
          body: Buffer.from([]),
        });
      }
    }
  );
}

/**
 * Mocks the MercadoLivre Connect RPC.
 */
export async function mockMlConnect(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/affiliates.v1.MercadoLivreService/Connect', async (route) => {
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
          'grpc-status': '3', // INVALID_ARGUMENT
          'grpc-message': 'Invalid token format',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the MercadoLivre Preview RPC.
 */
export async function mockMlPreview(
  page: Page,
  scenario: 'success' | 'not_found' | 'error' = 'success'
): Promise<void> {
  await page.route('**/affiliates.v1.MercadoLivreService/Preview', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from([]),
      });
    } else if (scenario === 'not_found') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '5', // NOT_FOUND
          'grpc-message': 'Product not found',
        },
        body: Buffer.from([]),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to fetch preview',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the MercadoLivre CreateLink RPC.
 */
export async function mockMlCreateLink(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/affiliates.v1.MercadoLivreService/CreateLink', async (route) => {
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
          'grpc-status': '13',
          'grpc-message': 'Failed to create link',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks all affiliate-related RPCs for a complete test flow.
 */
export async function mockAllAffiliateApis(page: Page): Promise<void> {
  await mockListCredentials(page);
  await mockUpsertCredential(page);
  await mockDeleteCredential(page);
  await mockMlConnect(page);
  await mockMlPreview(page);
  await mockMlCreateLink(page);
}

export { expect } from '@playwright/test';
