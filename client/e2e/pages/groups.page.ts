import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Groups page.
 * Encapsulates all group management interactions.
 */
export class GroupsPage {
  readonly page: Page;

  // Page elements
  readonly pageTitle: Locator;
  readonly groupCountBadge: Locator;

  // Actions
  readonly syncButton: Locator;
  readonly searchInput: Locator;
  readonly filterButton: Locator;

  // Table/List elements
  readonly groupsTable: Locator;
  readonly groupRows: Locator;
  readonly emptyState: Locator;

  // Filter options
  readonly filterFavorites: Locator;
  readonly filterActive: Locator;
  readonly filterAll: Locator;

  // Alerts
  readonly whatsappAlert: Locator;
  readonly syncSuccessToast: Locator;
  readonly errorToast: Locator;

  // Loading states
  readonly loadingSpinner: Locator;
  readonly syncingIndicator: Locator;

  constructor(page: Page) {
    this.page = page;

    // Page elements
    this.pageTitle = page.locator('h1');
    this.groupCountBadge = page.locator('[class*="badge"]').first();

    // Actions
    this.syncButton = page.getByRole('button', {
      name: /sincronizar|sync/i,
    });
    this.searchInput = page
      .locator('input[type="search"]')
      .or(page.getByPlaceholder(/buscar|search|pesquisar/i));
    this.filterButton = page.getByRole('button', {
      name: /filtrar|filter/i,
    });

    // Table/List elements
    this.groupsTable = page.locator('table');
    this.groupRows = page
      .locator('table tbody tr')
      .or(page.locator('[class*="group-card"], [class*="group-item"]'));
    this.emptyState = page.getByText(/nenhum grupo|no groups|sem grupos|grupos.*encontrados/i);

    // Filter options
    this.filterFavorites = page.getByRole('button', {
      name: /favoritos|favorites/i,
    });
    this.filterActive = page.getByRole('button', {
      name: /ativos|active/i,
    });
    this.filterAll = page.getByRole('button', {
      name: /todos|all/i,
    });

    // Alerts
    this.whatsappAlert = page.locator('[role="alert"]').filter({
      hasText: /whatsapp.*desconectado|whatsapp.*disconnected/i,
    });
    this.syncSuccessToast = page.locator('[data-sonner-toast]').filter({
      hasText: /sincronizado|synced|sucesso|success/i,
    });
    this.errorToast = page.locator('[data-sonner-toast]').filter({
      hasText: /erro|error|falha|failed/i,
    });

    // Loading states
    this.loadingSpinner = page.locator('[class*="animate-spin"]').first();
    this.syncingIndicator = page.getByText(/sincronizando|syncing/i);
  }

  /**
   * Navigates to the groups page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/groups');
    await this.page.waitForSelector('h1', { state: 'visible' });
  }

  /**
   * Waits for the page to load.
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
        // Ignore timeout
      });
  }

  /**
   * Gets a group row by name.
   */
  getGroupByName(name: string): Locator {
    return this.page.getByRole('row').filter({ hasText: name });
  }

  /**
   * Gets the favorite toggle for a group row.
   */
  getFavoriteToggle(row: Locator): Locator {
    return row
      .locator('button')
      .filter({ has: this.page.locator('svg.lucide-star') })
      .or(row.locator('button[aria-label*="favorito"]'));
  }

  /**
   * Gets the delete button for a group row.
   */
  getDeleteButton(row: Locator): Locator {
    return row
      .getByRole('button', { name: /excluir|delete/i })
      .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-trash') }));
  }

  /**
   * Syncs groups from WhatsApp.
   */
  async syncGroups(): Promise<void> {
    await this.syncButton.click();
  }

  /**
   * Searches for a group.
   */
  async searchGroup(query: string): Promise<void> {
    await this.searchInput.fill(query);
  }

  /**
   * Clears the search input.
   */
  async clearSearch(): Promise<void> {
    await this.searchInput.clear();
  }

  /**
   * Filters to show only favorites.
   */
  async filterByFavorites(): Promise<void> {
    await this.filterFavorites.click();
  }

  /**
   * Filters to show only active groups.
   */
  async filterByActive(): Promise<void> {
    await this.filterActive.click();
  }

  /**
   * Shows all groups.
   */
  async showAll(): Promise<void> {
    await this.filterAll.click();
  }

  /**
   * Toggles favorite status for a group.
   */
  async toggleFavorite(groupName: string): Promise<void> {
    const row = this.getGroupByName(groupName);
    const toggle = this.getFavoriteToggle(row);
    await toggle.click();
  }

