import type { Locator, Page } from '@playwright/test';

/**
 * Page Object Model for the WhatsApp page.
 * Encapsulates all WhatsApp instance management interactions.
 */
export class WhatsAppPage {
  readonly page: Page;

  // Page elements
  readonly pageTitle: Locator;

  // Connection status
  readonly connectionStatusCard: Locator;
  readonly statusBadge: Locator;
  readonly phoneNumber: Locator;
  readonly profileName: Locator;

  // QR code elements
  readonly qrCodeCard: Locator;
  readonly qrCodeImage: Locator;
  readonly pairingCode: Locator;
  readonly refreshQRButton: Locator;
  readonly copyPairingCodeButton: Locator;

  // Action buttons
  readonly connectButton: Locator;
  readonly disconnectButton: Locator;
  readonly restartButton: Locator;
  readonly syncGroupsButton: Locator;

  // Stats section
  readonly statsCard: Locator;
  readonly messagesSentToday: Locator;
  readonly messagesReceivedToday: Locator;
  readonly groupsCount: Locator;
  readonly contactsCount: Locator;
  readonly uptimeDisplay: Locator;

  // Alerts and toasts
  readonly connectionAlert: Locator;
  readonly errorAlert: Locator;
  readonly successToast: Locator;

  // Confirmation dialog
  readonly confirmDialog: Locator;
  readonly confirmButton: Locator;
  readonly cancelButton: Locator;

  // Auto-sync toggle
  readonly autoSyncToggle: Locator;

  // Import progress
  readonly importProgressBar: Locator;

  // How it works section
  readonly howItWorksCard: Locator;

  // Last sync display
  readonly lastSyncDisplay: Locator;

  // Error boundary card
  readonly errorBoundaryCard: Locator;

  constructor(page: Page) {
    this.page = page;

    // Page elements
    this.pageTitle = page.locator('h1');

    // Connection status
    this.connectionStatusCard = page
      .locator('[class*="card"]')
      .filter({ hasText: /status|conexao|connection/i })
      .first();
    this.statusBadge = page
      .locator('[class*="badge"]')
      .filter({ hasText: /conectado|connected|desconectado|disconnected/i });
    this.phoneNumber = page.locator('[data-testid="phone-number"]').or(page.getByText(/\+\d{10,}/));
    this.profileName = page
      .locator('[data-testid="profile-name"]')
      .or(this.connectionStatusCard.locator('p, span').first());

    // QR code elements
    this.qrCodeCard = page.locator('[class*="card"]').filter({ hasText: /qr code|parear|pair/i });
    this.qrCodeImage = page.locator('img[alt*="QR"]').or(page.locator('[data-testid="qr-code"]'));
    this.pairingCode = page
      .locator('[data-testid="pairing-code"]')
      .or(page.getByText(/\d{3}-\d{3}-\d{3}/));
    this.refreshQRButton = page.getByRole('button', {
      name: /atualizar|refresh|novo qr|new qr/i,
    });
    this.copyPairingCodeButton = page.getByRole('button', {
      name: /copiar.*codigo|copy.*code/i,
    });

    // Action buttons
    this.connectButton = page.getByRole('button', {
      name: /conectar|connect|parear|pair/i,
    });
    this.disconnectButton = page.getByRole('button', {
      name: /desconectar|disconnect|logout/i,
    });
    this.restartButton = page.getByRole('button', {
      name: /reiniciar|restart|reboot/i,
    });
    this.syncGroupsButton = page.getByRole('button', {
      name: /sincronizar|sync.*grupos|sync.*groups/i,
    });

    // Stats section
    this.statsCard = page
      .locator('[class*="card"]')
      .filter({ hasText: /estatisticas|statistics|stats/i });
    this.messagesSentToday = page
      .locator('[data-testid="messages-sent"]')
      .or(this.statsCard.locator('span').filter({ hasText: /enviadas|sent/i }));
    this.messagesReceivedToday = page
      .locator('[data-testid="messages-received"]')
      .or(this.statsCard.locator('span').filter({ hasText: /recebidas|received/i }));
    this.groupsCount = page.locator('[data-testid="groups-count"]');
    this.contactsCount = page.locator('[data-testid="contacts-count"]');
    this.uptimeDisplay = page
      .locator('[data-testid="uptime"]')
      .or(this.statsCard.locator('span').filter({ hasText: /uptime|tempo/i }));

    // Alerts and toasts
    this.connectionAlert = page.locator('[role="alert"]').filter({
      hasText: /whatsapp|conexao|connection/i,
    });
    this.errorAlert = page.locator('[role="alert"]').filter({
      hasText: /erro|error|falha|failed/i,
    });
    this.successToast = page.locator('[data-sonner-toast]');

    // Confirmation dialog
    this.confirmDialog = page
      .locator('[role="alertdialog"]')
      .or(page.locator('[role="dialog"]').filter({ hasText: /confirmar|confirm/i }));
    this.confirmButton = this.confirmDialog.getByRole('button', {
      name: /confirmar|confirm|sim|yes/i,
    });
    this.cancelButton = this.confirmDialog.getByRole('button', {
      name: /cancelar|cancel|nao|no/i,
    });

    // Auto-sync toggle
    this.autoSyncToggle = page.getByRole('button', {
      name: /ativar|ativado|activate|activated/i,
    });

    // Import progress
    this.importProgressBar = page.locator('[role="progressbar"]');

    // How it works section
    this.howItWorksCard = page.getByText(/como funciona\??/i);

    // Last sync display
    this.lastSyncDisplay = page.getByText(/último sync|last sync/i);

    // Error boundary card
    this.errorBoundaryCard = page.locator('[class*="card"]').filter({
      hasText: /erro na conexão whatsapp/i,
    });
  }

