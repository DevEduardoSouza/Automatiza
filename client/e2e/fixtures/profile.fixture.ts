import type { Page } from '@playwright/test';
import { MOCK_AUTH_USER } from './auth.fixture';

/**
 * Mock profile data for e2e tests.
 */
export const MOCK_PROFILE_DATA = {
  id: MOCK_AUTH_USER.id,
  email: MOCK_AUTH_USER.email,
  displayName: MOCK_AUTH_USER.displayName,
  plan: 'pro',
  phoneNumber: '+5511999999999',
  maxAutomations: 10,
  billingNextPaymentAt: '2026-02-15T00:00:00Z',
  planAccessUntil: '2026-03-15T00:00:00Z',
  isBlocked: false,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-15T00:00:00Z',
};

/**
 * Mock limits data for e2e tests.
 */
export const MOCK_LIMITS_DATA = {
  automationsCount: 5,
  maxAutomations: 10,
  connectedInstancesCount: 2,
};

/**
 * Mock sessions for e2e tests.
 * Contains 1 current session + 2 other sessions with different user agents.
 */
export const MOCK_SESSIONS = [
  {
    sessionId: 'session-current',
    expiresAt: new Date(Date.now() + 3600000).toISOString(),
    deviceFingerprint: 'fp-current',
    ipAddress: '10.0.0.1',
    userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/120.0.0.0',
    createdAt: '2026-02-20T10:00:00Z',
    lastActivityAt: new Date().toISOString(),
    isCurrent: true,
  },
  {
    sessionId: 'session-mobile',
    expiresAt: new Date(Date.now() + 3600000).toISOString(),
    deviceFingerprint: 'fp-mobile',
    ipAddress: '192.168.1.100',
    userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0) Safari/604.1',
    createdAt: '2026-02-19T08:00:00Z',
    lastActivityAt: new Date(Date.now() - 30 * 60 * 1000).toISOString(),
    isCurrent: false,
  },
  {
    sessionId: 'session-firefox',
    expiresAt: new Date(Date.now() + 3600000).toISOString(),
    deviceFingerprint: 'fp-firefox',
    ipAddress: '172.16.0.50',
    userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Firefox/119.0',
    createdAt: '2026-02-18T14:00:00Z',
    lastActivityAt: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(),
    isCurrent: false,
  },
];

/**
 * Mocks the GetProfile endpoint.
 */
export async function mockProfileGetApi(page: Page): Promise<void> {
  await page.route('**/profiles.v1.ProfilesService/GetProfile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ profile: MOCK_PROFILE_DATA }),
    });
  });
}

/**
 * Mocks the GetUserLimits endpoint.
 */
export async function mockProfileLimitsApi(page: Page): Promise<void> {
  await page.route('**/profiles.v1.ProfilesService/GetUserLimits', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ limits: MOCK_LIMITS_DATA }),
    });
  });
}

/**
 * Mocks the ListSessions endpoint.
 */
export async function mockListSessionsApi(
  page: Page,
  sessions = MOCK_SESSIONS
): Promise<void> {
  await page.route('**/auth.v1.AuthService/ListSessions', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        sessions,
        currentSessionId: 'session-current',
      }),
    });
  });
}

/**
 * Mocks the RevokeSession endpoint.
 */
export async function mockRevokeSessionApi(
  page: Page,
  scenario: 'success' | 'error'
): Promise<void> {
  await page.route('**/auth.v1.AuthService/RevokeSession', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({}),
      });
    } else {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'internal', message: 'Failed to revoke session' }),
      });
    }
  });
}

/**
 * Mocks the Logout endpoint for logout-all-devices.
 */
export async function mockLogoutAllApi(page: Page): Promise<void> {
  await page.route('**/auth.v1.AuthService/Logout', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({}),
    });
  });
}

/**
 * Composite setup: auth + profile + limits (without sessions).
 */
export async function setupProfilePageMocks(page: Page): Promise<void> {
  const { setupAuthMocks } = await import('./auth.fixture');
  await setupAuthMocks(page);
  await mockProfileGetApi(page);
  await mockProfileLimitsApi(page);
}

/**
 * Composite setup: auth + profile + limits + sessions.
 */
export async function setupProfileWithSessionsMocks(page: Page): Promise<void> {
  await setupProfilePageMocks(page);
  await mockListSessionsApi(page);
}