  /**
   * Deletes a group.
   */
  async deleteGroup(groupName: string): Promise<void> {
    const row = this.getGroupByName(groupName);
    const deleteBtn = this.getDeleteButton(row);
    await deleteBtn.click();

    // Confirm deletion if dialog appears
    const confirmBtn = this.page.getByRole('button', {
      name: /confirmar|confirm|excluir|delete/i,
    });
    if (await confirmBtn.isVisible()) {
      await confirmBtn.click();
    }
  }

  /**
   * Gets the count of displayed groups.
   */
  async getGroupCount(): Promise<number> {
    return this.groupRows.count();
  }

  /**
   * Checks if WhatsApp disconnected alert is shown.
   */
  async isWhatsAppDisconnected(): Promise<boolean> {
    return this.whatsappAlert.isVisible();
  }

  /**
   * Checks if empty state is shown.
   */
  async isEmpty(): Promise<boolean> {
    return this.emptyState.isVisible();
  }

  /**
   * Checks if syncing is in progress.
   */
  async isSyncing(): Promise<boolean> {
    return this.syncingIndicator.isVisible() || this.loadingSpinner.isVisible();
  }

  /**
   * Waits for sync to complete.
   */
  async waitForSyncComplete(): Promise<void> {
    await this.syncingIndicator.waitFor({ state: 'hidden', timeout: 30000 }).catch(() => {});
    await this.loadingSpinner.waitFor({ state: 'hidden', timeout: 10000 }).catch(() => {});
  }

  /**
   * Gets the sync status text from the badge.
   */
  async getSyncStatus(groupName: string): Promise<string | null> {
    const row = this.getGroupByName(groupName);
    const badge = row.locator('[class*="badge"]').first();
    return badge.textContent();
  }

  // ============================================================
  // Messaging Dialog Helpers
  // ============================================================

  /**
   * Gets the "Send Message" trigger button in bulk actions.
   */
  get sendMessageButton(): Locator {
    return this.page.getByRole('button', {
      name: /enviar mensagem|send message|mensagem/i,
    });
  }

  /**
   * Gets the "Send Audio" trigger button in bulk actions.
   */
  get sendAudioButton(): Locator {
    return this.page.getByRole('button', {
      name: /enviar áudio|enviar audio|send audio|áudio|audio/i,
    });
  }

  /**
   * Gets the "Send Poll" trigger button in bulk actions.
   */
  get sendPollButton(): Locator {
    return this.page.getByRole('button', {
      name: /enviar enquete|send poll|enquete/i,
    });
  }

  /**
   * Gets the active dialog container.
   */
  get dialog(): Locator {
    return this.page.locator('[role="dialog"]');
  }

  /**
   * Gets the dialog title.
   */
  get dialogTitle(): Locator {
    return this.dialog.locator('[class*="DialogTitle"], h2').first();
  }

  /**
   * Gets the message textarea inside the send message dialog.
   */
  get messageTextarea(): Locator {
    return this.dialog.locator('textarea#message').or(this.dialog.locator('textarea').first());
  }

  /**
   * Gets the submit button inside the active dialog.
   */
  get dialogSubmitButton(): Locator {
    return this.dialog.getByRole('button', {
      name: /enviar mensagem|enviar áudio|enviar enquete|enviar audio|send message|send audio|send poll/i,
    });
  }

  /**
   * Gets the mention button inside the message dialog.
   */
  get mentionButton(): Locator {
    return this.dialog.getByRole('button', {
      name: /mencionar|mention/i,
    });
  }

  /**
   * Gets the mention @todos option.
   */
  get mentionAllOption(): Locator {
    return this.page.getByText(/mencionar todos|mention all/i);
  }

  /**
   * Gets the @todos mention alert inside the message dialog.
   */
  get mentionAlert(): Locator {
    return this.dialog.locator('[role="alert"]').or(this.dialog.locator('.bg-blue-50'));
  }

  /**
   * Gets the audio file input inside the audio dialog.
   */
  get audioFileInput(): Locator {
    return this.dialog
      .locator('input#audio[type="file"]')
      .or(this.dialog.locator('input[type="file"][accept*="audio"]'));
  }

  /**
   * Gets the PTT switch inside the audio dialog.
   */
  get audioPttSwitch(): Locator {
    return this.dialog.locator('button[role="switch"]');
  }

  /**
   * Gets the audio limit alert (>10 groups).
   */
  get audioLimitAlert(): Locator {
    return this.dialog.locator('[role="alert"]').filter({
      hasText: /limite|limit/i,
    });
  }

