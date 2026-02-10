import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Jobs page.
 * Encapsulates all job management interactions.
 */
export class JobsPage {
  readonly page: Page;

  // Page elements
  readonly pageTitle: Locator;
  readonly jobCountBadge: Locator;

  // Actions
  readonly createButton: Locator;

  // Table/List elements
  readonly jobsTable: Locator;
  readonly jobRows: Locator;
  readonly emptyState: Locator;

  // Filter/tabs
  readonly tabAll: Locator;
  readonly tabPending: Locator;
  readonly tabRunning: Locator;
  readonly tabCompleted: Locator;
  readonly tabFailed: Locator;

  // Dialog elements
  readonly dialogContent: Locator;
  readonly dialogTitle: Locator;

  // Form fields
  readonly nameInput: Locator;
  readonly typeSelect: Locator;
  readonly messageInput: Locator;
  readonly scheduledAtInput: Locator;
  readonly groupSelector: Locator;

  // Poll-specific fields
  readonly pollQuestionInput: Locator;
  readonly pollOptionsContainer: Locator;
  readonly addPollOptionButton: Locator;

  // Form actions
  readonly saveButton: Locator;
  readonly cancelButton: Locator;

  // Confirmation dialog
  readonly confirmDialog: Locator;
  readonly confirmButton: Locator;

  // Loading states
  readonly loadingSpinner: Locator;

  constructor(page: Page) {
    this.page = page;

    // Page elements
    this.pageTitle = page.locator('h1');
    this.jobCountBadge = page.locator('[class*="badge"]').first();

    // Actions
    this.createButton = page.getByRole('button', {
      name: /nova.*tarefa|new.*job|criar|create/i,
    });

    // Table/List elements
    this.jobsTable = page.locator('table');
    this.jobRows = page
      .locator('table tbody tr')
      .or(page.locator('[class*="job-card"], [class*="job-item"]'));
    this.emptyState = page.getByText(/nenhuma.*tarefa|no.*jobs|sem.*tarefas/i);

    // Filter/tabs
    this.tabAll = page.getByRole('tab', { name: /todos|all/i });
    this.tabPending = page.getByRole('tab', { name: /pendente|pending/i });
    this.tabRunning = page.getByRole('tab', { name: /executando|running/i });
    this.tabCompleted = page.getByRole('tab', { name: /concluido|completed/i });
    this.tabFailed = page.getByRole('tab', { name: /falha|failed/i });

    // Dialog elements
    this.dialogContent = page.locator('[role="dialog"]');
    this.dialogTitle = this.dialogContent.locator('h2').first();

    // Form fields
    this.nameInput = page.getByLabel(/nome/i).or(page.locator('input[placeholder*="nome"]'));
    this.typeSelect = page.getByLabel(/tipo/i).or(page.locator('[data-testid="job-type"]'));
    this.messageInput = page.getByLabel(/mensagem/i).or(page.locator('textarea'));
    this.scheduledAtInput = page
      .getByLabel(/agendamento|scheduled/i)
      .or(page.locator('input[type="datetime-local"]'));
    this.groupSelector = page
      .getByLabel(/grupos/i)
      .or(page.locator('[data-testid="group-selector"]'));

    // Poll-specific fields
    this.pollQuestionInput = page
      .getByLabel(/pergunta/i)
      .or(page.locator('input[placeholder*="pergunta"]'));
    this.pollOptionsContainer = page
      .locator('[data-testid="poll-options"]')
      .or(page.locator('[class*="poll-options"]'));
    this.addPollOptionButton = page.getByRole('button', {
      name: /adicionar.*opcao|add.*option/i,
    });

    // Form actions
    this.saveButton = page.getByRole('button', { name: /salvar|criar|save|create/i }).last();
    this.cancelButton = page.getByRole('button', { name: /cancelar|cancel/i });

    // Confirmation dialog
    this.confirmDialog = page
      .locator('[role="alertdialog"]')
      .or(page.locator('[role="dialog"]').filter({ hasText: /confirmar|confirm/i }));
    this.confirmButton = this.confirmDialog.getByRole('button', {
      name: /confirmar|confirm|sim|yes|excluir|delete/i,
    });

    // Loading states
    this.loadingSpinner = page.locator('[class*="animate-spin"]').first();
  }

