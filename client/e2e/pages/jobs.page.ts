import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Jobs (Agendamentos) page.
 * Encapsulates all job scheduling interactions.
 *
 * Key UI notes:
 * - Page title is an h2 (not h1), inside the jobs-page container.
 * - Desktop table is visible on md+ (hidden md:block).
 * - Mobile card list is visible below md (md:hidden).
 * - Job form opens in a Dialog (role="dialog").
 * - Action type select renders as a button trigger (SelectTrigger).
 * - Edit button is a text button (not icon); delete is a Trash2 icon button.
 * - Delete button only appears for pending/failed jobs.
 */
export class JobsPage {
  readonly page: Page;

  // -- Page structure --
  readonly pageTitle: Locator;
  readonly pageSubtitle: Locator;
  readonly warningAlert: Locator;
  readonly howItWorksCard: Locator;

  // -- New job button (dialog trigger) --
  readonly newJobButton: Locator;

  // -- Dialog --
  readonly dialogContent: Locator;
  readonly dialogTitle: Locator;

  // -- Form: action type --
  readonly actionTypeSelect: Locator;

  // -- Form: date/time --
  readonly dateInput: Locator;
  readonly timeInput: Locator;

  // -- Form: submit --
  readonly submitButton: Locator;

  // -- Form: send_message fields --
  readonly messageTextarea: Locator;
  readonly enhanceButton: Locator;
  readonly mediaInput: Locator;
  readonly mentionButton: Locator;

  // -- Form: send_poll fields --
  readonly pollQuestionTextarea: Locator;
  readonly addPollOptionButton: Locator;

  // -- Form: send_ml_offer fields --
  readonly mlUrlInput: Locator;
  readonly mlIncludeImageSwitch: Locator;
  readonly mlDispatchTextarea: Locator;

  // -- Form: update_description fields --
  readonly descriptionTextarea: Locator;

  // -- Form: change_group_name fields --
  readonly groupNameInput: Locator;
  readonly autoNumberSwitch: Locator;

  // -- Form: change_group_photo fields --
  readonly groupPhotoInput: Locator;

  // -- Form: group selector --
  readonly groupSearchInput: Locator;
  readonly groupSelectorContainer: Locator;

  // -- Table (desktop, md+) --
  readonly jobsTable: Locator;
  readonly jobRows: Locator;

  // -- Mobile cards (below md) --
  readonly mobileCards: Locator;

  // -- Pagination --
  readonly loadMoreButton: Locator;
  readonly showingCountText: Locator;

  // -- States --
  readonly emptyState: Locator;
  readonly loadingSpinner: Locator;

  // -- Confirmation dialog --
  readonly confirmDialog: Locator;
  readonly confirmButton: Locator;

  // -- Toast --
  readonly successToast: Locator;

