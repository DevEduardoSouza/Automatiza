import type { Page } from '@playwright/test';

// ============================================================
// Mock Data
// ============================================================

/**
 * Mock redirect groups for E2E tests.
 */
export const MOCK_REDIRECT_GROUPS = [
  {
    id: 'rg-1',
    name: 'Promocoes Tech',
    waGroupId: '123456789@g.us',
    inviteCode: 'ABC123',
    membersCount: 150,
    memberLimit: 200,
    priority: 1,
    clickLimit: 50,
    isFavorite: true,
  },
  {
    id: 'rg-2',
    name: 'Ofertas Casa',
    waGroupId: '987654321@g.us',
    inviteCode: 'DEF456',
    membersCount: 85,
    memberLimit: 200,
    priority: 2,
    clickLimit: 30,
    isFavorite: false,
  },
  {
    id: 'rg-3',
    name: 'Eletronicos VIP',
    waGroupId: '111222333@g.us',
    inviteCode: 'GHI789',
    membersCount: 200,
    memberLimit: 200,
    priority: 3,
    clickLimit: 0,
    isFavorite: true,
  },
];

/**
 * Mock saved redirect links.
 */
export const MOCK_SAVED_LINKS = [
  {
    id: 'link-1',
    userId: 'user-123',
    name: 'Link Promocional',
    slug: 'promo2026',
    distributionStrategy: 1, // CLICK_LIMIT
    groupPriorities: [MOCK_REDIRECT_GROUPS[0], MOCK_REDIRECT_GROUPS[1]],
    isActive: true,
    totalClicks: 142,
    createdAt: '2026-01-15T10:00:00Z',
    updatedAt: '2026-02-10T14:30:00Z',
  },
  {
    id: 'link-2',
    userId: 'user-123',
    name: 'Link VIP',
    slug: 'vip-entry',
    distributionStrategy: 2, // MEMBER_LIMIT
    groupPriorities: [MOCK_REDIRECT_GROUPS[2]],
    isActive: false,
    totalClicks: 57,
    createdAt: '2026-01-20T08:00:00Z',
    updatedAt: '2026-02-05T16:00:00Z',
  },
];

/**
 * Mock groups for the group selection dialog (from GroupsService).
 */
export const MOCK_GROUPS_FOR_REDIRECT = [
  {
    id: 'rg-1',
    jid: '123456789@g.us',
    name: 'Promocoes Tech',
    description: 'Grupo de promocoes',
    participantsCount: 150,
    isFavorite: true,
    isActive: true,
    createdAt: '2026-01-01T00:00:00Z',
    lastSyncedAt: new Date().toISOString(),
    syncStatus: 'SYNCED',
    waGroupId: '123456789@g.us',
    membersCount: 150,
    memberLimit: 200,
    status: 1, // OPEN
    isAdmin: true,
    inviteCode: 'ABC123',
    userId: 'user-123',
    instanceId: 'inst-1',
    updatedAt: new Date().toISOString(),
  },
  {
    id: 'rg-2',
    jid: '987654321@g.us',
    name: 'Ofertas Casa',
    description: 'Grupo de ofertas',
    participantsCount: 85,
    isFavorite: false,
    isActive: true,
    createdAt: '2026-01-05T00:00:00Z',
    lastSyncedAt: new Date().toISOString(),
    syncStatus: 'SYNCED',
    waGroupId: '987654321@g.us',
    membersCount: 85,
    memberLimit: 200,
    status: 1,
    isAdmin: true,
    inviteCode: 'DEF456',
    userId: 'user-123',
    instanceId: 'inst-1',
    updatedAt: new Date().toISOString(),
  },
  {
    id: 'rg-3',
    jid: '111222333@g.us',
    name: 'Eletronicos VIP',
    description: 'Ofertas exclusivas',
    participantsCount: 200,
    isFavorite: true,
    isActive: true,
    createdAt: '2026-01-10T00:00:00Z',
    lastSyncedAt: new Date().toISOString(),
    syncStatus: 'SYNCED',
    waGroupId: '111222333@g.us',
    membersCount: 200,
    memberLimit: 200,
    status: 1,
    isAdmin: true,
    inviteCode: 'GHI789',
    userId: 'user-123',
    instanceId: 'inst-1',
    updatedAt: new Date().toISOString(),
  },
];

/**
 * Mock public redirect success response.
 */
export const MOCK_PUBLIC_REDIRECT_SUCCESS = {
  success: true,
  redirect_url: 'https://chat.whatsapp.com/ABC123XYZ',
  group_name: 'Promocoes Tech',
};

// ============================================================
// Mock API Functions
// ============================================================

/**
 * Mocks the ListRedirectLinks API.
 */
