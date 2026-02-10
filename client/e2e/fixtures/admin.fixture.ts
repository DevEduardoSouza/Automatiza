import type { Page } from '@playwright/test';

/**
 * User roles enum.
 */
export const USER_ROLES = {
  USER: 1,
  MODERATOR: 2,
  ADMIN: 3,
  SUPER_ADMIN: 4,
} as const;

/**
 * User plan types enum.
 */
export const PLAN_TYPES = {
  FREE: 1,
  BASIC: 2,
  PRO: 3,
  ENTERPRISE: 4,
} as const;

/**
 * Mock users data for admin panel.
 */
export const MOCK_ADMIN_USERS = [
  {
    id: 'user-1',
    email: 'admin@example.com',
    name: 'Admin User',
    role: USER_ROLES.ADMIN,
    plan: PLAN_TYPES.ENTERPRISE,
    isActive: true,
    createdAt: new Date(Date.now() - 90 * 24 * 3600000).toISOString(),
    lastLoginAt: new Date().toISOString(),
    automationsCount: 10,
    groupsCount: 25,
  },
  {
    id: 'user-2',
    email: 'pro@example.com',
    name: 'Pro User',
    role: USER_ROLES.USER,
    plan: PLAN_TYPES.PRO,
    isActive: true,
    createdAt: new Date(Date.now() - 30 * 24 * 3600000).toISOString(),
    lastLoginAt: new Date(Date.now() - 24 * 3600000).toISOString(),
    automationsCount: 5,
    groupsCount: 12,
  },
  {
    id: 'user-3',
    email: 'free@example.com',
    name: 'Free User',
    role: USER_ROLES.USER,
    plan: PLAN_TYPES.FREE,
    isActive: true,
    createdAt: new Date(Date.now() - 7 * 24 * 3600000).toISOString(),
    lastLoginAt: new Date(Date.now() - 3 * 24 * 3600000).toISOString(),
    automationsCount: 1,
    groupsCount: 2,
  },
  {
    id: 'user-4',
    email: 'banned@example.com',
    name: 'Banned User',
    role: USER_ROLES.USER,
    plan: PLAN_TYPES.BASIC,
    isActive: false,
    createdAt: new Date(Date.now() - 60 * 24 * 3600000).toISOString(),
    lastLoginAt: new Date(Date.now() - 30 * 24 * 3600000).toISOString(),
    automationsCount: 0,
    groupsCount: 0,
  },
];

/**
 * Mock system stats for admin panel.
 */
export const MOCK_SYSTEM_STATS = {
  totalUsers: 1250,
  activeUsers: 890,
  totalAutomations: 3500,
  activeAutomations: 2100,
  totalGroups: 8500,
  totalOffersSent: 125000,
  offersSentToday: 4500,
  systemUptime: 99.9,
  averageResponseTime: 45,
  errorRate: 0.1,
};

/**
 * Mock audit logs for admin panel.
 */
export const MOCK_AUDIT_LOGS = [
  {
    id: 'audit-1',
    action: 'USER_LOGIN',
    userId: 'user-1',
    userEmail: 'admin@example.com',
    ipAddress: '192.168.1.1',
    timestamp: new Date().toISOString(),
    details: { success: true },
  },
  {
    id: 'audit-2',
    action: 'USER_PLAN_CHANGED',
    userId: 'user-2',
    userEmail: 'pro@example.com',
    adminId: 'user-1',
    timestamp: new Date(Date.now() - 3600000).toISOString(),
    details: { fromPlan: 'BASIC', toPlan: 'PRO' },
  },
  {
    id: 'audit-3',
    action: 'USER_BANNED',
    userId: 'user-4',
    userEmail: 'banned@example.com',
    adminId: 'user-1',
    timestamp: new Date(Date.now() - 7 * 24 * 3600000).toISOString(),
    details: { reason: 'Terms of service violation' },
  },
];

/**
 * Mocks the admin users list API.
 */
export async function mockAdminUsersListApi(
  page: Page,
  users = MOCK_ADMIN_USERS,
  options: { totalCount?: number; hasMore?: boolean } = {}
): Promise<void> {
  const { totalCount = users.length, hasMore = false } = options;

  await page.route('**/admin.v1.AdminService/ListUsers', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify({ users, totalCount, hasMore })),
    });
  });
}

/**
 * Mocks the system stats API.
 */
