import { expect, test } from '@playwright/test';
import { TEST_USER, mockPasswordResetEmail } from './fixtures/auth.fixture';

test.describe('Authentication Flow', () => {
  test.describe('Login Page', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/auth');
    });

    test('should display login form', async ({ page }) => {
      // Verify email input
      const emailInput = page.locator('#login-email');
      await expect(emailInput).toBeVisible();
      await expect(emailInput).toHaveAttribute('type', 'email');

      // Verify password input
      const passwordInput = page.locator('#login-password');
      await expect(passwordInput).toBeVisible();
      await expect(passwordInput).toHaveAttribute('type', 'password');

      // Verify submit button
      const submitButton = page.locator('button[type="submit"]');
      await expect(submitButton).toBeVisible();

      // Verify forgot password link
      const forgotLink = page.locator('a[href="/forgot-password"]');
      await expect(forgotLink).toBeVisible();
    });

    test('should toggle password visibility', async ({ page }) => {
      const passwordInput = page.locator('#login-password');
      const toggleButton = page
        .locator('button')
        .filter({ has: page.locator('svg') })
        .last();

      // Initially password should be hidden
      await expect(passwordInput).toHaveAttribute('type', 'password');

      // Click toggle button
      await toggleButton.click();

      // Password should be visible
      await expect(passwordInput).toHaveAttribute('type', 'text');

      // Click again to hide
      await toggleButton.click();

      // Password should be hidden again
      await expect(passwordInput).toHaveAttribute('type', 'password');
    });

    test('should show validation error for empty fields', async ({ page }) => {
      // Try to submit without filling fields
      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Form should not navigate away (still on auth page)
      await expect(page).toHaveURL(/\/auth/);
    });

    test('should navigate to forgot password page', async ({ page }) => {
      const forgotLink = page.locator('a[href="/forgot-password"]');
      await forgotLink.click();

      await page.waitForURL('/forgot-password', { timeout: 5000 });
    });

    test('should switch to magic link mode', async ({ page }) => {
      // Find and click magic link button
      const magicLinkButton = page.locator('button').filter({ hasText: /magic link/i });
      await magicLinkButton.click();

      // Verify magic link form is shown
      const magicEmailInput = page.locator('#magic-email');
      await expect(magicEmailInput).toBeVisible();

      // Verify back button is shown
      const backButton = page.locator('button').filter({ hasText: /voltar|back/i });
      await expect(backButton).toBeVisible();
    });

    test('should switch back to password mode from magic link', async ({ page }) => {
      // Switch to magic link mode
      const magicLinkButton = page.locator('button').filter({ hasText: /magic link/i });
      await magicLinkButton.click();

      // Switch back to password mode
      const backButton = page.locator('button').filter({ hasText: /voltar|back/i });
      await backButton.click();

      // Verify password form is shown again
      const passwordInput = page.locator('#login-password');
      await expect(passwordInput).toBeVisible();
    });

    test('should toggle theme', async ({ page }) => {
      const themeButton = page
        .locator('button')
        .filter({ has: page.locator('svg') })
        .first();

      // Click theme toggle
      await themeButton.click();

      // Theme should have changed (verify via class or data attribute)
      const html = page.locator('html');
      const isDark = await html.evaluate((el) => el.classList.contains('dark'));

      // Toggle again
      await themeButton.click();

      const isDarkAfter = await html.evaluate((el) => el.classList.contains('dark'));

      // Theme should have changed
      expect(isDark).not.toBe(isDarkAfter);
    });

    test('should disable submit button while loading', async ({ page }) => {
      // Mock slow auth response
      await page.route('**/auth/v1/token**', async (route) => {
        await new Promise((resolve) => setTimeout(resolve, 1000));
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            access_token: 'mock-token',
            user: { id: 'mock-id' },
          }),
        });
      });

      // Fill form
      await page.locator('#login-email').fill(TEST_USER.email);
      await page.locator('#login-password').fill(TEST_USER.password);

      // Submit
      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Button should be disabled during loading
      await expect(submitButton).toBeDisabled();
    });
  });

  test.describe('Magic Link Flow', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/auth');
      // Switch to magic link mode
      const magicLinkButton = page.locator('button').filter({ hasText: /magic link/i });
      await magicLinkButton.click();
    });

    test('should send magic link successfully', async ({ page }) => {
      // Mock magic link API
      await page.route('**/auth/v1/magiclink**', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({}),
        });
      });

      // Fill email
      const emailInput = page.locator('#magic-email');
      await emailInput.fill(TEST_USER.email);

      // Submit
      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Success state should be shown
      await page.waitForSelector('text=/enviado|sent|success/i', { timeout: 5000 });
    });

    test('should show validation error for invalid email', async ({ page }) => {
      const emailInput = page.locator('#magic-email');
      await emailInput.fill('invalid-email');

      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Should remain on same page
      await expect(emailInput).toBeVisible();
    });
  });

  test.describe('Complete Auth Integration', () => {
    test('should complete forgot password flow from login', async ({ page }) => {
      await mockPasswordResetEmail(page);

      // Start at login
      await page.goto('/auth');

      // Click forgot password
      const forgotLink = page.locator('a[href="/forgot-password"]');
      await forgotLink.click();

      // Wait for forgot password page
      await page.waitForURL('/forgot-password', { timeout: 5000 });

      // Fill email and submit
      const emailInput = page.locator('#forgot-email');
      await emailInput.fill(TEST_USER.email);

      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Wait for success state
      await page.waitForSelector('text=/enviado|sent/i', { timeout: 5000 });

      // Navigate back to login
      const backButton = page.locator('button').filter({ hasText: /voltar|back/i });
      await backButton.click();

      // Should be back at login
      await page.waitForURL('/auth', { timeout: 5000 });
    });
  });
});
