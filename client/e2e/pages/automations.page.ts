import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Offer Automations page.
 * Encapsulates all automation page interactions.
 */
export class AutomationsPage {
  readonly page: Page;

  // Page elements
  readonly pageTitle: Locator;
  readonly automationCountBadge: Locator;
  readonly createButton: Locator;
  readonly createFirstButton: Locator;
  readonly noAutomationsMessage: Locator;
  readonly whatsappDisconnectedAlert: Locator;
  readonly limitReachedAlert: Locator;

  // Dialog elements
  readonly dialogTitle: Locator;
  readonly nameInput: Locator;
  readonly startTimeInput: Locator;
  readonly endTimeInput: Locator;
  readonly saveButton: Locator;
  readonly cancelButton: Locator;
  readonly dialogContent: Locator;

  // Mode buttons
  readonly searchModeButton: Locator;
  readonly monitorModeButton: Locator;
  readonly listModeButton: Locator;

  // Table elements
  readonly automationTable: Locator;
  readonly automationRows: Locator;
  readonly cardsList: Locator;

  // Mode-specific elements
  readonly modeSelector: Locator;
  readonly categoriesSection: Locator;
  readonly monitorGroupsSection: Locator;
  readonly storesSection: Locator;
  readonly delayInput: Locator;
  readonly delayUnitSelector: Locator;
  readonly mlLinkListSelector: Locator;
  readonly listModeInfoAlert: Locator;
  readonly listModeStoreAlert: Locator;
  readonly textRulesSection: Locator;
  readonly prefixTextarea: Locator;
  readonly suffixTextarea: Locator;
  readonly removeMentionsCheckbox: Locator;
  readonly removePhonesCheckbox: Locator;
  readonly offerPreviewButton: Locator;
  readonly textsSection: Locator;
  readonly dispatchTextSelector: Locator;
  readonly prioritySelector: Locator;
  readonly filterTypeSelector: Locator;

  // Preview & AI elements
  readonly previewDialog: Locator;
  readonly previewOfferItems: Locator;
  readonly suggestAiButton: Locator;
  readonly couponSelector: Locator;