  /**
   * Gets the poll question textarea.
   */
  get pollQuestionTextarea(): Locator {
    return this.dialog
      .locator('textarea#poll-question')
      .or(this.dialog.locator('textarea').first());
  }

  /**
   * Gets all poll option inputs.
   */
  get pollOptionInputs(): Locator {
    return this.dialog
      .locator('input[placeholder*="Op"]')
      .or(this.dialog.locator('input[placeholder*="op"]'));
  }

  /**
   * Gets the "Add Option" button in the poll dialog.
   */
  get addPollOptionButton(): Locator {
    return this.dialog.getByRole('button', {
      name: /adicionar op|add option/i,
    });
  }

  /**
   * Gets the remove option buttons in the poll dialog.
   */
  get removePollOptionButtons(): Locator {
    return this.dialog
      .locator('button[aria-label*="remover"], button[aria-label*="remove"]')
      .or(this.dialog.locator('button').filter({ has: this.page.locator('svg.lucide-trash-2') }));
  }

  /**
   * Gets a toast by matching text pattern.
   */
  getToast(textPattern: RegExp): Locator {
    return this.page.locator('[data-sonner-toast]').filter({ hasText: textPattern });
  }

  // ============================================================
  // Messaging Actions
  // ============================================================

  /**
   * Opens the send message dialog.
   */
  async openSendMessageDialog(): Promise<void> {
    await this.sendMessageButton.click();
    await this.dialog.waitFor({ state: 'visible' });
  }

  /**
   * Opens the send audio dialog.
   */
  async openSendAudioDialog(): Promise<void> {
    await this.sendAudioButton.click();
    await this.dialog.waitFor({ state: 'visible' });
  }

  /**
   * Opens the send poll dialog.
   */
  async openSendPollDialog(): Promise<void> {
    await this.sendPollButton.click();
    await this.dialog.waitFor({ state: 'visible' });
  }

  /**
   * Types a message in the message dialog textarea.
   */
  async typeMessage(text: string): Promise<void> {
    await this.messageTextarea.fill(text);
  }

  /**
   * Clicks the submit/send button inside the dialog.
   */
  async submitDialog(): Promise<void> {
    await this.dialogSubmitButton.click();
  }

  /**
   * Inserts @todos mention via the mention popover.
   */
  async insertMentionAll(): Promise<void> {
    await this.mentionButton.click();
    await this.mentionAllOption.click();
  }

  /**
   * Types the poll question.
   */
  async typePollQuestion(question: string): Promise<void> {
    await this.pollQuestionTextarea.fill(question);
  }

  /**
   * Fills a poll option by index (0-based).
   */
  async fillPollOption(index: number, value: string): Promise<void> {
    await this.pollOptionInputs.nth(index).fill(value);
  }

  /**
   * Adds a new poll option.
   */
  async addPollOption(): Promise<void> {
    await this.addPollOptionButton.click();
  }

  /**
   * Selects a group checkbox by name.
   */
  async selectGroup(groupName: string): Promise<void> {
    const row = this.getGroupByName(groupName);
    const checkbox = row.locator('input[type="checkbox"]').or(row.getByRole('checkbox'));
    if (await checkbox.isVisible()) {
      await checkbox.check();
    }
  }

  /**
   * Selects all groups using the "Select All" checkbox.
   */
  async selectAllGroups(): Promise<void> {
    const selectAll = this.page
      .getByRole('checkbox', {
        name: /selecionar todos|select all/i,
      })
      .or(this.page.locator('thead input[type="checkbox"]'));
    if (await selectAll.isVisible()) {
      await selectAll.check();
    }
  }

  // ============================================================
  // Members Dialog Helpers
  // ============================================================

  /**
   * Gets the members dialog container.
   */
  get membersDialog(): Locator {
    return this.page.locator('[role="dialog"]').filter({
      hasText: /membros|members|participantes|participants/i,
    });
  }

  /**
   * Gets the participant rows inside the members dialog.
   */
  get memberRows(): Locator {
    return this.membersDialog.locator('.space-y-1 > div');
  }

  /**
   * Gets the add member phone input inside the members dialog.
   */
  get addMemberInput(): Locator {
    return this.membersDialog.locator('input').first();
  }

  /**
   * Gets the add member button (UserPlus icon button).
   */
  get addMemberButton(): Locator {
    return this.membersDialog
      .locator('button')
      .filter({ has: this.page.locator('svg.lucide-user-plus') });
  }

  /**
   * Gets the leave group button inside the members dialog.
   */
  get leaveGroupButton(): Locator {
    return this.membersDialog.locator('button').filter({ hasText: /sair do grupo|leave group/i });
  }

