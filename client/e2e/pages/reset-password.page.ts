import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Reset Password page.
 * Encapsulates all reset password page interactions.
 */
export class ResetPasswordPage {
  readonly page: Page;

  // Form locators
  readonly pageTitle: Locator;
  readonly passwordInput: Locator;
  readonly confirmPasswordInput: Locator;
  readonly submitButton: Locator;
  readonly loadingSkeleton: Locator;
  readonly errorMessage: Locator;

  constructor(page: Page) {
    this.page = page;

    this.pageTitle = page.locator('h1, h2').filter({ hasText: /redefinir|reset|nova senha/i });
    this.passwordInput = page.locator('input[name="password"]');
    this.confirmPasswordInput = page.locator('input[name="confirmPassword"]');
    this.submitButton = page.locator('button[type="submit"]');
    this.loadingSkeleton = page.locator('[class*="skeleton"]');
    this.errorMessage = page.locator('[class*="error"], [role="alert"]');
  }

  /**
   * Navigates to the reset password page.
   * Note: This requires a valid session token.
   */
  async goto(): Promise<void> {
    await this.page.goto('/reset-password');
  }

  /**
   * Waits for the form to be visible.
   */
  async waitForForm(): Promise<void> {
    await this.page.waitForSelector('input[name="password"]', {
      state: 'visible',
      timeout: 5000,
    });
  }

  /**
   * Submits the reset password form.
   */
  async submitNewPassword(password: string, confirmPassword: string): Promise<void> {
    await this.passwordInput.fill(password);
    await this.confirmPasswordInput.fill(confirmPassword);
    await this.submitButton.click();
  }

  /**
   * Waits for redirect to auth page after success.
   */
  async waitForSuccessRedirect(): Promise<void> {
    await this.page.waitForURL('/auth', { timeout: 10000 });
  }

  /**
   * Checks if loading state is visible.
   */
  async isLoading(): Promise<boolean> {
    return this.loadingSkeleton.first().isVisible();
  }

  /**
   * Checks if form is visible.
   */
  async isFormVisible(): Promise<boolean> {
    return this.passwordInput.isVisible();
  }

  /**
   * Gets validation error message.
   */
  async getValidationError(): Promise<string | null> {
    const errorElement = this.page.locator('[class*="text-red"], [class*="error"]').first();
    if (await errorElement.isVisible()) {
      return errorElement.textContent();
    }
    return null;
  }
}