  /**
   * Navigates to the WhatsApp page.
   */
  async goto(): Promise<void> {
    await this.page.goto('/whatsapp');
    await this.page.waitForSelector('h1', { state: 'visible' });
  }

  /**
   * Waits for the page to load.
   */
  async waitForLoad(): Promise<void> {
    await this.page.waitForSelector('[class*="card"]', { state: 'visible' });
    // Wait for loading to complete
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
      .catch(() => {
        // Ignore timeout
      });
  }

  /**
   * Checks if WhatsApp is connected.
   */
  async isConnected(): Promise<boolean> {
    try {
      const badge = await this.statusBadge.textContent();
      return /conectado|connected/i.test(badge || '');
    } catch {
      return false;
    }
  }

  /**
   * Clicks the connect button and waits for QR code.
   */
  async startConnection(): Promise<void> {
    await this.connectButton.click();
    await this.qrCodeImage.waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});
  }

  /**
   * Disconnects WhatsApp instance.
   */
  async disconnect(): Promise<void> {
    await this.disconnectButton.click();
    if (await this.confirmDialog.isVisible()) {
      await this.confirmButton.click();
    }
  }

  /**
   * Restarts the WhatsApp instance.
   */
  async restart(): Promise<void> {
    await this.restartButton.click();
    if (await this.confirmDialog.isVisible()) {
      await this.confirmButton.click();
    }
  }

  /**
   * Syncs groups from WhatsApp.
   */
  async syncGroups(): Promise<void> {
    await this.syncGroupsButton.click();
  }

  /**
   * Refreshes the QR code.
   */
  async refreshQRCode(): Promise<void> {
    await this.refreshQRButton.click();
  }

  /**
   * Copies the pairing code.
   */
  async copyPairingCode(): Promise<void> {
    await this.copyPairingCodeButton.click();
  }

  /**
   * Gets the phone number displayed.
   */
  async getPhoneNumber(): Promise<string | null> {
    try {
      return await this.phoneNumber.textContent();
    } catch {
      return null;
    }
  }

  /**
   * Checks if QR code is visible.
   */
  async isQRCodeVisible(): Promise<boolean> {
    return this.qrCodeImage.isVisible();
  }

  /**
   * Checks if pairing code is visible.
   */
  async isPairingCodeVisible(): Promise<boolean> {
    return this.pairingCode.isVisible();
  }

  /**
   * Checks if there is an error displayed.
   */
  async hasError(): Promise<boolean> {
    return this.errorAlert.isVisible();
  }

  /**
   * Gets the connection status text.
   */
  async getStatusText(): Promise<string | null> {
    try {
      return await this.statusBadge.textContent();
    } catch {
      return null;
    }
  }
}
