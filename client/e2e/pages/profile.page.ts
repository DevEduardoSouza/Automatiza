import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Profile page.
 * Encapsulates all profile page interactions.
 */
export class ProfilePage {
  readonly page: Page;

  // Locators
  readonly pageTitle: Locator;
  readonly phoneInput: Locator;
  readonly phoneSaveButton: Locator;
  readonly currentPasswordInput: Locator;
  readonly newPasswordInput: Locator;
  readonly confirmPasswordInput: Locator;
  readonly passwordSaveButton: Locator;
  readonly emailDisplay: Locator;
  readonly planBadge: Locator;

  constructor(page: Page) {
    this.page = page;

    // Page elements
    this.pageTitle = page.locator('h1');
    this.phoneInput = page.locator('#phone_number');
    this.phoneSaveButton = page
      .locator('button')
      .filter({ hasText: /salvar|save/i })
      .first();

    this.currentPasswordInput = page.locator('#current_password');
    this.newPasswordInput = page.locator('#new_password');
    this.confirmPasswordInput = page.locator('#confirm_password');
    this.passwordSaveButton = page
      .locator('button')
      .filter({ hasText: /alterar senha|change password|salvar/i })
      .last();

    this.emailDisplay = page.locator('text=Email').first();
    this.planBadge = page.locator('[class*="badge"]').first();
  }

  /**
   * Navigates to the profile page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/profile');
    await this.page.waitForSelector('h1', { state: 'visible' });
  }

  /**
   * Updates the phone number.
   */
  async updatePhone(phoneNumber: string): Promise<void> {
    await this.phoneInput.clear();
    await this.phoneInput.fill(phoneNumber);
    await this.phoneSaveButton.click();
  }

  /**
   * Changes the password.
   */
  async changePassword(
    currentPassword: string,
    newPassword: string,
    confirmPassword: string
  ): Promise<void> {
    await this.currentPasswordInput.fill(currentPassword);
    await this.newPasswordInput.fill(newPassword);
    await this.confirmPasswordInput.fill(confirmPassword);
    await this.passwordSaveButton.click();
  }

  /**
   * Clears all password fields.
   */
  async clearPasswordFields(): Promise<void> {
    await this.currentPasswordInput.clear();
    await this.newPasswordInput.clear();
    await this.confirmPasswordInput.clear();
  }

  /**
   * Checks if the page is loaded.
   */
  async isLoaded(): Promise<boolean> {
    return this.pageTitle.isVisible();
  }

  /**
   * Waits for the page to be fully loaded.
   */
  async waitForLoad(): Promise<void> {
    await this.page.waitForSelector('#phone_number', { state: 'visible' });
    await this.page.waitForSelector('#current_password', { state: 'visible' });
  }
}
