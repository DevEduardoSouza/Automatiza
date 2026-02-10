import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Dashboard page.
 * Encapsulates all dashboard page interactions.
 */
export class DashboardPage {
  readonly page: Page;

  // Stats cards
  readonly totalOffersSentCard: Locator;
  readonly offersTodayCard: Locator;
  readonly totalGroupsCard: Locator;
  readonly totalAutomationsCard: Locator;

  // Status indicators
  readonly whatsappStatusBadge: Locator;
  readonly connectionStatusIndicator: Locator;

  // Charts
  readonly offersChart: Locator;
  readonly chartContainer: Locator;

  // Activity section
  readonly recentActivitySection: Locator;
  readonly activityList: Locator;
  readonly activityItems: Locator;

  // Quick actions
  readonly quickActionsSection: Locator;
  readonly syncGroupsButton: Locator;
  readonly newAutomationButton: Locator;
  readonly viewAllAutomationsLink: Locator;

  // Loading states
  readonly loadingSpinner: Locator;
  readonly errorAlert: Locator;

  constructor(page: Page) {
    this.page = page;

    // Stats cards - using common patterns
    this.totalOffersSentCard = page
      .locator('[class*="card"]')
      .filter({ hasText: /ofertas enviadas|offers sent/i })
      .first();
    this.offersTodayCard = page
      .locator('[class*="card"]')
      .filter({ hasText: /hoje|today/i })
      .first();
    this.totalGroupsCard = page
      .locator('[class*="card"]')
      .filter({ hasText: /grupos|groups/i })
      .first();
    this.totalAutomationsCard = page
      .locator('[class*="card"]')
      .filter({ hasText: /automacoes|automations/i })
      .first();

    // Status indicators
    this.whatsappStatusBadge = page
      .locator('[class*="badge"]')
      .filter({ hasText: /whatsapp|conectado|connected|desconectado|disconnected/i });
    this.connectionStatusIndicator = page
      .locator('[data-testid="connection-status"]')
      .or(page.locator('[class*="status-indicator"]'));

    // Charts
    this.offersChart = page.locator('.recharts-wrapper').first();
    this.chartContainer = page.locator('[class*="chart"]').first();

    // Activity section
    this.recentActivitySection = page
      .locator('section')
      .filter({ hasText: /atividade recente|recent activity/i });
    this.activityList = page
      .locator('[class*="activity-list"]')
      .or(page.locator('ul').filter({ has: page.locator('[class*="activity"]') }));
    this.activityItems = page
      .locator('[class*="activity-item"]')
      .or(page.locator('li').filter({ has: page.locator('[class*="activity"]') }));

    // Quick actions
    this.quickActionsSection = page
      .locator('section')
      .filter({ hasText: /acoes rapidas|quick actions/i });
    this.syncGroupsButton = page.getByRole('button', {
      name: /sincronizar grupos|sync groups/i,
    });
    this.newAutomationButton = page.getByRole('button', {
      name: /nova automacao|new automation/i,
    });
    this.viewAllAutomationsLink = page.getByRole('link', {
      name: /ver todas|view all|automacoes/i,
    });

    // Loading states
    this.loadingSpinner = page.locator('[class*="animate-spin"]').first();
    this.errorAlert = page.locator('[role="alert"]').filter({
      hasText: /erro|error|falha|failed/i,
    });
  }

  /**
   * Navigates to the dashboard page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/dashboard');
    await this.page.waitForSelector('h1', { state: 'visible' });
  }

  /**
   * Waits for dashboard data to load.
   */
  async waitForLoad(): Promise<void> {
    // Wait for loading to complete
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

    // Wait for at least one stat card to be visible
    await this.page.waitForSelector('[class*="card"]', { state: 'visible' });
  }

  /**
   * Gets the value from a stat card.
   */
  async getStatValue(cardLocator: Locator): Promise<string> {
    const valueElement = cardLocator.locator('h2, h3, [class*="value"], span').first();
    return valueElement.textContent() || '';
  }

  /**
   * Checks if WhatsApp is connected.
   */
  async isWhatsAppConnected(): Promise<boolean> {
    const badge = await this.whatsappStatusBadge.textContent();
    return /conectado|connected/i.test(badge || '');
  }

  /**
   * Clicks to sync groups.
   */
  async syncGroups(): Promise<void> {
    await this.syncGroupsButton.click();
  }

  /**
   * Clicks to create new automation.
   */
  async createNewAutomation(): Promise<void> {
    await this.newAutomationButton.click();
  }

  /**
   * Gets the count of recent activity items.
   */
  async getActivityCount(): Promise<number> {
    return this.activityItems.count();
  }

  /**
   * Checks if the chart is visible.
   */
  async isChartVisible(): Promise<boolean> {
    return (
      this.offersChart.isVisible().catch(() => false) ||
      this.chartContainer.isVisible().catch(() => false)
    );
  }

  /**
   * Checks if there is an error state.
   */
  async hasError(): Promise<boolean> {
    return this.errorAlert.isVisible();
  }

  /**
   * Checks if the page is loaded.
   */
  async isLoaded(): Promise<boolean> {
    try {
      await this.page.waitForSelector('[class*="card"]', { state: 'visible', timeout: 5000 });
      return true;
    } catch {
      return false;
    }
  }
}
