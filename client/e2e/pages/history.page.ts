import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the History page.
 *
 * Key UI notes:
 * - Page title is an h2 (not h1).
 * - 4 stat cards in a 2x2 / 1x4 grid (total, completed, running, failed).
 * - Filters use shadcn Select (trigger is a button with role="combobox").
 * - Desktop table is visible on md+ (hidden md:block).
 * - Mobile card list is visible below md (md:hidden).
 * - Pagination with first/prev/page-info/next/last buttons.
 * - Clear history button opens an AlertDialog.
 * - Loading spinner is a div with animate-spin class.
 */
export class HistoryPage {
  readonly page: Page;

  // -- Page structure --
  readonly pageTitle: Locator;
  readonly pageSubtitle: Locator;

  // -- Stats cards --
  readonly statCards: Locator;

  // -- Table --
  readonly table: Locator;
  readonly tableRows: Locator;

  // -- Mobile cards --
  readonly mobileCards: Locator;

  // -- Empty state --
  readonly emptyState: Locator;

  // -- Loading spinner --
  readonly loadingSpinner: Locator;

  // -- Filters --
  readonly eventTypeFilter: Locator;
  readonly statusFilter: Locator;
  readonly clearFiltersButton: Locator;

  // -- Pagination --
  readonly paginationSection: Locator;
  readonly firstPageButton: Locator;
  readonly prevPageButton: Locator;
  readonly nextPageButton: Locator;
  readonly lastPageButton: Locator;
  readonly paginationInfo: Locator;

  // -- Actions --
  readonly clearHistoryButton: Locator;

  // -- Confirmation dialog (AlertDialog) --
  readonly confirmDialog: Locator;
  readonly confirmButton: Locator;
  readonly cancelButton: Locator;

  constructor(page: Page) {
    this.page = page;

    // Page title is an h2
    this.pageTitle = page.locator('h2').first();
    this.pageSubtitle = page.locator('h2 + p').first();

    // Stat cards: 4 cards in grid-cols-2 / lg:grid-cols-4
    this.statCards = page.locator('.grid.grid-cols-2 > div');

    // Desktop table (hidden md:block)
    this.table = page.locator('.hidden.md\\:block table').or(page.locator('table').first());
    this.tableRows = page.locator('table tbody tr');

    // Mobile cards (md:hidden)
    this.mobileCards = page.locator('.md\\:hidden > div');

    // Empty state text
    this.emptyState = page.getByText(
      /nenhum.*registro|no.*records|sem.*historico|no.*history/i
    );

    // Loading spinner
    this.loadingSpinner = page.locator('[class*="animate-spin"]').first();

    // Filters: shadcn Select triggers (role="combobox")
    // First combobox is event type, second is status
    this.eventTypeFilter = page.locator('[role="combobox"]').first();
    this.statusFilter = page.locator('[role="combobox"]').nth(1);
    this.clearFiltersButton = page.locator('button').filter({
      has: page.locator('.lucide-x'),
    });

    // Pagination: buttons with ChevronsLeft, ChevronLeft, ChevronRight, ChevronsRight icons
    this.paginationSection = page.locator('.flex.items-center.gap-0\\.5, .flex.items-center.gap-1');
    this.firstPageButton = page.getByRole('button', {
      name: /primeira|first/i,
    });
    this.prevPageButton = page.getByRole('button', {
      name: /anterior|previous/i,
    });
    this.nextPageButton = page.getByRole('button', {
      name: /pr[oó]xima|next/i,
    });
    this.lastPageButton = page.getByRole('button', {
      name: /[uú]ltima|last/i,
    });
    // Pagination info text: "X-Y de Z" or "1 / 5"
    this.paginationInfo = page.getByText(/\d+\s*[/-]\s*\d+/);

    // Clear history button (has Trash2 icon)
    this.clearHistoryButton = page
      .locator('button')
      .filter({ has: page.locator('.lucide-trash-2, .lucide-trash2') });

    // AlertDialog for clear confirmation
    this.confirmDialog = page.locator('[role="alertdialog"]');
    this.confirmButton = this.confirmDialog.getByRole('button', {
      name: /confirmar|confirm/i,
    });
    this.cancelButton = this.confirmDialog.getByRole('button', {
      name: /cancelar|cancel/i,
    });
  }

  /**
   * Navigates to the history page and waits for the h2 title to appear.
   */
  async goto(): Promise<void> {
    await this.page.goto('/history');
    await this.page.waitForSelector('h2', { state: 'visible' });
  }

  /**
   * Waits for the page to fully load (title visible + spinner gone).
   */
  async waitForLoad(): Promise<void> {
    await this.page.waitForSelector('h2', { state: 'visible' });
    await this.page
      .waitForFunction(
        () => {
          const spinner = document.querySelector('[class*="animate-spin"]');
          return !spinner || getComputedStyle(spinner).display === 'none';
        },
        { timeout: 10000 }
      )
      .catch(() => {});
  }

  /**
   * Gets the displayed stat value from a stat card by index (0-3).
   * Index order: 0=total, 1=completed, 2=running, 3=failed.
   */
  async getStatValue(index: number): Promise<string | null> {
    const card = this.statCards.nth(index);
    const valueEl = card.locator('.text-xl, .text-2xl').first();
    return valueEl.textContent();
  }

  /**
   * Clears history by clicking the clear button and confirming in the dialog.
   */
  async clearHistory(): Promise<void> {
    await this.clearHistoryButton.click();
    await this.confirmDialog.waitFor({ state: 'visible' });
    await this.confirmButton.click();
  }

  /**
   * Selects an event type filter option by clicking the combobox and choosing an option.
   * @param optionText - Visible text of the option (regex-compatible)
   */
  async selectEventTypeFilter(optionText: string): Promise<void> {
    await this.eventTypeFilter.click();
    const option = this.page
      .locator('[role="option"]')
      .filter({ hasText: new RegExp(optionText, 'i') });
    await option.click();
  }

  /**
   * Selects a status filter option by clicking the combobox and choosing an option.
   * @param optionText - Visible text of the option (regex-compatible)
   */
  async selectStatusFilter(optionText: string): Promise<void> {
    await this.statusFilter.click();
    const option = this.page
      .locator('[role="option"]')
      .filter({ hasText: new RegExp(optionText, 'i') });
    await option.click();
  }

  /**
   * Returns the count of table rows in the desktop table.
   */
  async getTableRowCount(): Promise<number> {
    return this.tableRows.count();
  }

  /**
   * Checks whether the empty state message is visible.
   */
  async isEmpty(): Promise<boolean> {
    return this.emptyState.isVisible();
  }
}
