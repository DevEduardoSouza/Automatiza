import type { Locator, Page } from '@playwright/test';

type AdminTab = 'dashboard' | 'users' | 'plans' | 'categories' | 'team' | 'emails' | 'settings';

/**
 * Page Object Model for the Admin page.
 * Covers all 7 tabs: Dashboard, Users, Plans, Categories, Team, Emails, Settings.
 */
export class AdminPage {
  readonly page: Page;

  // -- Navigation tabs (role="tab" with i18n text) --
  readonly tabDashboard: Locator;
  readonly tabUsers: Locator;
  readonly tabPlans: Locator;
  readonly tabCategories: Locator;
  readonly tabTeam: Locator;
  readonly tabEmails: Locator;
  readonly tabSettings: Locator;

  // -- Dashboard section --
  readonly statsCards: Locator;
  readonly observabilityCard: Locator;
  readonly refreshObsButton: Locator;
  readonly shopeeAuditCard: Locator;
  readonly auditDaysSelect: Locator;
  readonly webhookSection: Locator;
  readonly webhookLogsTable: Locator;
  readonly webhookSimulatorForm: Locator;

  // -- Users section --
  readonly usersTable: Locator;
  readonly userRows: Locator;
  readonly userSearchInput: Locator;
  readonly statusFilter: Locator;
  readonly createUserButton: Locator;
  readonly createUserDialog: Locator;
  readonly userDetailsDialog: Locator;
  readonly blockUserButton: Locator;
  readonly unblockUserButton: Locator;
  readonly resetPasswordButton: Locator;
  readonly adminToggleButton: Locator;

  // -- Plans section --
  readonly plansTable: Locator;
  readonly createPlanButton: Locator;
  readonly planDialog: Locator;
  readonly planForm: Locator;
  readonly deletePlanButton: Locator;

  // -- Categories section --
  readonly categoriesList: Locator;
  readonly createCategoryButton: Locator;
  readonly categoryDialog: Locator;
  readonly subcategoryButtons: Locator;

  // -- Team section --
  readonly teamTable: Locator;
  readonly addAdminButton: Locator;
  readonly addAdminDialog: Locator;
  readonly removeAdminButton: Locator;

  // -- Email Templates section --
  readonly templatesTable: Locator;
  readonly createTemplateButton: Locator;
  readonly templateDialog: Locator;
  readonly searchInput: Locator;
  readonly toggleActiveButton: Locator;

  // -- Settings section --
  readonly settingsCards: Locator;
  readonly saveSettingsButton: Locator;

  // -- Shared --
  readonly accessDeniedAlert: Locator;
  readonly loadingSpinner: Locator;
  readonly dialogContent: Locator;

