import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the Coupons page.
 * Encapsulates all coupon management interactions.
 */
export class CouponsPage {
  readonly page: Page;

  // Page elements
  readonly pageTitle: Locator;
  readonly couponCountBadge: Locator;

  // Actions
  readonly createButton: Locator;
  readonly searchInput: Locator;

  // Filters
  readonly filterAll: Locator;
  readonly filterActive: Locator;
  readonly filterExpired: Locator;
  readonly filterByStore: Locator;

  // Table/List elements
  readonly couponsTable: Locator;
  readonly couponRows: Locator;
  readonly emptyState: Locator;

  // Dialog elements
  readonly dialogContent: Locator;
  readonly dialogTitle: Locator;

  // Form fields
  readonly codeInput: Locator;
  readonly descriptionInput: Locator;
  readonly discountValueInput: Locator;
  readonly discountTypeSelect: Locator;
  readonly minOrderValueInput: Locator;
  readonly maxDiscountInput: Locator;
  readonly storeSelect: Locator;
  readonly expiresAtInput: Locator;

  // Form actions
  readonly saveButton: Locator;
  readonly cancelButton: Locator;

  // Confirmation dialog
  readonly confirmDialog: Locator;
  readonly confirmButton: Locator;

  // Toasts
  readonly successToast: Locator;
  readonly errorToast: Locator;

  constructor(page: Page) {
    this.page = page;

    // Page elements
    this.pageTitle = page.locator('h1');
    this.couponCountBadge = page.locator('[class*="badge"]').first();

    // Actions
    this.createButton = page.getByRole('button', {
      name: /novo.*cupom|new.*coupon|criar|create/i,
    });
    this.searchInput = page
      .locator('input[type="search"]')
      .or(page.getByPlaceholder(/buscar|search/i));

    // Filters
    this.filterAll = page.getByRole('button', { name: /todos|all/i });
    this.filterActive = page.getByRole('button', { name: /ativos|active/i });
    this.filterExpired = page.getByRole('button', { name: /expirados|expired/i });
    this.filterByStore = page
      .getByRole('combobox', { name: /loja|store/i })
      .or(page.locator('[data-testid="store-filter"]'));

    // Table/List elements
    this.couponsTable = page.locator('table');
    this.couponRows = page
      .locator('table tbody tr')
      .or(page.locator('[class*="coupon-card"], [class*="coupon-item"]'));
    this.emptyState = page.getByText(/nenhum.*cupom|no.*coupon|sem.*cupons/i);

    // Dialog elements
    this.dialogContent = page.locator('[role="dialog"]');
    this.dialogTitle = this.dialogContent.locator('h2').first();

    // Form fields
    this.codeInput = page.getByLabel(/codigo/i).or(page.locator('input[placeholder*="codigo"]'));
    this.descriptionInput = page
      .getByLabel(/descricao/i)
      .or(page.locator('textarea, input[placeholder*="descricao"]'));
    this.discountValueInput = page
      .getByLabel(/valor.*desconto|discount.*value/i)
      .or(page.locator('input[type="number"]').first());
    this.discountTypeSelect = page
      .getByLabel(/tipo.*desconto|discount.*type/i)
      .or(page.locator('[data-testid="discount-type"]'));
    this.minOrderValueInput = page.getByLabel(/valor.*minimo|min.*order/i);
    this.maxDiscountInput = page.getByLabel(/desconto.*maximo|max.*discount/i);
    this.storeSelect = page.getByLabel(/loja/i).or(page.locator('[data-testid="store-select"]'));
    this.expiresAtInput = page
      .getByLabel(/validade|expires/i)
      .or(page.locator('input[type="date"]'));

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

    // Toasts
    this.successToast = page.locator('[data-sonner-toast]').filter({
      hasText: /sucesso|success|criado|created/i,
    });
    this.errorToast = page.locator('[data-sonner-toast]').filter({
      hasText: /erro|error|falha|failed/i,
    });
  }

  /**
   * Navigates to the coupons page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/coupons');
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
   * Opens the create coupon dialog.
   */
  async openCreateDialog(): Promise<void> {
    await this.createButton.click();
    await this.dialogContent.waitFor({ state: 'visible' });
  }

  /**
   * Gets a coupon row by code.
   */
  getCouponByCode(code: string): Locator {
    return this.page.getByRole('row').filter({ hasText: code });
  }

