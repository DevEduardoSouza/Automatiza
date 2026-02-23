import { expect, test } from '@playwright/test';
import {
  MOCK_PROFILE_DATA,
  setupProfilePageMocks,
} from './fixtures/profile.fixture';
import { ProfilePage } from './pages/profile.page';

test.describe('Profile - Password Visibility', () => {
  let profilePage: ProfilePage;

  test.beforeEach(async ({ page }) => {
    profilePage = new ProfilePage(page);
    await setupProfilePageMocks(page);
    await page.goto('/profile');
    await profilePage.waitForLoad();
  });

  test('toggles current password visibility', async ({ page }) => {
    const input = page.locator('#current_password');
    await expect(input).toHaveAttribute('type', 'password');

    await profilePage.togglePasswordVisibility('current');
    await expect(input).toHaveAttribute('type', 'text');

    await profilePage.togglePasswordVisibility('current');
    await expect(input).toHaveAttribute('type', 'password');
  });

  test('toggles new password visibility', async ({ page }) => {
    const input = page.locator('#new_password');
    await expect(input).toHaveAttribute('type', 'password');

    await profilePage.togglePasswordVisibility('new');
    await expect(input).toHaveAttribute('type', 'text');

    await profilePage.togglePasswordVisibility('new');
    await expect(input).toHaveAttribute('type', 'password');
  });

  test('toggles confirm password visibility', async ({ page }) => {
    const input = page.locator('#confirm_password');
    await expect(input).toHaveAttribute('type', 'password');

    await profilePage.togglePasswordVisibility('confirm');
    await expect(input).toHaveAttribute('type', 'text');

    await profilePage.togglePasswordVisibility('confirm');
    await expect(input).toHaveAttribute('type', 'password');
  });

  test('toggles are independent', async ({ page }) => {
    const currentInput = page.locator('#current_password');
    const newInput = page.locator('#new_password');
    const confirmInput = page.locator('#confirm_password');

    // Toggle only current
    await profilePage.togglePasswordVisibility('current');

    await expect(currentInput).toHaveAttribute('type', 'text');
    await expect(newInput).toHaveAttribute('type', 'password');
    await expect(confirmInput).toHaveAttribute('type', 'password');

    // Toggle new as well
    await profilePage.togglePasswordVisibility('new');

    await expect(currentInput).toHaveAttribute('type', 'text');
    await expect(newInput).toHaveAttribute('type', 'text');
    await expect(confirmInput).toHaveAttribute('type', 'password');
  });
});

test.describe('Profile - Account Info Display', () => {
  let profilePage: ProfilePage;

  test.beforeEach(async ({ page }) => {
    profilePage = new ProfilePage(page);
    await setupProfilePageMocks(page);
    await page.goto('/profile');
    await profilePage.waitForLoad();
  });

  test('displays plan badge uppercase', async ({ page }) => {
    const badge = page.locator('span').filter({ hasText: 'PRO' });
    await expect(badge).toBeVisible();
  });

  test('displays automations limit text', async ({ page }) => {
    // Should show something like "5/10 automations" or translated equivalent
    const limitsText = page.locator('span').filter({
      hasText: /\d+\/\d+|automa/i,
    });
    await expect(limitsText.first()).toBeVisible();
  });

  test('displays user ID', async ({ page }) => {
    const userId = page.locator('.font-mono').filter({
      hasText: MOCK_PROFILE_DATA.id,
    });
    await expect(userId).toBeVisible();
  });

  test('displays WhatsApp instances count', async ({ page }) => {
    // Should show "2" somewhere for connected instances
    const instancesText = page.getByText(/2.*inst/i);
    await expect(instancesText).toBeVisible();
  });

  test('displays email', async ({ page }) => {
    await expect(page.getByText(MOCK_PROFILE_DATA.email)).toBeVisible();
  });
});

test.describe('Profile - Loading State', () => {
  test('shows spinner during delayed API response', async ({ page }) => {
    const { setupAuthMocks } = await import('./fixtures/auth.fixture');
    await setupAuthMocks(page);

    // Delay the profile response
    await page.route('**/profiles.v1.ProfilesService/GetProfile', async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 2000));
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ profile: MOCK_PROFILE_DATA }),
      });
    });

    // Also delay limits
    await page.route('**/profiles.v1.ProfilesService/GetUserLimits', async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 2000));
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ limits: { automationsCount: 5, maxAutomations: 10, connectedInstancesCount: 2 } }),
      });
    });

    await page.goto('/profile');

    // Loading spinner should be visible
    const spinner = page.locator('.animate-spin');
    await expect(spinner.first()).toBeVisible({ timeout: 3000 });

    // Wait for content to load
    await page.waitForSelector('#phone_number', { state: 'visible', timeout: 10000 });

    // Spinner should be gone
    const loadingText = page.locator('text=/carregando|loading/i');
    await expect(loadingText).not.toBeVisible();
  });
});
