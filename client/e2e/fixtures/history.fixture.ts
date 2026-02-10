import type { Page } from '@playwright/test';

/**
 * Action types enum matching the proto.
 */
export const ACTION_TYPES = {
  OFFER_SENT: 1,
  AUTOMATION_RUN: 2,
  GROUP_SYNC: 3,
  WHATSAPP_CONNECT: 4,
  WHATSAPP_DISCONNECT: 5,
  MESSAGE_SENT: 6,
  JOB_EXECUTED: 7,
} as const;

/**
 * Mock history entries.
 */
export const MOCK_HISTORY_ENTRIES = [
  {
    id: 'history-1',
    actionType: ACTION_TYPES.OFFER_SENT,
    description: 'Offer sent to Promocoes Tech',
    timestamp: new Date().toISOString(),
    metadata: {
      groupName: 'Promocoes Tech',
      offerId: 'offer-123',
      productName: 'iPhone 15 Pro',
      originalPrice: 7999.0,
      discountPrice: 6499.0,
      store: 'SHOPEE',
    },
    success: true,
  },
  {
    id: 'history-2',
    actionType: ACTION_TYPES.AUTOMATION_RUN,
    description: 'Automation "Search Shopee" executed',
    timestamp: new Date(Date.now() - 1800000).toISOString(),
    metadata: {
      automationId: 'automation-1',
      automationName: 'Search Shopee',
      offersFound: 15,
      offersSent: 12,
      duration: 45,
    },
    success: true,
  },
  {
    id: 'history-3',
    actionType: ACTION_TYPES.GROUP_SYNC,
    description: 'Groups synchronized',
    timestamp: new Date(Date.now() - 3600000).toISOString(),
    metadata: {
      groupsAdded: 2,
      groupsUpdated: 8,
      groupsRemoved: 0,
    },
    success: true,
  },
  {
    id: 'history-4',
    actionType: ACTION_TYPES.OFFER_SENT,
    description: 'Failed to send offer',
    timestamp: new Date(Date.now() - 5400000).toISOString(),
    metadata: {
      groupName: 'Ofertas Casa',
      error: 'Group not accessible',
    },
    success: false,
  },
  {
    id: 'history-5',
    actionType: ACTION_TYPES.JOB_EXECUTED,
    description: 'Scheduled job executed',
    timestamp: new Date(Date.now() - 7200000).toISOString(),
    metadata: {
      jobId: 'job-1',
      jobName: 'Daily Message',
      groupsSent: 5,
    },
    success: true,
  },
];

/**
 * Mock history stats.
 */
export const MOCK_HISTORY_STATS = {
  totalActions: 1250,
  actionsToday: 45,
  successRate: 98.5,
  byActionType: {
    [ACTION_TYPES.OFFER_SENT]: 850,
    [ACTION_TYPES.AUTOMATION_RUN]: 200,
    [ACTION_TYPES.GROUP_SYNC]: 100,
    [ACTION_TYPES.JOB_EXECUTED]: 100,
  },
};

/**
 * Mocks the history list API.
 */
export async function mockHistoryListApi(
  page: Page,
  entries = MOCK_HISTORY_ENTRIES,
  options: { hasMore?: boolean; totalCount?: number } = {}
): Promise<void> {
  const { hasMore = false, totalCount = entries.length } = options;

  await page.route('**/history.v1.HistoryService/ListHistory', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(
        JSON.stringify({
          entries,
          hasMore,
          totalCount,
          nextCursor: hasMore ? 'cursor-next' : null,
        })
      ),
    });
  });
}

/**
 * Mocks the history stats API.
 */
export async function mockHistoryStatsApi(page: Page, stats = MOCK_HISTORY_STATS): Promise<void> {
  await page.route('**/history.v1.HistoryService/GetStats', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify(stats)),
    });
  });
}

/**
 * Mocks the history entry details API.
 */
export async function mockHistoryEntryApi(
  page: Page,
  entry = MOCK_HISTORY_ENTRIES[0]
): Promise<void> {
  await page.route('**/history.v1.HistoryService/GetEntry', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify({ entry })),
    });
  });
}

/**
 * Mocks the clear history API.
 */
export async function mockClearHistoryApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/history.v1.HistoryService/ClearHistory', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify({ deletedCount: 100 })),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to clear history',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks history with filter applied.
 */
export async function mockHistoryFilteredApi(page: Page, actionType: number): Promise<void> {
  const filtered = MOCK_HISTORY_ENTRIES.filter((e) => e.actionType === actionType);

  await page.route('**/history.v1.HistoryService/ListHistory', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(
        JSON.stringify({
          entries: filtered,
          hasMore: false,
          totalCount: filtered.length,
        })
      ),
    });
  });
}

/**
 * Mocks history with error state.
 */
export async function mockHistoryErrorApi(page: Page): Promise<void> {
  await page.route('**/history.v1.HistoryService/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: {
        'grpc-status': '13', // INTERNAL
        'grpc-message': 'Failed to load history',
      },
      body: Buffer.from([]),
    });
  });
}

/**
 * Sets up all common mocks for history tests.
 */
export async function setupHistoryMocks(page: Page): Promise<void> {
  await mockHistoryListApi(page);
  await mockHistoryStatsApi(page);
  await mockClearHistoryApi(page);
}

/**
 * Sets up mocks for empty history state.
 */
export async function setupEmptyHistoryMocks(page: Page): Promise<void> {
  await mockHistoryListApi(page, []);
  await mockHistoryStatsApi(page, {
    totalActions: 0,
    actionsToday: 0,
    successRate: 0,
    byActionType: {
      [ACTION_TYPES.OFFER_SENT]: 0,
      [ACTION_TYPES.AUTOMATION_RUN]: 0,
      [ACTION_TYPES.GROUP_SYNC]: 0,
      [ACTION_TYPES.JOB_EXECUTED]: 0,
    },
  });
}

/**
 * Sets up mocks for paginated history.
 */
export async function setupPaginatedHistoryMocks(page: Page): Promise<void> {
  await mockHistoryListApi(page, MOCK_HISTORY_ENTRIES, {
    hasMore: true,
    totalCount: 100,
  });
  await mockHistoryStatsApi(page, {
    ...MOCK_HISTORY_STATS,
    totalActions: 100,
  });
}
