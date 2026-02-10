import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Admin page.
 * Encapsulates all admin panel interactions.
 */
export class AdminPage {
  readonly page: Page;

  // Page elements
  readonly pageTitle: Locator;

  // Navigation tabs
  readonly tabUsers: Locator;
  readonly tabStats: Locator;
  readonly tabAuditLogs: Locator;
  readonly tabSettings: Locator;

  // Stats section
  readonly statsSection: Locator;
  readonly totalUsersCard: Locator;
  readonly activeUsersCard: Locator;
  readonly totalAutomationsCard: Locator;
  readonly totalOffersCard: Locator;
  readonly systemUptimeCard: Locator;
  readonly errorRateCard: Locator;

  // Users section
  readonly usersTable: Locator;
  readonly userRows: Locator;
  readonly userSearchInput: Locator;
  readonly userFilterRole: Locator;
  readonly userFilterPlan: Locator;

  // Audit logs section
  readonly auditLogsTable: Locator;
  readonly auditLogRows: Locator;
  readonly auditDateFilter: Locator;
  readonly auditActionFilter: Locator;

  // User action buttons
  readonly editRoleButton: Locator;
  readonly editPlanButton: Locator;
  readonly banUserButton: Locator;
  readonly unbanUserButton: Locator;
  readonly impersonateButton: Locator;

  // Dialogs
  readonly dialogContent: Locator;
  readonly roleSelect: Locator;
  readonly planSelect: Locator;
  readonly confirmButton: Locator;
  readonly cancelButton: Locator;

  // Access denied
  readonly accessDeniedAlert: Locator;

  // Loading states
  readonly loadingSpinner: Locator;

  constructor(page: Page) {
    this.page = page;

    // Page elements
    this.pageTitle = page.locator('h1');

    // Navigation tabs
    this.tabUsers = page.getByRole('tab', { name: /usuarios|users/i });
    this.tabStats = page.getByRole('tab', { name: /estatisticas|stats/i });
    this.tabAuditLogs = page.getByRole('tab', { name: /auditoria|audit|logs/i });
    this.tabSettings = page.getByRole('tab', { name: /configuracoes|settings/i });

    // Stats section
    this.statsSection = page
      .locator('section')
      .filter({
        has: page.locator('[class*="card"]'),
      })
      .first();
    this.totalUsersCard = page
      .locator('[class*="card"]')
      .filter({ hasText: /total.*usuarios|total.*users/i });
    this.activeUsersCard = page
      .locator('[class*="card"]')
      .filter({ hasText: /usuarios.*ativos|active.*users/i });
    this.totalAutomationsCard = page
      .locator('[class*="card"]')
      .filter({ hasText: /automacoes|automations/i });
    this.totalOffersCard = page.locator('[class*="card"]').filter({ hasText: /ofertas|offers/i });
    this.systemUptimeCard = page.locator('[class*="card"]').filter({ hasText: /uptime/i });
    this.errorRateCard = page.locator('[class*="card"]').filter({ hasText: /erro|error.*rate/i });

    // Users section
    this.usersTable = page.locator('table').first();
    this.userRows = page.locator('table tbody tr');
    this.userSearchInput = page
      .locator('input[type="search"]')
      .or(page.getByPlaceholder(/buscar.*usuario|search.*user/i));
    this.userFilterRole = page
      .getByRole('combobox', { name: /role|funcao/i })
      .or(page.locator('[data-testid="role-filter"]'));
    this.userFilterPlan = page
      .getByRole('combobox', { name: /plano|plan/i })
      .or(page.locator('[data-testid="plan-filter"]'));

    // Audit logs section
    this.auditLogsTable = page.locator('table').last();
    this.auditLogRows = this.auditLogsTable.locator('tbody tr');
    this.auditDateFilter = page.getByRole('button', { name: /data|date/i });
    this.auditActionFilter = page.getByRole('combobox', { name: /acao|action/i });

    // User action buttons - will be used with row context
    this.editRoleButton = page.getByRole('button', { name: /editar.*role|edit.*role|funcao/i });
    this.editPlanButton = page.getByRole('button', { name: /editar.*plano|edit.*plan/i });
    this.banUserButton = page.getByRole('button', { name: /banir|ban/i });
    this.unbanUserButton = page.getByRole('button', { name: /desbanir|unban/i });
    this.impersonateButton = page.getByRole('button', { name: /impersonar|impersonate/i });

    // Dialogs
    this.dialogContent = page.locator('[role="dialog"]');
    this.roleSelect = this.dialogContent.getByRole('combobox', { name: /role|funcao/i });
    this.planSelect = this.dialogContent.getByRole('combobox', { name: /plano|plan/i });
    this.confirmButton = this.dialogContent.getByRole('button', {
      name: /confirmar|confirm|salvar|save/i,
    });
    this.cancelButton = this.dialogContent.getByRole('button', {
      name: /cancelar|cancel/i,
    });

    // Access denied
    this.accessDeniedAlert = page.locator('[role="alert"]').filter({
      hasText: /acesso.*negado|access.*denied|sem.*permissao|no.*permission/i,
    });

    // Loading states
    this.loadingSpinner = page.locator('[class*="animate-spin"]').first();
  }

