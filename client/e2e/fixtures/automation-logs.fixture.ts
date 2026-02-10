import type { Page } from '@playwright/test';

/**
 * Mocks the auth API so the app considers the user authenticated.
 * Without this, ProtectedRoute redirects to /auth.
 */
export async function mockAuthApi(page: Page): Promise<void> {
  const now = new Date();
  const oneHourLater = new Date(now.getTime() + 3600_000);
  const oneDayAgo = new Date(now.getTime() - 86400_000);
  const oneHourAgo = new Date(now.getTime() - 3600_000);

  await page.route('**/auth.v1.AuthService/GetCurrentUser', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        user: {
          id: 'test-user-id',
          email: 'test@example.com',
          displayName: 'Test User',
          role: 'user',
          emailVerified: true,
          createdAt: oneDayAgo.toISOString(),
        },
        session: {
          sessionId: 'test-session-id',
          expiresAt: oneHourLater.toISOString(),
          createdAt: oneHourAgo.toISOString(),
          lastActivityAt: now.toISOString(),
          isCurrent: true,
        },
      }),
    });
  });
}

/**
 * Mock dispatch log data for automation logs E2E tests.
 */
export const MOCK_DISPATCH_LOGS = [
  {
    id: '1',
    automationId: 'auto-1',
    groupId: 'group-1',
    groupName: 'Promo Tech',
    canonicalKey: 'shopee:123:456',
    offerTitle: 'iPhone 15 Pro',
    offerUrl: 'https://shopee.com.br/product/123/456',
    offerPrice: 4999.99,
    status: 'sent',
    createdAt: new Date().toISOString(),
    store: 'shopee',
    productCategory: 'Eletronicos',
    productPrice: 4999.99,
    couponId: 'coupon-1',
    couponCode: 'TECH10',
    couponApplied: true,
    productUrl: 'https://shopee.com.br/product/123/456',
    productPageUrl: 'https://shopee.com.br/product/123/456',
    affiliateLink: 'https://shope.ee/abc123',
  },
  {
    id: '2',
    automationId: 'auto-1',
    groupId: 'group-2',
    groupName: 'Ofertas Casa',
    canonicalKey: 'amazon:789:012',
    offerTitle: 'Echo Dot',
    offerUrl: 'https://amazon.com.br/dp/B0123',
    status: 'queued',
    createdAt: new Date(Date.now() - 300_000).toISOString(),
    store: 'amazon',
    productUrl: 'https://amazon.com.br/dp/B0123',
  },
  {
    id: '3',
    automationId: 'auto-2',
    groupId: 'group-1',
    groupName: 'Promo Tech',
    canonicalKey: 'ml:345:678',
    offerTitle: 'Samsung Galaxy',
    status: 'skipped',
    createdAt: new Date(Date.now() - 600_000).toISOString(),
    store: 'ml',
    productUrl: 'https://mercadolivre.com.br/item/345',
  },
  {
    id: '4',
    automationId: 'auto-1',
    groupId: 'group-1',
    groupName: 'Promo Tech',
    canonicalKey: 'shopee:111:222',
    offerTitle: 'AirPods Pro',
    status: 'failed',
    errorMessage: 'instância não encontrada',
    createdAt: new Date(Date.now() - 900_000).toISOString(),
    store: 'shopee',
    productUrl: 'https://shopee.com.br/product/111/222',
  },
  {
    id: '5',
    automationId: 'auto-2',
    groupId: 'group-2',
    groupName: 'Ofertas Casa',
    canonicalKey: 'amazon:333:444',
    offerTitle: 'Kindle',
    status: 'sent',
    createdAt: new Date(Date.now() - 1200_000).toISOString(),
    store: 'amazon',
    couponId: 'coupon-2',
    couponCode: 'BOOK20',
    couponApplied: false,
    couponSkipReason: 'Preço abaixo do mínimo',
    productUrl: 'https://amazon.com.br/dp/B0456',
    affiliateLink: 'https://amzn.to/xyz789',
  },
];

/**
 * Mock automations for the filter dropdown.
 */