  /**
   * Gets the copy button for a coupon row.
   */
  getCopyButton(row: Locator): Locator {
    return row
      .getByRole('button', { name: /copiar|copy/i })
      .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-copy') }));
  }

  /**
   * Gets the edit button for a coupon row.
   */
  getEditButton(row: Locator): Locator {
    return row
      .getByRole('button', { name: /editar|edit/i })
      .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-edit') }));
  }

  /**
   * Gets the delete button for a coupon row.
   */
  getDeleteButton(row: Locator): Locator {
    return row
      .getByRole('button', { name: /excluir|delete/i })
      .or(row.locator('button').filter({ has: this.page.locator('svg.lucide-trash') }));
  }

  /**
   * Gets the toggle status button for a coupon row.
   */
  getToggleStatusButton(row: Locator): Locator {
    return row.locator('button[role="switch"]');
  }

  /**
   * Gets the status badge for a coupon row.
   */
  getStatusBadge(row: Locator): Locator {
    return row.locator('[class*="badge"]').first();
  }

  /**
   * Fills the coupon form.
   */
  async fillCouponForm(options: {
    code: string;
    description?: string;
    discountValue: number;
    discountType?: 'percentage' | 'fixed';
    minOrderValue?: number;
    maxDiscount?: number;
    expiresAt?: string;
  }): Promise<void> {
    await this.codeInput.fill(options.code);

    if (options.description) {
      await this.descriptionInput.fill(options.description);
    }

    await this.discountValueInput.fill(String(options.discountValue));

    if (options.discountType) {
      await this.discountTypeSelect.click();
      const typeOption = this.page.getByRole('option', {
        name: new RegExp(
          options.discountType === 'percentage' ? 'porcentagem|percentage' : 'fixo|fixed',
          'i'
        ),
      });
      await typeOption.click();
    }

    if (options.minOrderValue !== undefined) {
      await this.minOrderValueInput.fill(String(options.minOrderValue));
    }

    if (options.maxDiscount !== undefined) {
      await this.maxDiscountInput.fill(String(options.maxDiscount));
    }

    if (options.expiresAt) {
      await this.expiresAtInput.fill(options.expiresAt);
    }
  }

  /**
   * Saves the coupon form.
   */
  async save(): Promise<void> {
    await this.saveButton.click();
  }

  /**
   * Cancels the coupon form.
   */
  async cancel(): Promise<void> {
    await this.cancelButton.click();
  }

  /**
   * Copies a coupon code.
   */
  async copyCoupon(code: string): Promise<void> {
    const row = this.getCouponByCode(code);
    const copyBtn = this.getCopyButton(row);
    await copyBtn.click();
  }

  /**
   * Edits a coupon.
   */
  async editCoupon(code: string): Promise<void> {
    const row = this.getCouponByCode(code);
    const editBtn = this.getEditButton(row);
    await editBtn.click();
    await this.dialogContent.waitFor({ state: 'visible' });
  }

  /**
   * Deletes a coupon.
   */
  async deleteCoupon(code: string): Promise<void> {
    const row = this.getCouponByCode(code);
    const deleteBtn = this.getDeleteButton(row);
    await deleteBtn.click();

    if (await this.confirmDialog.isVisible()) {
      await this.confirmButton.click();
    }
  }

  /**
   * Toggles coupon status.
   */
  async toggleCouponStatus(code: string): Promise<void> {
    const row = this.getCouponByCode(code);
    const toggle = this.getToggleStatusButton(row);
    await toggle.click();
  }

  /**
   * Filters coupons by status.
   */
  async filterByStatus(status: 'all' | 'active' | 'expired'): Promise<void> {
    const filters = {
      all: this.filterAll,
      active: this.filterActive,
      expired: this.filterExpired,
    };
    await filters[status].click();
  }

  /**
   * Searches for coupons.
   */
  async search(query: string): Promise<void> {
    await this.searchInput.fill(query);
  }

  /**
   * Clears search input.
   */
  async clearSearch(): Promise<void> {
    await this.searchInput.clear();
  }

  /**
   * Gets the count of displayed coupons.
   */
  async getCouponCount(): Promise<number> {
    return this.couponRows.count();
  }

  /**
   * Checks if empty state is shown.
   */
  async isEmpty(): Promise<boolean> {
    return this.emptyState.isVisible();
  }

  /**
   * Gets the status of a coupon.
   */
  async getCouponStatus(code: string): Promise<string | null> {
    const row = this.getCouponByCode(code);
    const badge = this.getStatusBadge(row);
    return badge.textContent();
  }
}