  constructor(page: Page) {
    this.page = page;

    // The jobs page uses h2 for the title, inside a .jobs-page container
    this.pageTitle = page.locator('.jobs-page h2').first();
    this.pageSubtitle = page.locator('.jobs-page h2 + p').first();

    // Warning alert card (role="alert")
    this.warningAlert = page.locator('[role="alert"]').first();

    // "Como funciona" / "How it works" section (identified by HelpCircle icon context)
    this.howItWorksCard = page.getByText(/como funciona\??|how it works\??/i);

    // New job button - the dialog trigger button with Plus icon
    this.newJobButton = page
      .locator('.jobs-page')
      .getByRole('button')
      .filter({ hasText: /novo.*agendamento|new.*job|agendar|schedule/i })
      .first();

    // Dialog
    this.dialogContent = page.locator('[role="dialog"]');
    this.dialogTitle = this.dialogContent.locator('h2').first();

    // Action type select trigger (rendered as button by Radix Select)
    this.actionTypeSelect = this.dialogContent.locator('#action-type');

    // Date and time inputs inside dialog
    this.dateInput = this.dialogContent.locator('#date');
    this.timeInput = this.dialogContent.locator('#time');

    // Submit button inside dialog (last button with auth-btn-glow in dialog, or matching text)
    this.submitButton = this.dialogContent
      .getByRole('button')
      .filter({ hasText: /agendar|salvar|save|schedule/i })
      .last();

    // -- send_message form fields --
    this.messageTextarea = this.dialogContent.locator('#payload');
    this.enhanceButton = this.dialogContent.getByRole('button', {
      name: /aprimorar|enhance|sparkles/i,
    });
    this.mediaInput = this.dialogContent.locator('#media');
    this.mentionButton = this.dialogContent.getByRole('button', {
      name: /mencion|mention|@/i,
    });

    // -- send_poll form fields --
    this.pollQuestionTextarea = this.dialogContent.locator('#poll-question');
    this.addPollOptionButton = this.dialogContent.getByRole('button', {
      name: /adicionar.*op[cç][aã]o|add.*option/i,
    });

    // -- send_ml_offer form fields --
    this.mlUrlInput = this.dialogContent.locator('#ml-url');
    this.mlIncludeImageSwitch = this.dialogContent.locator('#ml-include-image');
    this.mlDispatchTextarea = this.dialogContent.locator('#ml-text');

    // -- update_description fields (reuses #payload textarea) --
    this.descriptionTextarea = this.dialogContent.locator('#payload');

    // -- change_group_name fields (reuses #payload input) --
    this.groupNameInput = this.dialogContent.locator('#payload');
    this.autoNumberSwitch = this.dialogContent.locator('#auto-number-job');

    // -- change_group_photo fields --
    this.groupPhotoInput = this.dialogContent.locator('#group-photo');

    // -- Group selector --
    this.groupSearchInput = this.dialogContent
      .locator('input')
      .filter({ has: page.locator('~ .lucide-search, + .lucide-search') })
      .or(
        this.dialogContent.locator('input[placeholder]').filter({
          hasText: /buscar.*grupo|search.*group/i,
        })
      )
      .first();
    // Fallback: find the search input by its sibling Search icon
    this.groupSearchInput = this.dialogContent
      .locator('.relative')
      .filter({ has: page.locator('.lucide-search') })
      .locator('input')
      .first();
    this.groupSelectorContainer = this.dialogContent
      .locator('.border.rounded-lg')
      .filter({ has: page.locator('[role="checkbox"]') })
      .first();

    // -- Table (desktop) --
    this.jobsTable = page.locator('.jobs-page table');
    this.jobRows = page.locator('.jobs-page table tbody tr');

    // -- Mobile cards (md:hidden container children) --
    this.mobileCards = page.locator('.jobs-page .md\\:hidden > div > div');

    // -- Pagination --
    this.loadMoreButton = page.locator('.jobs-page').getByRole('button', {
      name: /carregar.*mais|load.*more/i,
    });
    this.showingCountText = page.getByText(/exibindo.*de|showing.*of/i);

    // -- States --
    this.emptyState = page.getByText(/nenhum.*agendamento|no.*jobs|sem.*agendamento/i);
    this.loadingSpinner = page.locator('.jobs-page [class*="animate-spin"]').first();

    // -- Confirmation dialog --
    this.confirmDialog = page
      .locator('[role="alertdialog"]')
      .or(page.locator('[role="dialog"]').filter({ hasText: /confirmar|confirm/i }));
    this.confirmButton = this.confirmDialog.getByRole('button', {
      name: /confirmar|confirm|sim|yes|excluir|delete/i,
    });

    // -- Toast --
    this.successToast = page.locator('[data-sonner-toast]');
  }

  /**
   * Navigates to the jobs page and waits for the title to be visible.
   */
  async goto(): Promise<void> {
    await this.page.goto('/jobs');
    await this.page.waitForSelector('.jobs-page h2', { state: 'visible' });
  }

  /**
   * Waits for the page to fully load (title visible + spinner gone).
   */
  async waitForLoad(): Promise<void> {
    await this.page.waitForSelector('.jobs-page h2', { state: 'visible' });
    // Wait for loading spinner to disappear
    await this.page
      .waitForFunction(
        () => {
          const spinner = document.querySelector('.jobs-page [class*="animate-spin"]');
          return !spinner || getComputedStyle(spinner).display === 'none';
        },
        { timeout: 10000 }
      )
      .catch(() => {});
  }