export const MOCK_LOG_AUTOMATIONS = [
  {
    id: 'auto-1',
    name: 'Shopee Promos',
    mode: 1,
    status: 1,
    stores: [1],
    sendGroups: ['group-1', 'group-2'],
  },
  {
    id: 'auto-2',
    name: 'Amazon Deals',
    mode: 1,
    status: 1,
    stores: [2],
    sendGroups: ['group-1'],
  },
];

/**
 * Mock groups for the filter dropdown.
 */
export const MOCK_LOG_GROUPS = [
  {
    id: 'group-1',
    name: 'Promo Tech',
    waGroupId: '120363@g.us',
    userId: 'test-user-id',
    isFavorite: true,
    isAdmin: true,
  },
  {
    id: 'group-2',
    name: 'Ofertas Casa',
    waGroupId: '120364@g.us',
    userId: 'test-user-id',
    isFavorite: false,
    isAdmin: true,
  },
];

/**
 * Mock queue status response.
 */
export const MOCK_QUEUE_STATUS = {
  queue: { waiting: 5, delayed: 2, active: 1, queued: 7 },
  settings: { limitPerMinute: 60, workerConcurrency: 5 },
  estimatedMinutes: 1,
};

/**
 * Mocks the ListDispatchLogs API.
 */
export async function mockDispatchLogsApi(
  page: Page,
  logs = MOCK_DISPATCH_LOGS,
  totalCount?: number
): Promise<void> {
  await page.route('**/automations.v1.AutomationsService/ListDispatchLogs', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        logs,
        totalCount: totalCount ?? logs.length,
        hasMore: false,
      }),
    });
  });
}

/**
 * Mocks the ListAutomations API for the filter dropdown.
 */
export async function mockLogAutomationsApi(
  page: Page,
  automations = MOCK_LOG_AUTOMATIONS
): Promise<void> {
  await page.route('**/automations.v1.AutomationsService/ListAutomations', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ automations }),
    });
  });
}

/**
 * Mocks the groups API for the filter dropdown.
 */
export async function mockLogGroupsApi(page: Page, groups = MOCK_LOG_GROUPS): Promise<void> {
  await page.route('**/groups.v1.GroupsService/ListGroups', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ groups }),
    });
  });
}

/**
 * Mocks the GetQueueStatus API.
 */
export async function mockQueueStatusApi(page: Page, status = MOCK_QUEUE_STATUS): Promise<void> {
  await page.route('**/jobs.v1.JobsService/GetQueueStatus', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(status),
    });
  });
}

/**
 * Mocks the ListJobs API (needed because useJobsConnect also fetches jobs).
 */
export async function mockListJobsApi(page: Page): Promise<void> {
  await page.route('**/jobs.v1.JobsService/ListJobs', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ jobs: [] }),
    });
  });
}

/**
 * Mocks ancillary APIs called on app load (settings, profile, limits, whatsapp).
 * Prevents the retry interceptor from flooding the network with unmocked calls.
 */
export async function mockAppShellApis(page: Page): Promise<void> {
  await page.route('**/settings.v1.SettingsService/**', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
  });
  await page.route('**/profiles.v1.ProfilesService/**', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
  });
  await page.route('**/whatsapp.v1.WhatsAppService/**', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
  });
  await page.route('**/billing.v1.BillingService/**', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
  });
}

/**
 * Sets up all mocks needed for the automation logs page.
 * IMPORTANT: mockAppShellApis (catch-all) must be registered FIRST because
 * Playwright evaluates routes in reverse order (most recently added wins).
 * Specific mocks registered after the catch-all will take priority.
 */
export async function setupAutomationLogsMocks(page: Page): Promise<void> {
  await mockAppShellApis(page);
  await mockAuthApi(page);
  await mockDispatchLogsApi(page);
  await mockLogAutomationsApi(page);
  await mockLogGroupsApi(page);
  await mockQueueStatusApi(page);
  await mockListJobsApi(page);
}

/**
 * Sets up mocks for empty state testing.
 */
export async function setupEmptyAutomationLogsMocks(page: Page): Promise<void> {
  await mockAppShellApis(page);
  await mockAuthApi(page);
  await mockDispatchLogsApi(page, [], 0);
  await mockLogAutomationsApi(page);
  await mockLogGroupsApi(page);
  await mockQueueStatusApi(page);
  await mockListJobsApi(page);
}