export async function mockSystemStatsApi(page: Page, stats = MOCK_SYSTEM_STATS): Promise<void> {
  await page.route('**/admin.v1.AdminService/GetSystemStats', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify(stats)),
    });
  });
}

/**
 * Mocks the audit logs API.
 */
export async function mockAuditLogsApi(page: Page, logs = MOCK_AUDIT_LOGS): Promise<void> {
  await page.route('**/admin.v1.AdminService/ListAuditLogs', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify({ logs, hasMore: false })),
    });
  });
}

/**
 * Mocks the update user role API.
 */
export async function mockUpdateUserRoleApi(
  page: Page,
  scenario: 'success' | 'forbidden' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/UpdateUserRole', async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify({ user: MOCK_ADMIN_USERS[1] })),
        });
        break;
      case 'forbidden':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '7', // PERMISSION_DENIED
            'grpc-message': 'Cannot modify admin user',
          },
          body: Buffer.from([]),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '13', // INTERNAL
            'grpc-message': 'Failed to update user role',
          },
          body: Buffer.from([]),
        });
        break;
    }
  });
}

/**
 * Mocks the update user plan API.
 */
export async function mockUpdateUserPlanApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/UpdateUserPlan', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify({ user: MOCK_ADMIN_USERS[1] })),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to update user plan',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the ban/unban user API.
 */
export async function mockToggleUserStatusApi(
  page: Page,
  scenario: 'success' | 'self_ban' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/ToggleUserStatus', async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(
            JSON.stringify({
              user: { ...MOCK_ADMIN_USERS[3], isActive: true },
            })
          ),
        });
        break;
      case 'self_ban':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '9', // FAILED_PRECONDITION
            'grpc-message': 'Cannot ban yourself',
          },
          body: Buffer.from([]),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '13', // INTERNAL
            'grpc-message': 'Failed to update user status',
          },
          body: Buffer.from([]),
        });
        break;
    }
  });
}

/**
 * Mocks the impersonate user API.
 */
export async function mockImpersonateUserApi(
  page: Page,
  scenario: 'success' | 'forbidden' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/ImpersonateUser', async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(
            JSON.stringify({
              token: 'impersonated-token-123',
              expiresAt: new Date(Date.now() + 3600000).toISOString(),
            })
          ),
        });
        break;
      case 'forbidden':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '7', // PERMISSION_DENIED
            'grpc-message': 'Cannot impersonate admin user',
          },
          body: Buffer.from([]),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '13', // INTERNAL
            'grpc-message': 'Failed to impersonate user',
          },
          body: Buffer.from([]),
        });
        break;
    }
  });
}

/**
 * Mocks the current user as admin.
 */
export async function mockCurrentUserAsAdmin(page: Page): Promise<void> {
  await page.route('**/profiles.v1.ProfilesService/GetProfile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(
        JSON.stringify({
          profile: {
            id: 'user-1',
            email: 'admin@example.com',
            name: 'Admin User',
            role: USER_ROLES.ADMIN,
            plan: PLAN_TYPES.ENTERPRISE,
          },
        })
      ),
    });
  });
}

/**
 * Mocks access denied for non-admin users.
 */
export async function mockAdminAccessDenied(page: Page): Promise<void> {
  await page.route('**/admin.v1.AdminService/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: {
        'grpc-status': '7', // PERMISSION_DENIED
        'grpc-message': 'Admin access required',
      },
      body: Buffer.from([]),
    });
  });
}

/**
 * Sets up all common mocks for admin tests.
 */
export async function setupAdminMocks(page: Page): Promise<void> {
  await mockCurrentUserAsAdmin(page);
  await mockAdminUsersListApi(page);
  await mockSystemStatsApi(page);
  await mockAuditLogsApi(page);
  await mockUpdateUserRoleApi(page);
  await mockUpdateUserPlanApi(page);
  await mockToggleUserStatusApi(page);
  await mockImpersonateUserApi(page);
}

/**
 * Sets up mocks for admin with no users.
 */
export async function setupEmptyAdminMocks(page: Page): Promise<void> {
  await mockCurrentUserAsAdmin(page);
  await mockAdminUsersListApi(page, []);
  await mockSystemStatsApi(page, {
    ...MOCK_SYSTEM_STATS,
    totalUsers: 0,
    activeUsers: 0,
  });
  await mockAuditLogsApi(page, []);
}