  /**
   * Opens the create job dialog by clicking the "Novo Agendamento" button.
   */
  async openCreateDialog(): Promise<void> {
    await this.newJobButton.click();
    await this.dialogContent.waitFor({ state: 'visible' });
  }

  /**
   * Selects an action type from the Select dropdown.
   * Clicks the trigger, then clicks the option matching the provided text.
   * @param type - The visible text of the option (e.g., "Enviar mensagem", "Enviar enquete")
   *               or the value (e.g., "send_message", "send_poll")
   */
  async selectActionType(type: string): Promise<void> {
    await this.actionTypeSelect.click();
    // SelectContent renders in a portal; find the option in the page
    const option = this.page
      .locator('[role="option"]')
      .filter({ hasText: new RegExp(type, 'i') });
    await option.click();
  }

  /**
   * Fills the date and time inputs in the job form.
   * @param date - Date string in YYYY-MM-DD format
   * @param time - Time string in HH:MM format
   */
  async fillDateAndTime(date: string, time: string): Promise<void> {
    await this.dateInput.fill(date);
    await this.timeInput.fill(time);
  }

  /**
   * Selects groups by name in the group selector.
   * For each name, finds the checkbox label matching the name and clicks it.
   * @param names - Array of group names to select
   */
  async selectGroups(names: string[]): Promise<void> {
    for (const name of names) {
      // Find the group item by its label text and click
      const groupItem = this.dialogContent
        .locator('label, [role="checkbox"]')
        .filter({ hasText: name })
        .first()
        .or(
          this.dialogContent.locator('div').filter({ hasText: name }).locator('[role="checkbox"]')
        );
      // Click the container div that has onClick handler
      const clickTarget = this.dialogContent
        .locator('.cursor-pointer')
        .filter({ hasText: name })
        .filter({ has: this.page.locator('[role="checkbox"]') })
        .first();
      await clickTarget.click();
    }
  }

  /**
   * Fills the message textarea (send_message action type).
   * @param msg - The message text
   */
  async fillMessageForm(msg: string): Promise<void> {
    await this.messageTextarea.fill(msg);
  }

  /**
   * Fills the poll form (send_poll action type).
   * @param question - The poll question
   * @param options - Array of poll option texts to fill
   */
  async fillPollForm(question: string, options: string[]): Promise<void> {
    await this.pollQuestionTextarea.fill(question);
    // Poll options are Input elements inside the poll form
    const optionInputs = this.dialogContent
      .locator('input[placeholder]')
      .filter({ hasNotText: /buscar|search/i });
    for (let i = 0; i < options.length; i++) {
      const input = optionInputs.nth(i);
      const isVisible = await input.isVisible().catch(() => false);
      if (!isVisible && i >= 2) {
        // Need to add more options
        await this.addPollOptionButton.click();
      }
      await optionInputs.nth(i).fill(options[i]);
    }
  }

  /**
   * Fills the ML offer form (send_ml_offer action type).
   * @param url - The product URL
   * @param text - Optional dispatch text
   * @param includeImage - Whether to toggle include image switch
   */
  async fillMlOfferForm(
    url: string,
    text?: string,
    includeImage?: boolean
  ): Promise<void> {
    await this.mlUrlInput.fill(url);
    if (includeImage !== undefined) {
      const isChecked = await this.mlIncludeImageSwitch.isChecked();
      if (isChecked !== includeImage) {
        await this.mlIncludeImageSwitch.click();
      }
    }
    if (text) {
      await this.mlDispatchTextarea.fill(text);
    }
  }

  /**
   * Fills the description textarea (update_description action type).
   * @param desc - The new description text
   */
  async fillDescriptionForm(desc: string): Promise<void> {
    await this.descriptionTextarea.fill(desc);
  }

  /**
   * Fills the group name form (change_group_name action type).
   * @param name - The new group name (or base name if auto-numbering)
   * @param autoNumber - Whether to enable auto-numbering
   */
  async fillGroupNameForm(name: string, autoNumber?: boolean): Promise<void> {
    if (autoNumber !== undefined) {
      const isChecked = await this.autoNumberSwitch.isChecked();
      if (isChecked !== autoNumber) {
        await this.autoNumberSwitch.click();
      }
    }
    await this.groupNameInput.fill(name);
  }

