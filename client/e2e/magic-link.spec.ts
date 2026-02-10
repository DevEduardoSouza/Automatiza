import { expect, test } from '@playwright/test';
import { TEST_USER, waitForToast } from './fixtures/auth.fixture';

/**
 * E2E tests for Magic Link authentication flow.
 * Covers sending magic links and verifying tokens.
 */
test.describe('Magic Link Authentication', () => {
  test.describe('Sending Magic Link', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/auth');
    });

    test('should switch to magic link mode', async ({ page }) => {
      // Find and click magic link button
      const magicLinkButton = page.locator('button').filter({ hasText: /magic link|e-mail/i });
      await magicLinkButton.click();

      // Verify magic link form is shown
      const magicEmailInput = page.locator('#magic-email');
      await expect(magicEmailInput).toBeVisible();

      // Verify title changed
      const title = page.locator('h2');
      await expect(title).toContainText(/link m.gico|magic link/i);

      // Verify back button is shown
      const backButton = page.locator('button').filter({ hasText: /voltar|back/i });
      await expect(backButton).toBeVisible();
    });

    test('should validate email format', async ({ page }) => {
      // Switch to magic link mode
      const magicLinkButton = page.locator('button').filter({ hasText: /magic link|e-mail/i });
      await magicLinkButton.click();

      // Fill invalid email
      const emailInput = page.locator('#magic-email');
      await emailInput.fill('invalid-email');

      // Submit form
      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Should remain on same page (native validation)
      await expect(emailInput).toBeVisible();
      await expect(page).toHaveURL(/\/auth/);
    });

    test('should show success after sending magic link', async ({ page }) => {
      // Mock SendMagicLink RPC
      await page.route('**/auth.v1.AuthService/SendMagicLink', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from([]),
        });
      });

      // Switch to magic link mode
      const magicLinkButton = page.locator('button').filter({ hasText: /magic link|e-mail/i });
      await magicLinkButton.click();

      // Fill email
      const emailInput = page.locator('#magic-email');
      await emailInput.fill(TEST_USER.email);

      // Submit form
      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Wait for success state
      await page.waitForSelector('text=/enviado|sent/i', { timeout: 5000 });

      // Verify success message
      const successTitle = page.locator('text=/enviado|sent/i');
      await expect(successTitle.first()).toBeVisible();

      // Verify email is shown in confirmation
      await expect(page.locator(`text=${TEST_USER.email}`)).toBeVisible();
    });

    test('should handle send errors', async ({ page }) => {
      // Mock SendMagicLink RPC error
      await page.route('**/auth.v1.AuthService/SendMagicLink', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '8', // RESOURCE_EXHAUSTED (rate limit)
            'grpc-message': 'Rate limit exceeded',
          },
          body: Buffer.from([]),
        });
      });

      // Switch to magic link mode
      const magicLinkButton = page.locator('button').filter({ hasText: /magic link|e-mail/i });
      await magicLinkButton.click();

      // Fill email
      const emailInput = page.locator('#magic-email');
      await emailInput.fill(TEST_USER.email);

      // Submit form
      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Should show error toast
      await waitForToast(page, /erro|error|falha|failed/i, 'error');
    });

    test('should allow switching back to password mode', async ({ page }) => {
      // Switch to magic link mode
      const magicLinkButton = page.locator('button').filter({ hasText: /magic link|e-mail/i });
      await magicLinkButton.click();

      // Verify in magic link mode
      const magicEmailInput = page.locator('#magic-email');
      await expect(magicEmailInput).toBeVisible();

      // Switch back to password mode
      const backButton = page.locator('button').filter({ hasText: /voltar|back/i });
      await backButton.click();

      // Verify password form is shown again
      const passwordInput = page.locator('#login-password');
      await expect(passwordInput).toBeVisible();
    });

    test('should disable submit button while sending', async ({ page }) => {
      // Mock slow response
      await page.route('**/auth.v1.AuthService/SendMagicLink', async (route) => {
        await new Promise((resolve) => setTimeout(resolve, 1000));
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from([]),
        });
      });

      // Switch to magic link mode
      const magicLinkButton = page.locator('button').filter({ hasText: /magic link|e-mail/i });
      await magicLinkButton.click();

      // Fill email
      const emailInput = page.locator('#magic-email');
      await emailInput.fill(TEST_USER.email);

      // Submit form
      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Button should be disabled during submission
      await expect(submitButton).toBeDisabled();

      // Button text should change to "Enviando..."
      await expect(submitButton).toContainText(/enviando|sending/i);
    });
  });

  test.describe('Verifying Magic Link Token', () => {
    test('should show loading while verifying', async ({ page }) => {
      // Mock slow VerifyMagicLink RPC
      await page.route('**/auth.v1.AuthService/VerifyMagicLink', async (route) => {
        await new Promise((resolve) => setTimeout(resolve, 2000));
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from([]),
        });
      });

      // Navigate to magic link verification page with token
      await page.goto('/auth/magic?token=valid-test-token');

      // Verify loading state
      const loadingSpinner = page.locator('svg.animate-spin');
      await expect(loadingSpinner).toBeVisible();

      // Verify loading text
      const loadingText = page.locator('text=/verificando|verifying/i');
      await expect(loadingText.first()).toBeVisible();
    });

    test('should redirect to dashboard on success', async ({ page }) => {
      // Mock successful VerifyMagicLink RPC
      await page.route('**/auth.v1.AuthService/VerifyMagicLink', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '0',
            'Set-Cookie': 'session_id=mock-session; Path=/; HttpOnly',
          },
          body: Buffer.from([]),
        });
      });

      // Mock GetCurrentUser to return user data
      await page.route('**/auth.v1.AuthService/GetCurrentUser', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from([]),
        });
      });

      // Navigate to magic link verification page with token
      await page.goto('/auth/magic?token=valid-test-token');

      // Should redirect to dashboard
      await page.waitForURL(/\/dashboard/, { timeout: 10000 });
    });

    test('should show error for invalid token', async ({ page }) => {
      // Mock failed VerifyMagicLink RPC
      await page.route('**/auth.v1.AuthService/VerifyMagicLink', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '3', // INVALID_ARGUMENT
            'grpc-message': 'Invalid or expired token',
          },
          body: Buffer.from([]),
        });
      });

      // Navigate to magic link verification page with invalid token
      await page.goto('/auth/magic?token=invalid-token');

      // Verify error state
      const errorIcon = page.locator('svg').filter({ has: page.locator('[class*="text-red"]') });
      await expect(errorIcon.first()).toBeVisible();

      // Verify error message
      const errorMessage = page.locator('text=/inv.lido|invalid|expirado|expired/i');
      await expect(errorMessage.first()).toBeVisible();
    });

    test('should show error for expired token', async ({ page }) => {
      // Mock failed VerifyMagicLink RPC (expired)
      await page.route('**/auth.v1.AuthService/VerifyMagicLink', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '3', // INVALID_ARGUMENT
            'grpc-message': 'Token expired',
          },
          body: Buffer.from([]),
        });
      });

      // Navigate to magic link verification page with expired token
      await page.goto('/auth/magic?token=expired-token');

      // Verify error message (same as invalid - security through obscurity)
      const errorMessage = page.locator('text=/inv.lido|invalid|expirado|expired/i');
      await expect(errorMessage.first()).toBeVisible();
    });

    test('should show error for missing token', async ({ page }) => {
      // Navigate to magic link verification page without token
      await page.goto('/auth/magic');

      // Verify error state without making API call
      const errorMessage = page.locator('text=/inv.lido|invalid/i');
      await expect(errorMessage.first()).toBeVisible();
    });

    test('should allow returning to login from error', async ({ page }) => {
      // Mock failed VerifyMagicLink RPC
      await page.route('**/auth.v1.AuthService/VerifyMagicLink', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '3', // INVALID_ARGUMENT
            'grpc-message': 'Invalid token',
          },
          body: Buffer.from([]),
        });
      });

      // Navigate to magic link verification page with invalid token
      await page.goto('/auth/magic?token=invalid-token');

      // Wait for error state
      await page.waitForSelector('text=/inv.lido|invalid/i', { timeout: 5000 });

      // Click back to login button
      const backButton = page.locator('a[href="/auth"]');
      await backButton.click();

      // Should navigate to login page
      await page.waitForURL('/auth', { timeout: 5000 });
    });
  });

  test.describe('Full Magic Link Flow', () => {
    test('should complete magic link flow from login page', async ({ page }) => {
      // Mock SendMagicLink RPC
      await page.route('**/auth.v1.AuthService/SendMagicLink', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from([]),
        });
      });

      // Start at login page
      await page.goto('/auth');

      // Switch to magic link mode
      const magicLinkButton = page.locator('button').filter({ hasText: /magic link|e-mail/i });
      await magicLinkButton.click();

      // Fill email
      const emailInput = page.locator('#magic-email');
      await emailInput.fill(TEST_USER.email);

      // Submit form
      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Wait for success state
      await page.waitForSelector('text=/enviado|sent/i', { timeout: 5000 });

      // Verify success message shows email
      await expect(page.locator(`text=${TEST_USER.email}`)).toBeVisible();

      // Return to password login
      const backButton = page.locator('button').filter({ hasText: /voltar|back/i });
      await backButton.click();

      // Verify password form is shown
      const passwordInput = page.locator('#login-password');
      await expect(passwordInput).toBeVisible();
    });
  });
});
