import type { Page } from '@playwright/test';
import { setupAuthMocks } from './auth.fixture';

// ============================================================
// Mock Data
// ============================================================

/**
 * Mock history events matching the HistoryEvent proto shape.
 * Covers different event types and statuses for comprehensive testing.
 */
export const MOCK_HISTORY_EVENTS = [
  {
    id: 'history-1',
    userId: 'user-123',
    eventType: 'EVENT_TYPE_MESSAGE_SENT',
    description: 'Message sent to Promocoes Tech',
    metadata: { groupName: 'Promocoes Tech' },
    createdAt: '2026-01-15T10:00:00Z',
    status: 'completed',
    errorMessage: '',
    completedAt: '2026-01-15T10:00:05Z',
    targetCount: 5,
    successCount: 5,
    errorCount: 0,
  },
  {
    id: 'history-2',
    userId: 'user-123',
    eventType: 'EVENT_TYPE_AUTOMATION_RUN',
    description: 'Automation "Search Shopee" executed',
    metadata: { automationName: 'Search Shopee', offersFound: 15 },
    createdAt: '2026-01-15T09:30:00Z',
    status: 'completed',
    errorMessage: '',
    completedAt: '2026-01-15T09:30:45Z',
    targetCount: 12,
    successCount: 12,
    errorCount: 0,
  },
  {
    id: 'history-3',
    userId: 'user-123',
    eventType: 'EVENT_TYPE_CLOSE_GROUPS',
    description: 'Groups closed for maintenance',
    metadata: {},
    createdAt: '2026-01-15T08:00:00Z',
    status: 'running',
    errorMessage: '',
    completedAt: '',
    targetCount: 3,
    successCount: 1,
    errorCount: 0,
  },
  {
    id: 'history-4',
    userId: 'user-123',
    eventType: 'EVENT_TYPE_CHANGE_NAME',
    description: 'Failed to change group name',
    metadata: { groupName: 'Ofertas Casa', error: 'Group not accessible' },
    createdAt: '2026-01-15T07:00:00Z',
    status: 'failed',
    errorMessage: 'Group not accessible',
    completedAt: '2026-01-15T07:00:02Z',
    targetCount: 1,
    successCount: 0,
    errorCount: 1,
  },
  {
    id: 'history-5',
    userId: 'user-123',
    eventType: 'EVENT_TYPE_GROUP_SYNC',
    description: 'Groups synchronized successfully',
    metadata: { groupsAdded: 2, groupsUpdated: 8 },
    createdAt: '2026-01-15T06:00:00Z',
    status: 'completed',
    errorMessage: '',
    completedAt: '2026-01-15T06:00:10Z',
    targetCount: 10,
    successCount: 10,
    errorCount: 0,
  },
];

/**
 * Mock history stats matching the HistoryStats proto shape.
 * Values: total / completed / running / failed.
 */
export const MOCK_HISTORY_STATS = {
  total: 5,
  completed: 3,
  running: 1,
  failed: 1,
};

// ============================================================
// Mock API Functions
// ============================================================

/**
 * Mocks the ListHistory ConnectRPC endpoint.
 * Returns events wrapped in the correct response envelope with pagination and stats.
 */
export async function mockHistoryListApi(
  page: Page,
  events = MOCK_HISTORY_EVENTS,
  options: {
    hasMore?: boolean;
    totalCount?: number;
    stats?: typeof MOCK_HISTORY_STATS;
    currentPage?: number;
    pageSize?: number;
    totalPages?: number;
  } = {}
): Promise<void> {
  const {
    hasMore = false,
    totalCount = events.length,
    stats = MOCK_HISTORY_STATS,
    currentPage = 1,
    pageSize = 20,
    totalPages = 1,
  } = options;

  await page.route('**/history.v1.HistoryService/ListHistory', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({
        events,
        hasMore,
        stats,
        totalCount,
        page: currentPage,
        pageSize,
        totalPages,
      }),
    });
  });
}

/**
 * Mocks the ClearHistory ConnectRPC endpoint.
 */
export async function mockClearHistoryApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/history.v1.HistoryService/ClearHistory', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({ deletedCount: 100 }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to clear history',
        },
        body: JSON.stringify({}),
      });
    }
  });
}

/**
 * Mocks history with a gRPC error for all endpoints.
 */
export async function mockHistoryErrorApi(page: Page): Promise<void> {
  await page.route('**/history.v1.HistoryService/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: {
        'grpc-status': '13',
        'grpc-message': 'Failed to load history',
      },
      body: JSON.stringify({}),
    });
  });
}

// ============================================================
// Setup Helpers
// ============================================================

/**
 * Sets up all common mocks for history tests.
 * Includes auth, list, and clear history endpoints.
 */
export async function setupHistoryMocks(page: Page): Promise<void> {
  await setupAuthMocks(page);
  await mockHistoryListApi(page);
  await mockClearHistoryApi(page);
}

/**
 * Sets up mocks for the empty history state.
 * Returns zero events and zero stats.
 */
export async function setupEmptyHistoryMocks(page: Page): Promise<void> {
  await setupAuthMocks(page);
  await mockHistoryListApi(page, [], {
    totalCount: 0,
    stats: { total: 0, completed: 0, running: 0, failed: 0 },
    totalPages: 0,
  });
  await mockClearHistoryApi(page);
}

/**
 * Sets up mocks for paginated history.
 * First page has hasMore=true, totalPages > 1.
 */
export async function setupPaginatedHistoryMocks(page: Page): Promise<void> {
  await setupAuthMocks(page);
  await mockHistoryListApi(page, MOCK_HISTORY_EVENTS, {
    hasMore: true,
    totalCount: 100,
    stats: { total: 100, completed: 70, running: 10, failed: 20 },
    currentPage: 1,
    pageSize: 20,
    totalPages: 5,
  });
  await mockClearHistoryApi(page);
}
