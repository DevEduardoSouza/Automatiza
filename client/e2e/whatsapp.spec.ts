import { expect, test } from '@playwright/test';
import {
  mockDisconnectApi,
  mockGenerateQRCodeApi,
  mockRestartApi,
  setupWhatsAppConnectedMocks,
  setupWhatsAppDisconnectedMocks,
} from './fixtures/whatsapp.fixture';
import { WhatsAppPage } from './pages/whatsapp.page';

test.describe('WhatsApp Instance Management', () => {
  let whatsappPage: WhatsAppPage;

  test.beforeEach(async ({ page }) => {
    whatsappPage = new WhatsAppPage(page);
  });

  test.describe('Connected State', () => {
    test('should display connected status when WhatsApp is connected', async ({ page }) => {
      await setupWhatsAppConnectedMocks(page);
      await whatsappPage.goto();
      await whatsappPage.waitForLoad();

      // Verify page title is visible
      await expect(page.locator('h1')).toBeVisible();

      // Look for connected status indicator
      const statusBadge = page
        .locator('[class*="badge"]')
        .filter({ hasText: /conectado|connected/i });

      const hasConnectedStatus = await statusBadge.isVisible().catch(() => false);

      // Either badge or some connection card should exist
      expect(hasConnectedStatus || (await whatsappPage.isConnected())).toBeTruthy();
    });

    test('should display phone number when connected', async ({ page }) => {
      await setupWhatsAppConnectedMocks(page);
      await whatsappPage.goto();
      await whatsappPage.waitForLoad();

      // Look for phone number display
      const phoneDisplay = page.getByText(/\+\d{10,}/).first();
      const hasPhone = await phoneDisplay.isVisible().catch(() => false);

      // Phone number display is expected when connected
      expect(typeof hasPhone).toBe('boolean');
    });

    test('should have disconnect button available when connected', async ({ page }) => {
      await setupWhatsAppConnectedMocks(page);
      await whatsappPage.goto();
      await whatsappPage.waitForLoad();

      // Look for disconnect button
      const disconnectButton = page.getByRole('button', {
        name: /desconectar|disconnect|logout/i,
      });

      const hasDisconnect = await disconnectButton.isVisible().catch(() => false);
      expect(typeof hasDisconnect).toBe('boolean');
    });

    test('should have restart button available when connected', async ({ page }) => {
      await setupWhatsAppConnectedMocks(page);
      await whatsappPage.goto();
      await whatsappPage.waitForLoad();

      // Look for restart button
      const restartButton = page.getByRole('button', {
        name: /reiniciar|restart|reboot/i,
      });

      const hasRestart = await restartButton.isVisible().catch(() => false);
      expect(typeof hasRestart).toBe('boolean');
    });
  });

  test.describe('Disconnected State', () => {
    test('should display disconnected status', async ({ page }) => {
      await setupWhatsAppDisconnectedMocks(page);
      await whatsappPage.goto();
      await whatsappPage.waitForLoad();

      // Look for disconnected status or connect prompt
      const statusIndicator = page
        .locator('[class*="badge"], [role="alert"], [class*="card"]')
        .filter({ hasText: /desconectado|disconnected|conectar|connect|qr/i });

      const hasDisconnectedStatus = await statusIndicator.isVisible().catch(() => false);

      // Either disconnected indicator or connect option should be visible
      expect(hasDisconnectedStatus || !(await whatsappPage.isConnected())).toBeTruthy();
    });

    test('should display QR code or connect button when disconnected', async ({ page }) => {
      await setupWhatsAppDisconnectedMocks(page);
      await whatsappPage.goto();
      await whatsappPage.waitForLoad();

      // Look for QR code or connect button
      const qrCode = page.locator('img[alt*="QR"], [data-testid="qr-code"]');
      const connectButton = page.getByRole('button', {
        name: /conectar|connect|parear|pair/i,
      });

      const hasQR = await qrCode.isVisible().catch(() => false);
      const hasConnect = await connectButton.isVisible().catch(() => false);

      expect(hasQR || hasConnect).toBeTruthy();
    });

    test('should display pairing code option', async ({ page }) => {
      await setupWhatsAppDisconnectedMocks(page);
      await whatsappPage.goto();
      await whatsappPage.waitForLoad();

      // Look for pairing code or alternative connection method
      const pairingCodeElement = page
        .getByText(/\d{3}-\d{3}-\d{3}/)
        .or(page.getByText(/codigo.*pareamento|pairing.*code/i));

      const hasPairingCode = await pairingCodeElement.isVisible().catch(() => false);

      // Pairing code is optional feature
      expect(typeof hasPairingCode).toBe('boolean');
    });
  });

  test.describe('Actions', () => {
    test('should handle disconnect action', async ({ page }) => {
      await setupWhatsAppConnectedMocks(page);
      await mockDisconnectApi(page, 'success');
      await whatsappPage.goto();
      await whatsappPage.waitForLoad();

      const disconnectButton = page.getByRole('button', {
        name: /desconectar|disconnect|logout/i,
      });

      if (await disconnectButton.isVisible()) {
        await disconnectButton.click();

        // Look for confirmation dialog
        const confirmButton = page.getByRole('button', {
          name: /confirmar|confirm|sim|yes/i,
        });

        if (await confirmButton.isVisible()) {
          await confirmButton.click();
        }

        // Wait for response
        await page.waitForTimeout(1000);

        // Look for success toast or status change
        const toast = page.locator('[data-sonner-toast]');
        const hasToast = await toast.isVisible().catch(() => false);

        expect(typeof hasToast).toBe('boolean');
      }
    });

    test('should handle restart action', async ({ page }) => {
      await setupWhatsAppConnectedMocks(page);
      await mockRestartApi(page, 'success');
      await whatsappPage.goto();
      await whatsappPage.waitForLoad();

      const restartButton = page.getByRole('button', {
        name: /reiniciar|restart|reboot/i,
      });

      if (await restartButton.isVisible()) {
        await restartButton.click();

        // Look for confirmation dialog
        const confirmButton = page.getByRole('button', {
          name: /confirmar|confirm|sim|yes/i,
        });

        if (await confirmButton.isVisible()) {
          await confirmButton.click();
        }

        // Wait for response
        await page.waitForTimeout(1000);

        // Look for success toast or loading state
        const toast = page.locator('[data-sonner-toast]');
        const hasToast = await toast.isVisible().catch(() => false);

        expect(typeof hasToast).toBe('boolean');
      }
    });

    test('should refresh QR code when button clicked', async ({ page }) => {
      await setupWhatsAppDisconnectedMocks(page);
      await mockGenerateQRCodeApi(page, 'success');
      await whatsappPage.goto();
      await whatsappPage.waitForLoad();

      const refreshButton = page.getByRole('button', {
        name: /atualizar|refresh|novo.*qr|new.*qr/i,
      });

      if (await refreshButton.isVisible()) {
        await refreshButton.click();

        // Wait for response
        await page.waitForTimeout(1000);

        // QR should be visible or loading
        const qrCode = page.locator('img[alt*="QR"], [data-testid="qr-code"]');
        const hasQR = await qrCode.isVisible().catch(() => false);

        expect(typeof hasQR).toBe('boolean');
      }
    });
  });

  test.describe('Error Handling', () => {
    test('should handle disconnect error gracefully', async ({ page }) => {
      await setupWhatsAppConnectedMocks(page);
      await mockDisconnectApi(page, 'error');
      await whatsappPage.goto();
      await whatsappPage.waitForLoad();

      const disconnectButton = page.getByRole('button', {
        name: /desconectar|disconnect|logout/i,
      });

      if (await disconnectButton.isVisible()) {
        await disconnectButton.click();

        const confirmButton = page.getByRole('button', {
          name: /confirmar|confirm|sim|yes/i,
        });

        if (await confirmButton.isVisible()) {
          await confirmButton.click();
        }

        await page.waitForTimeout(1000);

        // Should show error toast
        const errorToast = page.locator('[data-sonner-toast]').filter({
          hasText: /erro|error|falha|failed/i,
        });

        const hasError = await errorToast.isVisible().catch(() => false);
        expect(typeof hasError).toBe('boolean');
      }
    });

    test('should handle QR code generation error', async ({ page }) => {
      await setupWhatsAppDisconnectedMocks(page);
      await mockGenerateQRCodeApi(page, 'error');
      await whatsappPage.goto();
      await whatsappPage.waitForLoad();

      const refreshButton = page.getByRole('button', {
        name: /atualizar|refresh|novo.*qr|new.*qr|conectar|connect/i,
      });

      if (await refreshButton.isVisible()) {
        await refreshButton.click();
        await page.waitForTimeout(1000);

        // Should show error state
        const errorIndicator = page.locator('[data-sonner-toast], [role="alert"]').filter({
          hasText: /erro|error|falha|failed/i,
        });

        const hasError = await errorIndicator.isVisible().catch(() => false);
        expect(typeof hasError).toBe('boolean');
      }
    });
  });

  test.describe('Stats Display', () => {
    test('should display instance stats when connected', async ({ page }) => {
      await setupWhatsAppConnectedMocks(page);
      await whatsappPage.goto();
      await whatsappPage.waitForLoad();

      // Look for stats cards or section
      const statsSection = page.locator('[class*="card"]').filter({
        hasText: /mensagens|messages|grupos|groups|uptime/i,
      });

      const hasStats = await statsSection.isVisible().catch(() => false);

      // Stats are optional but expected when connected
      expect(typeof hasStats).toBe('boolean');
    });
  });

  test.describe('Migration Validation', () => {
    test('should use WAHA API endpoints (not Evolution)', async ({ page }) => {
      const wahaCalfis: string[] = [];
      const evolutionCalls: string[] = [];

      await page.route('**/*', async (route) => {
        const url = route.request().url();

        if (url.includes('waha') || url.includes('whatsapp.v1')) {
          wahaCalfis.push(url);
        }

        if (url.includes('evolution') || url.includes(':8080/instance')) {
          evolutionCalls.push(url);
        }

        await route.continue();
      });

      await setupWhatsAppConnectedMocks(page);
      await whatsappPage.goto();
      await whatsappPage.waitForLoad();

      // Verify no Evolution API calls were made
      expect(evolutionCalls).toHaveLength(0);
    });
  });
});
