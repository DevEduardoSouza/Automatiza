import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Profile page.
 * Encapsulates all profile page interactions.
 */
export class ProfilePage {
  readonly page: Page;

  // Locators - existing
  readonly pageTitle: Locator;
  readonly phoneInput: Locator;
  readonly phoneSaveButton: Locator;
  readonly currentPasswordInput: Locator;
  readonly newPasswordInput: Locator;
  readonly confirmPasswordInput: Locator;
  readonly passwordSaveButton: Locator;
  readonly emailDisplay: Locator;
  readonly planBadge: Locator;

  // Locators - sessions
  readonly sessionsSection: Locator;
  readonly sessionCards: Locator;
  readonly currentSessionBadge: Locator;
  readonly logoutAllButton: Locator;
  readonly revokeButtons: Locator;

  // Locators - dialogs
  readonly confirmDialog: Locator;
  readonly confirmDialogCancel: Locator;
  readonly confirmDialogAction: Locator;

  // Locators - password toggles
  readonly passwordToggleButtons: Locator;

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

    // Sessions
    this.sessionsSection = page.locator('[class*="rounded"]').filter({
      hasText: /sess|session/i,
    });
    this.sessionCards = page.locator('[class*="rounded"]').filter({
      has: page.locator('[class*="gap"]'),
    });
    this.currentSessionBadge = page.locator('span').filter({
      hasText: /atual|current/i,
    });
    this.logoutAllButton = page.locator('button').filter({
      hasText: /desconectar todos|logout all|sair de todos/i,
    });
    this.revokeButtons = page.locator('button').filter({
      has: page.locator('.sr-only'),
    });

    // Dialog
    this.confirmDialog = page.locator('[role="alertdialog"]');
    this.confirmDialogCancel = this.confirmDialog.locator('button').filter({
      hasText: /cancelar|cancel/i,
    });
    this.confirmDialogAction = this.confirmDialog.locator('button').filter({
      hasText: /confirmar|confirm|desconectar|logout|revogar|revoke/i,
    });

    // Password toggle buttons (eye icons)
    this.passwordToggleButtons = page.locator(
      'button[aria-label*="password" i], button[aria-label*="senha" i]'
    );
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

  /**
   * Gets the number of visible session cards (by counting revoke buttons + current badge).
   */
  async getSessionCount(): Promise<number> {
    // Count session items by looking for IP address text patterns
    const ipElements = this.page.locator('span').filter({ hasText: /\d+\.\d+\.\d+\.\d+/ });
    return ipElements.count();
  }

  /**
   * Revokes a session by clicking its revoke button (opens confirmation dialog).
   */
  async revokeSession(index: number): Promise<void> {
    const revokeBtn = this.revokeButtons.nth(index);
    await revokeBtn.click();
  }

  /**
   * Confirms the revoke action in the dialog.
   */
  async confirmRevoke(): Promise<void> {
    await this.confirmDialogAction.click();
  }

  /**
   * Cancels the revoke action in the dialog.
   */
  async cancelRevoke(): Promise<void> {
    await this.confirmDialogCancel.click();
  }

  /**
   * Triggers the "logout all devices" flow (opens confirmation dialog).
   */
  async logoutAllDevices(): Promise<void> {
    await this.logoutAllButton.click();
  }

  /**
   * Confirms the logout-all action in the dialog.
   */
  async confirmLogoutAll(): Promise<void> {
    await this.confirmDialogAction.click();
  }

  /**
   * Toggles password visibility for a given field.
   */
  async togglePasswordVisibility(
    field: 'current' | 'new' | 'confirm'
  ): Promise<void> {
    const fieldMap = {
      current: '#current_password',
      new: '#new_password',
      confirm: '#confirm_password',
    };
    const inputSelector = fieldMap[field];
    const container = this.page.locator(`${inputSelector}`).locator('..');
    const toggleBtn = container.locator(
      'button[aria-label*="password" i], button[aria-label*="senha" i]'
    );
    await toggleBtn.click();
  }

  /**
   * Checks if a password field is currently visible (type=text).
   */
  async isPasswordVisible(field: 'current' | 'new' | 'confirm'): Promise<boolean> {
    const fieldMap = {
      current: '#current_password',
      new: '#new_password',
      confirm: '#confirm_password',
    };
    const input = this.page.locator(fieldMap[field]);
    const type = await input.getAttribute('type');
    return type === 'text';
  }

  /**
   * Waits for the sessions section to finish loading.
   */
  async waitForSessionsLoad(): Promise<void> {
    // Wait for the spinner to disappear from sessions section
    await this.page.waitForFunction(() => {
      const spinners = document.querySelectorAll('.animate-spin');
      // Only loading spinners in the session area should be gone
      return spinners.length === 0;
    }, { timeout: 10000 });
  }
}
