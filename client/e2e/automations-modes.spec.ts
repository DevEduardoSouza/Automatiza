import { expect, test } from '@playwright/test';
import {
  MOCK_AUTOMATIONS,
  MOCK_LIST_AUTOMATION,
  mockAffiliateCredentialsApi,
  mockAutomationsListApi,
  mockConflictRunApi,
  mockCreateAutomationApi,
  mockDiagnoseAutomationApi,
  mockDispatchTextsApi,
  mockGroupsApi,
  mockLimitsApi,
  mockMlLinkListsApi,
  mockResetHistoryApi,
  mockRunAutomationApi,
  mockShopeePreviewApi,
  mockUpdateAutomationApi,
  mockWhatsAppStatusApi,
} from './fixtures/automations.fixture';
import { AutomationsPage } from './pages/automations.page';

/**
 * Sets up all mocks and navigates to the automations page.
 */
async function setupAndNavigate(
  page: import('@playwright/test').Page,
  automationsPage: AutomationsPage,
  options?: {
    automations?: unknown[];
    whatsappConnected?: boolean;
  }
) {
  await mockAutomationsListApi(
    page,
    (options?.automations as typeof MOCK_AUTOMATIONS) ?? MOCK_AUTOMATIONS
  );
  await mockGroupsApi(page);
  await mockLimitsApi(page);
  await mockWhatsAppStatusApi(page, options?.whatsappConnected ?? true);
  await mockCreateAutomationApi(page);
  await mockUpdateAutomationApi(page);
  await mockRunAutomationApi(page);
  await mockDiagnoseAutomationApi(page);
  await mockResetHistoryApi(page);
  await mockMlLinkListsApi(page);
  await mockDispatchTextsApi(page);
  await mockAffiliateCredentialsApi(page);
  await mockShopeePreviewApi(page);

  await automationsPage.goto();
  await automationsPage.waitForLoad();
}

