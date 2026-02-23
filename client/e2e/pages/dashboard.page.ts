import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Dashboard page.
 * Locators aligned with the actual UI rendered by dashboard.tsx.
 */
export class DashboardPage {
  readonly page: Page;

  // Header
  readonly pageTitle: Locator;

  // Stats cards (by i18n label text)
  readonly totalGroupsCard: Locator;
  readonly fullGroupsCard: Locator;
  readonly totalMembersCard: Locator;
  readonly activeLinksCard: Locator;

  // Announcement panel
  readonly announcementPanel: Locator;
  readonly announcementTitle: Locator;

  // Welcome panel
  readonly welcomePanel: Locator;
  readonly welcomeStep1: Locator;
  readonly welcomeStep2: Locator;
  readonly welcomeStep3: Locator;

  // Chart section
  readonly chartContainer: Locator;
  readonly periodWeekly: Locator;
  readonly periodMonthly: Locator;
  readonly periodYearly: Locator;

  // Chart footer
  readonly growthRate: Locator;
  readonly peakMembers: Locator;
  readonly activeGroups: Locator;

  // Loading / empty / error states
  readonly skeletonCards: Locator;
  readonly chartLoading: Locator;
  readonly noDataMessage: Locator;
  readonly errorToast: Locator;

  constructor(page: Page) {
    this.page = page;

    // The dashboard uses h2 for its title
    this.pageTitle = page.locator('h2').first();

    // Stats cards identified by their uppercase label text
    this.totalGroupsCard = page
      .locator('div')
      .filter({ hasText: /Total de Grupos/i })
      .first();
    this.fullGroupsCard = page
      .locator('div')
      .filter({ hasText: /Grupos Cheios/i })
      .first();
    this.totalMembersCard = page
      .locator('div')
      .filter({ hasText: /Total de Membros/i })
      .first();
    this.activeLinksCard = page
      .locator('div')
      .filter({ hasText: /Links Ativos/i })
      .first();

    // Announcement (conditionally rendered, identified by AlertCircle context)
    this.announcementPanel = page
      .locator('div')
      .filter({ hasText: /Novidade/i })
      .first();
    this.announcementTitle = page
      .locator('span')
      .filter({ hasText: /Novidade/i })
      .first();

    // Welcome panel
    this.welcomePanel = page
      .locator('div')
      .filter({ hasText: /Bem-vindo/i })
      .first();
    this.welcomeStep1 = page.locator('h3').filter({ hasText: /1\. Conecte/i });
    this.welcomeStep2 = page.locator('h3').filter({ hasText: /2\. Importe/i });
    this.welcomeStep3 = page.locator('h3').filter({ hasText: /3\. Configure/i });

    // Chart section
    this.chartContainer = page.locator('.recharts-wrapper').first();

    // Period buttons (by displayed text)
    this.periodWeekly = page.getByRole('button', { name: /Semanal/i });
    this.periodMonthly = page.getByRole('button', { name: /Mensal/i });
    this.periodYearly = page.getByRole('button', { name: /Anual/i });

    // Chart footer stats (by label text)
    this.growthRate = page
      .locator('div')
      .filter({ hasText: /Taxa de crescimento/i })
      .first();
    this.peakMembers = page
      .locator('div')
      .filter({ hasText: /Pico de membros/i })
      .first();
    this.activeGroups = page
      .locator('div')
      .filter({ hasText: /Grupos ativos/i })
      .first();

    // Loading/empty states
    this.skeletonCards = page.locator('[data-slot="skeleton"]');
    this.chartLoading = page.getByText(/Carregando dados/i);
    this.noDataMessage = page.getByText(/Sem dados/i);
    this.errorToast = page.locator('[data-sonner-toast]').first();
  }

  /**
   * Navigates to the dashboard page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/dashboard');
    await this.page.waitForSelector('h2', { state: 'visible', timeout: 10000 });
  }

  /**
   * Waits for dashboard data to finish loading (skeletons disappear).
   */
  async waitForLoad(): Promise<void> {
    await this.page
      .waitForFunction(
        () => {
          const skeletons = document.querySelectorAll('[data-slot="skeleton"]');
          return skeletons.length === 0;
        },
        { timeout: 10000 }
      )
      .catch(() => {
        // Ignore timeout, page might already be loaded
      });
  }

  /**
   * Gets the numeric value from a stat card locator.
   */
  async getStatValue(cardLocator: Locator): Promise<string> {
    const valueEl = cardLocator.locator('span.font-semibold').first();
    return (await valueEl.textContent()) ?? '';
  }

  /**
   * Checks if the page has fully loaded (title visible).
   */
  async isLoaded(): Promise<boolean> {
    try {
      await this.page.waitForSelector('h2', { state: 'visible', timeout: 5000 });
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Checks if an error toast is visible.
   */
  async hasError(): Promise<boolean> {
    return this.errorToast.isVisible().catch(() => false);
  }
}
