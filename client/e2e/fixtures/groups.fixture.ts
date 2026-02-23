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

// ============================================================
// Member Management Mock Data
// ============================================================

/**
 * Extended mock participants with roles for member management tests.
 */
export const MOCK_MEMBERS = [
  {
    id: '5511999990001@s.whatsapp.net',
    is_admin: false,
    is_super_admin: false,
    phone_number: '5511999990001',
  },
  {
    id: '5511999990002@s.whatsapp.net',
    is_admin: true,
    is_super_admin: false,
    phone_number: '5511999990002',
  },
  {
    id: '5511999990003@s.whatsapp.net',
    is_admin: false,
    is_super_admin: true,
    phone_number: '5511999990003',
  },
  {
    id: '5511999990004@s.whatsapp.net',
    is_admin: false,
    is_super_admin: false,
    phone_number: '5511999990004',
  },
];

// ============================================================
// Member Management Mock Functions
// ============================================================

/**
 * Mocks the AddGroupMember API.
 */
export async function mockAddGroupMemberApi(
  page: Page,
  scenario: 'success' | 'partial' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/AddGroupMember', async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify({ addedCount: 1, failedNumbers: [] })),
        });
        break;
      case 'partial':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify({ addedCount: 0, failedNumbers: ['5511999990099'] })),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '13',
            'grpc-message': 'Failed to add member',
          },
          body: Buffer.from([]),
        });
        break;
    }
  });
}

/**
 * Mocks the RemoveGroupMember API.
 */
export async function mockRemoveGroupMemberApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/RemoveGroupMember', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify({ removedCount: 1, failedNumbers: [] })),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to remove member',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the LeaveGroup API.
 */
export async function mockLeaveGroupApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/LeaveGroup', async (route) => {
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
          'grpc-status': '13',
          'grpc-message': 'Failed to leave group',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the PromoteMember API.
 */
export async function mockPromoteMemberApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/PromoteMember', async (route) => {
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
          'grpc-status': '13',
          'grpc-message': 'Failed to promote member',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the DemoteMember API.
 */
export async function mockDemoteMemberApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/DemoteMember', async (route) => {
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
          'grpc-status': '13',
          'grpc-message': 'Failed to demote member',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the UpdateGroupSubject API (for bulk rename).
 */
export async function mockUpdateGroupSubjectApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/UpdateGroupSubject', async (route) => {
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
          'grpc-status': '13',
          'grpc-message': 'Failed to update group subject',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the UpdateGroupSettings API (for close/open groups).
 */
export async function mockUpdateGroupSettingsApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/UpdateGroupSettings', async (route) => {
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
          'grpc-status': '13',
          'grpc-message': 'Failed to update group settings',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the UpdateGroupDescription API.
 */
export async function mockUpdateGroupDescriptionApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/UpdateGroupDescription', async (route) => {
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
          'grpc-status': '13',
          'grpc-message': 'Failed to update group description',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the UpdateGroupPicture API.
 */
export async function mockUpdateGroupPictureApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/UpdateGroupPicture', async (route) => {
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
          'grpc-status': '13',
          'grpc-message': 'Failed to update group picture',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the BulkUpdateGroups API.
 */
export async function mockBulkUpdateGroupsApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/BulkUpdateGroups', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify({ updatedCount: 2 })),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to bulk update groups',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the reconnect flow (CreateInstance + CheckStatus).
 */
export async function mockReconnectInstanceApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  const mockQr =
    'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';

  await page.route('**/whatsapp.v1.WhatsAppService/CreateInstance', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({
          success: true,
          instanceName: 'reconnect-instance',
          qrcodeBase64: mockQr,
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

  await page.route('**/whatsapp.v1.WhatsAppService/CheckStatus', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({
        success: true,
        state: scenario === 'success' ? 1 : 2,
        qrcodeBase64: scenario === 'success' ? mockQr : '',
      }),
    });
  });
}

/**
 * Sets up all mocks needed for member management tests.
 */
export async function setupMembersMocks(page: Page): Promise<void> {
  await mockGroupsListApi(page);
  await mockGroupsWhatsAppStatus(page);
  await mockSyncGroupsApi(page);
  await mockUpdateGroupApi(page);
  await mockDeleteGroupApi(page);
  await mockInstancesStatus(page, true);
  await mockGroupParticipantsApi(page, 'success');
  await mockAddGroupMemberApi(page);
  await mockRemoveGroupMemberApi(page);
  await mockLeaveGroupApi(page);
  await mockPromoteMemberApi(page);
  await mockDemoteMemberApi(page);
}

