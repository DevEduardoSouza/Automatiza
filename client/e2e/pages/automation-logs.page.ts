import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Automation Logs page.
 * Encapsulates all automation logs page interactions.
 */
export class AutomationLogsPage {
  readonly page: Page;

  // Page elements
  readonly pageTitle: Locator;
  readonly subtitle: Locator;
  readonly exportCsvButton: Locator;

  // Filter elements
  readonly filtersCard: Locator;
  readonly automationFilter: Locator;
  readonly storeFilter: Locator;
  readonly groupFilter: Locator;
  readonly statusFilter: Locator;
  readonly searchInput: Locator;

  // Queue status elements
  readonly queueStatusCard: Locator;
  readonly reconnectButton: Locator;

  // Empty state
  readonly emptyState: Locator;
  readonly emptyFilteredState: Locator;

  // Table (desktop)
  readonly virtualTable: Locator;

  // Cards (mobile)
  readonly mobileCards: Locator;
  readonly showMoreButton: Locator;

  constructor(page: Page) {
    this.page = page;

    // Page elements
    this.pageTitle = page.locator('h1').first();
    this.subtitle = page.locator('p.text-muted-foreground').first();
    this.exportCsvButton = page.getByRole('button', { name: /exportar csv|export csv/i });

    // Filter elements — find label first, navigate to parent container, get the combobox button
    this.automationFilter = this._getFilterTrigger(page, /Automação|Automation/i);
    this.storeFilter = this._getFilterTrigger(page, /^Loja$|^Store$/i);
    this.groupFilter = this._getFilterTrigger(page, /^Grupo$|^Group$/i);
    this.statusFilter = this._getFilterTrigger(page, /^Status$/i);
    this.searchInput = page.getByPlaceholder(/buscar produto|search product/i);

    // Queue status
    this.queueStatusCard = page.locator('text=Em fila').locator('..').locator('..');
    this.reconnectButton = page.getByRole('link', { name: /reconectar whatsapp|reconnect/i });

    // Empty state
    this.emptyState = page.getByText(/nenhum log registrado|no logs recorded/i);
    this.emptyFilteredState = page.getByText(/nenhum log corresponde|no logs match/i);

    // Table (desktop)
    this.virtualTable = page.locator('[class*="hidden md:block"]');

    // Cards (mobile)
    this.mobileCards = page.locator('.md\\:hidden');
    this.showMoreButton = page.getByRole('button', { name: /ver mais|show more/i });
  }

  /**
   * Creates a filter trigger locator by finding the label and navigating to its sibling button.
   */
  private _getFilterTrigger(page: Page, labelPattern: RegExp): Locator {
    return page
      .locator('label')
      .filter({ hasText: labelPattern })
      .locator('..')
      .locator('button')
      .first();
  }

  /**
   * Navigates to the automation logs page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/automation-logs');
    await this.page.waitForSelector('h1', { state: 'visible' });
  }

  /**
   * Waits for the page to be fully loaded (no loading state).
   */
  async waitForLoad(): Promise<void> {
    await this.page.waitForSelector('h1', { state: 'visible' });
    await this.page
      .waitForFunction(
        () => {
          const loading =
            document.body.textContent?.includes('Carregando') ||
            document.body.textContent?.includes('Loading');
          return !loading;
        },
        { timeout: 10000 }
      )
      .catch(() => {
        // Ignore timeout, page might already be loaded
      });
  }

  /**
   * Selects a filter option from a select dropdown.
   */
  async selectFilter(trigger: Locator, value: string): Promise<void> {
    await trigger.click();
    await this.page.getByRole('option', { name: new RegExp(value, 'i') }).click();
  }

  /**
   * Gets all visible status badges using the Badge component's base class.
   * Badge renders as <div class="inline-flex items-center rounded-full ...">.
   */
  getStatusBadges(): Locator {
    return this.page
      .locator('[class*="rounded-full"]')
      .filter({ hasText: /Enviado|Em fila|Ignorado|Erro/i });
  }

  /**
   * Gets badges with a specific status text.
   */
  getStatusBadgeByText(text: string): Locator {
    return this.page.locator('[class*="rounded-full"]').filter({ hasText: new RegExp(text, 'i') });
  }

  /**
   * Gets external product links.
   */
  getProductLinks(): Locator {
    return this.page.locator('a[target="_blank"]');
  }

  /**
   * Gets the reconnect link in error rows.
   */
  getReconnectLinks(): Locator {
    return this.page.getByRole('link', { name: /reconectar|reconnect/i });
  }

  /**
   * Gets coupon applied badge (contains ✓ character).
   */
  getCouponAppliedBadge(): Locator {
    return this.page.locator('[class*="rounded-full"]').filter({ hasText: /✓/ });
  }

  /**
   * Gets coupon not-applied badge (contains ✕ character).
   */
  getCouponNotAppliedBadge(): Locator {
    return this.page.locator('[class*="rounded-full"]').filter({ hasText: /✕/ });
  }

  /**
   * Gets coupon skip reason text.
   */
  getCouponSkipReason(): Locator {
    return this.page.locator('text=Preço abaixo do mínimo');
  }
}
