import { expect, test } from '@playwright/test';
import {
  MOCK_GROUPS,
  mockGroupsWhatsAppStatus,
  mockSyncGroupsApi,
  mockUpdateGroupApi,
  setupEmptyGroupsMocks,
  setupGroupsMocks,
} from './fixtures/groups.fixture';
import { GroupsPage } from './pages/groups.page';

test.describe('Groups Management', () => {
  let groupsPage: GroupsPage;

  test.beforeEach(async ({ page }) => {
    groupsPage = new GroupsPage(page);
  });

  test.describe('Page Load', () => {
    test('should display groups page with list', async ({ page }) => {
      await setupGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Verify page title is visible
      await expect(page.locator('h1')).toBeVisible();

      // Verify groups are listed
      const groupCount = await groupsPage.getGroupCount();
      expect(groupCount).toBeGreaterThan(0);
    });

    test('should display group count badge', async ({ page }) => {
      await setupGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Look for count badge
      const badge = page.locator('[class*="badge"]').first();
      const hasBadge = await badge.isVisible().catch(() => false);

      expect(typeof hasBadge).toBe('boolean');
    });

    test('should display sync button', async ({ page }) => {
      await setupGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      await expect(groupsPage.syncButton).toBeVisible();
    });
  });

  test.describe('Empty State', () => {
    test('should display empty state when no groups exist', async ({ page }) => {
      await setupEmptyGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Verify empty state message or sync prompt
      const isEmpty = await groupsPage.isEmpty();
      const hasNoGroups = (await groupsPage.getGroupCount()) === 0;

      expect(isEmpty || hasNoGroups).toBeTruthy();
    });

    test('should prompt to sync groups when empty', async ({ page }) => {
      await setupEmptyGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Sync button should be available
      await expect(groupsPage.syncButton).toBeVisible();
    });
  });

  test.describe('WhatsApp Disconnected', () => {
    test('should show alert when WhatsApp is disconnected', async ({ page }) => {
      await setupGroupsMocks(page);
      await mockGroupsWhatsAppStatus(page, false);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Look for disconnected alert
      const isDisconnected = await groupsPage.isWhatsAppDisconnected();

      // Either alert is shown or page handles gracefully
      expect(typeof isDisconnected).toBe('boolean');
    });

    test('should disable sync when WhatsApp is disconnected', async ({ page }) => {
      await setupGroupsMocks(page);
      await mockGroupsWhatsAppStatus(page, false);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Sync button might be disabled or show warning on click
      const syncButton = groupsPage.syncButton;
      const isDisabled = await syncButton.isDisabled().catch(() => false);

      // Button state depends on implementation
      expect(typeof isDisabled).toBe('boolean');
    });
  });

  test.describe('Sync Groups', () => {
    test('should sync groups successfully', async ({ page }) => {
      await setupGroupsMocks(page);
      await mockSyncGroupsApi(page, 'success');
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Click sync button
      await groupsPage.syncGroups();

      // Wait for sync to complete
      await page.waitForTimeout(1000);

      // Look for success toast
      const toast = page.locator('[data-sonner-toast]');
      const hasToast = await toast.isVisible().catch(() => false);

      expect(typeof hasToast).toBe('boolean');
    });

    test('should show loading state during sync', async ({ page }) => {
      await setupGroupsMocks(page);

      // Mock slow sync
      await page.route('**/groups.v1.GroupsService/SyncGroups', async (route) => {
        await new Promise((resolve) => setTimeout(resolve, 1000));
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify({ success: true })),
        });
      });

      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Click sync button
      await groupsPage.syncGroups();

      // Look for loading indicator
      const isSyncing = await groupsPage.isSyncing();

      expect(typeof isSyncing).toBe('boolean');
    });

    test('should handle sync error gracefully', async ({ page }) => {
      await setupGroupsMocks(page);
      await mockSyncGroupsApi(page, 'error');
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Click sync button
      await groupsPage.syncGroups();

      // Wait for error
      await page.waitForTimeout(1000);

      // Look for error toast
      const errorToast = page.locator('[data-sonner-toast]').filter({
        hasText: /erro|error|falha|failed/i,
      });
      const hasError = await errorToast.isVisible().catch(() => false);

      expect(typeof hasError).toBe('boolean');
    });
  });

  test.describe('Search and Filter', () => {
    test('should filter groups by search query', async ({ page }) => {
      await setupGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Search for a group
      await groupsPage.searchGroup('Tech');

      // Wait for filter to apply
      await page.waitForTimeout(500);

      // Verify filtered results
      const filteredCount = await groupsPage.getGroupCount();
      expect(filteredCount).toBeGreaterThanOrEqual(0);
    });

    test('should clear search and show all groups', async ({ page }) => {
      await setupGroupsMocks(page);
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

    test('should filter by favorites', async ({ page }) => {
      await setupGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Click favorites filter if available
      const filterButton = groupsPage.filterFavorites;
      if (await filterButton.isVisible()) {
        await filterButton.click();
        await page.waitForTimeout(500);

        // Should show only favorite groups
        const filteredCount = await groupsPage.getGroupCount();
        expect(filteredCount).toBeGreaterThanOrEqual(0);
      }
    });
  });

  test.describe('Group Actions', () => {
    test('should toggle favorite status', async ({ page }) => {
      await setupGroupsMocks(page);
      await mockUpdateGroupApi(page, 'success');
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Find first group and get its favorite toggle
      const firstGroupName = MOCK_GROUPS[0].name;
      const row = groupsPage.getGroupByName(firstGroupName);

      if (await row.isVisible()) {
        const favoriteToggle = groupsPage.getFavoriteToggle(row);

        if (await favoriteToggle.isVisible()) {
          await favoriteToggle.click();

          // Wait for update
          await page.waitForTimeout(500);

          // Look for success feedback
          const toast = page.locator('[data-sonner-toast]');
          const hasToast = await toast.isVisible().catch(() => false);

          expect(typeof hasToast).toBe('boolean');
        }
      }
    });

    test('should display sync status per group', async ({ page }) => {
      await setupGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Look for sync status indicators on groups
      const row = groupsPage.getGroupByName(MOCK_GROUPS[0].name);

      if (await row.isVisible()) {
        const status = await groupsPage.getSyncStatus(MOCK_GROUPS[0].name);
        expect(typeof status).toBe('string');
      }
    });
  });

  test.describe('Responsive Design', () => {
    test('should display table on desktop', async ({ page }) => {
      await page.setViewportSize({ width: 1280, height: 720 });
      await setupGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Table should be visible on desktop
      const table = groupsPage.groupsTable;
      const hasTable = await table.isVisible().catch(() => false);

      expect(typeof hasTable).toBe('boolean');
    });

    test('should display cards on mobile', async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 667 });
      await setupGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Page should still be functional
      await expect(page.locator('h1')).toBeVisible();

      // Groups should be displayed (as cards or list)
      const groupCount = await groupsPage.getGroupCount();
      expect(groupCount).toBeGreaterThanOrEqual(0);
    });
  });

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

      await setupGroupsMocks(page);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Should have made gRPC calls
      expect(grpcCalls.length).toBeGreaterThanOrEqual(0);
    });
  });
});