/**
 * Sets up all mocks needed for bulk actions tests.
 */
export async function setupBulkActionsMocks(page: Page): Promise<void> {
  await mockGroupsListApi(page);
  await mockGroupsWhatsAppStatus(page);
  await mockSyncGroupsApi(page);
  await mockUpdateGroupApi(page);
  await mockDeleteGroupApi(page);
  await mockInstancesStatus(page, true);
  await mockUpdateGroupSubjectApi(page);
  await mockUpdateGroupSettingsApi(page);
  await mockUpdateGroupDescriptionApi(page);
  await mockUpdateGroupPictureApi(page);
  await mockBulkUpdateGroupsApi(page);
}

/**
 * Sets up all mocks needed for connection management tests.
 */
export async function setupConnectionMocks(page: Page, connected = true): Promise<void> {
  await mockGroupsListApi(page);
  await mockGroupsWhatsAppStatus(page, connected);
  await mockSyncGroupsApi(page);
  await mockUpdateGroupApi(page);
  await mockDeleteGroupApi(page);
  await mockInstancesStatus(page, connected);
  await mockReconnectInstanceApi(page);
}

// ============================================================
// Sync Status & SSE Mocks
// ============================================================

/**
 * Mocks the GetSyncStatus API for polling sync progress.
 */
export async function mockGetSyncStatusApi(
  page: Page,
  scenario: 'completed' | 'in_progress' | 'failed' | 'not_found' = 'completed'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/GetSyncStatus', async (route) => {
    switch (scenario) {
      case 'completed':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: { 'grpc-status': '0' },
          body: JSON.stringify({ status: 'completed', error: '' }),
        });
        break;
      case 'in_progress':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: { 'grpc-status': '0' },
          body: JSON.stringify({ status: 'in_progress', error: '' }),
        });
        break;
      case 'failed':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: { 'grpc-status': '0' },
          body: JSON.stringify({ status: 'failed', error: 'Sync failed due to timeout' }),
        });
        break;
      case 'not_found':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: {
            'grpc-status': '5',
            'grpc-message': 'Job not found',
          },
          body: JSON.stringify({}),
        });
        break;
    }
  });
}

/**
 * Mocks the ToggleGroupFavorite API.
 */
export async function mockToggleGroupFavoriteApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/ToggleGroupFavorite', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({ success: true }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to toggle favorite',
        },
        body: JSON.stringify({}),
      });
    }
  });
}

/**
 * Mocks the GetGroupInviteCode API.
 */
export async function mockGetGroupInviteCodeApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/GetGroupInviteCode', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({ inviteCode: 'ABC123XYZ' }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to get invite code',
        },
        body: JSON.stringify({}),
      });
    }
  });
}

/**
 * Mocks the SSE endpoint for real-time group updates.
 */
export async function mockGroupsSSEEndpoint(page: Page): Promise<void> {
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

/**
 * Mocks instances service upsert for reconnection flow.
 */
export async function mockUpsertInstanceStatusApi(page: Page): Promise<void> {
  await page.route('**/instances.v1.InstancesService/UpsertInstanceStatus', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({ id: 'inst-e2e' }),
    });
  });
}

/**
 * Mocks ListUserInstances for instance enumeration.
 */
export async function mockListUserInstancesApi(page: Page): Promise<void> {
  await page.route('**/instances.v1.InstancesService/ListUserInstances', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({
        instances: [
          {
            id: 'inst-1',
            instanceId: 'test-instance-001',
            status: 'connected',
          },
        ],
      }),
    });
  });
}

/**
 * Mocks the instances ReconnectInstance API.
 */
export async function mockReconnectInstanceViaInstancesApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  const mockQr =
    'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';

  await page.route('**/instances.v1.InstancesService/ReconnectInstance', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({
          success: true,
          qrcodeBase64: mockQr,
        }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to reconnect instance',
        },
        body: JSON.stringify({}),
      });
    }
  });
}

// ============================================================
// Network Error Mocks
// ============================================================

/**
 * Mocks the groups list API with a network error.
 */
export async function mockGroupsListApiNetworkError(page: Page): Promise<void> {
  await page.route('**/groups.v1.GroupsService/ListGroups', async (route) => {
    await route.abort('connectionfailed');
  });
}