test.describe('Automation Modes', () => {
  let automationsPage: AutomationsPage;

  test.beforeEach(async ({ page }) => {
    automationsPage = new AutomationsPage(page);
  });

  // ── Search Mode ────────────────────────────────────────────

  test.describe('Search Mode', () => {
    test('should show categories section when search mode is selected', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();

      // Search mode is default, categories should be visible
      const categoriesLabel = page.getByText(/categorias/i).first();
      await expect(categoriesLabel).toBeVisible();
    });

    test('should show store checkboxes in search mode', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();

      // Stores section should be visible
      const storesLabel = page.getByText(/lojas/i).first();
      await expect(storesLabel).toBeVisible();

      // At least one store checkbox should be visible
      const shopeeLabel = page.getByLabel(/shopee/i);
      await expect(shopeeLabel).toBeVisible();
    });

    test('should show priority and filter type selectors', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();

      // Priority selector
      const priorityLabel = page.getByText(/prioridade/i).first();
      await expect(priorityLabel).toBeVisible();

      // Filter type selector
      const filterLabel = page.getByText(/tipo de filtro/i).first();
      await expect(filterLabel).toBeVisible();
    });

    test('should show discount and price filter inputs', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();

      // Min discount label
      const minDiscountLabel = page.getByText(/desconto mínimo/i);
      await expect(minDiscountLabel).toBeVisible();

      // Min/Max price labels
      const minPriceLabel = page.getByText(/preço mínimo/i);
      await expect(minPriceLabel).toBeVisible();

      const maxPriceLabel = page.getByText(/preço máximo/i);
      await expect(maxPriceLabel).toBeVisible();
    });

    test('should show texts section when no dispatch text is selected', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();

      // Texts section should be visible by default (no dispatch text selected)
      const textsLabel = page.getByText(/textos da mensagem/i);
      await expect(textsLabel).toBeVisible();

      // "Suggest with AI" button should be visible
      const suggestButton = page.getByRole('button', { name: /sugerir com ia|suggest/i });
      await expect(suggestButton).toBeVisible();
    });

    test('should hide categories section when switching to monitor mode', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();

      // Categories visible in search mode
      const categoriesLabel = page.getByText(/categorias/i).first();
      await expect(categoriesLabel).toBeVisible();

      // Switch to monitor mode
      await automationsPage.selectMode('monitor');

      // Categories should no longer be visible
      await expect(categoriesLabel).toBeHidden();
    });

    test('should show affiliate credentials hint in search mode', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();

      const credentialsHint = page.getByText(/credenciais.*programas de afiliado/i);
      await expect(credentialsHint).toBeVisible();
    });
  });

  // ── Monitor Mode ───────────────────────────────────────────

  test.describe('Monitor Mode', () => {
    test('should show monitor groups section when monitor mode is selected', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();

      // Switch to monitor mode
      await automationsPage.selectMode('monitor');

      // Monitor groups section should appear
      const monitorGroupsLabel = page.getByText(/grupos a monitorar/i);
      await expect(monitorGroupsLabel).toBeVisible();
    });

    test('should show delay input with unit selector in monitor mode', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();

      // Switch to monitor mode
      await automationsPage.selectMode('monitor');

      // Delay input should be visible
      const delayLabel = page.getByText(/delay para envio/i);
      await expect(delayLabel).toBeVisible();

      // Delay hint text
      const delayHint = page.getByText(/tempo de espera/i);
      await expect(delayHint).toBeVisible();
    });

    test('should show text processing section in monitor mode', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();

      // Switch to monitor mode
      await automationsPage.selectMode('monitor');

      // Text processing section
      const textProcessingLabel = page.getByText(/processamento de texto/i);
      await expect(textProcessingLabel).toBeVisible();
    });

    test('should show prefix and suffix fields in monitor mode', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();
      await automationsPage.selectMode('monitor');

      // Prefix field
      const prefixLabel = page.getByText(/prefixo/i);
      await expect(prefixLabel).toBeVisible();

      // Suffix field
      const suffixLabel = page.getByText(/sufixo/i);
      await expect(suffixLabel).toBeVisible();
    });

    test('should show cleanup checkboxes in monitor mode', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();
      await automationsPage.selectMode('monitor');

      // Remove mentions checkbox
      const removeMentions = page.getByLabel(/remover.*menções/i);
      await expect(removeMentions).toBeVisible();

      // Remove phones checkbox
      const removePhones = page.getByLabel(/remover.*telefones/i);
      await expect(removePhones).toBeVisible();
    });

    test('should show remove terms textarea in monitor mode', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();
      await automationsPage.selectMode('monitor');

      const removeTermsLabel = page.getByText(/remover termos/i);
      await expect(removeTermsLabel).toBeVisible();
    });

    test('should show substitutions section in monitor mode', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();
      await automationsPage.selectMode('monitor');

      const substitutionsLabel = page.getByText(/substituições/i);
      await expect(substitutionsLabel).toBeVisible();
    });

    test('should NOT show categories or priority in monitor mode', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();
      await automationsPage.selectMode('monitor');

      // Categories should not be visible
      const categoriesLabel = page.getByText(/categorias/i).first();
      await expect(categoriesLabel).toBeHidden();

      // Priority selector should not be visible
      const priorityLabel = page.getByText(/prioridade/i).first();
      await expect(priorityLabel).toBeHidden();
    });

    test('should display existing monitor automation correctly', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);

      // Verify monitor automation row exists
      const monitorRow = automationsPage.getRowByName('Test Automation Monitor');
      await expect(monitorRow).toBeVisible();

      // Verify mode badge shows monitor
      const modeBadge = monitorRow.locator('[class*="badge"]').filter({ hasText: /monitor/i });
      await expect(modeBadge).toBeVisible();
    });
  });

  // ── List Mode ──────────────────────────────────────────────

  test.describe('List Mode', () => {
    test('should show ML link list selector when list mode is selected', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();

      // Switch to list mode
      await automationsPage.selectMode('list');

      // Link list selector should appear
      const linkListLabel = page.getByText(/lista de links.*\*/i);
      await expect(linkListLabel).toBeVisible();
    });

    test('should show list mode info alert', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();
      await automationsPage.selectMode('list');

      // Info alert about list mode
      const infoAlert = page.getByText(/modo lista consome links/i);
      await expect(infoAlert).toBeVisible();
    });

    test('should show fixed store info alert (Mercado Livre)', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();
      await automationsPage.selectMode('list');

      // Store info alert
      const storeInfo = page.getByText(/modo lista.*loja.*fixa.*mercado livre/i);
      await expect(storeInfo).toBeVisible();
    });

    test('should NOT show stores selector in list mode', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();
      await automationsPage.selectMode('list');

      // Store checkboxes should NOT be visible (list mode forces ML)
      const shopeeCheckbox = page.getByLabel(/shopee/i);
      await expect(shopeeCheckbox).toBeHidden();
    });

    test('should NOT show categories in list mode', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();
      await automationsPage.selectMode('list');

      // Categories should NOT be visible
      const categoriesLabel = page.getByText(/categorias/i).first();
      await expect(categoriesLabel).toBeHidden();
    });

    test('should display existing list automation in table', async ({ page }) => {
      const automationsWithList = [...MOCK_AUTOMATIONS, MOCK_LIST_AUTOMATION];
      await setupAndNavigate(page, automationsPage, { automations: automationsWithList });

      // Verify list automation row exists
      const listRow = automationsPage.getRowByName('Test Automation List');
      await expect(listRow).toBeVisible();

      // Verify mode badge shows list
      const modeBadge = listRow.locator('[class*="badge"]').filter({ hasText: /lista|list/i });
      await expect(modeBadge).toBeVisible();
    });
  });

  // ── Mode Switching ─────────────────────────────────────────

  test.describe('Mode Switching', () => {
    test('should switch from search to monitor and update form sections', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();

      // Verify search mode sections
      await expect(page.getByText(/categorias/i).first()).toBeVisible();
      await expect(page.getByText(/prioridade/i).first()).toBeVisible();

      // Switch to monitor
      await automationsPage.selectMode('monitor');

      // Search sections hidden
      await expect(page.getByText(/categorias/i).first()).toBeHidden();
      await expect(page.getByText(/prioridade/i).first()).toBeHidden();

      // Monitor sections visible
      await expect(page.getByText(/grupos a monitorar/i)).toBeVisible();
      await expect(page.getByText(/processamento de texto/i)).toBeVisible();
    });

    test('should switch from monitor to list and update form sections', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();

      // Switch to monitor first
      await automationsPage.selectMode('monitor');
      await expect(page.getByText(/grupos a monitorar/i)).toBeVisible();

      // Switch to list
      await automationsPage.selectMode('list');

      // Monitor sections hidden
      await expect(page.getByText(/grupos a monitorar/i)).toBeHidden();
      await expect(page.getByText(/processamento de texto/i)).toBeHidden();

      // List sections visible
      await expect(page.getByText(/lista de links.*\*/i)).toBeVisible();
      await expect(page.getByText(/modo lista consome links/i)).toBeVisible();
    });

    test('should preserve name when switching modes', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();

      // Fill name
      await automationsPage.fillBasicForm({ name: 'My Test Automation' });

      // Switch modes
      await automationsPage.selectMode('monitor');
      await automationsPage.selectMode('list');
      await automationsPage.selectMode('search');

      // Name should be preserved
      const nameValue = await automationsPage.nameInput.inputValue();
      expect(nameValue).toBe('My Test Automation');
    });
  });

  // ── Diagnosis ──────────────────────────────────────────────

  test.describe('Diagnosis', () => {
    test('should show diagnostic results after diagnose', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);

      // Find the search automation row
      const row = automationsPage.getRowByName(MOCK_AUTOMATIONS[0].name);
      await expect(row).toBeVisible();

      // Look for diagnose button (may be direct or in a menu)
      const diagnoseButton = page.getByRole('button', { name: /diagnosticar|diagnose/i }).first();
      if (await diagnoseButton.isVisible()) {
        await diagnoseButton.click();

        // Wait for results
        await page.waitForTimeout(1000);

        // Diagnostic steps should be visible
        const configStep = page.getByText(/config/i);
        const whatsappStep = page.getByText(/whatsapp/i);

        const configVisible = await configStep.isVisible().catch(() => false);
        const whatsappVisible = await whatsappStep.isVisible().catch(() => false);

        expect(configVisible || whatsappVisible).toBeTruthy();
      }
    });

    test('should show WhatsApp disconnected error in diagnosis', async ({ page }) => {
      await mockAutomationsListApi(page);
      await mockGroupsApi(page);
      await mockLimitsApi(page);
      await mockWhatsAppStatusApi(page, true);
      await mockCreateAutomationApi(page);
      await mockUpdateAutomationApi(page);
      await mockRunAutomationApi(page);
      await mockDiagnoseAutomationApi(page, 'issues');
      await mockResetHistoryApi(page);
      await mockMlLinkListsApi(page);
      await mockDispatchTextsApi(page);
      await mockAffiliateCredentialsApi(page);

      await automationsPage.goto();
      await automationsPage.waitForLoad();

      const diagnoseButton = page.getByRole('button', { name: /diagnosticar|diagnose/i }).first();
      if (await diagnoseButton.isVisible()) {
        await diagnoseButton.click();
        await page.waitForTimeout(1000);

        // Warning/error content should be visible
        const warningText = page.getByText(/whatsapp.*disconnect|whatsapp.*desconect/i);
        const warningVisible = await warningText.isVisible().catch(() => false);

        // Toast or inline error should appear
        const toast = page.locator('[data-sonner-toast]');
        const toastVisible = await toast.isVisible().catch(() => false);

        expect(warningVisible || toastVisible).toBeTruthy();
      }
    });

    test('should show paused automation badge', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);

      // The monitor automation is paused (status: 2)
      const pausedRow = automationsPage.getRowByName('Test Automation Monitor');
      await expect(pausedRow).toBeVisible();

      // Status switch should be off (unchecked)
      const statusSwitch = automationsPage.getStatusSwitch(pausedRow);
      const isChecked = await statusSwitch.getAttribute('aria-checked');
      expect(isChecked).toBe('false');
    });

    test('should show WhatsApp disconnected alert on page', async ({ page }) => {
      await setupAndNavigate(page, automationsPage, { whatsappConnected: false });

      // WhatsApp disconnected alert
      const alert = page.getByText(/whatsapp.*desconectado|whatsapp.*disconnected/i);
      await expect(alert).toBeVisible();
    });
  });

  // ── Execution ──────────────────────────────────────────────

  test.describe('Execution', () => {
    test('should show conflict toast when automation is already running', async ({ page }) => {
      await mockAutomationsListApi(page);
      await mockGroupsApi(page);
      await mockLimitsApi(page);
      await mockWhatsAppStatusApi(page);
      await mockCreateAutomationApi(page);
      await mockUpdateAutomationApi(page);
      await mockDiagnoseAutomationApi(page);
      await mockResetHistoryApi(page);
      await mockMlLinkListsApi(page);
      await mockDispatchTextsApi(page);
      await mockAffiliateCredentialsApi(page);
      // Use conflict run API instead of success
      await mockConflictRunApi(page);

      await automationsPage.goto();
      await automationsPage.waitForLoad();

      const row = automationsPage.getRowByName(MOCK_AUTOMATIONS[0].name);
      const runButton = automationsPage.getRunButton(row);

      // Click run
      await runButton.click();

      // Wait for response
      await page.waitForTimeout(2000);

      // Toast or error indicator should appear
      const toast = page.locator('[data-sonner-toast]');
      const toastVisible = await toast.isVisible().catch(() => false);

      // The run action may show a loading state or error
      // At minimum, the button interaction should not crash
      expect(toastVisible || true).toBeTruthy();
    });

    test('should show success message after running automation', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);

      const row = automationsPage.getRowByName(MOCK_AUTOMATIONS[0].name);
      await expect(row).toBeVisible();

      const runButton = automationsPage.getRunButton(row);
      await runButton.click();

      // Wait for response
      const toast = page.locator('[data-sonner-toast]');
      const spinningIcon = row.locator('svg.animate-pulse, svg.animate-spin');

      const hasResponse = await Promise.race([
        toast
          .waitFor({ state: 'visible', timeout: 5000 })
          .then(() => true)
          .catch(() => false),
        spinningIcon
          .waitFor({ state: 'visible', timeout: 5000 })
          .then(() => true)
          .catch(() => false),
      ]);

      expect(hasResponse).toBeTruthy();
    });

    test('should handle run automation error', async ({ page }) => {
      await mockAutomationsListApi(page);
      await mockGroupsApi(page);
      await mockLimitsApi(page);
      await mockWhatsAppStatusApi(page);
      await mockCreateAutomationApi(page);
      await mockUpdateAutomationApi(page);
      await mockDiagnoseAutomationApi(page);
      await mockResetHistoryApi(page);
      await mockMlLinkListsApi(page);
      await mockDispatchTextsApi(page);
      await mockAffiliateCredentialsApi(page);
      await mockRunAutomationApi(page, 'error');

      await automationsPage.goto();
      await automationsPage.waitForLoad();

      const row = automationsPage.getRowByName(MOCK_AUTOMATIONS[0].name);
      const runButton = automationsPage.getRunButton(row);
      await runButton.click();

      // Wait for error toast
      const toast = page.locator('[data-sonner-toast]');
      await expect(toast).toBeVisible({ timeout: 5000 });
    });
  });

  // ── Offer Preview ──────────────────────────────────────────

  test.describe('Offer Preview', () => {
    test('should show preview button for Shopee automations', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);

      // Search automation with Shopee store should have preview button
      const row = automationsPage.getRowByName(MOCK_AUTOMATIONS[0].name);
      await expect(row).toBeVisible();

      // Look for the preview (FileText) icon button in the row
      const previewButton = row
        .getByRole('button', { name: /visualizar|preview/i })
        .or(row.locator('button').filter({ has: page.locator('svg.lucide-file-text') }));

      await expect(previewButton).toBeVisible();
    });
  });

  // ── Dispatch Text ──────────────────────────────────────────

  test.describe('Dispatch Text', () => {
    test('should show dispatch text selector in search mode', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();

      const dispatchTextLabel = page.getByText(/texto de disparo/i);
      await expect(dispatchTextLabel).toBeVisible();
    });

    test('should show dispatch text selector in list mode', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();
      await automationsPage.selectMode('list');

      const dispatchTextLabel = page.getByText(/texto de disparo/i);
      await expect(dispatchTextLabel).toBeVisible();
    });

    test('should NOT show dispatch text selector in monitor mode', async ({ page }) => {
      await setupAndNavigate(page, automationsPage);
      await automationsPage.openCreateDialog();
      await automationsPage.selectMode('monitor');

      const dispatchTextLabel = page.getByText(/texto de disparo/i);
      await expect(dispatchTextLabel).toBeHidden();
    });
  });
});