export async function mockListRedirectLinksApi(
  page: Page,
  scenario: 'success' | 'empty' | 'error' = 'success'
): Promise<void> {
  await page.route('**/redirect.v1.RedirectLinkService/ListRedirectLinks', async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: { 'grpc-status': '0' },
          body: JSON.stringify({ links: MOCK_SAVED_LINKS }),
        });
        break;
      case 'empty':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: { 'grpc-status': '0' },
          body: JSON.stringify({ links: [] }),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: {
            'grpc-status': '13',
            'grpc-message': 'Failed to list redirect links',
          },
          body: JSON.stringify({}),
        });
        break;
    }
  });
}

/**
 * Mocks the ListGroups API for the group selection dialog.
 */
export async function mockListGroupsForRedirectApi(
  page: Page,
  groups = MOCK_GROUPS_FOR_REDIRECT
): Promise<void> {
  await page.route('**/groups.v1.GroupsService/ListGroups', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({ groups }),
    });
  });
}

/**
 * Mocks the CreateRedirectLink API.
 */
export async function mockCreateRedirectLinkApi(
  page: Page,
  scenario: 'success' | 'slug_exists' | 'error' = 'success'
): Promise<void> {
  await page.route('**/redirect.v1.RedirectLinkService/CreateRedirectLink', async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: { 'grpc-status': '0' },
          body: JSON.stringify({
            link: {
              ...MOCK_SAVED_LINKS[0],
              id: 'link-new',
              slug: 'new-slug',
            },
          }),
        });
        break;
      case 'slug_exists':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: {
            'grpc-status': '6', // ALREADY_EXISTS
            'grpc-message': 'Slug already exists',
          },
          body: JSON.stringify({}),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: {
            'grpc-status': '13',
            'grpc-message': 'Failed to create redirect link',
          },
          body: JSON.stringify({}),
        });
        break;
    }
  });
}

/**
 * Mocks the UpdateRedirectLink API.
 */
export async function mockUpdateRedirectLinkApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/redirect.v1.RedirectLinkService/UpdateRedirectLink', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({ link: MOCK_SAVED_LINKS[0] }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to update redirect link',
        },
        body: JSON.stringify({}),
      });
    }
  });
}

/**
 * Mocks the DeleteRedirectLink API.
 */
export async function mockDeleteRedirectLinkApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/redirect.v1.RedirectLinkService/DeleteRedirectLink', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({}),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to delete redirect link',
        },
        body: JSON.stringify({}),
      });
    }
  });
}

/**
 * Mocks the CheckSlugAvailability API.
 */
export async function mockCheckSlugAvailabilityApi(
  page: Page,
  available = true
): Promise<void> {
  await page.route(
    '**/redirect.v1.RedirectLinkService/CheckSlugAvailability',
    async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({ available }),
      });
    }
  );
}

/**
 * Mocks the public redirect REST API.
 */
export async function mockPublicRedirectApi(
  page: Page,
  scenario:
    | 'success'
    | 'not_found'
    | 'inactive'
    | 'groups_full'
    | 'temporary'
    | 'no_groups' = 'success'
): Promise<void> {
  await page.route('**/api/v1/redirect', async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_PUBLIC_REDIRECT_SUCCESS),
        });
        break;
      case 'not_found':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            success: false,
            error: 'link not found',
          }),
        });
        break;
      case 'inactive':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            success: false,
            error: 'link is inactive',
          }),
        });
        break;
      case 'groups_full':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            success: false,
            error: 'no available groups (all groups are full or inactive)',
          }),
        });
        break;
      case 'temporary':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            success: false,
            error: 'service temporarily unavailable',
          }),
        });
        break;
      case 'no_groups':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            success: false,
            error: 'no target groups configured',
          }),
        });
        break;
    }
  });
}

// ============================================================
// Auth Mock (inline for self-containment)
// ============================================================

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

// ============================================================
// Composite Setup Functions
// ============================================================

/**
 * Sets up all mocks for the redirect links page (authenticated).
 */
export async function setupRedirectLinkMocks(page: Page): Promise<void> {
  await setupAuthMocks(page);
  await mockListRedirectLinksApi(page);
  await mockListGroupsForRedirectApi(page);
  await mockCreateRedirectLinkApi(page);
  await mockUpdateRedirectLinkApi(page);
  await mockDeleteRedirectLinkApi(page);
  await mockCheckSlugAvailabilityApi(page);
}

/**
 * Sets up mocks for the empty redirect links state.
 */
export async function setupEmptyRedirectMocks(page: Page): Promise<void> {
  await setupAuthMocks(page);
  await mockListRedirectLinksApi(page, 'empty');
  await mockListGroupsForRedirectApi(page);
}

/**
 * Sets up mocks for public redirect page scenarios.
 */
export async function setupPublicRedirectMocks(
  page: Page,
  scenario:
    | 'success'
    | 'not_found'
    | 'inactive'
    | 'groups_full'
    | 'temporary'
    | 'no_groups' = 'success'
): Promise<void> {
  await mockPublicRedirectApi(page, scenario);
}