  constructor(page: Page) {
    this.page = page;

    // Page elements
    this.pageTitle = page.locator('h1').first();
    this.automationCountBadge = page.locator('[class*="badge"]').first();
    this.createButton = page.getByRole('button', { name: /nova|criar|new/i }).first();
    this.createFirstButton = page.getByRole('button', { name: /criar primeira|create first/i });
    this.noAutomationsMessage = page.getByText(/nenhuma automacao|no automations/i);
    this.whatsappDisconnectedAlert = page.getByText(
      /whatsapp.*desconectado|whatsapp.*disconnected/i
    );
    this.limitReachedAlert = page.getByText(/limite.*atingido|limit.*reached/i);

    // Dialog elements
    this.dialogContent = page.locator('[role="dialog"]');
    this.dialogTitle = this.dialogContent.locator('h2').first();
    this.nameInput = page.getByLabel(/nome/i).or(page.locator('input[placeholder*="nome"]'));
    this.startTimeInput = page.getByLabel(/inicio|start/i).or(page.locator('input[value="08:00"]'));
    this.endTimeInput = page.getByLabel(/fim|end/i).or(page.locator('input[value="22:00"]'));
    this.saveButton = page
      .getByRole('button', { name: /salvar|criar e ativar|save|create/i })
      .last();
    this.cancelButton = page.getByRole('button', { name: /cancelar|cancel/i });

    // Mode buttons
    this.searchModeButton = page.getByRole('button', { name: /busca|search/i });
    this.monitorModeButton = page.getByRole('button', { name: /monitor/i });
    this.listModeButton = page.getByRole('button', { name: /lista|list/i });

    // Table elements
    this.automationTable = page.locator('table');
    this.automationRows = page.locator('table tbody tr');
    this.cardsList = page.locator('[class*="card"]');

    // Mode-specific elements
    this.modeSelector = this.dialogContent.locator('button[role="combobox"]').first();
    this.categoriesSection = this.dialogContent.getByText(/categorias/i).first();
    this.monitorGroupsSection = this.dialogContent
      .getByText(/grupos a monitorar|monitorar/i)
      .first();
    this.storesSection = this.dialogContent.getByText(/lojas/i).first();
    this.delayInput = this.dialogContent.locator('input[type="number"]').first();
    this.delayUnitSelector = this.dialogContent
      .locator('button[role="combobox"]')
      .filter({ hasText: /segundos|minutos|seconds|minutes/i });
    this.mlLinkListSelector = this.dialogContent
      .locator('button[role="combobox"]')
      .filter({ hasText: /selecione uma lista|lista/i })
      .first();
    this.listModeInfoAlert = this.dialogContent.getByText(/modo lista consome links|list mode/i);
    this.listModeStoreAlert = this.dialogContent.getByText(/modo lista.*loja.*fixa|mercado livre/i);
    this.textRulesSection = this.dialogContent.getByText(/processamento de texto|text processing/i);
    this.prefixTextarea = this.dialogContent.locator('textarea').filter({ hasText: '' }).nth(0);
    this.suffixTextarea = this.dialogContent.locator('textarea').filter({ hasText: '' }).nth(1);
    this.removeMentionsCheckbox = this.dialogContent.getByLabel(
      /remover.*menções|remove.*mentions/i
    );
    this.removePhonesCheckbox = this.dialogContent.getByLabel(/remover.*telefones|remove.*phones/i);
    this.offerPreviewButton = page
      .getByRole('button', { name: /visualizar ofertas|preview/i })
      .or(page.locator('button').filter({ has: page.locator('svg.lucide-file-text') }));
    this.textsSection = this.dialogContent.getByText(/textos da mensagem|message texts/i);
    this.dispatchTextSelector = this.dialogContent
      .locator('button[role="combobox"]')
      .filter({ hasText: /padrão do sistema|dispatch|disparo/i });
    this.prioritySelector = this.dialogContent
      .locator('button[role="combobox"]')
      .filter({ hasText: /desconto|economia|comissão|discount|price|commission/i });
    this.filterTypeSelector = this.dialogContent
      .locator('button[role="combobox"]')
      .filter({ hasText: /leve|pesado|light|heavy/i });

    // Preview & AI elements
    this.previewDialog = page.locator('[role="dialog"]').filter({ hasText: /preview|ofertas/i });
    this.previewOfferItems = this.previewDialog.locator('[data-testid="preview-offer-item"]').or(
      this.previewDialog.locator('.offer-item')
    );
    this.suggestAiButton = this.dialogContent
      .getByRole('button', { name: /sugerir.*ia|suggest.*ai|ia/i })
      .or(this.dialogContent.locator('button').filter({ has: page.locator('svg') }).filter({ hasText: /ia|ai/i }));
    this.couponSelector = this.dialogContent
      .locator('button[role="combobox"]')
      .filter({ hasText: /cupom|coupon|selecione/i });
  }

  /**
   * Navigates to the automations page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/offer-automations');
    await this.page.waitForSelector('h1', { state: 'visible' });
  }

  /**
   * Opens the create automation dialog.
   */
  async openCreateDialog(): Promise<void> {
    await this.createButton.click();
    await this.dialogContent.waitFor({ state: 'visible' });
  }

  /**
   * Fills the basic automation form.
   */
  async fillBasicForm(options: {
    name: string;
    startTime?: string;
    endTime?: string;
  }): Promise<void> {
    await this.nameInput.fill(options.name);
    if (options.startTime) {
      await this.startTimeInput.clear();
      await this.startTimeInput.fill(options.startTime);
    }
    if (options.endTime) {
      await this.endTimeInput.clear();
      await this.endTimeInput.fill(options.endTime);
    }
  }

