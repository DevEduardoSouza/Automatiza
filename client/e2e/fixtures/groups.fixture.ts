import type { Page } from '@playwright/test';

/**
 * Mock groups data.
 */
export const MOCK_GROUPS = [
  {
    id: 'group-1',
    jid: '123456789@g.us',
    name: 'Promocoes Tech',
    description: 'Grupo de promocoes de tecnologia',
    participantsCount: 150,
    isFavorite: true,
    isActive: true,
    createdAt: '2026-01-01T00:00:00Z',
    lastSyncedAt: new Date().toISOString(),
    syncStatus: 'SYNCED',
  },
  {
    id: 'group-2',
    jid: '987654321@g.us',
    name: 'Ofertas Casa',
    description: 'Grupo de ofertas para casa',
    participantsCount: 85,
    isFavorite: false,
    isActive: true,
    createdAt: '2026-01-05T00:00:00Z',
    lastSyncedAt: new Date().toISOString(),
    syncStatus: 'SYNCED',
  },
  {
    id: 'group-3',
    jid: '111222333@g.us',
    name: 'Eletronicos VIP',
    description: 'Ofertas exclusivas de eletronicos',
    participantsCount: 200,
    isFavorite: true,
    isActive: false,
    createdAt: '2026-01-10T00:00:00Z',
    lastSyncedAt: new Date(Date.now() - 86400000).toISOString(),
    syncStatus: 'PENDING_SYNC',
  },
];

/**
 * Mock group sync result.
 */
export const MOCK_SYNC_RESULT = {
  success: true,
  groupsAdded: 2,
  groupsUpdated: 5,
  groupsRemoved: 1,
  syncedAt: new Date().toISOString(),
};

/**
 * Mocks the groups list API.
 */
export async function mockGroupsListApi(page: Page, groups = MOCK_GROUPS): Promise<void> {
  await page.route('**/groups.v1.GroupsService/ListGroups', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify({ groups })),
    });
  });
}

/**
 * Mocks the sync groups API.
 */
export async function mockSyncGroupsApi(
  page: Page,
  scenario: 'success' | 'error' | 'partial' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/SyncGroups', async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify(MOCK_SYNC_RESULT)),
        });
        break;
      case 'partial':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(
            JSON.stringify({
              success: true,
              groupsAdded: 1,
              groupsUpdated: 2,
              groupsRemoved: 0,
              warnings: ['Some groups could not be accessed'],
              syncedAt: new Date().toISOString(),
            })
          ),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '13', // INTERNAL
            'grpc-message': 'WhatsApp not connected',
          },
          body: Buffer.from([]),
        });
        break;
    }
  });
}

/**
 * Mocks the update group API (for favorite toggle).
 */
export async function mockUpdateGroupApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/UpdateGroup', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify({ group: MOCK_GROUPS[0] })),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to update group',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the get group details API.
 */
export async function mockGetGroupApi(page: Page, group = MOCK_GROUPS[0]): Promise<void> {
  await page.route('**/groups.v1.GroupsService/GetGroup', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify({ group })),
    });
  });
}

/**
 * Mocks the delete group API.
 */
export async function mockDeleteGroupApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/DeleteGroup', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from([]),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to delete group',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the WhatsApp status for groups page.
 */
export async function mockGroupsWhatsAppStatus(page: Page, connected = true): Promise<void> {
  await page.route('**/whatsapp.v1.WhatsAppService/GetStatus', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(
        JSON.stringify({
          connected,
          status: connected ? 'CONNECTED' : 'DISCONNECTED',
        })
      ),
    });
  });
}

/**
 * Sets up all common mocks for groups tests.
 */
export async function setupGroupsMocks(page: Page): Promise<void> {
  await mockGroupsListApi(page);
  await mockGroupsWhatsAppStatus(page);
  await mockSyncGroupsApi(page);
  await mockUpdateGroupApi(page);
  await mockDeleteGroupApi(page);
}

/**
 * Sets up mocks for empty groups state.
 */
export async function setupEmptyGroupsMocks(page: Page): Promise<void> {
  await mockGroupsListApi(page, []);
  await mockGroupsWhatsAppStatus(page);
  await mockSyncGroupsApi(page);
}

// ============================================================
// Messaging Mock Data
// ============================================================

/**
 * Mock participants data for mention expansion.
 */
export const MOCK_PARTICIPANTS = [
  { id: '5511999990001@s.whatsapp.net', isAdmin: false, isSuperAdmin: false },
  { id: '5511999990002@s.whatsapp.net', isAdmin: true, isSuperAdmin: false },
  { id: '5511999990003@s.whatsapp.net', isAdmin: false, isSuperAdmin: false },
];

/**
 * Mock send message result.
 */
export const MOCK_SEND_MESSAGE_RESULT = {
  messageId: 'msg-001',
  actionId: 'action-001',
};

/**
 * Mock send audio result.
 */
export const MOCK_SEND_AUDIO_RESULT = {
  messageId: 'audio-001',
  actionId: 'action-002',
};

/**
 * Mock send poll result.
 */
export const MOCK_SEND_POLL_RESULT = {
  messageId: 'poll-001',
  actionId: 'action-003',
};

// ============================================================
// Messaging Mock Functions
// ============================================================

/**
 * Mocks the SendGroupMessage API.
 */
export async function mockSendMessageApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/SendGroupMessage', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify(MOCK_SEND_MESSAGE_RESULT)),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to send message',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the SendGroupImage API (used when message has media attached).
 */
export async function mockSendImageApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/SendGroupImage', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify(MOCK_SEND_MESSAGE_RESULT)),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to send image',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the SendGroupAudio API.
 */
export async function mockSendAudioApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/SendGroupAudio', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify(MOCK_SEND_AUDIO_RESULT)),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to send audio',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the SendGroupPoll API.
 */
export async function mockSendPollApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/SendGroupPoll', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify(MOCK_SEND_POLL_RESULT)),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to send poll',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the GetGroupParticipants API.
 */
export async function mockGroupParticipantsApi(
  page: Page,
  scenario: 'success' | 'empty' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/GetGroupParticipants', async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify({ participants: MOCK_PARTICIPANTS })),
        });
        break;
      case 'empty':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify({ participants: [] })),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '13', // INTERNAL
            'grpc-message': 'Failed to fetch participants',
          },
          body: Buffer.from([]),
        });
        break;
    }
  });
}

/**
 * Mocks the instances service for connection checks.
 */
export async function mockInstancesStatus(page: Page, connected = true): Promise<void> {
  await page.route('**/instances.v1.InstancesService/GetLatestInstance', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(
        JSON.stringify({
          instance: {
            instanceId: 'test-instance-001',
            status: connected ? 'connected' : 'disconnected',
            state: connected ? 'open' : 'close',
          },
        })
      ),
    });
  });

  await page.route('**/instances.v1.InstancesService/CheckInstanceStatus', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(
        JSON.stringify({
          success: connected,
          state: connected ? 'open' : 'close',
        })
      ),
    });
  });
}

/**
 * Sets up all mocks needed for messaging tests.
 */
export async function setupMessagingMocks(page: Page): Promise<void> {
  await mockGroupsListApi(page);
  await mockGroupsWhatsAppStatus(page);
  await mockSyncGroupsApi(page);
  await mockUpdateGroupApi(page);
  await mockDeleteGroupApi(page);
  await mockInstancesStatus(page, true);
  await mockSendMessageApi(page);
  await mockSendImageApi(page);
  await mockSendAudioApi(page);
  await mockSendPollApi(page);
  await mockGroupParticipantsApi(page);
}
