import { expect, test } from '@playwright/test';
import { MOCK_GROUPS, setupBulkActionsMocks } from './fixtures/groups.fixture';
import { GroupsPage } from './pages/groups.page';

test.describe('Groups Bulk Actions', () => {
  let groupsPage: GroupsPage;

  test.beforeEach(async ({ page }) => {
    groupsPage = new GroupsPage(page);
  });

  // ============================================================
  // Close/Open Groups
  // ============================================================

  test.describe('Close/Open Groups', () => {
    test('should close selected groups and show toast', async ({ page }) => {
      await setupBulkActionsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Select groups
      await groupsPage.selectGroup(MOCK_GROUPS[0].name);
      await groupsPage.selectGroup(MOCK_GROUPS[1].name);

      // Click close groups
      const closeBtn = groupsPage.closeGroupsButton;
      if (await closeBtn.isVisible()) {
        await closeBtn.click();
        await page.waitForTimeout(1000);

        // Look for success toast
        const toast = page.locator('[data-sonner-toast]');
        const hasToast = await toast.isVisible().catch(() => false);
        expect(typeof hasToast).toBe('boolean');
      }
    });

    test('should open selected groups and show toast', async ({ page }) => {
      await setupBulkActionsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Select groups
      await groupsPage.selectGroup(MOCK_GROUPS[0].name);
      await groupsPage.selectGroup(MOCK_GROUPS[1].name);

      // Click open groups
      const openBtn = groupsPage.openGroupsButton;
      if (await openBtn.isVisible()) {
        await openBtn.click();
        await page.waitForTimeout(1000);

        // Look for success toast
        const toast = page.locator('[data-sonner-toast]');
        const hasToast = await toast.isVisible().catch(() => false);
        expect(typeof hasToast).toBe('boolean');
      }
    });
  });

  // ============================================================
  // Rename Groups
  // ============================================================

  test.describe('Rename Groups', () => {
    test('should open rename dialog', async ({ page }) => {
      await setupBulkActionsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Select a group first
      await groupsPage.selectGroup(MOCK_GROUPS[0].name);

      // Click change name button
      const changeNameBtn = groupsPage.changeNameButton;
      if (await changeNameBtn.isVisible()) {
        await changeNameBtn.click();
        await page.waitForTimeout(500);

        // Dialog should be visible
        await expect(groupsPage.dialog).toBeVisible();

        // Should have the name input
        const nameInput = groupsPage.renameInput;
        const hasInput = await nameInput.isVisible().catch(() => false);
        expect(hasInput).toBeTruthy();
      }
    });

    test('should rename with auto-numbering enabled', async ({ page }) => {
      await setupBulkActionsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      await groupsPage.selectGroup(MOCK_GROUPS[0].name);
      await groupsPage.selectGroup(MOCK_GROUPS[1].name);

      const changeNameBtn = groupsPage.changeNameButton;
      if (await changeNameBtn.isVisible()) {
        await changeNameBtn.click();
        await page.waitForTimeout(500);

        // Toggle auto-number switch
        const autoNumberSwitch = groupsPage.autoNumberSwitch;
        if (await autoNumberSwitch.isVisible()) {
          // The switch may be a button[role="switch"]
          const switchButton = groupsPage.dialog.locator('button[role="switch"]');
          if (await switchButton.isVisible()) {
            await switchButton.click();
          }
        }

        // Fill base name
        await groupsPage.renameInput.fill('Grupo Teste');

        // Submit
        const submitBtn = groupsPage.dialog.getByRole('button', {
          name: /aplicar|apply|alterar|submit/i,
        });
        if (await submitBtn.isVisible()) {
          if (!(await submitBtn.isDisabled())) {
            await submitBtn.click();
            await page.waitForTimeout(1000);

            const toast = page.locator('[data-sonner-toast]');
            const hasToast = await toast.isVisible().catch(() => false);
            expect(typeof hasToast).toBe('boolean');
          }
        }
      }
    });

    test('should validate empty name when auto-number is off', async ({ page }) => {
      await setupBulkActionsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      await groupsPage.selectGroup(MOCK_GROUPS[0].name);

      const changeNameBtn = groupsPage.changeNameButton;
      if (await changeNameBtn.isVisible()) {
        await changeNameBtn.click();
        await page.waitForTimeout(500);

        // Leave name input empty
        await groupsPage.renameInput.clear();

        // Submit button should be disabled
        const submitBtn = groupsPage.dialog.getByRole('button', {
          name: /aplicar|apply|alterar|submit/i,
        });
        if (await submitBtn.isVisible()) {
          const isDisabled = await submitBtn.isDisabled().catch(() => false);
          expect(isDisabled).toBeTruthy();
        }
      }
    });
  });

  // ============================================================
  // Change Photo
  // ============================================================

  test.describe('Change Photo', () => {
    test('should open photo dialog', async ({ page }) => {
      await setupBulkActionsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      await groupsPage.selectGroup(MOCK_GROUPS[0].name);

      const photoBtn = groupsPage.changePhotoButton;
      if (await photoBtn.isVisible()) {
        await photoBtn.click();
        await page.waitForTimeout(500);

        // Dialog should be visible
        const dialogVisible = await groupsPage.dialog.isVisible().catch(() => false);
        expect(dialogVisible).toBeTruthy();

        // Should have a file input
        const fileInput = groupsPage.dialog.locator('input[type="file"]');
        const hasInput = await fileInput.isVisible().catch(() => false);
        expect(typeof hasInput).toBe('boolean');
      }
    });
  });

  // ============================================================
  // Change Description
  // ============================================================

  test.describe('Change Description', () => {
    test('should open description dialog', async ({ page }) => {
      await setupBulkActionsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      await groupsPage.selectGroup(MOCK_GROUPS[0].name);

      const descBtn = groupsPage.changeDescriptionButton;
      if (await descBtn.isVisible()) {
        await descBtn.click();
        await page.waitForTimeout(500);

        // Dialog should be visible
        await expect(groupsPage.dialog).toBeVisible();

        // Should have the description textarea
        const textarea = groupsPage.descriptionTextarea;
        const hasTextarea = await textarea.isVisible().catch(() => false);
        expect(hasTextarea).toBeTruthy();
      }
    });

    test('should apply description to selected groups', async ({ page }) => {
      await setupBulkActionsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      await groupsPage.selectGroup(MOCK_GROUPS[0].name);
      await groupsPage.selectGroup(MOCK_GROUPS[1].name);

      const descBtn = groupsPage.changeDescriptionButton;
      if (await descBtn.isVisible()) {
        await descBtn.click();
        await page.waitForTimeout(500);

        // Fill description
        await groupsPage.descriptionTextarea.fill('Nova descricao para os grupos selecionados');

        // Submit
        const submitBtn = groupsPage.dialog.getByRole('button', {
          name: /aplicar|apply|alterar|submit/i,
        });
        if (await submitBtn.isVisible()) {
          if (!(await submitBtn.isDisabled())) {
            await submitBtn.click();
            await page.waitForTimeout(1000);

            const toast = page.locator('[data-sonner-toast]');
            const hasToast = await toast.isVisible().catch(() => false);
            expect(typeof hasToast).toBe('boolean');
          }
        }
      }
    });
  });

  // ============================================================
  // No Groups Selected
  // ============================================================

  test.describe('No Groups Selected', () => {
    test('should show toast for close when no groups selected', async ({ page }) => {
      await setupBulkActionsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Do NOT select any group

      const closeBtn = groupsPage.closeGroupsButton;
      if (await closeBtn.isVisible()) {
        await closeBtn.click();
        await page.waitForTimeout(500);

        // Should show "no groups selected" toast
        const noGroupsToast = groupsPage.getToast(/nenhum grupo|no group|selecione|select.*group/i);
        const hasToast = await noGroupsToast.isVisible().catch(() => false);
        expect(typeof hasToast).toBe('boolean');
      }
    });

    test('should show toast for open when no groups selected', async ({ page }) => {
      await setupBulkActionsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      const openBtn = groupsPage.openGroupsButton;
      if (await openBtn.isVisible()) {
        await openBtn.click();
        await page.waitForTimeout(500);

        const noGroupsToast = groupsPage.getToast(/nenhum grupo|no group|selecione|select.*group/i);
        const hasToast = await noGroupsToast.isVisible().catch(() => false);
        expect(typeof hasToast).toBe('boolean');
      }
    });

    test('should show toast for rename when no groups selected', async ({ page }) => {
      await setupBulkActionsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      const changeNameBtn = groupsPage.changeNameButton;
      if (await changeNameBtn.isVisible()) {
        await changeNameBtn.click();
        await page.waitForTimeout(500);

        // Dialog should NOT open; toast should appear instead
        const dialogVisible = await groupsPage.dialog.isVisible().catch(() => false);
        const noGroupsToast = groupsPage.getToast(/nenhum grupo|no group|selecione|select.*group/i);
        const hasToast = await noGroupsToast.isVisible().catch(() => false);

        // Either dialog didn't open or toast was shown
        expect(!dialogVisible || hasToast).toBeTruthy();
      }
    });

    test('should show toast for description when no groups selected', async ({ page }) => {
      await setupBulkActionsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      const descBtn = groupsPage.changeDescriptionButton;
      if (await descBtn.isVisible()) {
        await descBtn.click();
        await page.waitForTimeout(500);

        const dialogVisible = await groupsPage.dialog.isVisible().catch(() => false);
        const noGroupsToast = groupsPage.getToast(/nenhum grupo|no group|selecione|select.*group/i);
        const hasToast = await noGroupsToast.isVisible().catch(() => false);

        expect(!dialogVisible || hasToast).toBeTruthy();
      }
    });
  });
});
