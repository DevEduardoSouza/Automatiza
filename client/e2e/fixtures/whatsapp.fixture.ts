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

// ============================================================
// QR Code Flow Mocks (ConnectRPC endpoints used by useWhatsApp)
// ============================================================

/**
 * Mock QR code base64 data for CreateInstance response.
 */
export const MOCK_QR_BASE64 =
  'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';

/**
 * Mocks the CreateInstance ConnectRPC endpoint.
 */
export async function mockCreateInstanceApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/whatsapp.v1.WhatsAppService/CreateInstance', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({
          success: true,
          instanceName: 'e2e-session-001',
          friendlyName: 'E2E Test',
          qrcodeBase64: MOCK_QR_BASE64,
        }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to create instance',
        },
        body: JSON.stringify({}),
      });
    }
  });
}

/**
 * Mocks the DeleteInstance ConnectRPC endpoint.
 */
export async function mockDeleteInstanceApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/whatsapp.v1.WhatsAppService/DeleteInstance', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({ success: true, message: 'Instance deleted' }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to delete instance',
        },
        body: JSON.stringify({}),
      });
    }
  });
}

/**
 * Mocks the CheckStatus ConnectRPC endpoint.
 */
export async function mockCheckStatusApi(
  page: Page,
  state: 'open' | 'close' | 'connecting' = 'connecting',
  qrCode?: string
): Promise<void> {
  await page.route('**/whatsapp.v1.WhatsAppService/CheckStatus', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({
        success: true,
        state: state === 'open' ? 1 : state === 'close' ? 2 : 3,
        qrcodeBase64: qrCode ?? '',
      }),
    });
  });
}

/**
 * Mocks the SSE endpoint with a basic connected event.
 */
export async function mockSSEEndpoint(page: Page): Promise<void> {
  await page.route('**/api/v1/sse/events**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'text/event-stream',
      headers: {
        'cache-control': 'no-cache',
        connection: 'keep-alive',
      },
      body: 'event: connected\ndata: {"type":"connected","data":{},"timestamp":0}\n\n',
    });
  });
}

// ============================================================
// Groups Sync Mocks (SyncGroups + GetSyncStatus)
// ============================================================

/**
 * Mocks the SyncGroups ConnectRPC endpoint.
 */
export async function mockSyncGroupsApi(
  page: Page,
  scenario: 'success' | 'success-with-job' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/SyncGroups', async (route) => {
    if (scenario === 'error') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to sync groups',
        },
        body: JSON.stringify({}),
      });
    } else if (scenario === 'success-with-job') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({
          success: true,
          message: 'Sync started',
          job_id: 'job-e2e-001',
        }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({
          success: true,
          message: 'Groups synced successfully',
        }),
      });
    }
  });
}

/**
 * Mocks the GetSyncStatus ConnectRPC endpoint.
 */
export async function mockGetSyncStatusApi(
  page: Page,
  status: 'completed' | 'in_progress' | 'failed' = 'completed',
  progress = 100
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/GetSyncStatus', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({
        status,
        progress,
        total: 10,
        synced: Math.floor(10 * (progress / 100)),
      }),
    });
  });
}

// ============================================================
// Connected State Mocks (loadConnectionState flow)
// ============================================================

/**
 * Sets up all mocks needed for connected state via loadConnectionState flow.
 * Uses the ConnectRPC endpoints that useWhatsApp hook actually calls.
 *
 * Flow: GetLatestInstance(connected instance) → CheckStatus(open) → connected=true
 */
export async function setupConnectedStateMocks(page: Page): Promise<void> {
  await mockSSEEndpoint(page);

  // Catch-all for unhandled service routes (prevents 404s, added first = lowest priority)
  await page.route('**/instances.v1.InstancesService/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({}),
    });
  });

  await page.route('**/groups.v1.GroupsService/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({ groups: [] }),
    });
  });

  // Specific routes (added after catch-all → take priority in Playwright LIFO matching)

  await page.route('**/instances.v1.InstancesService/GetLatestInstance', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({
        instance: {
          id: 'inst-e2e-connected',
          user_id: 'user-e2e',
          instance_id: 'session-e2e-001',
          status: 'connected',
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        },
      }),
    });
  });

  await page.route('**/instances.v1.InstancesService/UpsertInstanceStatus', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({ id: 'inst-e2e-connected' }),
    });
  });

  // CheckStatus returns open (state=1) → triggers connected transition
  await mockCheckStatusApi(page, 'open');

  // DeleteInstance (WAHA) for disconnect flow
  await mockDeleteInstanceApi(page);
}

/**
 * Mocks the instances service endpoints needed for QR code flow.
 */
async function mockInstancesForQrFlow(page: Page): Promise<void> {
  await page.route('**/instances.v1.InstancesService/GetLatestInstance', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({ instance: null }),
    });
  });

  await page.route('**/instances.v1.InstancesService/CleanupInvalidInstances', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({ deletedCount: 0 }),
    });
  });

  await page.route('**/instances.v1.InstancesService/UpsertInstanceStatus', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({ id: 'inst-e2e' }),
    });
  });

  await page.route('**/instances.v1.InstancesService/ListUserInstances', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({ instances: [] }),
    });
  });
}

/**
 * Sets up all mocks needed for QR code generation flow (disconnected state).
 * Combines WhatsApp service mocks + instances service mocks + SSE.
 */
export async function setupQRCodeFlowMocks(page: Page): Promise<void> {
  await mockSSEEndpoint(page);
  await mockInstancesForQrFlow(page);
  await mockCreateInstanceApi(page);
  await mockDeleteInstanceApi(page);
  await mockCheckStatusApi(page, 'connecting', MOCK_QR_BASE64);

  // Groups service (minimal mocks to prevent 404s)
  await page.route('**/groups.v1.GroupsService/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({ groups: [] }),
    });
  });
}