  constructor(page: Page) {
    this.page = page;

    // Navigation tabs - match i18n: Dashboard, Usuarios, Planos, Categorias, Equipe, E-mails, Config
    this.tabDashboard = page.getByRole('tab', { name: /dashboard/i });
    this.tabUsers = page.getByRole('tab', { name: /usu[aá]rios|users/i });
    this.tabPlans = page.getByRole('tab', { name: /planos|plans/i });
    this.tabCategories = page.getByRole('tab', { name: /categorias|categories/i });
    this.tabTeam = page.getByRole('tab', { name: /equipe|team/i });
    this.tabEmails = page.getByRole('tab', { name: /e-?mails/i });
    this.tabSettings = page.getByRole('tab', { name: /config|settings|configura/i });

    // Dashboard - stats cards grid
    this.statsCards = page.locator('.grid').filter({
      has: page.locator('[class*="rounded"]'),
    }).first();
    this.observabilityCard = page.locator('[class*="rounded"]').filter({
      hasText: /observ/i,
    }).first();
    this.refreshObsButton = this.observabilityCard.getByRole('button', {
      name: /atualizar|refresh/i,
    });
    this.shopeeAuditCard = page.locator('[class*="rounded"]').filter({
      hasText: /shopee.*audit|auditoria.*shopee/i,
    }).first();
    this.auditDaysSelect = this.shopeeAuditCard.locator('button[role="combobox"]').first();
    this.webhookSection = page.locator('[class*="rounded"]').filter({
      hasText: /webhook/i,
    }).first();
    this.webhookLogsTable = this.webhookSection.locator('table').first();
    this.webhookSimulatorForm = this.webhookSection.locator('form').or(
      this.webhookSection.locator('[class*="space-y"]').filter({
        has: page.locator('input'),
      }).first()
    );

    // Users section
    this.usersTable = page.locator('table').first();
    this.userRows = page.locator('table tbody tr');
    this.userSearchInput = page.getByPlaceholder(/buscar.*usu|search.*user/i);
    this.statusFilter = page.locator('[role="group"]').filter({
      hasText: /ativo|active/i,
    });
    this.createUserButton = page.getByRole('button', { name: /criar.*usu|novo.*usu|new.*user|create.*user/i });
    this.createUserDialog = page.locator('[role="dialog"]').filter({
      hasText: /criar.*usu|new.*user|create.*user/i,
    });
    this.userDetailsDialog = page.locator('[role="dialog"]').filter({
      hasText: /detalhes|details|perfil|profile/i,
    });
    this.blockUserButton = page.getByRole('button', { name: /bloquear|block/i });
    this.unblockUserButton = page.getByRole('button', { name: /desbloquear|unblock/i });
    this.resetPasswordButton = page.getByRole('button', { name: /redefinir.*senha|reset.*password/i });
    this.adminToggleButton = page.getByRole('button', { name: /admin/i });

    // Plans section
    this.plansTable = page.locator('table').first();
    this.createPlanButton = page.getByRole('button', { name: /novo.*plano|new.*plan/i });
    this.planDialog = page.locator('[role="dialog"]').filter({
      hasText: /plano|plan/i,
    });
    this.planForm = this.planDialog.locator('form').or(this.planDialog.locator('[class*="space-y"]').first());
    this.deletePlanButton = page.getByRole('button', { name: /excluir|delete|remover|remove/i });

    // Categories section
    this.categoriesList = page.locator('[class*="space-y"]').filter({
      has: page.locator('[class*="rounded"]'),
    }).first();
    this.createCategoryButton = page.getByRole('button', { name: /nova.*categoria|new.*category/i });
    this.categoryDialog = page.locator('[role="dialog"]').filter({
      hasText: /categoria|category/i,
    });
    this.subcategoryButtons = page.getByRole('button', { name: /subcategoria|subcategory/i });

    // Team section
    this.teamTable = page.locator('table').filter({
      has: page.locator('th'),
    }).first();
    this.addAdminButton = page.getByRole('button', { name: /adicionar|add/i }).first();
    this.addAdminDialog = page.locator('[role="dialog"]').filter({
      hasText: /admin/i,
    });
    this.removeAdminButton = page.getByRole('button', { name: /remover|remove|excluir|delete/i });

    // Email Templates section
    this.templatesTable = page.locator('table').first();
    this.createTemplateButton = page.getByRole('button', { name: /criar.*template|novo.*template|new.*template|create.*template/i });
    this.templateDialog = page.locator('[role="dialog"]').filter({
      hasText: /template/i,
    });
    this.searchInput = page.getByPlaceholder(/buscar.*template|search.*template/i);
    this.toggleActiveButton = page.locator('button[role="switch"]');

    // Settings section
    this.settingsCards = page.locator('[class*="rounded"]').filter({
      has: page.locator('input'),
    });
    this.saveSettingsButton = page.getByRole('button', { name: /salvar|save/i });

    // Shared
    this.accessDeniedAlert = page.locator('[role="alert"]').filter({
      hasText: /acesso.*negado|access.*denied|sem.*permiss|no.*permission|n[aã]o.*autorizado|unauthorized/i,
    });
    this.loadingSpinner = page.locator('[class*="animate-spin"]').first();
    this.dialogContent = page.locator('[role="dialog"]');
  }

  /** Navigate to the admin page. */
  async goto(): Promise<void> {
    await this.page.goto('/admin');
    await this.page.waitForSelector('h2', { state: 'visible' });
  }

  /** Wait for the admin page content to finish loading. */
  async waitForLoad(): Promise<void> {
    await this.page.waitForSelector('h2', { state: 'visible' });
    await this.page
      .waitForFunction(
        () => {
          const spinner = document.querySelector('[class*="animate-spin"]');
          return !spinner;
        },
        { timeout: 10000 }
      )
      .catch(() => {});
  }

  /** Switch to a specific admin tab. */
  async switchTab(tab: AdminTab): Promise<void> {
    const tabMap: Record<AdminTab, Locator> = {
      dashboard: this.tabDashboard,
      users: this.tabUsers,
      plans: this.tabPlans,
      categories: this.tabCategories,
      team: this.tabTeam,
      emails: this.tabEmails,
      settings: this.tabSettings,
    };
    await tabMap[tab].click();
  }

  /** Check if access is denied (non-admin user). */
  async isAccessDenied(): Promise<boolean> {
    return this.accessDeniedAlert.isVisible();
  }

  /** Search for a user by name or email in the Users tab. */
  async searchUser(query: string): Promise<void> {
    await this.userSearchInput.fill(query);
  }

  /** Clear the user search input. */
  async clearSearch(): Promise<void> {
    await this.userSearchInput.clear();
  }

  /** Get a user row by email text. */
  getUserByEmail(email: string): Locator {
    return this.userRows.filter({ hasText: email });
  }

  /** Get the count of visible user rows. */
  async getUserCount(): Promise<number> {
    return this.userRows.count();
  }
}