  /**
   * Selects the automation mode from the Select dropdown.
   */
  async selectMode(mode: 'search' | 'monitor' | 'list'): Promise<void> {
    // Try Select dropdown first (the actual implementation uses a Select component)
    const modeSelect = this.dialogContent.locator('button[role="combobox"]').first();
    if (await modeSelect.isVisible().catch(() => false)) {
      await modeSelect.click();
      const modeLabels: Record<string, RegExp> = {
        search: /busca de promoções|search/i,
        monitor: /monitoramento|monitor/i,
        list: /lista de links|list/i,
      };
      const option = this.page.getByRole('option', { name: modeLabels[mode] });
      if (await option.isVisible().catch(() => false)) {
        await option.click();
        return;
      }
    }
    // Fallback to button-based mode selection
    const modeButton = {
      search: this.searchModeButton,
      monitor: this.monitorModeButton,
      list: this.listModeButton,
    }[mode];
    await modeButton.click();
  }

  /**
   * Saves the automation form.
   */
  async save(): Promise<void> {
    await this.saveButton.click();
  }

  /**
   * Cancels the automation form.
   */
  async cancel(): Promise<void> {
    await this.cancelButton.click();
  }

  /**
   * Gets a row by automation name.
   */
  getRowByName(name: string): Locator {
    return this.page.getByRole('row').filter({ hasText: name });
  }

  /**
   * Gets the edit button for an automation row.
   */
  getEditButton(row: Locator): Locator {
    return row
      .getByRole('button', { name: /editar|edit/i })
      .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-edit') }));
  }

  /**
   * Gets the run button for an automation row.
   */
  getRunButton(row: Locator): Locator {
    return row
      .getByRole('button', { name: /executar|run/i })
      .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-play') }));
  }

  /**
   * Gets the reset history button for an automation row.
   */
  getResetButton(row: Locator): Locator {
    return row
      .getByRole('button', { name: /resetar|reset/i })
      .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-rotate-ccw') }));
  }

  /**
   * Gets the delete button for an automation row.
   */
  getDeleteButton(row: Locator): Locator {
    return row
      .getByRole('button', { name: /excluir|delete/i })
      .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-trash-2') }));
  }

  /**
   * Gets the status switch for an automation row.
   */
  getStatusSwitch(row: Locator): Locator {
    return row.locator('button[role="switch"]');
  }

  /**
   * Clicks the edit button for an automation.
   */
  async editAutomation(name: string): Promise<void> {
    const row = this.getRowByName(name);
    await this.getEditButton(row).click();
    await this.dialogContent.waitFor({ state: 'visible' });
  }

  /**
   * Runs an automation immediately.
   */
  async runAutomation(name: string): Promise<void> {
    const row = this.getRowByName(name);
    await this.getRunButton(row).click();
  }

  /**
   * Resets the history of an automation.
   */
  async resetHistory(name: string): Promise<void> {
    const row = this.getRowByName(name);
    await this.getResetButton(row).click();
  }

  /**
   * Deletes an automation.
   */
  async deleteAutomation(name: string): Promise<void> {
    const row = this.getRowByName(name);
    await this.getDeleteButton(row).click();
  }

  /**
   * Toggles the status of an automation.
   */
  async toggleStatus(name: string): Promise<void> {
    const row = this.getRowByName(name);
    await this.getStatusSwitch(row).click();
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
    await this.page.waitForSelector('h1', { state: 'visible' });
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
  }

  /**
   * Gets the number of automations displayed.
   */
  async getAutomationCount(): Promise<number> {
    const count = await this.automationRows.count();
    return count;
  }

  /**
   * Checks if an automation exists.
   */
  async automationExists(name: string): Promise<boolean> {
    const row = this.getRowByName(name);
    return row.isVisible();
  }

  /**
   * Clicks the preview button for an automation in the table.
   */
  async clickPreviewButton(automationName: string): Promise<void> {
    const row = this.getRowByName(automationName);
    const previewBtn = row
      .getByRole('button', { name: /visualizar|preview/i })
      .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-eye') }));
    await previewBtn.click();
  }
}
