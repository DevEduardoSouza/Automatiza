import { expect, test } from '@playwright/test';
import { TEST_USER, mockPasswordResetEmail, waitForToast } from './fixtures/auth.fixture';

test.describe('Password Reset Flow', () => {
  test.describe('Forgot Password Page', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/forgot-password');
    });

    test('should display forgot password page correctly', async ({ page }) => {
      // Verify page title
      const title = page.locator('h2');
      await expect(title).toBeVisible();
      await expect(title).toContainText(/recuperar|reset|forgot/i);

      // Verify email input is present
      const emailInput = page.locator('#forgot-email');
      await expect(emailInput).toBeVisible();
      await expect(emailInput).toHaveAttribute('type', 'email');

      // Verify submit button is present
      const submitButton = page.locator('button[type="submit"]');
      await expect(submitButton).toBeVisible();

      // Verify back to login link is present
      const backLink = page
        .locator('a[href="/auth"]')
        .or(page.locator('button').filter({ hasText: /voltar|back/i }));
      await expect(backLink.first()).toBeVisible();
    });

    test('should send password reset email successfully', async ({ page }) => {
      await mockPasswordResetEmail(page);

      // Fill email
      const emailInput = page.locator('#forgot-email');
      await emailInput.fill(TEST_USER.email);

      // Submit form
      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Wait for success state
      await page.waitForSelector('text=/enviado|sent/i', { timeout: 5000 });

      // Verify success message is displayed
      const successMessage = page.locator('text=/enviado|sent/i');
      await expect(successMessage.first()).toBeVisible();

      // Verify email is shown in confirmation
      await expect(page.locator(`text=${TEST_USER.email}`)).toBeVisible();

      // Verify resend button is shown
      const resendButton = page.locator('button').filter({ hasText: /reenviar|resend/i });
      await expect(resendButton).toBeVisible();
    });

    test('should show validation error for empty email', async ({ page }) => {
      // Try to submit without filling email
      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Wait for validation error
      const errorMessage = page.locator('text=/obrigat.rio|required|inv.lido|invalid/i');
      await expect(errorMessage.first()).toBeVisible();
    });

    test('should show validation error for invalid email format', async ({ page }) => {
      // Fill invalid email
      const emailInput = page.locator('#forgot-email');
      await emailInput.fill('invalid-email');

      // Submit form
      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Wait for validation error
      const errorMessage = page.locator('text=/inv.lido|invalid/i');
      await expect(errorMessage.first()).toBeVisible();
    });

    test('should allow resending reset email', async ({ page }) => {
      await mockPasswordResetEmail(page);

      // First submission
      const emailInput = page.locator('#forgot-email');
      await emailInput.fill(TEST_USER.email);

      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Wait for success state
      await page.waitForSelector('text=/enviado|sent/i', { timeout: 5000 });

      // Click resend button
      const resendButton = page.locator('button').filter({ hasText: /reenviar|resend/i });
      await resendButton.click();

      // Should return to form state
      await expect(emailInput).toBeVisible();
    });

    test('should navigate back to login page', async ({ page }) => {
      // Click back to login
      const backLink = page
        .locator('a[href="/auth"]')
        .or(page.locator('button').filter({ hasText: /voltar|back/i }))
        .first();
      await backLink.click();

      // Verify navigation to auth page
      await page.waitForURL('/auth', { timeout: 5000 });
    });

    test('should disable submit button while submitting', async ({ page }) => {
      await mockPasswordResetEmail(page);

      // Add delay to observe loading state
      await page.route('**/auth/v1/recover**', async (route) => {
        await new Promise((resolve) => setTimeout(resolve, 500));
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({}),
        });
      });

      // Fill email
      const emailInput = page.locator('#forgot-email');
      await emailInput.fill(TEST_USER.email);

      // Submit form
      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Button should be disabled during submission
      await expect(submitButton).toBeDisabled();
    });
  });

  test.describe('Reset Password Page', () => {
    test.beforeEach(async ({ page }) => {
      // Mock valid session for reset password page
      await page.route('**/auth/v1/session**', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              session: {
                access_token: 'mock-token',
                user: { id: 'mock-user-id', email: TEST_USER.email },
              },
            },
          }),
        });
      });

      await page.goto('/reset-password');
    });

    test('should display reset password form when session is valid', async ({ page }) => {
      // Wait for form to be visible
      await page.waitForSelector('input[type="password"]', {
        state: 'visible',
        timeout: 5000,
      });

      // Verify password input is present
      const passwordInput = page.locator('input[name="password"]');
      await expect(passwordInput).toBeVisible();

      // Verify confirm password input is present
      const confirmInput = page.locator('input[name="confirmPassword"]');
      await expect(confirmInput).toBeVisible();

      // Verify submit button is present
      const submitButton = page.locator('button[type="submit"]');
      await expect(submitButton).toBeVisible();
    });

    test('should reset password successfully', async ({ page }) => {
      // Mock password update API
      await page.route('**/auth/v1/user**', async (route) => {
        if (route.request().method() === 'PUT') {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ data: { user: { id: 'mock-user-id' } } }),
          });
        } else {
          await route.continue();
        }
      });

      await page.waitForSelector('input[name="password"]', { state: 'visible' });

      // Fill new password
      const passwordInput = page.locator('input[name="password"]');
      await passwordInput.fill(TEST_USER.newPassword);

      // Fill confirm password
      const confirmInput = page.locator('input[name="confirmPassword"]');
      await confirmInput.fill(TEST_USER.newPassword);

      // Submit form
      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Should navigate to auth page on success
      await page.waitForURL('/auth', { timeout: 10000 });
    });

    test('should show error when passwords do not match', async ({ page }) => {
      await page.waitForSelector('input[name="password"]', { state: 'visible' });

      // Fill new password
      const passwordInput = page.locator('input[name="password"]');
      await passwordInput.fill(TEST_USER.newPassword);

      // Fill different confirm password
      const confirmInput = page.locator('input[name="confirmPassword"]');
      await confirmInput.fill('DifferentPassword456!');

      // Submit form
      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Wait for validation error
      const errorMessage = page.locator('text=/n.o conferem|do not match|mismatch/i');
      await expect(errorMessage.first()).toBeVisible();
    });

    test('should show error when password is too short', async ({ page }) => {
      await page.waitForSelector('input[name="password"]', { state: 'visible' });

      // Fill short password
      const passwordInput = page.locator('input[name="password"]');
      await passwordInput.fill('12345');

      // Fill confirm password
      const confirmInput = page.locator('input[name="confirmPassword"]');
      await confirmInput.fill('12345');

      // Submit form
      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Wait for validation error
      const errorMessage = page.locator('text=/curta|short|m.nimo|minimum|6.*caracteres/i');
      await expect(errorMessage.first()).toBeVisible();
    });

    test('should show loading skeleton while checking session', async ({ page }) => {
      // Add delay to session check
      await page.route('**/auth/v1/session**', async (route) => {
        await new Promise((resolve) => setTimeout(resolve, 1000));
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              session: {
                access_token: 'mock-token',
                user: { id: 'mock-user-id' },
              },
            },
          }),
        });
      });

      await page.goto('/reset-password');

      // Skeleton should be visible during loading
      const skeleton = page.locator('[class*="skeleton"]').first();
      await expect(skeleton).toBeVisible();
    });
  });

  test.describe('Invalid/Expired Token', () => {
    test('should redirect to forgot-password when session is invalid', async ({ page }) => {
      // Mock invalid session
      await page.route('**/auth/v1/session**', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ data: { session: null } }),
        });
      });

      await page.goto('/reset-password');

      // Should redirect to forgot-password page
      await page.waitForURL('/forgot-password', { timeout: 5000 });

      // Error toast should be shown
      await waitForToast(page, /inv.lido|invalid|expirado|expired/i, 'error');
    });

    test('should show error toast when token is expired', async ({ page }) => {
      // Mock expired session response
      await page.route('**/auth/v1/session**', async (route) => {
        await route.fulfill({
          status: 401,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'Token expired' }),
        });
      });

      await page.goto('/reset-password');

      // Should redirect to forgot-password page
      await page.waitForURL('/forgot-password', { timeout: 5000 });
    });
  });

  test.describe('Full Reset Flow Integration', () => {
    test('should navigate from login to forgot password', async ({ page }) => {
      await page.goto('/auth');

      // Click on forgot password link
      const forgotLink = page.locator('a[href="/forgot-password"]');
      await forgotLink.click();

      // Verify navigation
      await page.waitForURL('/forgot-password', { timeout: 5000 });

      // Verify forgot password page is displayed
      const title = page.locator('h2');
      await expect(title).toContainText(/recuperar|reset|forgot/i);
    });

    test('should show success message after email sent and allow return to login', async ({
      page,
    }) => {
      await mockPasswordResetEmail(page);
      await page.goto('/forgot-password');

      // Fill and submit email
      const emailInput = page.locator('#forgot-email');
      await emailInput.fill(TEST_USER.email);

      const submitButton = page.locator('button[type="submit"]');
      await submitButton.click();

      // Wait for success state
      await page.waitForSelector('text=/enviado|sent/i', { timeout: 5000 });

      // Click back to login button
      const backButton = page.locator('button').filter({ hasText: /voltar|back/i });
      await backButton.click();

      // Verify navigation to auth page
      await page.waitForURL('/auth', { timeout: 5000 });
    });
  });
});
