import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the History page.
 * Encapsulates all history page interactions.
 */
export class HistoryPage {
  readonly page: Page;

  // Page elements
  readonly pageTitle: Locator;

  // Stats section
  readonly statsSection: Locator;
  readonly totalActionsCard: Locator;
  readonly actionsTodayCard: Locator;
  readonly successRateCard: Locator;

  // Filters
  readonly filterSection: Locator;
  readonly filterByType: Locator;
  readonly dateRangePicker: Locator;
  readonly searchInput: Locator;
  readonly clearFiltersButton: Locator;

  // Action type filters
  readonly filterOfferSent: Locator;
  readonly filterAutomationRun: Locator;
  readonly filterGroupSync: Locator;
  readonly filterJobExecuted: Locator;
  readonly filterAll: Locator;

  // History list
  readonly historyList: Locator;
  readonly historyEntries: Locator;
  readonly emptyState: Locator;

  // Entry details
  readonly entryDetailDialog: Locator;
  readonly entryTimestamp: Locator;
  readonly entryDescription: Locator;
  readonly entryMetadata: Locator;

  // Actions
  readonly clearHistoryButton: Locator;
  readonly exportButton: Locator;
  readonly loadMoreButton: Locator;

  // Confirmation dialog
  readonly confirmDialog: Locator;
  readonly confirmButton: Locator;
  readonly cancelButton: Locator;

  // Loading states
  readonly loadingSpinner: Locator;

  constructor(page: Page) {
    this.page = page;

    // Page elements
    this.pageTitle = page.locator('h1');

    // Stats section
    this.statsSection = page
      .locator('section')
      .filter({
        has: page.locator('[class*="card"]'),
      })
      .first();
    this.totalActionsCard = page
      .locator('[class*="card"]')
      .filter({ hasText: /total.*acoes|total.*actions/i });
    this.actionsTodayCard = page.locator('[class*="card"]').filter({ hasText: /hoje|today/i });
    this.successRateCard = page
      .locator('[class*="card"]')
      .filter({ hasText: /taxa.*sucesso|success.*rate/i });

    // Filters
    this.filterSection = page.locator('[class*="filter"]').first();
    this.filterByType = page
      .locator('[data-testid="filter-type"]')
      .or(page.getByRole('combobox').first());
    this.dateRangePicker = page
      .locator('[data-testid="date-range"]')
      .or(page.getByRole('button', { name: /data|date/i }));
    this.searchInput = page
      .locator('input[type="search"]')
      .or(page.getByPlaceholder(/buscar|search/i));
    this.clearFiltersButton = page.getByRole('button', {
      name: /limpar.*filtros|clear.*filters/i,
    });

    // Action type filters
    this.filterOfferSent = page.getByRole('button', {
      name: /oferta.*enviada|offer.*sent/i,
    });
    this.filterAutomationRun = page.getByRole('button', {
      name: /automacao.*executada|automation.*run/i,
    });
    this.filterGroupSync = page.getByRole('button', {
      name: /sincronizacao|sync/i,
    });
    this.filterJobExecuted = page.getByRole('button', {
      name: /job|tarefa/i,
    });
    this.filterAll = page.getByRole('button', {
      name: /todos|all/i,
    });

    // History list
    this.historyList = page
      .locator('[class*="history-list"]')
      .or(page.locator('ul, [role="list"]').first());
    this.historyEntries = page
      .locator('[class*="history-entry"], [class*="history-item"]')
      .or(page.locator('li[class*="entry"]'));
    this.emptyState = page.getByText(/nenhum.*historico|no.*history|sem.*registros|no.*records/i);

    // Entry details
    this.entryDetailDialog = page.locator('[role="dialog"]');
    this.entryTimestamp = this.entryDetailDialog
      .locator('[class*="timestamp"]')
      .or(this.entryDetailDialog.getByText(/\d{2}\/\d{2}\/\d{4}/));
    this.entryDescription = this.entryDetailDialog.locator('[class*="description"]');
    this.entryMetadata = this.entryDetailDialog
      .locator('[class*="metadata"]')
      .or(this.entryDetailDialog.locator('pre, code'));

    // Actions
    this.clearHistoryButton = page.getByRole('button', {
      name: /limpar.*historico|clear.*history/i,
    });
    this.exportButton = page.getByRole('button', {
      name: /exportar|export/i,
    });
    this.loadMoreButton = page.getByRole('button', {
      name: /carregar.*mais|load.*more|ver.*mais|see.*more/i,
    });

    // Confirmation dialog
    this.confirmDialog = page
      .locator('[role="alertdialog"]')
      .or(page.locator('[role="dialog"]').filter({ hasText: /confirmar|confirm/i }));
    this.confirmButton = this.confirmDialog.getByRole('button', {
      name: /confirmar|confirm|sim|yes|limpar|clear/i,
    });
    this.cancelButton = this.confirmDialog.getByRole('button', {
      name: /cancelar|cancel|nao|no/i,
    });

    // Loading states
    this.loadingSpinner = page.locator('[class*="animate-spin"]').first();
  }

