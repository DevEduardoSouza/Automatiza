import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Forgot Password page.
 * Encapsulates all forgot password page interactions.
 */
export class ForgotPasswordPage {
  readonly page: Page;

  // Form locators
  readonly pageTitle: Locator;
  readonly emailInput: Locator;
  readonly submitButton: Locator;
  readonly backToLoginLink: Locator;
  readonly resendButton: Locator;
  readonly successMessage: Locator;

  constructor(page: Page) {
    this.page = page;

    this.pageTitle = page.locator('h2');
    this.emailInput = page.locator('#forgot-email');
    this.submitButton = page.locator('button[type="submit"]');
    this.backToLoginLink = page
      .locator('a[href="/auth"]')
      .or(page.locator('button').filter({ hasText: /voltar|back/i }));
    this.resendButton = page.locator('button').filter({ hasText: /reenviar|resend/i });
    this.successMessage = page.locator('text=/enviado|sent/i');
  }

  /**
   * Navigates to the forgot password page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/forgot-password');
    await this.page.waitForSelector('#forgot-email', { state: 'visible' });
  }

  /**
   * Submits the forgot password form.
   */
  async submitEmail(email: string): Promise<void> {
    await this.emailInput.fill(email);
    await this.submitButton.click();
  }

  /**
   * Waits for the success state.
   */
  async waitForSuccess(): Promise<void> {
    await this.page.waitForSelector('text=/enviado|sent/i', { timeout: 5000 });
  }

  /**
   * Clicks the resend button.
   */
  async clickResend(): Promise<void> {
    await this.resendButton.click();
    await this.page.waitForSelector('#forgot-email', { state: 'visible' });
  }

  /**
   * Navigates back to login.
   */
  async goBackToLogin(): Promise<void> {
    await this.backToLoginLink.first().click();
    await this.page.waitForURL('/auth', { timeout: 5000 });
  }

  /**
   * Checks if success state is shown.
   */
  async isSuccessStateVisible(): Promise<boolean> {
    return this.successMessage.first().isVisible();
  }

  /**
   * Checks if form is visible.
   */
  async isFormVisible(): Promise<boolean> {
    return this.emailInput.isVisible();
  }
}
