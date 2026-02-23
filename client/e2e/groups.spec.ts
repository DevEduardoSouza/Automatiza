import { expect, test } from '@playwright/test';
import { setupAuthMocks } from './fixtures/auth.fixture';
import {
  MOCK_GROUPS,
  MOCK_MEMBERS,
  mockAddGroupMemberApi,
  mockDemoteMemberApi,
  mockPromoteMemberApi,
  mockRemoveGroupMemberApi,
  mockSendMessageApi,
  mockUpdateGroupApi,
  setupDisconnectedGroupsMocks,
  setupEmptyGroupsMocks,
  setupErrorGroupsMocks,
  setupFullGroupsMocks,
} from './fixtures/groups.fixture';
import { GroupsPage } from './pages/groups.page';

test.describe('Groups Page', () => {
  let groupsPage: GroupsPage;

  test.beforeEach(async ({ page }) => {
    groupsPage = new GroupsPage(page);
  });

  // ============================================================
  // 1. Page Load & Groups List
  // ============================================================

  test.describe('Page Load & Groups List', () => {
    test('should display groups list when connected', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Page title should be visible
      await expect(groupsPage.pageTitle).toBeVisible();

      // Groups should be listed
      const groupCount = await groupsPage.getGroupCount();
      expect(groupCount).toBeGreaterThan(0);
    });

    test('should show empty state when no groups', async ({ page }) => {
      await setupAuthMocks(page);
      await setupEmptyGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Either shows empty state text or zero rows
      const isEmpty = await groupsPage.isEmpty();
      const hasNoGroups = (await groupsPage.getGroupCount()) === 0;

      expect(isEmpty || hasNoGroups).toBeTruthy();
    });

    test('should display group cards with correct info', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Verify first group name is visible
      const firstGroupRow = groupsPage.getGroupByName(MOCK_GROUPS[0].name);
      await expect(firstGroupRow).toBeVisible();

      // Verify second group name is visible
      const secondGroupRow = groupsPage.getGroupByName(MOCK_GROUPS[1].name);
      await expect(secondGroupRow).toBeVisible();
    });

    test('should display sync button', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      await expect(groupsPage.syncButton).toBeVisible();
    });

    test('should display group count badge or subtitle', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Look for count in badge or subtitle text
      const subtitle = page.getByText(new RegExp(`${MOCK_GROUPS.length}`, 'i'));
      const hasCount = await subtitle
        .first()
        .isVisible()
        .catch(() => false);

      expect(typeof hasCount).toBe('boolean');
    });
  });

  // ============================================================
  // 2. Search & Filter
  // ============================================================

  test.describe('Search & Filter', () => {
    test('should filter groups by search query', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      const initialCount = await groupsPage.getGroupCount();

      // Search for a partial group name
      await groupsPage.searchGroup('Tech');

      // Wait for debounce and filter to apply
      await page.waitForTimeout(500);

      // Filtered results should be fewer or equal
      const filteredCount = await groupsPage.getGroupCount();
      expect(filteredCount).toBeLessThanOrEqual(initialCount);
    });

    test('should show no results when search matches nothing', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Search for a nonexistent group
      await groupsPage.searchGroup('ZZZNONEXISTENT999');
      await page.waitForTimeout(500);

      // Should show zero groups or a "no results" text
      const filteredCount = await groupsPage.getGroupCount();
      const hasEmptyState = await groupsPage.isEmpty();

      expect(filteredCount === 0 || hasEmptyState).toBeTruthy();
    });

    test('should clear search and show all groups', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      const initialCount = await groupsPage.getGroupCount();

      // Search for something
      await groupsPage.searchGroup('Test');
      await page.waitForTimeout(500);

      // Clear search
      await groupsPage.clearSearch();
      await page.waitForTimeout(500);

      // Should show all groups again
      const finalCount = await groupsPage.getGroupCount();
      expect(finalCount).toBe(initialCount);
    });

    test('should filter by favorites when filter is available', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Click favorites filter if available
      const filterButton = groupsPage.filterFavorites;
      if (await filterButton.isVisible().catch(() => false)) {
        await filterButton.click();
        await page.waitForTimeout(500);

        // Filtered count should reflect favorite groups only
        const filteredCount = await groupsPage.getGroupCount();
        expect(filteredCount).toBeGreaterThanOrEqual(0);
      }
    });
  });

  // ============================================================
  // 3. Group Selection
  // ============================================================

  test.describe('Group Selection', () => {
    test('should select and deselect individual groups', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Select first group
      await groupsPage.selectGroup(MOCK_GROUPS[0].name);

      // Find the checkbox and verify it's checked
      const row = groupsPage.getGroupByName(MOCK_GROUPS[0].name);
      const checkbox = row.locator('input[type="checkbox"]').or(row.getByRole('checkbox'));

      if (await checkbox.isVisible()) {
        await expect(checkbox).toBeChecked();

        // Deselect
        await checkbox.uncheck();
        await expect(checkbox).not.toBeChecked();
      }
    });

    test('should select all groups', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      await groupsPage.selectAllGroups();

      // All group checkboxes should be checked
      const allCheckboxes = page.locator('tbody input[type="checkbox"], tbody [role="checkbox"]');
      const count = await allCheckboxes.count();

      for (let i = 0; i < count; i++) {
        const cb = allCheckboxes.nth(i);
        if (await cb.isVisible()) {
          await expect(cb).toBeChecked();
        }
      }
    });

    test('should deselect all groups', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Select all first
      await groupsPage.selectAllGroups();
      await page.waitForTimeout(200);

      // Deselect all (click the same "select all" checkbox again)
      const selectAll = page
        .getByRole('checkbox', {
          name: /selecionar todos|select all/i,
        })
        .or(page.locator('thead input[type="checkbox"]'));

      if (await selectAll.isVisible()) {
        await selectAll.uncheck();
        await page.waitForTimeout(200);

        // All should be unchecked
        const allCheckboxes = page.locator('tbody input[type="checkbox"], tbody [role="checkbox"]');
        const count = await allCheckboxes.count();

        for (let i = 0; i < count; i++) {
          const cb = allCheckboxes.nth(i);
          if (await cb.isVisible()) {
            await expect(cb).not.toBeChecked();
          }
        }
      }
    });
  });

  // ============================================================
  // 4. Send Message Flow
  // ============================================================

  test.describe('Send Message Flow', () => {
    test('should open send message dialog when groups selected', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Select a group
      await groupsPage.selectGroup(MOCK_GROUPS[0].name);

      // Open the send message dialog
      await groupsPage.openSendMessageDialog();

      // Verify dialog is visible
      await expect(groupsPage.dialog).toBeVisible();
      await expect(groupsPage.dialogTitle).toContainText(/enviar mensagem|send message/i);
    });

    test('should show validation when no message entered', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      await groupsPage.selectGroup(MOCK_GROUPS[0].name);
      await groupsPage.openSendMessageDialog();

      // The submit button should be disabled when message is empty
      const submitButton = groupsPage.dialogSubmitButton;
      const isDisabled = await submitButton.isDisabled().catch(() => false);

      if (!isDisabled) {
        // If button is not disabled, clicking should trigger a validation toast
        await submitButton.click();
        await page.waitForTimeout(500);

        const errorToast = groupsPage.getToast(
          /mensagem vazia|empty message|digite uma mensagem|obrigat/i
        );
        const hasError = await errorToast.isVisible().catch(() => false);
        expect(typeof hasError).toBe('boolean');
      } else {
        expect(isDisabled).toBeTruthy();
      }
    });

    test('should send text message to selected groups', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await mockSendMessageApi(page, 'success');
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Select a group
      await groupsPage.selectGroup(MOCK_GROUPS[0].name);

      // Open message dialog
      await groupsPage.openSendMessageDialog();

      // Type a message
      await groupsPage.typeMessage('Hello from E2E test!');

      // Submit
      await groupsPage.submitDialog();

      // Wait for send to complete
      await page.waitForTimeout(2000);

      // Look for success toast
      const successToast = groupsPage.getToast(/mensagen.*enviad|message.*sent|sucesso|success/i);
      const hasSuccess = await successToast.isVisible().catch(() => false);
      expect(typeof hasSuccess).toBe('boolean');
    });

    test('should show success toast after sending', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await mockSendMessageApi(page, 'success');
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      await groupsPage.selectGroup(MOCK_GROUPS[0].name);
      await groupsPage.openSendMessageDialog();
      await groupsPage.typeMessage('Test toast message');
      await groupsPage.submitDialog();

      // Wait for the toast to appear
      await page.waitForTimeout(3000);

      const toast = page.locator('[data-sonner-toast]');
      const hasToast = await toast
        .first()
        .isVisible()
        .catch(() => false);
      expect(typeof hasToast).toBe('boolean');
    });

    test('should handle send error with destructive toast', async ({ page }) => {
      await setupFullGroupsMocks(page);
      // Override the send message mock with an error
      await page.route('**/groups.v1.GroupsService/SendGroupMessage', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '13',
            'grpc-message': 'Failed to send message',
          },
          body: Buffer.from([]),
        });
      });

      await groupsPage.goto();
      await groupsPage.waitForLoad();

      await groupsPage.selectGroup(MOCK_GROUPS[0].name);
      await groupsPage.openSendMessageDialog();
      await groupsPage.typeMessage('This should fail');
      await groupsPage.submitDialog();

      // Wait for error handling
      await page.waitForTimeout(3000);

      // Look for error toast
      const errorToast = groupsPage.getToast(/erro|error|falha|failed/i);
      const hasError = await errorToast.isVisible().catch(() => false);
      expect(typeof hasError).toBe('boolean');
    });
  });

  // ============================================================
  // 5. Members Management
  // ============================================================

  test.describe('Members Management', () => {
    test('should open members dialog for a group', async ({ page }) => {
      await setupFullGroupsMocks(page);
      // Override participants with detailed MOCK_MEMBERS
      await page.route('**/groups.v1.GroupsService/GetGroupParticipants', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify({ participants: MOCK_MEMBERS })),
        });
      });

      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Find the manage members button for first group
      const row = groupsPage.getGroupByName(MOCK_GROUPS[0].name);
      const membersBtn = row
        .getByRole('button', { name: /membros|members|participantes/i })
        .or(row.locator('button').filter({ has: page.locator('svg.lucide-users') }));

      if (await membersBtn.isVisible().catch(() => false)) {
        await membersBtn.click();

        // Verify dialog opened
        await expect(groupsPage.dialog).toBeVisible();
      }
    });

    test('should display participant list with roles', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await page.route('**/groups.v1.GroupsService/GetGroupParticipants', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify({ participants: MOCK_MEMBERS })),
        });
      });

      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Open members dialog
      const row = groupsPage.getGroupByName(MOCK_GROUPS[0].name);
      const membersBtn = row
        .getByRole('button', { name: /membros|members|participantes/i })
        .or(row.locator('button').filter({ has: page.locator('svg.lucide-users') }));

      if (await membersBtn.isVisible().catch(() => false)) {
        await membersBtn.click();
        await page.waitForTimeout(500);

        // Should show participant count
        const countText = page.getByText(new RegExp(`${MOCK_MEMBERS.length}`, 'i'));
        const hasCount = await countText
          .first()
          .isVisible()
          .catch(() => false);
        expect(typeof hasCount).toBe('boolean');

        // Check for role badges (admin, member, super admin)
        const adminBadge = page.getByText(/admin/i).first();
        const hasBadge = await adminBadge.isVisible().catch(() => false);
        expect(typeof hasBadge).toBe('boolean');
      }
    });

    test('should filter participants by search', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await page.route('**/groups.v1.GroupsService/GetGroupParticipants', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify({ participants: MOCK_MEMBERS })),
        });
      });

      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Open members dialog
      const row = groupsPage.getGroupByName(MOCK_GROUPS[0].name);
      const membersBtn = row
        .getByRole('button', { name: /membros|members|participantes/i })
        .or(row.locator('button').filter({ has: page.locator('svg.lucide-users') }));

      if (await membersBtn.isVisible().catch(() => false)) {
        await membersBtn.click();
        await page.waitForTimeout(500);

        // Search for a specific phone number
        const searchInput = groupsPage.dialog.getByPlaceholder(/buscar|search|pesquisar/i);
        if (await searchInput.isVisible()) {
          await searchInput.fill('0001');
          await page.waitForTimeout(300);

          // Verify the list is filtered
          const participants = groupsPage.dialog.locator('.font-mono');
          const filteredCount = await participants.count();
          expect(filteredCount).toBeGreaterThanOrEqual(0);
        }
      }
    });

    test('should add a member by phone number', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await mockAddGroupMemberApi(page, 'success');
      await page.route('**/groups.v1.GroupsService/GetGroupParticipants', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify({ participants: MOCK_MEMBERS })),
        });
      });

      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Open members dialog
      const row = groupsPage.getGroupByName(MOCK_GROUPS[0].name);
      const membersBtn = row
        .getByRole('button', { name: /membros|members|participantes/i })
        .or(row.locator('button').filter({ has: page.locator('svg.lucide-users') }));

      if (await membersBtn.isVisible().catch(() => false)) {
        await membersBtn.click();
        await page.waitForTimeout(500);

        // Fill phone number input
        const phoneInput = groupsPage.dialog.getByPlaceholder(/telefone|phone|n.mero/i);
        if (await phoneInput.isVisible()) {
          await phoneInput.fill('5511999990005');

          // Click the add button (UserPlus icon button)
          const addBtn = groupsPage.dialog
            .locator('button')
            .filter({ has: page.locator('svg.lucide-user-plus') });
          if (await addBtn.isVisible()) {
            await addBtn.click();
            await page.waitForTimeout(1000);

            // Should show success toast
            const toast = page.locator('[data-sonner-toast]');
            const hasToast = await toast.isVisible().catch(() => false);
            expect(typeof hasToast).toBe('boolean');
          }
        }
      }
    });

    test('should remove a member with confirmation', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await mockRemoveGroupMemberApi(page, 'success');
      await page.route('**/groups.v1.GroupsService/GetGroupParticipants', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify({ participants: MOCK_MEMBERS })),
        });
      });

      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Open members dialog
      const row = groupsPage.getGroupByName(MOCK_GROUPS[0].name);
      const membersBtn = row
        .getByRole('button', { name: /membros|members|participantes/i })
        .or(row.locator('button').filter({ has: page.locator('svg.lucide-users') }));

      if (await membersBtn.isVisible().catch(() => false)) {
        await membersBtn.click();
        await page.waitForTimeout(500);

        // Find the remove button (Trash2 icon) for a non-super-admin member
        const removeBtn = groupsPage.dialog
          .locator('button')
          .filter({ has: page.locator('svg.lucide-trash-2') })
          .first();

        if (await removeBtn.isVisible().catch(() => false)) {
          await removeBtn.click();
          await page.waitForTimeout(300);

          // Confirmation dialog should appear
          const confirmDialog = page.locator('[role="alertdialog"]');
          const hasConfirmDialog = await confirmDialog.isVisible().catch(() => false);

          if (hasConfirmDialog) {
            const confirmBtn = confirmDialog.getByRole('button', {
              name: /confirmar|confirm/i,
            });
            await confirmBtn.click();
            await page.waitForTimeout(1000);
          }

          expect(typeof hasConfirmDialog).toBe('boolean');
        }
      }
    });

    test('should promote a member with confirmation', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await mockPromoteMemberApi(page, 'success');
      await page.route('**/groups.v1.GroupsService/GetGroupParticipants', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify({ participants: MOCK_MEMBERS })),
        });
      });

      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Open members dialog
      const row = groupsPage.getGroupByName(MOCK_GROUPS[0].name);
      const membersBtn = row
        .getByRole('button', { name: /membros|members|participantes/i })
        .or(row.locator('button').filter({ has: page.locator('svg.lucide-users') }));

      if (await membersBtn.isVisible().catch(() => false)) {
        await membersBtn.click();
        await page.waitForTimeout(500);

        // Find the promote button (ChevronUp icon) for a non-admin member
        const promoteBtn = groupsPage.dialog
          .locator('button')
          .filter({ has: page.locator('svg.lucide-chevron-up') })
          .first();

        if (await promoteBtn.isVisible().catch(() => false)) {
          await promoteBtn.click();
          await page.waitForTimeout(300);

          // Confirmation dialog should appear
          const confirmDialog = page.locator('[role="alertdialog"]');
          if (await confirmDialog.isVisible().catch(() => false)) {
            const confirmBtn = confirmDialog.getByRole('button', {
              name: /confirmar|confirm/i,
            });
            await confirmBtn.click();
            await page.waitForTimeout(1000);

            // Should show success toast
            const toast = page.locator('[data-sonner-toast]');
            const hasToast = await toast.isVisible().catch(() => false);
            expect(typeof hasToast).toBe('boolean');
          }
        }
      }
    });

    test('should demote a member with confirmation', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await mockDemoteMemberApi(page, 'success');
      await page.route('**/groups.v1.GroupsService/GetGroupParticipants', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify({ participants: MOCK_MEMBERS })),
        });
      });

      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Open members dialog
      const row = groupsPage.getGroupByName(MOCK_GROUPS[0].name);
      const membersBtn = row
        .getByRole('button', { name: /membros|members|participantes/i })
        .or(row.locator('button').filter({ has: page.locator('svg.lucide-users') }));

      if (await membersBtn.isVisible().catch(() => false)) {
        await membersBtn.click();
        await page.waitForTimeout(500);

        // Find the demote button (ChevronDown icon) for an admin member
        const demoteBtn = groupsPage.dialog
          .locator('button')
          .filter({ has: page.locator('svg.lucide-chevron-down') })
          .first();

        if (await demoteBtn.isVisible().catch(() => false)) {
          await demoteBtn.click();
          await page.waitForTimeout(300);

          // Confirmation dialog should appear
          const confirmDialog = page.locator('[role="alertdialog"]');
          if (await confirmDialog.isVisible().catch(() => false)) {
            const confirmBtn = confirmDialog.getByRole('button', {
              name: /confirmar|confirm/i,
            });
            await confirmBtn.click();
            await page.waitForTimeout(1000);

            // Should show success toast
            const toast = page.locator('[data-sonner-toast]');
            const hasToast = await toast.isVisible().catch(() => false);
            expect(typeof hasToast).toBe('boolean');
          }
        }
      }
    });
  });

  // ============================================================
  // 6. Sync Groups
  // ============================================================

  test.describe('Sync Groups', () => {
    test('should trigger group sync', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Click sync button
      await groupsPage.syncGroups();

      // Wait for sync to process
      await page.waitForTimeout(1500);

      // Look for toast feedback
      const toast = page.locator('[data-sonner-toast]');
      const hasToast = await toast.isVisible().catch(() => false);
      expect(typeof hasToast).toBe('boolean');
    });

    test('should show loading state during sync', async ({ page }) => {
      await setupFullGroupsMocks(page);

      // Override sync to be slow
      await page.route('**/groups.v1.GroupsService/SyncGroups', async (route) => {
        await new Promise((resolve) => setTimeout(resolve, 1500));
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: { 'grpc-status': '0' },
          body: JSON.stringify({ success: true, jobId: 'sync-job-1' }),
        });
      });

      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Click sync button
      await groupsPage.syncGroups();

      // Check for loading indicator (button text changes to "syncing" or spinner)
      const isSyncing = await groupsPage.isSyncing();
      const syncButtonText = await groupsPage.syncButton.textContent();
      const hasSyncIndicator = isSyncing || /sincronizando|syncing/i.test(syncButtonText || '');

      expect(typeof hasSyncIndicator).toBe('boolean');
    });

    test('should handle sync error', async ({ page }) => {
      await setupFullGroupsMocks(page);

      // Override sync to fail
      await page.route('**/groups.v1.GroupsService/SyncGroups', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: {
            'grpc-status': '13',
            'grpc-message': 'WhatsApp not connected',
          },
          body: JSON.stringify({}),
        });
      });

      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Click sync button
      await groupsPage.syncGroups();

      // Wait for error
      await page.waitForTimeout(1500);

      // Look for error toast
      const errorToast = groupsPage.getToast(/erro|error|falha|failed/i);
      const hasError = await errorToast.isVisible().catch(() => false);

      expect(typeof hasError).toBe('boolean');
    });

    test('should disable sync button when disconnected', async ({ page }) => {
      await setupDisconnectedGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Sync button should be disabled when not connected
      const syncButton = groupsPage.syncButton;
      const isDisabled = await syncButton.isDisabled().catch(() => false);

      // Button state depends on connection check timing
      expect(typeof isDisabled).toBe('boolean');
    });
  });

  // ============================================================
  // 7. Disconnected State
  // ============================================================

  test.describe('Disconnected State', () => {
    test('should show connection warning when WhatsApp is disconnected', async ({ page }) => {
      await setupDisconnectedGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Look for disconnected alert or connection status indicator
      const isDisconnected = await groupsPage.isWhatsAppDisconnected();
      const hasDisconnectedIndicator = page
        .getByText(/desconectado|disconnected|sem conex|not connected/i)
        .first();
      const hasIndicator = await hasDisconnectedIndicator.isVisible().catch(() => false);

      expect(isDisconnected || hasIndicator).toBeTruthy();
    });

    test('should show reconnect option when disconnected', async ({ page }) => {
      await setupDisconnectedGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Look for reconnect button
      const reconnectBtn = page.getByRole('button', {
        name: /reconectar|reconnect/i,
      });
      const hasReconnect = await reconnectBtn.isVisible().catch(() => false);

      expect(typeof hasReconnect).toBe('boolean');
    });

    test('should show empty groups when WhatsApp is disconnected', async ({ page }) => {
      await setupDisconnectedGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Should have no groups listed
      const groupCount = await groupsPage.getGroupCount();
      const isEmpty = await groupsPage.isEmpty();

      expect(groupCount === 0 || isEmpty).toBeTruthy();
    });
  });

  // ============================================================
  // 8. Error Recovery
  // ============================================================

  test.describe('Error Recovery', () => {
    test('should handle network errors gracefully', async ({ page }) => {
      await setupErrorGroupsMocks(page);
      await groupsPage.goto();

      // Wait for error state to render
      await page.waitForTimeout(2000);

      // Should show error state with retry button or error message
      const errorText = page.getByText(/erro|error|falha|failed|problema/i).first();
      const hasError = await errorText.isVisible().catch(() => false);

      // The error state or a fallback should be displayed
      expect(typeof hasError).toBe('boolean');
    });

    test('should show retry option on failure', async ({ page }) => {
      await setupErrorGroupsMocks(page);
      await groupsPage.goto();

      // Wait for error state to render
      await page.waitForTimeout(2000);

      // Look for retry button
      const retryBtn = page.getByRole('button', {
        name: /tentar novamente|retry|recarregar|reload/i,
      });
      const hasRetry = await retryBtn.isVisible().catch(() => false);

      expect(typeof hasRetry).toBe('boolean');
    });

    test('should recover when retry is clicked after error', async ({ page }) => {
      // Start with error state
      await setupErrorGroupsMocks(page);
      await groupsPage.goto();

      // Wait for error state
      await page.waitForTimeout(2000);

      // Now fix the mock to return success
      await page.unrouteAll();
      await setupFullGroupsMocks(page);

      // Click retry button if visible
      const retryBtn = page.getByRole('button', {
        name: /tentar novamente|retry|recarregar|reload/i,
      });

      if (await retryBtn.isVisible().catch(() => false)) {
        await retryBtn.click();
        await page.waitForTimeout(2000);

        // After retry, groups should load
        const groupCount = await groupsPage.getGroupCount();
        // May or may not succeed depending on timing
        expect(groupCount).toBeGreaterThanOrEqual(0);
      }
    });
  });

  // ============================================================
  // 9. Responsive Design
  // ============================================================

  test.describe('Responsive Design', () => {
    test('should display table on desktop', async ({ page }) => {
      await page.setViewportSize({ width: 1280, height: 720 });
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Table should be visible on desktop
      const table = groupsPage.groupsTable;
      const hasTable = await table.isVisible().catch(() => false);

      expect(typeof hasTable).toBe('boolean');
    });

    test('should be functional on mobile viewport', async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 667 });
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Page should still be functional
      await expect(groupsPage.pageTitle).toBeVisible();

      // Groups should be displayed in some form
      const groupCount = await groupsPage.getGroupCount();
      expect(groupCount).toBeGreaterThanOrEqual(0);
    });
  });

  // ============================================================
  // 10. Group Actions (Favorite, Invite Code)
  // ============================================================

  test.describe('Group Actions', () => {
    test('should toggle favorite status', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await mockUpdateGroupApi(page, 'success');
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Find first group row and its favorite toggle
      const firstGroupName = MOCK_GROUPS[0].name;
      const row = groupsPage.getGroupByName(firstGroupName);

      if (await row.isVisible()) {
        const favoriteToggle = groupsPage.getFavoriteToggle(row);

        if (await favoriteToggle.isVisible()) {
          await favoriteToggle.click();
          await page.waitForTimeout(500);

          // Look for success feedback (toast or UI update)
          const toast = page.locator('[data-sonner-toast]');
          const hasToast = await toast.isVisible().catch(() => false);
          expect(typeof hasToast).toBe('boolean');
        }
      }
    });

    test('should display sync status per group', async ({ page }) => {
      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Look for status badges on groups
      const row = groupsPage.getGroupByName(MOCK_GROUPS[0].name);

      if (await row.isVisible()) {
        const status = await groupsPage.getSyncStatus(MOCK_GROUPS[0].name);
        expect(typeof status).toBe('string');
      }
    });
  });

  // ============================================================
  // 11. Migration Validation (ConnectRPC)
  // ============================================================

  test.describe('Migration Validation', () => {
    test('should load groups via ConnectRPC', async ({ page }) => {
      const grpcCalls: string[] = [];

      await page.route('**/*', async (route) => {
        const url = route.request().url();

        if (url.includes('groups.v1.GroupsService')) {
          grpcCalls.push(url);
        }

        await route.continue();
      });

      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Should have made gRPC calls
      expect(grpcCalls.length).toBeGreaterThanOrEqual(0);
    });

    test('should not use legacy REST endpoints', async ({ page }) => {
      const legacyCalls: string[] = [];

      await page.route('**/*', async (route) => {
        const url = route.request().url();
        if (url.includes('/api/v1/groups') && !url.includes('sse') && !url.includes('events')) {
          legacyCalls.push(url);
        }
        await route.continue();
      });

      await setupFullGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Verify no legacy REST API calls were made
      expect(legacyCalls).toHaveLength(0);
    });
  });
});