  /**
   * Navigates to the admin page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/admin');
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
   * Switches to a specific tab.
   */
  async switchTab(tab: 'users' | 'stats' | 'audit' | 'settings'): Promise<void> {
    const tabs = {
      users: this.tabUsers,
      stats: this.tabStats,
      audit: this.tabAuditLogs,
      settings: this.tabSettings,
    };
    await tabs[tab].click();
  }

  /**
   * Gets a user row by email.
   */
  getUserByEmail(email: string): Locator {
    return this.userRows.filter({ hasText: email });
  }

  /**
   * Gets user action buttons for a specific row.
   */
  getUserActions(row: Locator) {
    return {
      editRole: row
        .getByRole('button', { name: /role|funcao/i })
        .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-shield') })),
      editPlan: row
        .getByRole('button', { name: /plano|plan/i })
        .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-credit-card') })),
      ban: row
        .getByRole('button', { name: /banir|ban/i })
        .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-ban') })),
      unban: row
        .getByRole('button', { name: /desbanir|unban/i })
        .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-check') })),
      impersonate: row
        .getByRole('button', { name: /impersonar|impersonate/i })
        .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-user') })),
    };
  }

  /**
   * Searches for a user.
   */
  async searchUser(query: string): Promise<void> {
    await this.userSearchInput.fill(query);
  }

  /**
   * Clears user search.
   */
  async clearSearch(): Promise<void> {
    await this.userSearchInput.clear();
  }

  /**
   * Filters users by role.
   */
  async filterByRole(role: 'all' | 'user' | 'moderator' | 'admin'): Promise<void> {
    await this.userFilterRole.click();
    const option = this.page.getByRole('option', { name: new RegExp(role, 'i') });
    await option.click();
  }

  /**
   * Filters users by plan.
   */
  async filterByPlan(plan: 'all' | 'free' | 'basic' | 'pro' | 'enterprise'): Promise<void> {
    await this.userFilterPlan.click();
    const option = this.page.getByRole('option', { name: new RegExp(plan, 'i') });
    await option.click();
  }

  /**
   * Changes a user's role.
   */
  async changeUserRole(email: string, newRole: 'user' | 'moderator' | 'admin'): Promise<void> {
    const row = this.getUserByEmail(email);
    const actions = this.getUserActions(row);
    await actions.editRole.click();

    await this.roleSelect.click();
    const option = this.page.getByRole('option', { name: new RegExp(newRole, 'i') });
    await option.click();

    await this.confirmButton.click();
  }

  /**
   * Changes a user's plan.
   */
  async changeUserPlan(
    email: string,
    newPlan: 'free' | 'basic' | 'pro' | 'enterprise'
  ): Promise<void> {
    const row = this.getUserByEmail(email);
    const actions = this.getUserActions(row);
    await actions.editPlan.click();

    await this.planSelect.click();
    const option = this.page.getByRole('option', { name: new RegExp(newPlan, 'i') });
    await option.click();

    await this.confirmButton.click();
  }

  /**
   * Bans a user.
   */
  async banUser(email: string): Promise<void> {
    const row = this.getUserByEmail(email);
    const actions = this.getUserActions(row);
    await actions.ban.click();
    await this.confirmButton.click();
  }

  /**
   * Unbans a user.
   */
  async unbanUser(email: string): Promise<void> {
    const row = this.getUserByEmail(email);
    const actions = this.getUserActions(row);
    await actions.unban.click();
    await this.confirmButton.click();
  }

  /**
   * Impersonates a user.
   */
  async impersonateUser(email: string): Promise<void> {
    const row = this.getUserByEmail(email);
    const actions = this.getUserActions(row);
    await actions.impersonate.click();
    await this.confirmButton.click();
  }

  /**
   * Gets the count of displayed users.
   */
  async getUserCount(): Promise<number> {
    return this.userRows.count();
  }

  /**
   * Gets the count of audit log entries.
   */
  async getAuditLogCount(): Promise<number> {
    return this.auditLogRows.count();
  }

  /**
   * Checks if access is denied.
   */
  async isAccessDenied(): Promise<boolean> {
    return this.accessDeniedAlert.isVisible();
  }

  /**
   * Gets stat value from a card.
   */
  async getStatValue(card: Locator): Promise<string | null> {
    const value = card.locator('h2, h3, [class*="value"], span').first();
    return value.textContent();
  }

  /**
   * Gets user status (active/banned).
   */
  async getUserStatus(email: string): Promise<string | null> {
    const row = this.getUserByEmail(email);
    const badge = row.locator('[class*="badge"]').filter({
      hasText: /ativo|active|banido|banned/i,
    });
    return badge.textContent();
  }

  /**
   * Gets user role.
   */
  async getUserRole(email: string): Promise<string | null> {
    const row = this.getUserByEmail(email);
    const badge = row.locator('[class*="badge"]').filter({
      hasText: /admin|user|moderator/i,
    });
    return badge.textContent();
  }

  /**
   * Gets user plan.
   */
  async getUserPlan(email: string): Promise<string | null> {
    const row = this.getUserByEmail(email);
    const badge = row.locator('[class*="badge"]').filter({
      hasText: /free|basic|pro|enterprise/i,
    });
    return badge.textContent();
  }
}
