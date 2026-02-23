import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Public Redirect page (/r/:slug).
 * Handles loading, error, and manual redirect states.
 */
export class PublicRedirectPage {
  readonly page: Page;

  // Loading
  readonly loadingSpinner: Locator;
  readonly loadingTitle: Locator;

  // Error
  readonly errorTitle: Locator;
  readonly errorMessage: Locator;
  readonly retryButton: Locator;
  readonly backButton: Locator;

  // Manual (in-app browser)
  readonly manualTitle: Locator;
  readonly openWhatsAppBtn: Locator;
  readonly copyLinkBtn: Locator;
  readonly openInviteLinkBtn: Locator;
  readonly tiktokHint: Locator;

  // Group name
  readonly groupNameText: Locator;

  // Toast
  readonly toast: Locator;

  constructor(page: Page) {
    this.page = page;

    // Loading
    this.loadingSpinner = page.locator('.animate-spin');
    this.loadingTitle = page.getByText(/redirecionando|redirecting/i);

    // Error
    this.errorTitle = page.getByText(/erro no redirecionamento|redirect error/i);
    this.errorMessage = page.locator('.text-destructive, [class*="text-red"]');
    this.retryButton = page.getByRole('button', {
      name: /tentar novamente|retry|try again/i,
    });
    this.backButton = page.getByRole('button', {
      name: /voltar|back|go back/i,
    });

    // Manual (in-app browser)
    this.manualTitle = page.getByText(/abra no whatsapp|open in whatsapp/i);
    this.openWhatsAppBtn = page.getByRole('button', {
      name: /abrir no whatsapp|open in whatsapp/i,
    });
    this.copyLinkBtn = page.getByRole('button', {
      name: /copiar link|copy.*link|copiar.*convite/i,
    });
    this.openInviteLinkBtn = page.getByRole('button', {
      name: /abrir link do convite|open invite link/i,
    });
    this.tiktokHint = page.getByText(/tiktok/i);

    // Group name
    this.groupNameText = page.locator('[class*="font-semibold"], [class*="font-bold"]');

    // Toast
    this.toast = page.locator('[data-sonner-toast]');
  }

  /**
   * Navigates to the public redirect page.
   */
  async goto(slug: string): Promise<void> {
    await this.page.goto(`/r/${slug}`);
  }

  /**
   * Waits for the loading state to appear.
   */
  async waitForLoading(): Promise<void> {
    await this.loadingSpinner
      .waitFor({ state: 'visible', timeout: 3000 })
      .catch(() => {});
  }

  /**
   * Waits for the error state to appear.
   */
  async waitForError(): Promise<void> {
    await this.errorTitle.waitFor({ state: 'visible', timeout: 10000 });
  }

  /**
   * Waits for the manual redirect state (in-app browser).
   */
  async waitForManual(): Promise<void> {
    await this.manualTitle.waitFor({ state: 'visible', timeout: 10000 });
  }
}