  /**
   * Navigates to the jobs page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/jobs');
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
   * Opens the create job dialog.
   */
  async openCreateDialog(): Promise<void> {
    await this.createButton.click();
    await this.dialogContent.waitFor({ state: 'visible' });
  }

  /**
   * Gets a job row by name.
   */
  getJobByName(name: string): Locator {
    return this.page.getByRole('row').filter({ hasText: name });
  }

  /**
   * Gets the run now button for a job row.
   */
  getRunNowButton(row: Locator): Locator {
    return row
      .getByRole('button', { name: /executar.*agora|run.*now/i })
      .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-play') }));
  }

  /**
   * Gets the cancel button for a job row.
   */
  getCancelButton(row: Locator): Locator {
    return row
      .getByRole('button', { name: /cancelar|cancel/i })
      .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-x') }));
  }

  /**
   * Gets the edit button for a job row.
   */
  getEditButton(row: Locator): Locator {
    return row
      .getByRole('button', { name: /editar|edit/i })
      .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-edit') }));
  }

  /**
   * Gets the delete button for a job row.
   */
  getDeleteButton(row: Locator): Locator {
    return row
      .getByRole('button', { name: /excluir|delete/i })
      .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-trash') }));
  }

  /**
   * Gets the status badge for a job row.
   */
  getStatusBadge(row: Locator): Locator {
    return row.locator('[class*="badge"]').first();
  }

  /**
   * Fills the job form.
   */
  async fillJobForm(options: {
    name: string;
    type?: 'message' | 'poll' | 'ml_offer';
    message?: string;
    scheduledAt?: string;
  }): Promise<void> {
    await this.nameInput.fill(options.name);

    if (options.type) {
      await this.typeSelect.click();
      const typeOption = this.page.getByRole('option', {
        name: new RegExp(options.type, 'i'),
      });
      await typeOption.click();
    }

    if (options.message) {
      await this.messageInput.fill(options.message);
    }

    if (options.scheduledAt) {
      await this.scheduledAtInput.fill(options.scheduledAt);
    }
  }

  /**
   * Saves the job form.
   */
  async save(): Promise<void> {
    await this.saveButton.click();
  }

  /**
   * Cancels the job form.
   */
  async cancel(): Promise<void> {
    await this.cancelButton.click();
  }

  /**
   * Runs a job immediately.
   */
  async runJobNow(jobName: string): Promise<void> {
    const row = this.getJobByName(jobName);
    const runButton = this.getRunNowButton(row);
    await runButton.click();
  }

  /**
   * Cancels a pending job.
   */
  async cancelJob(jobName: string): Promise<void> {
    const row = this.getJobByName(jobName);
    const cancelBtn = this.getCancelButton(row);
    await cancelBtn.click();

    if (await this.confirmDialog.isVisible()) {
      await this.confirmButton.click();
    }
  }

  /**
   * Deletes a job.
   */
  async deleteJob(jobName: string): Promise<void> {
    const row = this.getJobByName(jobName);
    const deleteBtn = this.getDeleteButton(row);
    await deleteBtn.click();

    if (await this.confirmDialog.isVisible()) {
      await this.confirmButton.click();
    }
  }

  /**
   * Edits a job.
   */
  async editJob(jobName: string): Promise<void> {
    const row = this.getJobByName(jobName);
    const editBtn = this.getEditButton(row);
    await editBtn.click();
    await this.dialogContent.waitFor({ state: 'visible' });
  }

  /**
   * Filters jobs by status.
   */
  async filterByStatus(
    status: 'all' | 'pending' | 'running' | 'completed' | 'failed'
  ): Promise<void> {
    const tabs = {
      all: this.tabAll,
      pending: this.tabPending,
      running: this.tabRunning,
      completed: this.tabCompleted,
      failed: this.tabFailed,
    };
    await tabs[status].click();
  }

  /**
   * Gets the count of displayed jobs.
   */
  async getJobCount(): Promise<number> {
    return this.jobRows.count();
  }

  /**
   * Checks if empty state is shown.
   */
  async isEmpty(): Promise<boolean> {
    return this.emptyState.isVisible();
  }

  /**
   * Gets the status of a job.
   */
  async getJobStatus(jobName: string): Promise<string | null> {
    const row = this.getJobByName(jobName);
    const badge = this.getStatusBadge(row);
    return badge.textContent();
  }
}
