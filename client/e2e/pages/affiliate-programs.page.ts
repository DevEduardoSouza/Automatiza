import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Affiliate Programs page.
 * Encapsulates all affiliate program page interactions.
 */
export class AffiliateProgramsPage {
  readonly page: Page;

  // Page header
  readonly pageTitle: Locator;
  readonly pageSubtitle: Locator;

  // Store cards container
  readonly storeCardsGrid: Locator;

  constructor(page: Page) {
    this.page = page;

    this.pageTitle = page.locator('h1');
    this.pageSubtitle = page.locator('.text-muted-foreground').first();
    this.storeCardsGrid = page.locator('.grid.gap-6');
  }

  /**
   * Navigates to the affiliate programs page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/affiliate-programs');
    await this.page.waitForSelector('h1', { state: 'visible' });
  }

  /**
   * Gets a store card by store name.
   */
  getStoreCard(storeName: string): Locator {
    return this.page
      .locator('[data-testid="store-card"]')
      .filter({ hasText: storeName })
      .or(this.page.locator('.card, [class*="Card"]').filter({ hasText: storeName }));
  }

  /**
   * Gets the status badge for a store.
   */
  getStoreBadge(storeName: string): Locator {
    return this.getStoreCard(storeName).locator('[class*="Badge"], .badge');
  }

  /**
   * Checks if a store is marked as active.
   */
  async isStoreActive(storeName: string): Promise<boolean> {
    const badge = this.getStoreBadge(storeName);
    const text = await badge.textContent();
    return text?.toLowerCase().includes('ativ') || text?.toLowerCase().includes('active') || false;
  }

  /**
   * Gets an input field in a store card.
   */
  getStoreInput(storeName: string, fieldName: string): Locator {
    const card = this.getStoreCard(storeName);
    return card.locator(
      `input[name="${fieldName}"], input[id*="${fieldName}"], input[placeholder*="${fieldName}"]`
    );
  }

  /**
   * Gets the auto-generate switch for a store.
   */
  getAutoGenerateSwitch(storeName: string): Locator {
    const card = this.getStoreCard(storeName);
    return card.locator('[role="switch"], button[data-state]').first();
  }

  /**
   * Toggles auto-generate for a store.
   */
  async toggleAutoGenerate(storeName: string): Promise<void> {
    const switchBtn = this.getAutoGenerateSwitch(storeName);
    await switchBtn.click();
  }

  /**
   * Gets the save button for a store.
   */
  getSaveButton(storeName: string): Locator {
    const card = this.getStoreCard(storeName);
    return card.locator('button').filter({ hasText: /save|salvar/i });
  }

  /**
   * Gets the test button for a store.
   */
  getTestButton(storeName: string): Locator {
    const card = this.getStoreCard(storeName);
    return card.locator('button').filter({ hasText: /test|testar/i });
  }

  /**
   * Fills Shopee credentials.
   */
  async fillShopeeCredentials(appId: string, appSecret: string): Promise<void> {
    const card = this.getStoreCard('Shopee');

    // Find inputs by label or placeholder
    const appIdInput = card.locator('input').first();
    const appSecretInput = card.locator('input').nth(1);

    await appIdInput.fill(appId);
    await appSecretInput.fill(appSecret);
  }

  /**
   * Saves a store configuration.
   */
  async saveStore(storeName: string): Promise<void> {
    const saveButton = this.getSaveButton(storeName);
    await saveButton.click();
  }

  /**
   * Tests a store connection.
   */
  async testStore(storeName: string): Promise<void> {
    const testButton = this.getTestButton(storeName);
    await testButton.click();
  }

  /**
   * Gets the ML connection token input.
   */
  getMlConnectionTokenInput(): Locator {
    const card = this.getStoreCard('Mercado Livre');
    return card.locator('textarea, input[type="text"]').first();
  }

  /**
   * Gets the ML tag input.
   */
  getMlTagInput(): Locator {
    const card = this.getStoreCard('Mercado Livre');
    return card
      .locator('input')
      .filter({ hasText: /tag/i })
      .or(this.page.locator('input[placeholder*="tag"]'));
  }

  /**
   * Gets the ML test URL input.
   */
  getMlTestUrlInput(): Locator {
    const card = this.getStoreCard('Mercado Livre');
    return card.locator('input[type="url"], input[placeholder*="url"], input[placeholder*="http"]');
  }

  /**
   * Gets the ML validate token button.
   */
  getMlValidateButton(): Locator {
    const card = this.getStoreCard('Mercado Livre');
    return card.locator('button').filter({ hasText: /validar|validate/i });
  }

  /**
   * Gets the ML test result section.
   */
  getMlTestResult(): Locator {
    const card = this.getStoreCard('Mercado Livre');
    return card.locator('[data-testid="ml-test-result"], .test-result');
  }

  /**
   * Fills ML credentials.
   */
  async fillMlCredentials(connectionToken: string, tag: string, testUrl?: string): Promise<void> {
    const tokenInput = this.getMlConnectionTokenInput();
    await tokenInput.fill(connectionToken);

    // Tag might be auto-filled after token validation
    const tagInput = this.getMlTagInput();
    if (await tagInput.isVisible()) {
      await tagInput.fill(tag);
    }

    if (testUrl) {
      const urlInput = this.getMlTestUrlInput();
      if (await urlInput.isVisible()) {
        await urlInput.fill(testUrl);
      }
    }
  }

  /**
   * Validates ML token.
   */
  async validateMlToken(): Promise<void> {
    const validateBtn = this.getMlValidateButton();
    if (await validateBtn.isVisible()) {
      await validateBtn.click();
    }
  }

  /**
   * Gets all visible store names.
   */
  async getVisibleStoreNames(): Promise<string[]> {
    const cards = this.page.locator('[class*="CardTitle"], .card-title, h3');
    const count = await cards.count();
    const names: string[] = [];

    for (let i = 0; i < count; i++) {
      const text = await cards.nth(i).textContent();
      if (text) {
        names.push(text.trim());
      }
    }

    return names;
  }

  /**
   * Checks if the page is loading.
   */
  async isLoading(): Promise<boolean> {
    const loadingText = this.page.locator('text=/loading|carregando/i');
    return loadingText.isVisible();
  }

  /**
   * Waits for the page to finish loading.
   */
  async waitForLoad(): Promise<void> {
    await this.page.waitForSelector('h1', { state: 'visible' });
    // Wait for loading state to disappear
    await this.page
      .waitForFunction(
        () => {
          const loading = document.body.textContent?.toLowerCase();
          return !loading?.includes('loading') && !loading?.includes('carregando');
        },
        { timeout: 10000 }
      )
      .catch(() => {
        // Ignore timeout - page might not have loading state
      });
  }

  /**
   * Gets a coming soon card.
   */
  getComingSoonCard(storeName: string): Locator {
    return this.page
      .locator('[data-testid="coming-soon-card"], .coming-soon')
      .filter({ hasText: storeName });
  }

  /**
   * Checks if a store is marked as coming soon.
   */
  async isComingSoon(storeName: string): Promise<boolean> {
    const card = this.getComingSoonCard(storeName);
    const text = await card.textContent().catch(() => null);
    return (
      text?.toLowerCase().includes('em breve') ||
      text?.toLowerCase().includes('coming soon') ||
      false
    );
  }
}
