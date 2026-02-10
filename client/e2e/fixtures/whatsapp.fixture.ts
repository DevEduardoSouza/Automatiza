import type { Page } from '@playwright/test';

/**
 * Mock WhatsApp instance data.
 */
export const MOCK_WHATSAPP_INSTANCE = {
  id: 'instance-1',
  name: 'Main Instance',
  status: 'CONNECTED',
  phoneNumber: '+5511999999999',
  profileName: 'Automatiza Bot',
  profilePictureUrl: null,
  lastSeen: new Date().toISOString(),
  engine: 'GOWS',
};

/**
 * Mock disconnected instance.
 */
export const MOCK_DISCONNECTED_INSTANCE = {
  id: 'instance-1',
  name: 'Main Instance',
  status: 'DISCONNECTED',
  phoneNumber: null,
  profileName: null,
  profilePictureUrl: null,
  lastSeen: null,
  engine: 'GOWS',
};

/**
 * Mock QR code for pairing.
 */
export const MOCK_QR_CODE = {
  qrCode: 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUg...',
  expiresAt: new Date(Date.now() + 60000).toISOString(),
  pairingCode: '123-456-789',
};

/**
 * Mock instance stats.
 */
export const MOCK_INSTANCE_STATS = {
  messagesSentToday: 45,
  messagesReceivedToday: 120,
  groupsCount: 12,
  contactsCount: 250,
  uptime: 86400, // 24 hours in seconds
};

/**
 * Mocks the WhatsApp status API.
 */
export async function mockWhatsAppStatusApi(
  page: Page,
  instance = MOCK_WHATSAPP_INSTANCE
): Promise<void> {
  await page.route('**/whatsapp.v1.WhatsAppService/GetStatus', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(
        JSON.stringify({
          connected: instance.status === 'CONNECTED',
          status: instance.status,
          phoneNumber: instance.phoneNumber,
          profileName: instance.profileName,
        })
      ),
    });
  });
}

/**
 * Mocks the WhatsApp instance details API.
 */
export async function mockWhatsAppInstanceApi(
  page: Page,
  instance = MOCK_WHATSAPP_INSTANCE
): Promise<void> {
  await page.route('**/whatsapp.v1.WhatsAppService/GetInstance', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify({ instance })),
    });
  });
}

/**
 * Mocks the QR code generation API.
 */
export async function mockGenerateQRCodeApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/whatsapp.v1.WhatsAppService/GenerateQRCode', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify(MOCK_QR_CODE)),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to generate QR code',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the pairing code API.
 */
export async function mockPairingCodeApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/whatsapp.v1.WhatsAppService/GetPairingCode', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify({ pairingCode: '123-456-789' })),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to get pairing code',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the disconnect API.
 */
export async function mockDisconnectApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/whatsapp.v1.WhatsAppService/Disconnect', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify({ success: true })),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to disconnect',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the restart API.
 */
export async function mockRestartApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/whatsapp.v1.WhatsAppService/Restart', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify({ success: true })),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to restart instance',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the instance stats API.
 */
export async function mockInstanceStatsApi(page: Page, stats = MOCK_INSTANCE_STATS): Promise<void> {
  await page.route('**/whatsapp.v1.WhatsAppService/GetInstanceStats', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify(stats)),
    });
  });
}

/**
 * Sets up all common mocks for connected WhatsApp instance.
 */
export async function setupWhatsAppConnectedMocks(page: Page): Promise<void> {
  await mockWhatsAppStatusApi(page, MOCK_WHATSAPP_INSTANCE);
  await mockWhatsAppInstanceApi(page, MOCK_WHATSAPP_INSTANCE);
  await mockInstanceStatsApi(page);
  await mockDisconnectApi(page);
  await mockRestartApi(page);
}

/**
 * Sets up all common mocks for disconnected WhatsApp instance.
 */
export async function setupWhatsAppDisconnectedMocks(page: Page): Promise<void> {
  await mockWhatsAppStatusApi(page, MOCK_DISCONNECTED_INSTANCE);
  await mockWhatsAppInstanceApi(page, MOCK_DISCONNECTED_INSTANCE);
  await mockGenerateQRCodeApi(page);
  await mockPairingCodeApi(page);
}
