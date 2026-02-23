import type { Page } from '@playwright/test';

/**
 * Mock dashboard statistics data matching proto GetDashboardStatsResponse.
 */
export const MOCK_DASHBOARD_STATS = {
  stats: {
    totalGroups: 12,
    fullGroups: 3,
    totalMembers: 1250,
    activeLinks: 8,
  },
};

/**
 * Mock enabled announcement matching proto GetAnnouncementResponse.
 */
export const MOCK_ANNOUNCEMENT_ENABLED = {
  announcement: {
    enabled: true,
    title: 'Novidade!',
    body: 'Temos uma atualização importante para você.\nConfira as novidades.',
  },
};

/**
 * Mock disabled announcement.
 */
export const MOCK_ANNOUNCEMENT_DISABLED = {
  announcement: {
    enabled: false,
    title: '',
    body: '',
  },
};

/**
 * Mock weekly history data matching proto GetDashboardHistoryResponse.
 */
export const MOCK_HISTORY_WEEKLY = {
  dataPoints: [
    { label: 'Seg', members: 100, groups: 5, newMembers: 10, newGroups: 1 },
    { label: 'Ter', members: 120, groups: 6, newMembers: 20, newGroups: 1 },
    { label: 'Qua', members: 150, groups: 7, newMembers: 30, newGroups: 1 },
    { label: 'Qui', members: 180, groups: 7, newMembers: 30, newGroups: 0 },
    { label: 'Sex', members: 200, groups: 8, newMembers: 20, newGroups: 1 },
    { label: 'Sáb', members: 210, groups: 8, newMembers: 10, newGroups: 0 },
    { label: 'Dom', members: 230, groups: 9, newMembers: 20, newGroups: 1 },
  ],
};

/**
 * Mock empty stats (all zeros).
 */
export const MOCK_EMPTY_STATS = {
  stats: {
    totalGroups: 0,
    fullGroups: 0,
    totalMembers: 0,
    activeLinks: 0,
  },
};

/**
 * Mocks the GetDashboardStats endpoint.
 */
export async function mockGetDashboardStats(
  page: Page,
  data = MOCK_DASHBOARD_STATS
): Promise<void> {
  await page.route('**/dashboard.v1.DashboardService/GetDashboardStats', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(data),
    });
  });
}

/**
 * Mocks the GetAnnouncement endpoint.
 */
export async function mockGetAnnouncement(
  page: Page,
  data = MOCK_ANNOUNCEMENT_DISABLED
): Promise<void> {
  await page.route('**/dashboard.v1.DashboardService/GetAnnouncement', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(data),
    });
  });
}

/**
 * Mocks the GetDashboardHistory endpoint.
 */
export async function mockGetDashboardHistory(
  page: Page,
  data = MOCK_HISTORY_WEEKLY
): Promise<void> {
  await page.route('**/dashboard.v1.DashboardService/GetDashboardHistory', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(data),
    });
  });
}

/**
 * Sets up all dashboard mocks with populated data.
 */
export async function setupDashboardMocks(page: Page): Promise<void> {
  await mockGetDashboardStats(page);
  await mockGetAnnouncement(page, MOCK_ANNOUNCEMENT_ENABLED);
  await mockGetDashboardHistory(page);
}

/**
 * Sets up dashboard mocks with empty/zero state.
 */
export async function mockDashboardEmptyState(page: Page): Promise<void> {
  await mockGetDashboardStats(page, MOCK_EMPTY_STATS);
  await mockGetAnnouncement(page, MOCK_ANNOUNCEMENT_DISABLED);
  await mockGetDashboardHistory(page, { dataPoints: [] });
}

/**
 * Sets up dashboard mocks that return gRPC INTERNAL error.
 */
export async function mockDashboardError(page: Page): Promise<void> {
  await page.route('**/dashboard.v1.DashboardService/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: {
        'grpc-status': '13',
        'grpc-message': 'Failed to load dashboard data',
      },
      body: JSON.stringify({}),
    });
  });
}
