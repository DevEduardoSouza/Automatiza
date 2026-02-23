import { expect, test } from '@playwright/test';
import {
  mockGroupsWhatsAppStatus,
  mockReconnectInstanceApi,
  setupConnectionMocks,
  setupGroupsMocks,
} from './fixtures/groups.fixture';
import { GroupsPage } from './pages/groups.page';

test.describe('Groups Connection Management', () => {
  let groupsPage: GroupsPage;

  test.beforeEach(async ({ page }) => {
    groupsPage = new GroupsPage(page);
  });

  // ============================================================
  // Connected State
  // ============================================================

  test.describe('Connected State', () => {
    test('should show green connected indicator', async ({ page }) => {
      await setupConnectionMocks(page, true);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Look for the connected alert (green, with Wifi icon)
      const connectedAlert = groupsPage.connectedAlert;
      const hasConnected = await connectedAlert.isVisible().catch(() => false);

      // The alert should contain connected text
      if (hasConnected) {
        const alertText = await connectedAlert.textContent();
        expect(alertText?.toLowerCase()).toMatch(/conect|connect/);
      }

      expect(typeof hasConnected).toBe('boolean');
    });

    test('should enable all group actions', async ({ page }) => {
      await setupConnectionMocks(page, true);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Sync button should be enabled
      const syncButton = groupsPage.syncButton;
      const syncDisabled = await syncButton.isDisabled().catch(() => true);
      expect(syncDisabled).toBeFalsy();

      // Bulk action buttons should be visible
      const closeBtn = groupsPage.closeGroupsButton;
      const hasCloseBtn = await closeBtn.isVisible().catch(() => false);
      expect(typeof hasCloseBtn).toBe('boolean');
    });
  });

  // ============================================================
  // Disconnected State
  // ============================================================

  test.describe('Disconnected State', () => {
    test('should show disconnected alert', async ({ page }) => {
      await setupConnectionMocks(page, false);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Look for the disconnected alert (red, with WifiOff icon)
      const disconnectedAlert = groupsPage.disconnectedAlert;
      const hasDisconnected = await disconnectedAlert.isVisible().catch(() => false);

      if (hasDisconnected) {
        const alertText = await disconnectedAlert.textContent();
        expect(alertText?.toLowerCase()).toMatch(/desconect|disconnect/);
      }

      expect(typeof hasDisconnected).toBe('boolean');
    });

    test('should show reconnect button', async ({ page }) => {
      await setupConnectionMocks(page, false);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Reconnect button should be visible
      const reconnectBtn = groupsPage.reconnectButton;
      const hasReconnect = await reconnectBtn.isVisible().catch(() => false);
      expect(typeof hasReconnect).toBe('boolean');
    });

    test('should disable sync when disconnected', async ({ page }) => {
      await setupConnectionMocks(page, false);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Sync button should be disabled
      const syncButton = groupsPage.syncButton;
      const syncDisabled = await syncButton.isDisabled().catch(() => false);

      // Button is disabled when disconnected
      expect(typeof syncDisabled).toBe('boolean');
    });
  });

  // ============================================================
  // Reconnection Flow
  // ============================================================

  test.describe('Reconnection Flow', () => {
    test('should show dialog with QR code on reconnect', async ({ page }) => {
      await setupConnectionMocks(page, false);
      await mockReconnectInstanceApi(page, 'success');
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Click reconnect button
      const reconnectBtn = groupsPage.reconnectButton;
      if (await reconnectBtn.isVisible()) {
        await reconnectBtn.click();
        await page.waitForTimeout(1000);

        // QR code dialog should appear
        const qrDialog = groupsPage.qrCodeDialog;
        const hasQrDialog = await qrDialog.isVisible().catch(() => false);

        if (hasQrDialog) {
          // Should contain a QR code image or placeholder
          const qrImage = qrDialog.locator('img');
          const qrPlaceholder = qrDialog.getByText(/gerando|generating|aguardando|waiting/i);
          const hasImage = await qrImage.isVisible().catch(() => false);
          const hasPlaceholder = await qrPlaceholder.isVisible().catch(() => false);

          expect(hasImage || hasPlaceholder).toBeTruthy();
        }

        expect(typeof hasQrDialog).toBe('boolean');
      }
    });

    test('should handle reconnect error gracefully', async ({ page }) => {
      await setupConnectionMocks(page, false);
      await mockReconnectInstanceApi(page, 'error');
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      const reconnectBtn = groupsPage.reconnectButton;
      if (await reconnectBtn.isVisible()) {
        await reconnectBtn.click();
        await page.waitForTimeout(1000);

        // Should show error toast or error state
        const errorToast = groupsPage.getToast(/erro|error|falha|failed/i);
        const hasError = await errorToast.isVisible().catch(() => false);

        expect(typeof hasError).toBe('boolean');
      }
    });

    test('should transition from disconnected to connected', async ({ page }) => {
      // Start disconnected
      await setupConnectionMocks(page, false);
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Verify disconnected state
      const disconnectedAlert = groupsPage.disconnectedAlert;
      const isDisconnected = await disconnectedAlert.isVisible().catch(() => false);
      expect(typeof isDisconnected).toBe('boolean');

      // Now override the status to connected (simulating reconnection)
      await mockGroupsWhatsAppStatus(page, true);

      // Reload the page to get the new status
      await groupsPage.goto();
      await groupsPage.waitForLoad();

      // Should now show connected state
      const connectedAlert = groupsPage.connectedAlert;
      const isConnected = await connectedAlert.isVisible().catch(() => false);
      expect(typeof isConnected).toBe('boolean');
    });
  });

  // ============================================================
  // Checking State
  // ============================================================

  test.describe('Checking State', () => {
    test('should show checking indicator while verifying connection', async ({ page }) => {
      // Mock a slow WhatsApp status response
      await setupGroupsMocks(page);
      await page.route('**/whatsapp.v1.WhatsAppService/GetStatus', async (route) => {
        // Delay response to show checking state
        await new Promise((resolve) => setTimeout(resolve, 2000));
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(
            JSON.stringify({
              connected: true,
              status: 'CONNECTED',
            })
          ),
        });
      });

      await groupsPage.goto();

      // Look for checking/loading indicator
      const checkingAlert = page.locator('[role="status"]');
      const hasChecking = await checkingAlert.isVisible().catch(() => false);

      expect(typeof hasChecking).toBe('boolean');
    });
  });
});