/**
 * Mocks the groups list API with a gRPC UNAVAILABLE error.
 */
export async function mockGroupsListApiUnavailable(page: Page): Promise<void> {
  await page.route('**/groups.v1.GroupsService/ListGroups', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: {
        'grpc-status': '14', // UNAVAILABLE
        'grpc-message': 'Service temporarily unavailable',
      },
      body: JSON.stringify({}),
    });
  });
}

// ============================================================
// Comprehensive Setup Functions
// ============================================================

/**
 * Sets up all mocks for a fully connected groups page with all features.
 * This is the most comprehensive setup, suitable for most tests.
 */
export async function setupFullGroupsMocks(page: Page): Promise<void> {
  await setupAuthMocks(page);
  await mockGroupsSSEEndpoint(page);
  await mockGroupsListApi(page);
  await mockGroupsWhatsAppStatus(page);
  await mockSyncGroupsApi(page);
  await mockGetSyncStatusApi(page);
  await mockUpdateGroupApi(page);
  await mockDeleteGroupApi(page);
  await mockToggleGroupFavoriteApi(page);
  await mockGetGroupInviteCodeApi(page);
  await mockInstancesStatus(page, true);
  await mockUpsertInstanceStatusApi(page);
  await mockListUserInstancesApi(page);
  await mockSendMessageApi(page);
  await mockSendImageApi(page);
  await mockSendAudioApi(page);
  await mockSendPollApi(page);
  await mockGroupParticipantsApi(page);
  await mockAddGroupMemberApi(page);
  await mockRemoveGroupMemberApi(page);
  await mockLeaveGroupApi(page);
  await mockPromoteMemberApi(page);
  await mockDemoteMemberApi(page);
  await mockUpdateGroupSubjectApi(page);
  await mockUpdateGroupSettingsApi(page);
  await mockUpdateGroupDescriptionApi(page);
  await mockUpdateGroupPictureApi(page);
  await mockBulkUpdateGroupsApi(page);
  await mockReconnectInstanceApi(page);
  await mockReconnectInstanceViaInstancesApi(page);
}

/**
 * Sets up mocks for the disconnected state.
 */
export async function setupDisconnectedGroupsMocks(page: Page): Promise<void> {
  await setupAuthMocks(page);
  await mockGroupsSSEEndpoint(page);
  await mockGroupsListApi(page, []);
  await mockGroupsWhatsAppStatus(page, false);
  await mockSyncGroupsApi(page);
  await mockGetSyncStatusApi(page);
  await mockUpdateGroupApi(page);
  await mockDeleteGroupApi(page);
  await mockInstancesStatus(page, false);
  await mockUpsertInstanceStatusApi(page);
  await mockListUserInstancesApi(page);
  await mockReconnectInstanceApi(page);
  await mockReconnectInstanceViaInstancesApi(page);
}

/**
 * Sets up mocks for the error/loading failure state.
 */
export async function setupErrorGroupsMocks(page: Page): Promise<void> {
  await setupAuthMocks(page);
  await mockGroupsSSEEndpoint(page);
  await mockGroupsListApiUnavailable(page);
  await mockGroupsWhatsAppStatus(page);
  await mockInstancesStatus(page, true);
  await mockUpsertInstanceStatusApi(page);
  await mockListUserInstancesApi(page);
}

// ============================================================
// Auth Mocks (inline for self-contained fixture)
// ============================================================

/**
 * Sets up authentication mocks for the groups page.
 * Duplicates the pattern from auth.fixture.ts for self-containment.
 */
async function setupAuthMocks(page: Page): Promise<void> {
  await page.route('**/auth.v1.AuthService/GetCurrentUser', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        user: {
          id: 'user-123',
          email: 'test@example.com',
          displayName: 'Test User',
          role: 'user',
          emailVerified: true,
          createdAt: '2026-01-01T00:00:00Z',
        },
        session: {
          sessionId: 'session-123',
          expiresAt: new Date(Date.now() + 3600000).toISOString(),
          isCurrent: true,
        },
      }),
    });
  });

  await page.route('**/auth.v1.AuthService/Login', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        user: { id: 'user-123', email: 'test@example.com' },
        session: { sessionId: 'session-123' },
        csrfToken: 'mock-csrf-token',
      }),
    });
  });

  await page.route('**/auth.v1.AuthService/Logout', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({}),
    });
  });
}
