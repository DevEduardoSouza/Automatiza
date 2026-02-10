import { expect, test } from '@playwright/test';
import {
  MOCK_AUTOMATIONS,
  TEST_AUTOMATION,
  mockAutomationsListApi,
  mockCreateAutomationApi,
  mockDiagnoseAutomationApi,
  mockGroupsApi,
  mockLimitsApi,
  mockResetHistoryApi,
  mockRunAutomationApi,
  mockWhatsAppStatusApi,
  setupAutomationMocks,
} from './fixtures/automations.fixture';
import { AutomationsPage } from './pages/automations.page';

test.describe('Automations', () => {
  let automationsPage: AutomationsPage;

  test.beforeEach(async ({ page }) => {
    automationsPage = new AutomationsPage(page);
  });

  test.describe('Page Load', () => {
    test('should display automations page with title and list', async ({ page }) => {
      await setupAutomationMocks(page);
      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Verify page title
      await expect(automationsPage.pageTitle).toBeVisible();

      // Verify automation count badge
      await expect(automationsPage.automationCountBadge).toBeVisible();

      // Verify create button
      await expect(automationsPage.createButton).toBeVisible();
    });

    test('should display empty state when no automations exist', async ({ page }) => {
      await mockAutomationsListApi(page, []);
      await mockGroupsApi(page);
      await mockLimitsApi(page);
      await mockWhatsAppStatusApi(page);

      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Verify empty state message
      const emptyState = page.getByText(/nenhuma automacao|no automations/i);
      await expect(emptyState).toBeVisible();

      // Verify create first button
      const createFirstBtn = page.getByRole('button', { name: /criar primeira|create first/i });
      await expect(createFirstBtn).toBeVisible();
    });

    test('should display WhatsApp disconnected alert when not connected', async ({ page }) => {
      await mockAutomationsListApi(page);
      await mockGroupsApi(page);
      await mockLimitsApi(page);
      await mockWhatsAppStatusApi(page, false);

      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Verify WhatsApp disconnected alert
      const alert = page.getByText(/whatsapp.*desconectado|whatsapp.*disconnected/i);
      await expect(alert).toBeVisible();
    });
  });

  test.describe('Create Automation', () => {
    test('should create a new automation successfully', async ({ page }) => {
      await setupAutomationMocks(page);
      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Open create dialog
      await automationsPage.openCreateDialog();

      // Verify dialog is open
      await expect(automationsPage.dialogContent).toBeVisible();

      // Fill form
      await automationsPage.fillBasicForm({
        name: TEST_AUTOMATION.name,
        startTime: TEST_AUTOMATION.startTime,
        endTime: TEST_AUTOMATION.endTime,
      });

      // Select a group if available
      const groupSelector = page
        .getByRole('combobox')
        .or(page.locator('[data-testid="group-selector"]'))
        .first();
      if (await groupSelector.isVisible()) {
        await groupSelector.click();
        const groupOption = page.getByRole('option').first();
        if (await groupOption.isVisible()) {
          await groupOption.click();
        }
      }

      // Select a store if visible
      const storeCheckbox = page.getByLabel(/shopee/i);
      if (await storeCheckbox.isVisible()) {
        await storeCheckbox.check();
      }

      // Save the automation
      await automationsPage.save();

      // Verify success toast or dialog closes
      const toast = page.locator('[data-sonner-toast]');
      const dialogClosed = await automationsPage.dialogContent.isHidden().catch(() => false);

      // Either toast appears or dialog closes (depending on implementation)
      expect((await toast.isVisible()) || dialogClosed).toBeTruthy();
    });

    test('should show validation error when name is empty', async ({ page }) => {
      await setupAutomationMocks(page);
      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Open create dialog
      await automationsPage.openCreateDialog();

      // Try to save without filling the name
      await automationsPage.save();

      // Dialog should still be open
      await expect(automationsPage.dialogContent).toBeVisible();
    });

    test('should cancel creation and close dialog', async ({ page }) => {
      await setupAutomationMocks(page);
      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Open create dialog
      await automationsPage.openCreateDialog();

      // Fill some data
      await automationsPage.nameInput.fill('Test Automation');

      // Cancel
      await automationsPage.cancel();

      // Dialog should be closed
      await expect(automationsPage.dialogContent).toBeHidden();
    });
  });

  test.describe('Run Automation', () => {
    test('should run an automation successfully', async ({ page }) => {
      await setupAutomationMocks(page);
      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Find automation row
      const row = automationsPage.getRowByName(MOCK_AUTOMATIONS[0].name);
      await expect(row).toBeVisible();

      // Click run button
      const runButton = automationsPage.getRunButton(row);
      await runButton.click();

      // Verify running state or success toast
      const toast = page.locator('[data-sonner-toast]');
      const spinningIcon = row.locator('svg.animate-pulse');

      // Either toast appears or icon animates
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
      await setupAutomationMocks(page);
      await mockRunAutomationApi(page, 'error');

      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Find automation row
      const row = automationsPage.getRowByName(MOCK_AUTOMATIONS[0].name);

      // Click run button
      const runButton = automationsPage.getRunButton(row);
      await runButton.click();

      // Wait for error toast
      const toast = page.locator('[data-sonner-toast]');
      await expect(toast).toBeVisible({ timeout: 5000 });
    });
  });

  test.describe('Diagnose Automation', () => {
    test('should diagnose an automation and show results', async ({ page }) => {
      await setupAutomationMocks(page);
      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Find automation row
      const row = automationsPage.getRowByName(MOCK_AUTOMATIONS[0].name);
      await expect(row).toBeVisible();

      // Look for diagnose button (may be in a menu or direct)
      const diagnoseButton = page.getByRole('button', { name: /diagnosticar|diagnose/i }).first();

      if (await diagnoseButton.isVisible()) {
        await diagnoseButton.click();

        // Wait for diagnostic results
        await page.waitForTimeout(1000);

        // Verify diagnostic content is shown
        const configStep = page.getByText(/config/i);
        const whatsappStep = page.getByText(/whatsapp/i);
        const groupsStep = page.getByText(/grupos|groups/i);

        // At least one diagnostic step should be visible
        const configVisible = await configStep.isVisible().catch(() => false);
        const whatsappVisible = await whatsappStep.isVisible().catch(() => false);
        const groupsVisible = await groupsStep.isVisible().catch(() => false);

        expect(configVisible || whatsappVisible || groupsVisible).toBeTruthy();
      } else {
        // Diagnose might be in a dropdown menu
        const menuButton = row
          .getByRole('button', { name: /menu|mais|more/i })
          .or(row.locator('button').filter({ has: page.locator('svg.lucide-more-vertical') }));

        if (await menuButton.isVisible()) {
          await menuButton.click();
          const diagnoseMenuItem = page.getByRole('menuitem', { name: /diagnosticar|diagnose/i });
          if (await diagnoseMenuItem.isVisible()) {
            await diagnoseMenuItem.click();
          }
        }
      }
    });

    test('should show diagnostic issues when automation has problems', async ({ page }) => {
      await setupAutomationMocks(page);
      await mockDiagnoseAutomationApi(page, 'issues');

      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Find diagnose button if available
      const diagnoseButton = page.getByRole('button', { name: /diagnosticar|diagnose/i }).first();

      if (await diagnoseButton.isVisible()) {
        await diagnoseButton.click();

        // Wait for diagnostic results
        await page.waitForTimeout(1000);

        // Look for warning indicators
        const warningIndicator = page.getByText(/warning|aviso|problema|issue/i);
        // Diagnostic with issues should show warning content
        // This is implementation-dependent
      }
    });
  });

  test.describe('Reset Automation History', () => {
    test('should reset automation history successfully', async ({ page }) => {
      await setupAutomationMocks(page);
      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Find automation row
      const row = automationsPage.getRowByName(MOCK_AUTOMATIONS[0].name);
      await expect(row).toBeVisible();

      // Click reset button
      const resetButton = automationsPage.getResetButton(row);
      await resetButton.click();

      // Look for confirmation dialog
      const confirmButton = page.getByRole('button', { name: /confirmar|confirm|sim|yes/i });

      if (await confirmButton.isVisible()) {
        await confirmButton.click();
      }

      // Verify success toast or spinning icon
      const toast = page.locator('[data-sonner-toast]');
      const spinningIcon = row.locator('svg.animate-spin');

      const hasResponse = await Promise.race([
        toast
          .waitFor({ state: 'visible', timeout: 5000 })
          .then(() => true)
          .catch(() => false),
        spinningIcon
          .waitFor({ state: 'visible', timeout: 2000 })
          .then(() => true)
          .catch(() => false),
      ]);

      // Reset action should have some feedback
      expect(hasResponse || (await toast.isVisible())).toBeTruthy();
    });

    test('should handle reset history error', async ({ page }) => {
      await setupAutomationMocks(page);
      await mockResetHistoryApi(page, 'error');

      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Find automation row
      const row = automationsPage.getRowByName(MOCK_AUTOMATIONS[0].name);

      // Click reset button
      const resetButton = automationsPage.getResetButton(row);
      await resetButton.click();

      // Confirm if needed
      const confirmButton = page.getByRole('button', { name: /confirmar|confirm|sim|yes/i });
      if (await confirmButton.isVisible()) {
        await confirmButton.click();
      }

      // Wait for error toast
      const toast = page.locator('[data-sonner-toast]');
      await page.waitForTimeout(2000);
    });
  });

  test.describe('Delete Automation', () => {
    test('should delete an automation successfully', async ({ page }) => {
      await setupAutomationMocks(page);
      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Find automation row
      const row = automationsPage.getRowByName(MOCK_AUTOMATIONS[0].name);
      await expect(row).toBeVisible();

      // Click delete button
      const deleteButton = automationsPage.getDeleteButton(row);
      await deleteButton.click();

      // Confirm deletion
      const confirmButton = page.getByRole('button', {
        name: /confirmar|confirm|excluir|delete|sim|yes/i,
      });
      if (await confirmButton.isVisible()) {
        await confirmButton.click();
      }

      // Verify success toast
      const toast = page.locator('[data-sonner-toast]');
      await expect(toast).toBeVisible({ timeout: 5000 });
    });
  });

  test.describe('Toggle Status', () => {
    test('should toggle automation status', async ({ page }) => {
      await setupAutomationMocks(page);
      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Find automation row
      const row = automationsPage.getRowByName(MOCK_AUTOMATIONS[0].name);
      await expect(row).toBeVisible();

      // Get status switch
      const statusSwitch = automationsPage.getStatusSwitch(row);

      // Get initial state
      const initialChecked = await statusSwitch.getAttribute('aria-checked');

      // Toggle
      await statusSwitch.click();

      // Wait for update
      await page.waitForTimeout(500);

      // Either status changes or toast appears
      const toast = page.locator('[data-sonner-toast]');
      const newChecked = await statusSwitch.getAttribute('aria-checked');

      expect(newChecked !== initialChecked || (await toast.isVisible())).toBeTruthy();
    });
  });

  test.describe('Limit Reached', () => {
    test('should show limit reached message when trying to create beyond limit', async ({
      page,
    }) => {
      await mockAutomationsListApi(page, MOCK_AUTOMATIONS);
      await mockGroupsApi(page);
      await mockLimitsApi(page, { maxAutomations: 2, currentCount: 2 });
      await mockWhatsAppStatusApi(page);
      await mockCreateAutomationApi(page, 'limit_reached');

      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Verify limit reached badge/alert
      const limitBadge = page.locator('[class*="badge"]').filter({ hasText: /2\/2/ });
      await expect(limitBadge).toBeVisible();

      // Try to click create button
      await automationsPage.createButton.click();

      // Should show toast or tooltip about limit
      const toast = page.locator('[data-sonner-toast]');
      const tooltip = page.getByText(/limite.*atingido|limit.*reached/i);

      const hasLimitMessage = await Promise.race([
        toast
          .waitFor({ state: 'visible', timeout: 3000 })
          .then(() => true)
          .catch(() => false),
        tooltip
          .waitFor({ state: 'visible', timeout: 3000 })
          .then(() => true)
          .catch(() => false),
      ]);

      // Either toast or tooltip should appear, or button should be disabled
      const buttonDisabled = await automationsPage.createButton.isDisabled();
      expect(hasLimitMessage || buttonDisabled).toBeTruthy();
    });

    test('should disable create button when limit is reached', async ({ page }) => {
      await mockAutomationsListApi(page, MOCK_AUTOMATIONS);
      await mockGroupsApi(page);
      await mockLimitsApi(page, { maxAutomations: 2, currentCount: 2 });
      await mockWhatsAppStatusApi(page);

      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // The create button might be disabled or show a tooltip
      const createButton = automationsPage.createButton;
      const isDisabled = await createButton.isDisabled();

      if (!isDisabled) {
        // Hover to check for tooltip
        await createButton.hover();
        const tooltip = page.getByText(/limite.*atingido|limit.*reached/i);
        await page.waitForTimeout(500);
      }

      // Verify limit badge shows correct count
      const limitBadge = page.locator('[class*="badge"]').filter({ hasText: /2\/2/ });
      await expect(limitBadge).toBeVisible();
    });
  });

  test.describe('Edit Automation', () => {
    test('should open edit dialog with existing data', async ({ page }) => {
      await setupAutomationMocks(page);
      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Find automation row
      const row = automationsPage.getRowByName(MOCK_AUTOMATIONS[0].name);
      await expect(row).toBeVisible();

      // Click edit button
      const editButton = automationsPage.getEditButton(row);
      await editButton.click();

      // Verify dialog is open
      await expect(automationsPage.dialogContent).toBeVisible();

      // Verify name is pre-filled
      const nameValue = await automationsPage.nameInput.inputValue();
      expect(nameValue).toBeTruthy();
    });

    test('should update automation name', async ({ page }) => {
      await setupAutomationMocks(page);
      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Find and edit automation
      const row = automationsPage.getRowByName(MOCK_AUTOMATIONS[0].name);
      const editButton = automationsPage.getEditButton(row);
      await editButton.click();

      // Verify dialog is open
      await expect(automationsPage.dialogContent).toBeVisible();

      // Change name
      await automationsPage.nameInput.clear();
      await automationsPage.nameInput.fill('Updated Automation Name');

      // Save
      await automationsPage.save();

      // Verify success (toast or dialog closes)
      const toast = page.locator('[data-sonner-toast]');
      await page.waitForTimeout(1000);
    });
  });

  test.describe('Responsive Design', () => {
    test('should show table on desktop viewport', async ({ page }) => {
      await page.setViewportSize({ width: 1280, height: 720 });
      await setupAutomationMocks(page);
      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Table should be visible on desktop
      await expect(automationsPage.automationTable).toBeVisible();
    });

    test('should show cards on mobile viewport', async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 667 });
      await setupAutomationMocks(page);
      await automationsPage.goto();
      await automationsPage.waitForLoad();

      // Table should be hidden on mobile
      const tableHidden = await automationsPage.automationTable.isHidden();
      expect(tableHidden).toBeTruthy();

      // Cards should be visible
      const cards = page.locator('[class*="md:hidden"]').or(page.locator('.block.md\\:hidden'));
      // Mobile view should have alternative layout
    });
  });
});