  /**
   * Gets the member search input inside the members dialog.
   */
  get memberSearchInput(): Locator {
    return this.membersDialog.locator('input').nth(1);
  }

  /**
   * Gets the promote button (ChevronUp) for a participant row.
   */
  getPromoteButton(row: Locator): Locator {
    return row.locator('button').filter({ has: this.page.locator('svg.lucide-chevron-up') });
  }

  /**
   * Gets the demote button (ChevronDown) for a participant row.
   */
  getDemoteButton(row: Locator): Locator {
    return row.locator('button').filter({ has: this.page.locator('svg.lucide-chevron-down') });
  }

  /**
   * Gets the remove button (Trash2) for a participant row.
   */
  getRemoveButton(row: Locator): Locator {
    return row.locator('button').filter({ has: this.page.locator('svg.lucide-trash-2') });
  }

  /**
   * Opens the members dialog for a given group by clicking "Membros" in the table row.
   */
  async openMembersForGroup(groupName: string): Promise<void> {
    const row = this.getGroupByName(groupName);
    const membersBtn = row.getByRole('button', {
      name: /membros|members|gerenciar/i,
    });
    await membersBtn.click();
    await this.membersDialog.waitFor({ state: 'visible' });
  }

  // ============================================================
  // Connection Status Helpers
  // ============================================================

  /**
   * Gets the reconnect button inside the disconnected alert.
   */
  get reconnectButton(): Locator {
    return this.page.getByRole('button', {
      name: /reconectar|reconnect/i,
    });
  }

  /**
   * Gets the QR code dialog.
   */
  get qrCodeDialog(): Locator {
    return this.page.locator('[role="dialog"]').filter({
      hasText: /reconectar|reconnect|qr.*code/i,
    });
  }

  /**
   * Gets the connection status badge/alert.
   */
  get connectionStatusBadge(): Locator {
    return this.page.locator('[role="alert"]').first();
  }

  /**
   * Gets the connected status indicator.
   */
  get connectedAlert(): Locator {
    return this.page.locator('[role="alert"]').filter({
      has: this.page.locator('svg.lucide-wifi'),
    });
  }

  /**
   * Gets the disconnected status indicator.
   */
  get disconnectedAlert(): Locator {
    return this.page.locator('[role="alert"]').filter({
      has: this.page.locator('svg.lucide-wifi-off'),
    });
  }

  // ============================================================
  // Bulk Actions Helpers
  // ============================================================

  /**
   * Gets the close groups button in bulk actions.
   */
  get closeGroupsButton(): Locator {
    return this.page.getByRole('button', {
      name: /fechar.*grupo|close.*group/i,
    });
  }

  /**
   * Gets the open groups button in bulk actions.
   */
  get openGroupsButton(): Locator {
    return this.page.getByRole('button', {
      name: /abrir.*grupo|open.*group/i,
    });
  }

  /**
   * Gets the change name button in bulk actions.
   */
  get changeNameButton(): Locator {
    return this.page.getByRole('button', {
      name: /alterar nome|nome|change name|rename/i,
    });
  }

  /**
   * Gets the change photo button in bulk actions.
   */
  get changePhotoButton(): Locator {
    return this.page.getByRole('button', {
      name: /alterar foto|foto|change photo|photo/i,
    });
  }

  /**
   * Gets the change description button in bulk actions.
   */
  get changeDescriptionButton(): Locator {
    return this.page.getByRole('button', {
      name: /alterar descri|descri|change desc/i,
    });
  }

  /**
   * Gets the group name input inside the rename dialog.
   */
  get renameInput(): Locator {
    return this.dialog.locator('#groupName');
  }

  /**
   * Gets the auto-number switch inside the rename dialog.
   */
  get autoNumberSwitch(): Locator {
    return this.dialog.locator('#auto-number');
  }

  /**
   * Gets the description textarea inside the description dialog.
   */
  get descriptionTextarea(): Locator {
    return this.dialog.locator('#description');
  }

  /**
   * Gets the confirmation dialog.
   */
  get confirmDialog(): Locator {
    return this.page.locator('[role="alertdialog"]');
  }

  /**
   * Gets the confirm button inside the confirmation dialog.
   */
  get confirmButton(): Locator {
    return this.confirmDialog.getByRole('button', {
      name: /confirmar|confirm|continuar|continue/i,
    });
  }

  /**
   * Gets the cancel button inside the confirmation dialog.
   */
  get cancelButton(): Locator {
    return this.confirmDialog.getByRole('button', {
      name: /cancelar|cancel/i,
    });
  }
}
