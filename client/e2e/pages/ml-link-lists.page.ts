import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for ML Link Lists page.
 */
export class MlLinkListsPage {
  readonly page: Page;
  readonly pageTitle: Locator;
  readonly createListButton: Locator;
  readonly refreshButton: Locator;
  readonly listSelector: Locator;
  readonly editListButton: Locator;
  readonly deleteListButton: Locator;
  readonly resetConsumedButton: Locator;
  readonly singleUrlInput: Locator;
  readonly addLinkButton: Locator;
  readonly bulkTextarea: Locator;
  readonly importButton: Locator;
  readonly dialogContent: Locator;
  readonly dialogTitle: Locator;
  readonly listNameInput: Locator;
  readonly listDescriptionInput: Locator;
  readonly listActiveSwitch: Locator;
  readonly saveButton: Locator;
  readonly cancelButton: Locator;
  readonly confirmDeleteButton: Locator;
  readonly availableBadge: Locator;
  readonly sentBadge: Locator;
  readonly totalCount: Locator;
  readonly emptyState: Locator;
  readonly previewCard: Locator;
  readonly itemsTable: Locator;

  constructor(page: Page) {
    this.page = page;
    this.pageTitle = page.locator('h1');
    this.createListButton = page.getByRole('button', { name: /nova lista|new list/i });
    this.refreshButton = page.getByRole('button', { name: /atualizar|refresh/i }).first();
    this.listSelector = page.locator('[id="list-select"]');
    this.editListButton = page.getByRole('button', { name: /editar|edit/i });
    this.deleteListButton = page.getByRole('button', { name: /excluir|delete/i }).first();
    this.resetConsumedButton = page.getByRole('button', { name: /zerar enviados|reset sent/i });
    this.singleUrlInput = page.locator('[id="single-url"]');
    this.addLinkButton = page.getByRole('button', { name: /adicionar|add/i }).first();
    this.bulkTextarea = page.locator('[id="bulk-text"]');
    this.importButton = page.getByRole('button', { name: /importar|import/i });
    this.dialogContent = page.locator('[role="dialog"]');
    this.dialogTitle = page.locator('[role="dialog"] h2');
    this.listNameInput = page.locator('[id="list-name"]');
    this.listDescriptionInput = page.locator('[id="list-description"]');
    this.listActiveSwitch = page.locator('[role="switch"]');
    this.saveButton = page.getByRole('button', { name: /salvar|save/i });
    this.cancelButton = page.getByRole('button', { name: /cancelar|cancel/i });
    this.confirmDeleteButton = page.locator('[role="alertdialog"]').getByRole('button', {
      name: /excluir|delete|confirmar|confirm/i,
    });
    this.availableBadge = page.locator('text=/disponíve|available/i').first();
    this.sentBadge = page.locator('text=/enviado|sent/i').first();
    this.totalCount = page.locator('text=/total/i');
    this.emptyState = page.locator('text=/nenhum item|empty|sem itens/i');
    this.previewCard = page.locator('text=/preview/i').locator('..');
    this.itemsTable = page.locator('[data-testid="virtual-table"], table').first();
  }

  /**
   * Navigate to ML Link Lists page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/ml-link-lists');
  }

  /**
   * Wait for page to fully load.
   */
  async waitForLoad(): Promise<void> {
    await this.page.waitForLoadState('networkidle');
    await this.pageTitle.waitFor({ state: 'visible', timeout: 10000 });
  }

  /**
   * Open create list dialog.
   */
  async openCreateDialog(): Promise<void> {
    await this.createListButton.click();
    await this.dialogContent.waitFor({ state: 'visible' });
  }

  /**
   * Open edit list dialog.
   */
  async openEditDialog(): Promise<void> {
    await this.editListButton.click();
    await this.dialogContent.waitFor({ state: 'visible' });
  }

  /**
   * Fill list form.
   */
  async fillListForm(data: {
    name?: string;
    description?: string;
    isActive?: boolean;
  }): Promise<void> {
    if (data.name !== undefined) {
      await this.listNameInput.fill(data.name);
    }
    if (data.description !== undefined) {
      await this.listDescriptionInput.fill(data.description);
    }
    if (data.isActive !== undefined) {
      const isChecked = await this.listActiveSwitch.isChecked();
      if (isChecked !== data.isActive) {
        await this.listActiveSwitch.click();
      }
    }
  }

  /**
   * Save the current list form.
   */
  async saveList(): Promise<void> {
    await this.saveButton.click();
    await this.dialogContent.waitFor({ state: 'hidden', timeout: 5000 });
  }

  /**
   * Cancel the current dialog.
   */
  async cancelDialog(): Promise<void> {
    await this.cancelButton.click();
    await this.dialogContent.waitFor({ state: 'hidden' });
  }

