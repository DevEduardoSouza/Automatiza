import type { Page } from '@playwright/test';

/**
 * Mock dashboard statistics data.
 */
export const MOCK_DASHBOARD_STATS = {
  totalOffersSent: 1250,
  offersSentToday: 45,
  totalGroups: 12,
  activeGroups: 8,
  totalAutomations: 5,
  activeAutomations: 3,
  whatsappStatus: 'CONNECTED',
  lastActivity: new Date().toISOString(),
};

/**
 * Mock recent activity data.
 */
export const MOCK_RECENT_ACTIVITY = [
  {
    id: 'activity-1',
    type: 'OFFER_SENT',
    description: 'Offer sent to Test Group 1',
    timestamp: new Date().toISOString(),
    metadata: { groupName: 'Test Group 1', offerId: 'offer-123' },
  },
  {
    id: 'activity-2',
    type: 'AUTOMATION_RUN',
    description: 'Automation "Search Shopee" executed',
    timestamp: new Date(Date.now() - 3600000).toISOString(),
    metadata: { automationName: 'Search Shopee', offersFound: 10 },
  },
  {
    id: 'activity-3',
    type: 'GROUP_SYNC',
    description: 'Groups synchronized successfully',
    timestamp: new Date(Date.now() - 7200000).toISOString(),
    metadata: { groupsAdded: 2, groupsRemoved: 0 },
  },
];

/**
 * Mock chart data for offers over time.
 */
export const MOCK_OFFERS_CHART_DATA = [
  { date: '2026-01-25', count: 120 },
  { date: '2026-01-26', count: 150 },
  { date: '2026-01-27', count: 180 },
  { date: '2026-01-28', count: 140 },
  { date: '2026-01-29', count: 200 },
  { date: '2026-01-30', count: 165 },
  { date: '2026-01-31', count: 195 },
];

/**
 * Mocks the dashboard stats API.
 */
export async function mockDashboardStatsApi(
  page: Page,
  stats = MOCK_DASHBOARD_STATS
): Promise<void> {
  await page.route('**/dashboard.v1.DashboardService/GetStats', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify(stats)),
    });
  });
}

/**
 * Mocks the recent activity API.
 */
export async function mockRecentActivityApi(
  page: Page,
  activities = MOCK_RECENT_ACTIVITY
): Promise<void> {
  await page.route('**/dashboard.v1.DashboardService/GetRecentActivity', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify({ activities })),
    });
  });
}

/**
 * Mocks the offers chart data API.
 */
export async function mockOffersChartApi(page: Page, data = MOCK_OFFERS_CHART_DATA): Promise<void> {
  await page.route('**/dashboard.v1.DashboardService/GetOffersChart', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify({ data })),
    });
  });
}

/**
 * Mocks the WhatsApp status API for dashboard.
 */
export async function mockDashboardWhatsAppStatus(page: Page, connected = true): Promise<void> {
  await page.route('**/whatsapp.v1.WhatsAppService/GetStatus', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(
        JSON.stringify({
          connected,
          status: connected ? 'CONNECTED' : 'DISCONNECTED',
          phoneNumber: connected ? '+5511999999999' : null,
        })
      ),
    });
  });
}

/**
 * Mocks dashboard with empty state.
 */
export async function mockDashboardEmptyState(page: Page): Promise<void> {
  await mockDashboardStatsApi(page, {
    totalOffersSent: 0,
    offersSentToday: 0,
    totalGroups: 0,
    activeGroups: 0,
    totalAutomations: 0,
    activeAutomations: 0,
    whatsappStatus: 'DISCONNECTED',
    lastActivity: '',
  });
  await mockRecentActivityApi(page, []);
  await mockOffersChartApi(page, []);
  await mockDashboardWhatsAppStatus(page, false);
}

/**
 * Mocks dashboard with error state.
 */
export async function mockDashboardError(page: Page): Promise<void> {
  await page.route('**/dashboard.v1.DashboardService/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: {
        'grpc-status': '13', // INTERNAL
        'grpc-message': 'Failed to load dashboard data',
      },
      body: Buffer.from([]),
    });
  });
}

/**
 * Sets up all common mocks for dashboard tests.
 */
export async function setupDashboardMocks(page: Page): Promise<void> {
  await mockDashboardStatsApi(page);
  await mockRecentActivityApi(page);
  await mockOffersChartApi(page);
  await mockDashboardWhatsAppStatus(page);
}