  /**
   * Clicks the submit button in the job form dialog.
   */
  async submitForm(): Promise<void> {
    await this.submitButton.click();
  }

  /**
   * Gets a specific table row by zero-based index.
   * @param index - Row index (0-based)
   */
  getJobRow(index: number): Locator {
    return this.jobRows.nth(index);
  }

  /**
   * Gets a table row containing the specified text.
   * @param text - Text to search for in the row
   */
  getJobRowByText(text: string): Locator {
    return this.jobRows.filter({ hasText: text });
  }

  /**
   * Gets the edit button within a table row.
   * The edit button is a text button (not icon) with translated "Editar"/"Edit" text.
   * @param row - The table row locator
   */
  getEditButton(row: Locator): Locator {
    return row.getByRole('button', { name: /editar|edit/i }).or(
      row.locator('button').filter({ hasText: /editar|edit/i })
    );
  }

  /**
   * Gets the delete button within a table row.
   * The delete button is a Trash2 icon button, only present for pending/failed jobs.
   * @param row - The table row locator
   */
  getDeleteButton(row: Locator): Locator {
    return row
      .locator('button')
      .filter({ has: this.page.locator('.lucide-trash-2, .lucide-trash2') })
      .or(row.getByRole('button', { name: /excluir|delete|remover|remove/i }));
  }

  /**
   * Gets the status badge within a table row.
   * @param row - The table row locator
   */
  getStatusBadge(row: Locator): Locator {
    return row.locator('[class*="badge"], [data-slot="badge"]').first();
  }

  /**
   * Gets the edit button on a mobile card.
   * Mobile edit buttons show "Editar agendamento" / "Edit schedule" text.
   * @param card - The mobile card locator
   */
  getMobileEditButton(card: Locator): Locator {
    return card.getByRole('button', {
      name: /editar.*agendamento|edit.*schedule|editar|edit/i,
    });
  }

  /**
   * Gets the delete button on a mobile card.
   * @param card - The mobile card locator
   */
  getMobileDeleteButton(card: Locator): Locator {
    return card
      .locator('button')
      .filter({ has: this.page.locator('.lucide-trash-2, .lucide-trash2') });
  }

  /**
   * Clicks the "Carregar mais" / "Load more" button.
   */
  async loadMore(): Promise<void> {
    await this.loadMoreButton.click();
  }

  /**
   * Returns the count of job rows in the desktop table.
   */
  async getJobCount(): Promise<number> {
    return this.jobRows.count();
  }

  /**
   * Returns the count of mobile job cards.
   */
  async getMobileJobCount(): Promise<number> {
    return this.mobileCards.count();
  }

  /**
   * Checks whether the empty state message is visible.
   */
  async isEmpty(): Promise<boolean> {
    return this.emptyState.isVisible();
  }

  /**
   * Checks whether the loading spinner is visible.
   */
  async isLoading(): Promise<boolean> {
    return this.loadingSpinner.isVisible();
  }

  /**
   * Searches for groups within the dialog group selector.
   * @param term - Search term
   */
  async searchGroups(term: string): Promise<void> {
    await this.groupSearchInput.fill(term);
  }

  /**
   * Enhances the message using AI (clicks the Aprimorar button).
   */
  async enhanceMessage(): Promise<void> {
    await this.enhanceButton.click();
  }

  /**
   * Deletes a job from the table by clicking its delete button and confirming.
   * @param rowIndex - The zero-based index of the row to delete
   */
  async deleteJobByIndex(rowIndex: number): Promise<void> {
    const row = this.getJobRow(rowIndex);
    const deleteBtn = this.getDeleteButton(row);
    await deleteBtn.click();
    if (await this.confirmDialog.isVisible()) {
      await this.confirmButton.click();
    }
  }

  /**
   * Edits a job from the table by clicking its edit button.
   * @param rowIndex - The zero-based index of the row to edit
   */
  async editJobByIndex(rowIndex: number): Promise<void> {
    const row = this.getJobRow(rowIndex);
    const editBtn = this.getEditButton(row);
    await editBtn.click();
    await this.dialogContent.waitFor({ state: 'visible' });
  }
}