  /**
   * Select a list from the dropdown.
   */
  async selectList(listName: string): Promise<void> {
    await this.listSelector.click();
    await this.page.getByRole('option', { name: new RegExp(listName, 'i') }).click();
  }

  /**
   * Request delete for current list.
   */
  async requestDeleteList(): Promise<void> {
    await this.deleteListButton.click();
    await this.page.locator('[role="alertdialog"]').waitFor({ state: 'visible' });
  }

  /**
   * Confirm the delete action.
   */
  async confirmDelete(): Promise<void> {
    await this.confirmDeleteButton.click();
    await this.page.locator('[role="alertdialog"]').waitFor({ state: 'hidden', timeout: 5000 });
  }

  /**
   * Add a single URL.
   */
  async addSingleUrl(url: string): Promise<void> {
    await this.singleUrlInput.fill(url);
    await this.addLinkButton.click();
  }

  /**
   * Import bulk URLs.
   */
  async importBulkUrls(urls: string[]): Promise<void> {
    await this.bulkTextarea.fill(urls.join('\n'));
    await this.importButton.click();
  }

  /**
   * Request reset consumed items.
   */
  async requestResetConsumed(): Promise<void> {
    await this.resetConsumedButton.click();
    await this.page.locator('[role="alertdialog"]').waitFor({ state: 'visible' });
  }

  /**
   * Get the count of items displayed.
   */
  async getItemCount(): Promise<number> {
    const rows = this.page.locator('[data-testid="virtual-table"] [role="row"], table tbody tr');
    return rows.count();
  }

  /**
   * Get the list count from dropdown options.
   */
  async getListCount(): Promise<number> {
    await this.listSelector.click();
    const options = this.page.getByRole('option');
    const count = await options.count();
    await this.page.keyboard.press('Escape');
    return count;
  }

  /**
   * Check if page shows empty state.
   */
  async isEmpty(): Promise<boolean> {
    const emptyVisible = await this.emptyState.isVisible().catch(() => false);
    const noItems = (await this.getItemCount()) === 0;
    return emptyVisible || noItems;
  }

  /**
   * Check if preview card is visible.
   */
  async hasPreview(): Promise<boolean> {
    return this.previewCard.isVisible();
  }

  /**
   * Toggle item active status by index.
   */
  async toggleItemActive(index: number): Promise<void> {
    const row = this.page.locator(
      `[data-testid="virtual-table"] [role="row"]:nth-child(${index + 1}), table tbody tr:nth-child(${index + 1})`
    );
    const toggleButton = row.getByRole('button', { name: /ativar|desativar|activate|deactivate/i });
    await toggleButton.click();
  }

  /**
   * Move item up by index.
   */
  async moveItemUp(index: number): Promise<void> {
    const row = this.page.locator(
      `[data-testid="virtual-table"] [role="row"]:nth-child(${index + 1}), table tbody tr:nth-child(${index + 1})`
    );
    const upButton = row.locator('[aria-label*="up"], button:has(svg.lucide-arrow-up)');
    await upButton.click();
  }

  /**
   * Move item down by index.
   */
  async moveItemDown(index: number): Promise<void> {
    const row = this.page.locator(
      `[data-testid="virtual-table"] [role="row"]:nth-child(${index + 1}), table tbody tr:nth-child(${index + 1})`
    );
    const downButton = row.locator('[aria-label*="down"], button:has(svg.lucide-arrow-down)');
    await downButton.click();
  }

  /**
   * Request delete item by index.
   */
  async requestDeleteItem(index: number): Promise<void> {
    const row = this.page.locator(
      `[data-testid="virtual-table"] [role="row"]:nth-child(${index + 1}), table tbody tr:nth-child(${index + 1})`
    );
    const deleteButton = row.getByRole('button', { name: /delete/i });
    await deleteButton.click();
    await this.page.locator('[role="alertdialog"]').waitFor({ state: 'visible' });
  }

  /**
   * Get available count from badge.
   */
  async getAvailableCount(): Promise<number> {
    const text = await this.availableBadge.textContent();
    const match = text?.match(/\d+/);
    return match ? Number.parseInt(match[0], 10) : 0;
  }

  /**
   * Get sent count from badge.
   */
  async getSentCount(): Promise<number> {
    const text = await this.sentBadge.textContent();
    const match = text?.match(/\d+/);
    return match ? Number.parseInt(match[0], 10) : 0;
  }

  /**
   * Copy link by clicking copy button.
   */
  async copyLinkAtIndex(index: number): Promise<void> {
    const row = this.page.locator(
      `[data-testid="virtual-table"] [role="row"]:nth-child(${index + 1}), table tbody tr:nth-child(${index + 1})`
    );
    const copyButton = row.locator('text=/copiar|copy/i');
    await copyButton.click();
  }
}
