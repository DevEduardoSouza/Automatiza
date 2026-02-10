import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Auth page.
 * Encapsulates all authentication page interactions.
 */
export class AuthPage {
  readonly page: Page;

  // Password login locators
  readonly emailInput: Locator;
  readonly passwordInput: Locator;
  readonly submitButton: Locator;
  readonly forgotPasswordLink: Locator;
  readonly magicLinkButton: Locator;
  readonly passwordToggle: Locator;
  readonly themeToggle: Locator;

  // Magic link locators
  readonly magicEmailInput: Locator;
  readonly magicSubmitButton: Locator;
  readonly backToPasswordButton: Locator;

  constructor(page: Page) {
    this.page = page;

    // Password form
    this.emailInput = page.locator('#login-email');
    this.passwordInput = page.locator('#login-password');
    this.submitButton = page.locator('form button[type="submit"]').first();
    this.forgotPasswordLink = page.locator('a[href="/forgot-password"]');
    this.magicLinkButton = page.locator('button').filter({ hasText: /magic link/i });
    this.passwordToggle = page
      .locator('button[aria-label*="password"]')
      .or(page.locator('#login-password').locator('..').locator('button'));
    this.themeToggle = page.locator('button[aria-label*="mode"]');

    // Magic link form
    this.magicEmailInput = page.locator('#magic-email');
    this.magicSubmitButton = page.locator('form button[type="submit"]');
    this.backToPasswordButton = page.locator('button').filter({ hasText: /voltar|back/i });
  }

  /**
   * Navigates to the auth page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/auth');
    await this.page.waitForSelector('#login-email', { state: 'visible' });
  }

  /**
   * Performs password login.
   */
  async login(email: string, password: string): Promise<void> {
    await this.emailInput.fill(email);
    await this.passwordInput.fill(password);
    await this.submitButton.click();
  }

  /**
   * Switches to magic link mode.
   */
  async switchToMagicLink(): Promise<void> {
    await this.magicLinkButton.click();
    await this.page.waitForSelector('#magic-email', { state: 'visible' });
  }

  /**
   * Switches back to password mode.
   */
  async switchToPassword(): Promise<void> {
    await this.backToPasswordButton.click();
    await this.page.waitForSelector('#login-password', { state: 'visible' });
  }

  /**
   * Sends a magic link.
   */
  async sendMagicLink(email: string): Promise<void> {
    await this.magicEmailInput.fill(email);
    await this.magicSubmitButton.click();
  }

  /**
   * Toggles password visibility.
   */
  async togglePasswordVisibility(): Promise<void> {
    await this.passwordToggle.click();
  }

  /**
   * Toggles theme.
   */
  async toggleTheme(): Promise<void> {
    await this.themeToggle.click();
  }

  /**
   * Navigates to forgot password page.
   */
  async goToForgotPassword(): Promise<void> {
    await this.forgotPasswordLink.click();
    await this.page.waitForURL('/forgot-password', { timeout: 5000 });
  }

  /**
   * Checks if the page is in password mode.
   */
  async isPasswordMode(): Promise<boolean> {
    return this.passwordInput.isVisible();
  }

  /**
   * Checks if the page is in magic link mode.
   */
  async isMagicLinkMode(): Promise<boolean> {
    return this.magicEmailInput.isVisible();
  }
}
