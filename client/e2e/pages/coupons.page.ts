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
  readonly titleInput: Locator;
  readonly discountValueInput: Locator;
  readonly discountTypeSelect: Locator;
  readonly minOrderValueInput: Locator;
  readonly maxDiscountInput: Locator;
  readonly storeSelect: Locator;
  readonly expiresAtInput: Locator;

  // Form actions
  readonly saveButton: Locator;
  readonly cancelButton: Locator;

  // Toasts
  readonly successToast: Locator;
  readonly errorToast: Locator;

  constructor(page: Page) {
    this.page = page;

    // Page elements - page uses h2
    this.pageTitle = page.locator('h2').first();
    this.couponCountBadge = page.locator('[class*="badge"]').first();

    // Actions
    this.createButton = page.getByRole('button', {
      name: /novo.*cupom|new.*coupon|criar|create/i,
    });
    this.searchInput = page.getByPlaceholder(/buscar|search|pesquis/i);

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
    this.codeInput = page.getByLabel(/codigo|code/i).or(page.locator('#code'));
    this.titleInput = page.getByLabel(/titulo|title|descri/i).or(page.locator('#title'));
    this.discountValueInput = page
      .getByLabel(/valor.*desconto|discount.*value/i)
      .or(page.locator('input[type="number"]').first());
    this.discountTypeSelect = page
      .getByLabel(/tipo.*desconto|discount.*type/i)
      .or(page.locator('[data-testid="discount-type"]'));
    this.minOrderValueInput = page.getByLabel(/valor.*minimo|min.*order|preco.*minimo/i);
    this.maxDiscountInput = page.getByLabel(/desconto.*maximo|max.*discount/i);
    this.storeSelect = page.getByLabel(/loja/i).or(page.locator('[data-testid="store-select"]'));
    this.expiresAtInput = page
      .getByLabel(/validade|expires/i)
      .or(page.locator('input[type="datetime-local"]'));

    // Form actions
    this.saveButton = page.getByRole('button', { name: /salvar|criar|save|create/i }).last();
    this.cancelButton = page.getByRole('button', { name: /cancelar|cancel/i });

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
    await this.page.waitForSelector('h2', { state: 'visible' });
  }

  /**
   * Waits for the page to load.
   */
  async waitForLoad(): Promise<void> {
    await this.page.waitForSelector('h2', { state: 'visible' });
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
   * Gets a coupon row by code (desktop table).
   */
  getCouponByCode(code: string): Locator {
    return this.page.getByRole('row').filter({ hasText: code });
  }

  /**
   * Gets a coupon card by code (mobile view).
   */
  getCouponCardByCode(code: string): Locator {
    return this.page.locator('.md\\:hidden >> div').filter({ hasText: code }).first();
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
   * Uses aria-label to find the activate/deactivate button.
   */
  getToggleStatusButton(row: Locator): Locator {
    return row.getByRole('button', { name: /ativar|desativar|activate|deactivate/i });
  }

  /**
   * Gets the status badge for a coupon row.
   */
  getStatusBadge(row: Locator): Locator {
    return row.locator('[class*="badge"], span').filter({ hasText: /ativo|inativo|expirado|active|inactive|expired/i }).first();
  }

  /**
   * Fills the coupon form.
   */
  async fillCouponForm(options: {
    code: string;
    title?: string;
    discountValue: number;
    discountType?: 'percentage' | 'fixed';
    minOrderValue?: number;
    maxDiscount?: number;
    expiresAt?: string;
  }): Promise<void> {
    await this.codeInput.fill(options.code);

    if (options.title) {
      await this.titleInput.fill(options.title);
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
   * Deletes a coupon using window.confirm dialog.
   */
  async deleteCoupon(code: string): Promise<void> {
    const row = this.getCouponByCode(code);
    const deleteBtn = this.getDeleteButton(row);

    this.page.on('dialog', async (dialog) => {
      await dialog.accept();
    });
    await deleteBtn.click();
  }

  /**
   * Toggles coupon status using window.confirm dialog.
   */
  async toggleCouponStatus(code: string): Promise<void> {
    const row = this.getCouponByCode(code);
    const toggle = this.getToggleStatusButton(row);

    this.page.on('dialog', async (dialog) => {
      await dialog.accept();
    });
    await toggle.click();
  }

  /**
   * Filters coupons by status using Select component.
   */
  async filterByStatus(status: 'all' | 'active' | 'inactive' | 'expired'): Promise<void> {
    // The status filter is a Select component, not buttons
    const statusLabels: Record<string, RegExp> = {
      all: /todos|all/i,
      active: /^ativo|^active/i,
      inactive: /inativo|inactive/i,
      expired: /expirado|expired/i,
    };

    // Find the status Select trigger (second combobox in the filter section)
    const statusTrigger = this.page.locator('[role="combobox"]').nth(1);
    await statusTrigger.click();

    const option = this.page.getByRole('option', { name: statusLabels[status] });
    await option.click();
  }

  /**
   * Filters coupons by store using Select component.
   */
  async filterByStore(storeName: RegExp | string): Promise<void> {
    const storeTrigger = this.page.locator('[role="combobox"]').first();
    await storeTrigger.click();

    const option = this.page.getByRole('option', {
      name: typeof storeName === 'string' ? new RegExp(storeName, 'i') : storeName,
    });
    await option.click();
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
   * Gets the count of displayed coupons (desktop rows).
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
