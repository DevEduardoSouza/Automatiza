import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Redirect Links page.
 * Encapsulates all redirect link management interactions.
 */
export class RedirectPage {
  readonly page: Page;

  // Form elements
  readonly linkNameInput: Locator;
  readonly slugInput: Locator;
  readonly clickLimitBtn: Locator;
  readonly memberLimitBtn: Locator;
  readonly roundRobinBtn: Locator;
  readonly selectGroupsBtn: Locator;
  readonly submitButton: Locator;
  readonly cancelEditBtn: Locator;

  // Alerts
  readonly editingAlert: Locator;

  // Dialog
  readonly groupDialog: Locator;
  readonly dialogSearchInput: Locator;
  readonly selectAllBtn: Locator;
  readonly deselectAllBtn: Locator;
  readonly applyBtn: Locator;

  // Saved Links
  readonly savedLinksTitle: Locator;
  readonly emptyLinksMsg: Locator;

  // Delete Dialog
  readonly deleteDialogTitle: Locator;
  readonly deleteConfirmBtn: Locator;

  // Priority
  readonly prioritySection: Locator;

  // Toast
  readonly toast: Locator;

  // Loading
  readonly loadingSpinner: Locator;

  constructor(page: Page) {
    this.page = page;

    // Form elements
    this.linkNameInput = page.locator('#linkName');
    this.slugInput = page.locator('#slug');
    this.clickLimitBtn = page.getByRole('button', {
      name: /limite de cliques|click limit/i,
    });
    this.memberLimitBtn = page.getByRole('button', {
      name: /vagas dispon|member limit/i,
    });
    this.roundRobinBtn = page.getByRole('button', {
      name: /round robin/i,
    });
    this.selectGroupsBtn = page.getByRole('button', {
      name: /selecionar grupos|select groups/i,
    });
    this.submitButton = page.getByRole('button', {
      name: /criar e salvar|atualizar link|salvando|create|update|saving/i,
    });
    this.cancelEditBtn = page.getByRole('button', {
      name: /cancelar edi|cancel edit/i,
    });

    // Alerts
    this.editingAlert = page.locator('[role="alert"]').filter({
      hasText: /editando link|editing link/i,
    });

    // Dialog
    this.groupDialog = page.locator('[role="dialog"]');
    this.dialogSearchInput = this.groupDialog.locator('input[placeholder]');
    this.selectAllBtn = page.getByRole('button', {
      name: /selecionar todos|select all/i,
    });
    this.deselectAllBtn = page.getByRole('button', {
      name: /desmarcar todos|deselect all/i,
    });
    this.applyBtn = page.getByRole('button', {
      name: /aplicar sele|apply selection/i,
    });

    // Saved Links
    this.savedLinksTitle = page.getByText(/links salvos|saved links/i);
    this.emptyLinksMsg = page.getByText(/nenhum link salvo|no saved links/i);

    // Delete Dialog
    this.deleteDialogTitle = page.getByText(
      /excluir link de redirecionamento|delete redirect link/i
    );
    this.deleteConfirmBtn = page.getByRole('button', {
      name: /excluir link|delete link/i,
    });

    // Priority
    this.prioritySection = page.getByText(/prioridade de inser|insertion priority/i);

    // Toast
    this.toast = page.locator('[data-sonner-toast]');

    // Loading
    this.loadingSpinner = page.locator('[class*="animate-spin"]').first();
  }

  /**
   * Navigates to the redirect links page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/redirect-link');
    await this.page.waitForSelector('h2, h1', { state: 'visible' });
  }

  /**
   * Waits for the page to fully load.
   */
  async waitForLoad(): Promise<void> {
    await this.page.waitForSelector('h2, h1', { state: 'visible' });
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
   * Gets a saved link card by name.
   */
  getSavedLink(name: string): Locator {
    return this.page
      .locator('.rounded-lg, .rounded-xl, [class*="card"]')
      .filter({ hasText: name });
  }

  /**
   * Gets the copy button for a saved link.
   */
  getCopyBtn(name: string): Locator {
    return this.getSavedLink(name).getByRole('button', {
      name: /copiar|copy/i,
    });
  }

  /**
   * Gets the edit button for a saved link.
   */
  getEditBtn(name: string): Locator {
    return this.getSavedLink(name).getByRole('button', {
      name: /editar|edit/i,
    });
  }

  /**
   * Gets the delete button for a saved link.
   */
  getDeleteBtn(name: string): Locator {
    return this.getSavedLink(name).getByRole('button', {
      name: /excluir|delete|remover|remove/i,
    });
  }

  /**
   * Gets the move up button at a given index in the priority table.
   */
  getMoveUpBtn(index: number): Locator {
    return this.page
      .locator('button')
      .filter({ has: this.page.locator('svg.lucide-chevron-up, svg.lucide-arrow-up') })
      .nth(index);
  }

  /**
   * Gets the move down button at a given index in the priority table.
   */
  getMoveDownBtn(index: number): Locator {
    return this.page
      .locator('button')
      .filter({ has: this.page.locator('svg.lucide-chevron-down, svg.lucide-arrow-down') })
      .nth(index);
  }

  /**
   * Gets a toast by matching text pattern.
   */
  getToast(textPattern: RegExp): Locator {
    return this.page.locator('[data-sonner-toast]').filter({ hasText: textPattern });
  }

  // ============================================================
  // Actions
  // ============================================================

  /**
   * Fills the redirect link form.
   */
  async fillForm(name: string, slug?: string): Promise<void> {
    await this.linkNameInput.fill(name);
    if (slug) {
      await this.slugInput.clear();
      await this.slugInput.fill(slug);
    }
  }

  /**
   * Opens the group selection dialog.
   */
  async openGroupDialog(): Promise<void> {
    await this.selectGroupsBtn.click();
    await this.groupDialog.waitFor({ state: 'visible' });
  }

  /**
   * Selects a group by name in the dialog.
   */
  async selectGroup(name: string): Promise<void> {
    const groupRow = this.groupDialog.locator('label, [role="checkbox"]').filter({ hasText: name });
    await groupRow.click();
  }

  /**
   * Applies the group selection and closes the dialog.
   */
  async applySelection(): Promise<void> {
    await this.applyBtn.click();
  }

  /**
   * Submits the form (create or update).
   */
  async submitForm(): Promise<void> {
    await this.submitButton.click();
  }

  /**
   * Enters edit mode for a saved link.
   */
  async editLink(name: string): Promise<void> {
    const editBtn = this.getEditBtn(name);
    await editBtn.click();
  }

  /**
   * Deletes a saved link with confirmation.
   */
  async deleteLink(name: string): Promise<void> {
    const deleteBtn = this.getDeleteBtn(name);
    await deleteBtn.click();
    await this.deleteConfirmBtn.click();
  }

  /**
   * Copies a saved link to clipboard.
   */
  async copyLink(name: string): Promise<void> {
    const copyBtn = this.getCopyBtn(name);
    await copyBtn.click();
  }
}
