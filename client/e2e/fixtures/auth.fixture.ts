import { type Page, test as base, expect } from '@playwright/test';

/**
 * Test user credentials for E2E tests.
 * In a real environment, these would come from environment variables.
 */
export const TEST_USER = {
  email: process.env.E2E_TEST_EMAIL || 'test@example.com',
  password: process.env.E2E_TEST_PASSWORD || 'TestPassword123!',
  newPassword: 'NewTestPassword456!',
  invalidPassword: 'wrong',
  shortPassword: '12345',
};

/**
 * Mock user data for authenticated tests.
 */
export const MOCK_AUTH_USER = {
  id: 'user-123',
  email: TEST_USER.email,
  displayName: 'Test User',
  role: 'user',
  emailVerified: true,
  createdAt: '2026-01-01T00:00:00Z',
};

/**
 * Mock session data for authenticated tests.
 */
export const MOCK_AUTH_SESSION = {
  sessionId: 'session-123',
  expiresAt: new Date(Date.now() + 3600000).toISOString(),
  deviceFingerprint: 'mock-fingerprint',
  ipAddress: '127.0.0.1',
  userAgent: 'Playwright Test',
  createdAt: '2026-01-01T00:00:00Z',
  lastActivityAt: new Date().toISOString(),
  isCurrent: true,
};

/**
 * Extended test fixture with authentication helpers.
 */
export const test = base.extend<{
  authenticatedPage: Page;
}>({
  authenticatedPage: async ({ page }, use) => {
    await loginUser(page);
    await use(page);
  },
});

/**
 * Logs in a test user.
 * Uses the auth page form to perform login.
 */
export async function loginUser(page: Page): Promise<void> {
  await page.goto('/auth');

  // Wait for the login form to be visible
  await page.waitForSelector('#login-email', { state: 'visible' });

  // Fill in credentials
  await page.fill('#login-email', TEST_USER.email);
  await page.fill('#login-password', TEST_USER.password);

  // Submit the form
  await page.click('button[type="submit"]');

  // Wait for navigation to dashboard or profile page
  await page.waitForURL(/\/(dashboard|profile|groups)/, { timeout: 10000 });
}

/**
 * Logs out the current user.
 */
export async function logoutUser(page: Page): Promise<void> {
  // Look for logout button in sidebar or header
  const logoutButton = page.getByRole('button', { name: /logout|sair/i });

  if (await logoutButton.isVisible()) {
    await logoutButton.click();
    await page.waitForURL('/auth', { timeout: 10000 });
  }
}

/**
 * Waits for a toast notification with the given text.
 */
export async function waitForToast(
  page: Page,
  textPattern: RegExp | string,
  variant: 'success' | 'error' | 'default' = 'default'
): Promise<void> {
  const toastSelector = '[data-sonner-toast]';
  await page.waitForSelector(toastSelector, { state: 'visible', timeout: 5000 });

  const toast = page.locator(toastSelector).first();

  if (typeof textPattern === 'string') {
    await expect(toast).toContainText(textPattern);
  } else {
    await expect(toast).toHaveText(textPattern);
  }
}

/**
 * Mocks the password reset email API.
 */
export async function mockPasswordResetEmail(page: Page): Promise<void> {
  await page.route('**/auth.v1.AuthService/ForgotPassword', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({}),
    });
  });
}

/**
 * Mocks the profile API responses.
 */
export async function mockProfileApi(page: Page): Promise<void> {
  await page.route('**/profiles.v1.ProfilesService/GetProfile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        profile: {
          id: MOCK_AUTH_USER.id,
          email: MOCK_AUTH_USER.email,
          displayName: MOCK_AUTH_USER.displayName,
        },
      }),
    });
  });
}

/**
 * Mocks the password change API to simulate different scenarios.
 */
export async function mockPasswordChangeApi(
  page: Page,
  scenario: 'success' | 'wrong_password' | 'error'
): Promise<void> {
  await page.route('**/auth.v1.AuthService/ChangePassword', async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({}),
        });
        break;
      case 'wrong_password':
        await route.fulfill({
          status: 400,
          contentType: 'application/json',
          body: JSON.stringify({
            code: 'permission_denied',
            message: 'Invalid current password',
          }),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ code: 'internal', message: 'Internal server error' }),
        });
        break;
    }
  });
}

/**
 * Mocks phone update API.
 */
export async function mockPhoneUpdateApi(page: Page, scenario: 'success' | 'error'): Promise<void> {
  await page.route('**/profiles.v1.ProfilesService/UpdateProfile', async (route) => {
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
        body: JSON.stringify({ code: 'internal', message: 'Failed to update phone' }),
      });
    }
  });
}

/**
 * Sets up authentication mocks for protected pages.
 * Mocks the ConnectRPC GetCurrentUser endpoint to simulate an authenticated session.
 */
export async function setupAuthMocks(page: Page): Promise<void> {
  // Mock GetCurrentUser - the endpoint called on app mount to check auth
  await page.route('**/auth.v1.AuthService/GetCurrentUser', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        user: MOCK_AUTH_USER,
        session: MOCK_AUTH_SESSION,
      }),
    });
  });

  // Mock Login endpoint in case it's called
  await page.route('**/auth.v1.AuthService/Login', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        user: MOCK_AUTH_USER,
        session: MOCK_AUTH_SESSION,
        csrfToken: 'mock-csrf-token',
      }),
    });
  });

  // Mock Logout endpoint
  await page.route('**/auth.v1.AuthService/Logout', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({}),
    });
  });
}

/**
 * Sets up unauthenticated mocks (user not logged in).
 */
export async function setupUnauthenticatedMocks(page: Page): Promise<void> {
  await page.route('**/auth.v1.AuthService/GetCurrentUser', async (route) => {
    await route.fulfill({
      status: 401,
      contentType: 'application/json',
      body: JSON.stringify({
        code: 'unauthenticated',
        message: 'Not authenticated',
      }),
    });
  });
}

export { expect } from '@playwright/test';
