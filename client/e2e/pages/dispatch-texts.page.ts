import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Dispatch Texts page.
 * Matches the actual implementation in client/src/pages/dispatch-texts.tsx.
 */
export class DispatchTextsPage {
  readonly page: Page;

  // Page elements
  readonly pageTitle: Locator;

  // Actions
  readonly createButton: Locator;

  // Table/List elements
  readonly emptyState: Locator;

  // Dialog elements
  readonly dialogContent: Locator;
  readonly dialogTitle: Locator;

  // Form fields
  readonly nameInput: Locator;
  readonly bodyInput: Locator;
  readonly activeSwitch: Locator;

  // Variable chips
  readonly variableChips: Locator;

  // Preview
  readonly previewCard: Locator;

  // Form actions
  readonly saveButton: Locator;
  readonly cancelButton: Locator;

  // Delete confirmation dialog
  readonly confirmDialog: Locator;
  readonly confirmDeleteButton: Locator;
  readonly cancelDeleteButton: Locator;

  // Toasts
  readonly successToast: Locator;
  readonly errorToast: Locator;

  constructor(page: Page) {
    this.page = page;

    // Page elements
    this.pageTitle = page.locator('h1');

    // Actions - matches the actual Button with Plus icon
    this.createButton = page.getByRole('button', {
      name: /novo.*texto|new.*text|criar|create/i,
    });

    // Empty state - matches the actual empty div text
    this.emptyState = page.getByText(/nenhum.*texto|no.*text|ainda.*n[aã]o.*criou/i);

    // Dialog elements
    this.dialogContent = page.locator('[role="dialog"]');
    this.dialogTitle = this.dialogContent.locator('h2').first();

    // Form fields - matches actual IDs from the page
    this.nameInput = page.locator('#dispatch-text-name');
    this.bodyInput = page.locator('#dispatch-text-body');
    this.activeSwitch = page.locator('#dispatch-text-active');

    // Variable chips
    this.variableChips = page.locator('[role="list"] [role="listitem"]');

    // Preview
    this.previewCard = page.locator('pre');

    // Form actions
    this.saveButton = page.getByRole('button', { name: /salvar|criar|save|create/i }).last();
    this.cancelButton = page.getByRole('button', { name: /cancelar|cancel/i });

    // Delete confirmation dialog (AlertDialog)
    this.confirmDialog = page.locator('[role="alertdialog"]');
    this.confirmDeleteButton = this.confirmDialog.getByRole('button', {
      name: /confirmar|confirm|sim|yes|excluir|delete/i,
    });
    this.cancelDeleteButton = this.confirmDialog.getByRole('button', {
      name: /cancelar|cancel/i,
    });

    // Toasts
    this.successToast = page.locator('[data-sonner-toast]').filter({
      hasText: /sucesso|success|criado|created|atualizado|updated|exclu[ií]/i,
    });
    this.errorToast = page.locator('[data-sonner-toast]').filter({
      hasText: /erro|error|falha|failed/i,
    });
  }

  /**
   * Navigates to the dispatch texts page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/dispatch-texts');
    await this.page.waitForSelector('h1', { state: 'visible' });
  }

  /**
   * Waits for the page to finish loading.
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
   * Opens the create text dialog.
   */
  async openCreateDialog(): Promise<void> {
    await this.createButton.click();
    await this.dialogContent.waitFor({ state: 'visible' });
  }

  /**
   * Gets a text row by name (works with VirtualTable).
   */
  getTextByName(name: string): Locator {
    return this.page.getByRole('row').filter({ hasText: name });
  }

  /**
   * Gets the edit button for a text row.
   */
  getEditButton(row: Locator): Locator {
    return row.getByRole('button', { name: /editar|edit/i });
  }

  /**
   * Gets the delete button for a text row.
   */
  getDeleteButton(row: Locator): Locator {
    return row.getByRole('button', { name: /excluir|delete/i });
  }

  /**
   * Gets the status badge for a text row.
   */
  getStatusBadge(row: Locator): Locator {
    return row.locator('[class*="badge"]').first();
  }

  /**
   * Fills the text form.
   */
  async fillTextForm(options: {
    name: string;
    body: string;
    isActive?: boolean;
  }): Promise<void> {
    await this.nameInput.fill(options.name);
    await this.bodyInput.fill(options.body);

    if (options.isActive === false) {
      await this.activeSwitch.click();
    }
  }

  /**
   * Saves the text form.
   */
  async save(): Promise<void> {
    await this.saveButton.click();
  }

  /**
   * Cancels the text form.
   */
  async cancel(): Promise<void> {
    await this.cancelButton.click();
  }

  /**
   * Edits a text by name.
   */
  async editText(name: string): Promise<void> {
    const row = this.getTextByName(name);
    const editBtn = this.getEditButton(row);
    await editBtn.click();
    await this.dialogContent.waitFor({ state: 'visible' });
  }

  /**
   * Deletes a text by name (opens confirmation then confirms).
   */
  async deleteText(name: string): Promise<void> {
    const row = this.getTextByName(name);
    const deleteBtn = this.getDeleteButton(row);
    await deleteBtn.click();

    if (await this.confirmDialog.isVisible()) {
      await this.confirmDeleteButton.click();
    }
  }

  /**
   * Gets the count of displayed text rows.
   */
  async getTextCount(): Promise<number> {
    const rows = this.page.locator('[role="row"]');
    const count = await rows.count();
    // Subtract 1 for header row if present
    return Math.max(0, count - 1);
  }

  /**
   * Checks if empty state is shown.
   */
  async isEmpty(): Promise<boolean> {
    return this.emptyState.isVisible();
  }

  /**
   * Gets the status text of a text by name.
   */
  async getTextStatus(name: string): Promise<string | null> {
    const row = this.getTextByName(name);
    const badge = this.getStatusBadge(row);
    return badge.textContent();
  }

  /**
   * Inserts a variable by clicking its chip badge.
   */
  async insertVariable(variableKey: string): Promise<void> {
    const chip = this.page.locator('[role="listitem"]').filter({ hasText: variableKey });
    await chip.click();
  }

  /**
   * Gets the preview text content.
   */
  async getPreviewText(): Promise<string | null> {
    return this.previewCard.textContent();
  }

  /**
   * Gets the number of variable chips displayed.
   */
  async getVariableChipCount(): Promise<number> {
    return this.variableChips.count();
  }
}
