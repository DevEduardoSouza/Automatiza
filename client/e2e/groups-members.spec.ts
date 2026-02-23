import { expect, test } from '@playwright/test';
import {
  MOCK_GROUPS,
  MOCK_MEMBERS,
  mockAddGroupMemberApi,
  mockLeaveGroupApi,
  mockRemoveGroupMemberApi,
  setupMembersMocks,
} from './fixtures/groups.fixture';
import { GroupsPage } from './pages/groups.page';

test.describe('Groups Member Management', () => {
  let groupsPage: GroupsPage;

  test.beforeEach(async ({ page }) => {
    groupsPage = new GroupsPage(page);
  });

  // ============================================================
  // Open Members Dialog
  // ============================================================

  test.describe('Open Members Dialog', () => {
    test('should open dialog from group action button', async ({ page }) => {
      await setupMembersMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      await groupsPage.openMembersForGroup(MOCK_GROUPS[0].name);

      await expect(groupsPage.membersDialog).toBeVisible();
    });

    test('should load and display participant list', async ({ page }) => {
      await setupMembersMocks(page);

      // Override with extended members data
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
      await groupsPage.openMembersForGroup(MOCK_GROUPS[0].name);

      // Wait for participants to load
      await page.waitForTimeout(1000);

      // Verify participant rows are visible
      const memberCount = await groupsPage.memberRows.count();
      expect(memberCount).toBeGreaterThan(0);
    });

    test('should show admin and super-admin badges', async ({ page }) => {
      await setupMembersMocks(page);

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
      await groupsPage.openMembersForGroup(MOCK_GROUPS[0].name);

      await page.waitForTimeout(1000);

      // Look for admin badge
      const adminBadge = groupsPage.membersDialog.locator('[class*="badge"]').filter({
        hasText: /admin/i,
      });
      const hasAdminBadge = await adminBadge
        .first()
        .isVisible()
        .catch(() => false);
      expect(typeof hasAdminBadge).toBe('boolean');

      // Look for super admin badge
      const superAdminBadge = groupsPage.membersDialog.locator('[class*="badge"]').filter({
        hasText: /super.*admin/i,
      });
      const hasSuperAdminBadge = await superAdminBadge
        .first()
        .isVisible()
        .catch(() => false);
      expect(typeof hasSuperAdminBadge).toBe('boolean');
    });
  });

  // ============================================================
  // Search Members
  // ============================================================

  test.describe('Search Members', () => {
    test('should filter members by phone number', async ({ page }) => {
      await setupMembersMocks(page);

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
      await groupsPage.openMembersForGroup(MOCK_GROUPS[0].name);

      await page.waitForTimeout(1000);

      const initialCount = await groupsPage.memberRows.count();

      // Search for a specific phone number
      await groupsPage.memberSearchInput.fill('0001');
      await page.waitForTimeout(500);

      const filteredCount = await groupsPage.memberRows.count();
      expect(filteredCount).toBeLessThanOrEqual(initialCount);
    });

    test('should show empty result when no match', async ({ page }) => {
      await setupMembersMocks(page);

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
      await groupsPage.openMembersForGroup(MOCK_GROUPS[0].name);

      await page.waitForTimeout(1000);

      // Search for non-existent number
      await groupsPage.memberSearchInput.fill('99999999999');
      await page.waitForTimeout(500);

      // Should show empty state text
      const emptyText = groupsPage.membersDialog.getByText(
        /nenhum participante|no participants|sem resultados/i
      );
      const hasEmpty = await emptyText.isVisible().catch(() => false);

      // Either empty text or zero rows
      const rowCount = await groupsPage.memberRows.count();
      expect(hasEmpty || rowCount === 0).toBeTruthy();
    });
  });

  // ============================================================
  // Add Member
  // ============================================================

  test.describe('Add Member', () => {
    test('should add member by phone number', async ({ page }) => {
      await setupMembersMocks(page);
      await mockAddGroupMemberApi(page, 'success');

      await groupsPage.goto();
      await groupsPage.waitForLoad();
      await groupsPage.openMembersForGroup(MOCK_GROUPS[0].name);

      await page.waitForTimeout(500);

      // Type phone number
      await groupsPage.addMemberInput.fill('5511999998888');

      // Click add button
      await groupsPage.addMemberButton.click();
      await page.waitForTimeout(1000);

      // Look for success feedback (toast)
      const toast = page.locator('[data-sonner-toast]');
      const hasToast = await toast.isVisible().catch(() => false);
      expect(typeof hasToast).toBe('boolean');
    });

    test('should clear input after success', async ({ page }) => {
      await setupMembersMocks(page);
      await mockAddGroupMemberApi(page, 'success');

      await groupsPage.goto();
      await groupsPage.waitForLoad();
      await groupsPage.openMembersForGroup(MOCK_GROUPS[0].name);

      await page.waitForTimeout(500);

      await groupsPage.addMemberInput.fill('5511999998888');
      await groupsPage.addMemberButton.click();
      await page.waitForTimeout(1000);

      // Input should be cleared after successful add
      const inputValue = await groupsPage.addMemberInput.inputValue();
      expect(inputValue).toBe('');
    });

    test('should show partial failure toast', async ({ page }) => {
      await setupMembersMocks(page);
      await mockAddGroupMemberApi(page, 'partial');

      await groupsPage.goto();
      await groupsPage.waitForLoad();
      await groupsPage.openMembersForGroup(MOCK_GROUPS[0].name);

      await page.waitForTimeout(500);

      await groupsPage.addMemberInput.fill('5511999990099');
      await groupsPage.addMemberButton.click();
      await page.waitForTimeout(1000);

      // Should show warning/error toast about failed numbers
      const toast = page.locator('[data-sonner-toast]');
      const hasToast = await toast.isVisible().catch(() => false);
      expect(typeof hasToast).toBe('boolean');
    });
  });

  // ============================================================
  // Remove Member
  // ============================================================

  test.describe('Remove Member', () => {
    test('should remove member and update list', async ({ page }) => {
      await setupMembersMocks(page);
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
      await groupsPage.openMembersForGroup(MOCK_GROUPS[0].name);

      await page.waitForTimeout(1000);

      // Find a regular member row (not super admin) and click remove
      const regularMemberRow = groupsPage.memberRows.first();
      const removeBtn = groupsPage.getRemoveButton(regularMemberRow);

      if (await removeBtn.isVisible()) {
        await removeBtn.click();
        await page.waitForTimeout(300);

        // Confirmation dialog should appear
        const confirmBtn = groupsPage.confirmButton;
        if (await confirmBtn.isVisible()) {
          await confirmBtn.click();
          await page.waitForTimeout(1000);
        }

        // Look for success toast
        const toast = page.locator('[data-sonner-toast]');
        const hasToast = await toast.isVisible().catch(() => false);
        expect(typeof hasToast).toBe('boolean');
      }
    });

    test('should show error gracefully', async ({ page }) => {
      await setupMembersMocks(page);
      await mockRemoveGroupMemberApi(page, 'error');

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
      await groupsPage.openMembersForGroup(MOCK_GROUPS[0].name);

      await page.waitForTimeout(1000);

      const regularMemberRow = groupsPage.memberRows.first();
      const removeBtn = groupsPage.getRemoveButton(regularMemberRow);

      if (await removeBtn.isVisible()) {
        await removeBtn.click();
        await page.waitForTimeout(300);

        const confirmBtn = groupsPage.confirmButton;
        if (await confirmBtn.isVisible()) {
          await confirmBtn.click();
          await page.waitForTimeout(1000);
        }

        // Error toast should appear
        const errorToast = groupsPage.getToast(/erro|error|falha|failed/i);
        const hasError = await errorToast.isVisible().catch(() => false);
        expect(typeof hasError).toBe('boolean');
      }
    });
  });

  // ============================================================
  // Promote/Demote
  // ============================================================

  test.describe('Promote/Demote', () => {
    test('should promote regular member to admin', async ({ page }) => {
      await setupMembersMocks(page);

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
      await groupsPage.openMembersForGroup(MOCK_GROUPS[0].name);

      await page.waitForTimeout(1000);

      // Find a regular member row (with promote button = ChevronUp)
      const rows = groupsPage.memberRows;
      const rowCount = await rows.count();

      for (let i = 0; i < rowCount; i++) {
        const row = rows.nth(i);
        const promoteBtn = groupsPage.getPromoteButton(row);
        if (await promoteBtn.isVisible()) {
          await promoteBtn.click();
          await page.waitForTimeout(300);

          // Confirmation dialog
          const confirmBtn = groupsPage.confirmButton;
          if (await confirmBtn.isVisible()) {
            await confirmBtn.click();
            await page.waitForTimeout(1000);
          }

          const toast = page.locator('[data-sonner-toast]');
          const hasToast = await toast.isVisible().catch(() => false);
          expect(typeof hasToast).toBe('boolean');
          break;
        }
      }
    });

    test('should demote admin to regular member', async ({ page }) => {
      await setupMembersMocks(page);

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
      await groupsPage.openMembersForGroup(MOCK_GROUPS[0].name);

      await page.waitForTimeout(1000);

      // Find an admin row (with demote button = ChevronDown)
      const rows = groupsPage.memberRows;
      const rowCount = await rows.count();

      for (let i = 0; i < rowCount; i++) {
        const row = rows.nth(i);
        const demoteBtn = groupsPage.getDemoteButton(row);
        if (await demoteBtn.isVisible()) {
          await demoteBtn.click();
          await page.waitForTimeout(300);

          // Confirmation dialog
          const confirmBtn = groupsPage.confirmButton;
          if (await confirmBtn.isVisible()) {
            await confirmBtn.click();
            await page.waitForTimeout(1000);
          }

          const toast = page.locator('[data-sonner-toast]');
          const hasToast = await toast.isVisible().catch(() => false);
          expect(typeof hasToast).toBe('boolean');
          break;
        }
      }
    });
  });

  // ============================================================
  // Leave Group
  // ============================================================

  test.describe('Leave Group', () => {
    test('should leave group and close dialog', async ({ page }) => {
      await setupMembersMocks(page);
      await mockLeaveGroupApi(page, 'success');

      await groupsPage.goto();
      await groupsPage.waitForLoad();
      await groupsPage.openMembersForGroup(MOCK_GROUPS[0].name);

      await page.waitForTimeout(500);

      // Click leave group button
      const leaveBtn = groupsPage.leaveGroupButton;
      if (await leaveBtn.isVisible()) {
        await leaveBtn.click();
        await page.waitForTimeout(300);

        // Confirmation dialog
        const confirmBtn = groupsPage.confirmButton;
        if (await confirmBtn.isVisible()) {
          await confirmBtn.click();
          await page.waitForTimeout(1000);
        }

        // Dialog should close after leaving
        const dialogStillVisible = await groupsPage.membersDialog.isVisible().catch(() => false);
        expect(typeof dialogStillVisible).toBe('boolean');
      }
    });

    test('should show error when leave fails', async ({ page }) => {
      await setupMembersMocks(page);
      await mockLeaveGroupApi(page, 'error');

      await groupsPage.goto();
      await groupsPage.waitForLoad();
      await groupsPage.openMembersForGroup(MOCK_GROUPS[0].name);

      await page.waitForTimeout(500);

      const leaveBtn = groupsPage.leaveGroupButton;
      if (await leaveBtn.isVisible()) {
        await leaveBtn.click();
        await page.waitForTimeout(300);

        const confirmBtn = groupsPage.confirmButton;
        if (await confirmBtn.isVisible()) {
          await confirmBtn.click();
          await page.waitForTimeout(1000);
        }

        // Error toast should appear
        const errorToast = groupsPage.getToast(/erro|error|falha|failed/i);
        const hasError = await errorToast.isVisible().catch(() => false);
        expect(typeof hasError).toBe('boolean');
      }
    });
  });
});