  /**
   * Navigates to the history page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/history');
    await this.page.waitForSelector('h1', { state: 'visible' });
  }

  /**
   * Waits for the page to load.
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
      .catch(() => {});
  }

  /**
   * Gets a history entry by its description.
   */
  getEntryByDescription(description: string): Locator {
    return this.historyEntries.filter({ hasText: description });
  }

  /**
   * Filters history by action type.
   */
  async filterByActionType(
    type: 'offer_sent' | 'automation_run' | 'group_sync' | 'job' | 'all'
  ): Promise<void> {
    const filterButtons = {
      offer_sent: this.filterOfferSent,
      automation_run: this.filterAutomationRun,
      group_sync: this.filterGroupSync,
      job: this.filterJobExecuted,
      all: this.filterAll,
    };
    await filterButtons[type].click();
  }

  /**
   * Searches history.
   */
  async search(query: string): Promise<void> {
    await this.searchInput.fill(query);
  }

  /**
   * Clears search input.
   */
  async clearSearch(): Promise<void> {
    await this.searchInput.clear();
  }

  /**
   * Clears all filters.
   */
  async clearFilters(): Promise<void> {
    await this.clearFiltersButton.click();
  }

  /**
   * Opens entry details.
   */
  async openEntryDetails(description: string): Promise<void> {
    const entry = this.getEntryByDescription(description);
    await entry.click();
  }

  /**
   * Closes entry details dialog.
   */
  async closeEntryDetails(): Promise<void> {
    const closeButton = this.entryDetailDialog.locator('button').filter({
      has: this.page.locator('svg.lucide-x'),
    });
    await closeButton.click();
  }

  /**
   * Clears all history.
   */
  async clearHistory(): Promise<void> {
    await this.clearHistoryButton.click();
    if (await this.confirmDialog.isVisible()) {
      await this.confirmButton.click();
    }
  }

  /**
   * Exports history.
   */
  async exportHistory(): Promise<void> {
    await this.exportButton.click();
  }

  /**
   * Loads more entries.
   */
  async loadMore(): Promise<void> {
    await this.loadMoreButton.click();
  }

  /**
   * Gets the count of displayed entries.
   */
  async getEntryCount(): Promise<number> {
    return this.historyEntries.count();
  }

  /**
   * Checks if empty state is shown.
   */
  async isEmpty(): Promise<boolean> {
    return this.emptyState.isVisible();
  }

  /**
   * Checks if load more button is visible.
   */
  async hasMoreEntries(): Promise<boolean> {
    return this.loadMoreButton.isVisible();
  }

  /**
   * Gets stat value from a card.
   */
  async getStatValue(card: Locator): Promise<string | null> {
    const value = card.locator('h2, h3, [class*="value"], span').first();
    return value.textContent();
  }

  /**
   * Checks if entry was successful.
   */
  async isEntrySuccessful(description: string): Promise<boolean> {
    const entry = this.getEntryByDescription(description);
    const successIndicator = entry.locator('[class*="success"], svg.lucide-check');
    return successIndicator.isVisible();
  }

  /**
   * Checks if entry failed.
   */
  async isEntryFailed(description: string): Promise<boolean> {
    const entry = this.getEntryByDescription(description);
    const failIndicator = entry.locator('[class*="error"], [class*="failed"], svg.lucide-x');
    return failIndicator.isVisible();
  }
}
