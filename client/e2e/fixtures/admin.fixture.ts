import type { Page } from '@playwright/test';

// ============================================================
// gRPC-Web Route Helpers
// ============================================================

const GRPC_OK_HEADERS = { 'grpc-status': '0' };
const GRPC_WEB_CONTENT_TYPE = 'application/grpc-web+proto';

async function fulfillGrpc(route: import('@playwright/test').Route, data: unknown): Promise<void> {
  await route.fulfill({
    status: 200,
    contentType: GRPC_WEB_CONTENT_TYPE,
    headers: GRPC_OK_HEADERS,
    body: Buffer.from(JSON.stringify(data)),
  });
}

async function fulfillGrpcError(
  route: import('@playwright/test').Route,
  code: string,
  message: string
): Promise<void> {
  await route.fulfill({
    status: 200,
    contentType: GRPC_WEB_CONTENT_TYPE,
    headers: { 'grpc-status': code, 'grpc-message': message },
    body: Buffer.from([]),
  });
}

// ============================================================
// Mock Data Constants
// ============================================================

/** Dashboard: admin profile stats */
export const MOCK_ADMIN_PROFILE_STATS = {
  stats: {
    totalUsers: 1250,
    activeUsers: 890,
    inactiveUsers: 360,
    planStats: [
      { plan: 'free', count: 600 },
      { plan: 'pro', count: 450 },
      { plan: 'elite', count: 200 },
    ],
    growthData: [
      { date: '2026-02-15', users: 1200 },
      { date: '2026-02-16', users: 1215 },
      { date: '2026-02-17', users: 1230 },
      { date: '2026-02-18', users: 1240 },
      { date: '2026-02-19', users: 1250 },
    ],
  },
};

/** Dashboard: observability data */
export const MOCK_OBSERVABILITY = {
  data: {
    generatedAt: new Date().toISOString(),
    queue: {
      waiting: 12,
      delayed: 3,
      active: 5,
      queued: 20,
      limitPerMinute: 60,
      workerConcurrency: 4,
      estimatedMinutes: 5,
    },
    dispatchLogs: {
      windowMinutes: 60,
      total: 450,
      byStatus: { sent: 400, error: 30, skipped: 15, queued: 5 },
      topErrors: [
        { error: 'timeout sending message', count: 15 },
        { error: 'group not found', count: 10 },
      ],
      topUsers: [
        { userId: 'user-1', sent: 120, error: 5, skipped: 2, queued: 1 },
        { userId: 'user-2', sent: 80, error: 3, skipped: 1, queued: 0 },
      ],
    },
    totals: {
      usersWithAnyAutomation: 420,
      usersWithActiveAutomations: 310,
      automationsTotal: 850,
      automationsActive: 620,
      groupsTotal: 3200,
      avgGroupsPerAutomation: 3.8,
    },
    topUsers: {
      byGroupsSlots: [
        { userId: 'user-1', automations: 10, groupsSlots: 50 },
        { userId: 'user-2', automations: 7, groupsSlots: 35 },
      ],
    },
  },
  cached: false,
};

/** Dashboard: Shopee affiliate audit */
export const MOCK_SHOPEE_AUDIT = {
  data: {
    window: { days: 7, sinceIso: new Date(Date.now() - 7 * 86400000).toISOString() },
    scanned: 500,
    withIdentifier: 450,
    mismatches: 12,
    unknown: 8,
    missingCreds: 3,
    samples: [
      {
        userId: 'user-1',
        email: 'user1@example.com',
        phoneNumber: null,
        expectedAppId: 'app-123',
        expectedMmpPid: 'pid-abc',
        actualMmpPid: 'pid-xyz',
        actualAfSiteid: null,
        createdAt: new Date(Date.now() - 86400000).toISOString(),
        automationId: 'auto-1',
        automationName: 'Shopee Promo',
        groupId: 'group-1',
        groupName: 'Ofertas Shopee',
        status: 'mismatch',
        url: 'https://s.shopee.com.br/test',
      },
    ],
  },
};

/** Dashboard: webhook logs */
export const MOCK_WEBHOOK_LOGS = {
  logs: [
    {
      id: 'wh-log-1',
      email: 'buyer@example.com',
      eventType: 'order_approved',
      plan: 'pro',
      createdAt: new Date(Date.now() - 3600000).toISOString(),
    },
    {
      id: 'wh-log-2',
      email: 'buyer2@example.com',
      eventType: 'subscription_created',
      plan: 'elite',
      createdAt: new Date(Date.now() - 7200000).toISOString(),
    },
    {
      id: 'wh-log-3',
      email: 'buyer3@example.com',
      eventType: 'refund',
      plan: null,
      createdAt: new Date(Date.now() - 10800000).toISOString(),
    },
  ],
};

/** Users: profiles list */
export const MOCK_PROFILES = {
  profiles: [
    {
      id: 'user-1',
      email: 'admin@example.com',
      plan: 'elite',
      phoneNumber: '+5511999999999',
      maxAutomations: 50,
      isBlocked: false,
      billingNextPaymentAt: null,
      planAccessUntil: null,
      createdAt: new Date(Date.now() - 90 * 86400000).toISOString(),
      updatedAt: new Date().toISOString(),
    },
    {
      id: 'user-2',
      email: 'pro@example.com',
      plan: 'pro',
      phoneNumber: '+5511888888888',
      maxAutomations: 10,
      isBlocked: false,
      billingNextPaymentAt: new Date(Date.now() + 30 * 86400000).toISOString(),
      planAccessUntil: null,
      createdAt: new Date(Date.now() - 30 * 86400000).toISOString(),
      updatedAt: new Date().toISOString(),
    },
    {
      id: 'user-3',
      email: 'free@example.com',
      plan: 'free',
      phoneNumber: null,
      maxAutomations: 1,
      isBlocked: false,
      billingNextPaymentAt: null,
      planAccessUntil: null,
      createdAt: new Date(Date.now() - 7 * 86400000).toISOString(),
      updatedAt: new Date().toISOString(),
    },
    {
      id: 'user-4',
      email: 'blocked@example.com',
      plan: 'pro',
      phoneNumber: '+5511777777777',
      maxAutomations: 10,
      isBlocked: true,
      billingNextPaymentAt: null,
      planAccessUntil: null,
      createdAt: new Date(Date.now() - 60 * 86400000).toISOString(),
      updatedAt: new Date().toISOString(),
    },
  ],
  hasMore: false,
};

/** Users: admin users list */
export const MOCK_ADMIN_USERS_LIST = {
  admins: [
    {
      userId: 'user-1',
      email: 'admin@example.com',
      role: 'admin',
      grantedAt: new Date(Date.now() - 90 * 86400000).toISOString(),
    },
  ],
};

/** Users: user investigation */
export const MOCK_USER_INVESTIGATION = {
  user: {
    id: 'user-2',
    email: 'pro@example.com',
    plan: 'pro',
    phoneNumber: '+5511888888888',
    billingStatus: null,
    createdAt: new Date(Date.now() - 30 * 86400000).toISOString(),
    updatedAt: new Date().toISOString(),
  },
  isAdmin: false,
  instances: [
    {
      instanceId: 'inst-1',
      status: 'CONNECTED',
      updatedAt: new Date().toISOString(),
    },
  ],
  automationsTotal: 5,
  automationsActive: 3,
  automationsPaused: 2,
  automations: [
    {
      id: 'auto-1',
      name: 'Shopee Diario',
      status: 'active',
      mode: 'scheduled',
      intervalMinutes: 30,
      startTime: '08:00',
      endTime: '22:00',
      stores: ['shopee'],
      sendGroups: ['group-1', 'group-2'],
      categories: ['eletronicos'],
      lastRunAt: new Date(Date.now() - 1800000).toISOString(),
      nextRunAt: new Date(Date.now() + 1800000).toISOString(),
      createdAt: new Date(Date.now() - 15 * 86400000).toISOString(),
    },
    {
      id: 'auto-2',
      name: 'ML Semanal',
      status: 'paused',
      mode: 'manual',
      intervalMinutes: 60,
      startTime: null,
      endTime: null,
      stores: ['mercadolivre'],
      sendGroups: ['group-3'],
      categories: [],
      lastRunAt: null,
      nextRunAt: null,
      createdAt: new Date(Date.now() - 10 * 86400000).toISOString(),
    },
  ],
};

/** Plans: subscription plans */
export const MOCK_PLANS = {
  plans: [
    {
      id: 'plan-free',
      name: 'Free',
      slug: 'free',
      description: 'Plano gratuito',
      priceMonthly: 0,
      priceSemester: undefined,
      priceAnnual: undefined,
      groupLimit: 1,
      storageLimit: '1000000000',
      features: ['1 automacao', '1 grupo'],
      kiwifyProductIdMonthly: '',
      kiwifyProductIdSemester: '',
      kiwifyProductIdAnnual: '',
      isActive: true,
      createdAt: new Date(Date.now() - 180 * 86400000).toISOString(),
      updatedAt: new Date(Date.now() - 30 * 86400000).toISOString(),
    },
    {
      id: 'plan-pro',
      name: 'Pro',
      slug: 'pro',
      description: 'Plano profissional',
      priceMonthly: 49.9,
      priceSemester: 249.9,
      priceAnnual: 449.9,
      groupLimit: 10,
      storageLimit: '5000000000',
      features: ['10 automacoes', '10 grupos', 'Suporte prioritario'],
      kiwifyProductIdMonthly: 'kiwify-pro-monthly',
      kiwifyProductIdSemester: 'kiwify-pro-semester',
      kiwifyProductIdAnnual: 'kiwify-pro-annual',
      isActive: true,
      createdAt: new Date(Date.now() - 180 * 86400000).toISOString(),
      updatedAt: new Date(Date.now() - 10 * 86400000).toISOString(),
    },
    {
      id: 'plan-elite',
      name: 'Elite',
      slug: 'elite',
      description: 'Plano premium',
      priceMonthly: 99.9,
      priceSemester: 499.9,
      priceAnnual: 899.9,
      groupLimit: 50,
      storageLimit: '20000000000',
      features: ['50 automacoes', '50 grupos', 'Suporte VIP', 'API acesso'],
      kiwifyProductIdMonthly: 'kiwify-elite-monthly',
      kiwifyProductIdSemester: 'kiwify-elite-semester',
      kiwifyProductIdAnnual: 'kiwify-elite-annual',
      isActive: true,
      createdAt: new Date(Date.now() - 180 * 86400000).toISOString(),
      updatedAt: new Date(Date.now() - 5 * 86400000).toISOString(),
    },
  ],
};

/** Categories: product categories with subcategories */
export const MOCK_CATEGORIES = {
  categories: [
    {
      id: 'cat-1',
      slug: 'eletronicos',
      name: 'Eletronicos',
      icon: 'zap',
      displayOrder: 1,
      isActive: true,
      hasRefinements: true,
      isSystem: true,
      createdAt: new Date(Date.now() - 180 * 86400000).toISOString(),
      updatedAt: new Date(Date.now() - 30 * 86400000).toISOString(),
      subcategories: [
        {
          id: 'sub-1',
          categoryId: 'cat-1',
          slug: 'celulares',
          name: 'Celulares',
          displayKey: 'celulares',
          groupName: 'Smartphones',
          displayOrder: 1,
          isActive: true,
          isSystem: true,
          createdAt: new Date(Date.now() - 180 * 86400000).toISOString(),
          updatedAt: new Date(Date.now() - 30 * 86400000).toISOString(),
        },
        {
          id: 'sub-2',
          categoryId: 'cat-1',
          slug: 'notebooks',
          name: 'Notebooks',
          displayKey: 'notebooks',
          groupName: 'Computadores',
          displayOrder: 2,
          isActive: true,
          isSystem: false,
          createdAt: new Date(Date.now() - 90 * 86400000).toISOString(),
          updatedAt: new Date(Date.now() - 15 * 86400000).toISOString(),
        },
      ],
    },
    {
      id: 'cat-2',
      slug: 'moda',
      name: 'Moda',
      icon: 'shirt',
      displayOrder: 2,
      isActive: true,
      hasRefinements: false,
      isSystem: false,
      createdAt: new Date(Date.now() - 90 * 86400000).toISOString(),
      updatedAt: new Date(Date.now() - 15 * 86400000).toISOString(),
      subcategories: [],
    },
  ],
};

/** Email templates */
export const MOCK_EMAIL_TEMPLATES = {
  templates: [
    {
      id: 'tmpl-1',
      name: 'welcome_email',
      subject: 'Bem-vindo ao Automatiza!',
      body: '<html><body><h1>Bem-vindo!</h1><p>Seu email: {{email}}</p><p>Sua senha: {{password}}</p></body></html>',
      description: 'Template de boas-vindas enviado ao criar usuario',
      isActive: true,
      createdAt: new Date(Date.now() - 60 * 86400000).toISOString(),
      updatedAt: new Date(Date.now() - 5 * 86400000).toISOString(),
    },
    {
      id: 'tmpl-2',
      name: 'password_reset',
      subject: 'Redefinicao de senha',
      body: '<html><body><h1>Redefinir senha</h1><p>Nova senha: {{password}}</p></body></html>',
      description: 'Template enviado ao redefinir senha',
      isActive: true,
      createdAt: new Date(Date.now() - 30 * 86400000).toISOString(),
      updatedAt: new Date(Date.now() - 2 * 86400000).toISOString(),
    },
    {
      id: 'tmpl-3',
      name: 'plan_upgrade',
      subject: 'Upgrade de plano confirmado',
      body: '<html><body><h1>Parabens!</h1><p>Seu plano agora e {{plan_name}}</p></body></html>',
      description: null,
      isActive: false,
      createdAt: new Date(Date.now() - 10 * 86400000).toISOString(),
      updatedAt: new Date(Date.now() - 10 * 86400000).toISOString(),
    },
  ],
};

/** Settings: system settings */
export const MOCK_SETTINGS = {
  settings: [
    { key: 'smtp_host', value: 'smtp.gmail.com', description: 'SMTP host' },
    { key: 'smtp_port', value: '587', description: 'SMTP port' },
    { key: 'smtp_username', value: 'noreply@automatiza.com', description: 'SMTP username' },
    { key: 'smtp_password', value: '***', description: 'SMTP password' },
    { key: 'smtp_from_email', value: 'noreply@automatiza.com', description: 'From email' },
    { key: 'smtp_from_name', value: 'Automatiza', description: 'From name' },
    { key: 'kiwify_webhook_token', value: 'tok-123', description: 'Kiwify webhook token' },
    { key: 'waha_api_key', value: 'waha-key', description: 'WAHA API key' },
    { key: 'waha_api_url', value: 'http://waha:3000', description: 'WAHA API URL' },
    { key: 'ai_api_key', value: 'ai-key', description: 'AI API key' },
    { key: 'ai_api_url', value: 'https://api.openai.com', description: 'AI API URL' },
    { key: 'ai_model', value: 'gpt-4o-mini', description: 'AI model' },
    { key: 'support_whatsapp', value: '+5511999999999', description: 'Support WhatsApp' },
    { key: 'tutorial_link', value: 'https://docs.automatiza.com', description: 'Tutorial link' },
    { key: 'dashboard_announcement_enabled', value: '1', description: 'Announcement enabled' },
    { key: 'dashboard_announcement_title', value: 'Novidades!', description: 'Announcement title' },
    { key: 'dashboard_announcement_body', value: 'Novo recurso disponivel', description: 'Announcement body' },
    { key: 'link_safety_mode', value: 'block', description: 'Link safety mode' },
    { key: 'link_safety_blocked_hosts', value: '', description: 'Blocked hosts' },
    { key: 'link_safety_allowed_hosts', value: 's.shopee.com.br,shopee.com.br,www.shopee.com.br', description: 'Allowed hosts' },
    { key: 'link_safety_enforce_allowlist', value: '0', description: 'Enforce allowlist' },
    { key: 'link_safety_trusted_redirect_hosts', value: '', description: 'Trusted redirect hosts' },
    { key: 'link_safety_max_redirects', value: '6', description: 'Max redirects' },
    { key: 'link_safety_redirect_timeout_ms', value: '3500', description: 'Redirect timeout' },
    { key: 'link_safety_redirect_cache_seconds', value: '21600', description: 'Redirect cache seconds' },
  ],
};

// ============================================================
// Individual Mock Functions — Dashboard
// ============================================================

/**
 * Mocks admin.v1.AdminService/GetAdminProfileStats.
 */
export async function mockGetAdminProfileStats(
  page: Page,
  scenario: 'success' | 'error' = 'success',
  data = MOCK_ADMIN_PROFILE_STATS
): Promise<void> {
  await page.route('**/admin.v1.AdminService/GetAdminProfileStats', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, data);
    } else {
      await fulfillGrpcError(route, '13', 'Failed to fetch profile stats');
    }
  });
}

/**
 * Mocks admin.v1.AdminService/GetObservability.
 */
export async function mockGetObservability(
  page: Page,
  scenario: 'success' | 'error' = 'success',
  data = MOCK_OBSERVABILITY
): Promise<void> {
  await page.route('**/admin.v1.AdminService/GetObservability', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, data);
    } else {
      await fulfillGrpcError(route, '13', 'Failed to fetch observability data');
    }
  });
}

/**
 * Mocks admin.v1.AdminService/GetShopeeAffiliateAudit.
 */
export async function mockGetShopeeAffiliateAudit(
  page: Page,
  scenario: 'success' | 'error' = 'success',
  data = MOCK_SHOPEE_AUDIT
): Promise<void> {
  await page.route('**/admin.v1.AdminService/GetShopeeAffiliateAudit', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, data);
    } else {
      await fulfillGrpcError(route, '13', 'Failed to fetch Shopee audit');
    }
  });
}

/**
 * Mocks admin.v1.AdminService/ListWebhookLogs.
 */
export async function mockListWebhookLogs(
  page: Page,
  scenario: 'success' | 'error' = 'success',
  data = MOCK_WEBHOOK_LOGS
): Promise<void> {
  await page.route('**/admin.v1.AdminService/ListWebhookLogs', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, data);
    } else {
      await fulfillGrpcError(route, '13', 'Failed to fetch webhook logs');
    }
  });
}

/**
 * Mocks webhooks.v1.WebhooksService/SimulateKiwifyWebhook.
 */
export async function mockSimulateKiwifyWebhook(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/webhooks.v1.WebhooksService/SimulateKiwifyWebhook', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, { success: true });
    } else {
      await fulfillGrpcError(route, '13', 'Webhook simulation failed');
    }
  });
}

// ============================================================
// Individual Mock Functions — Users
// ============================================================

/**
 * Mocks profiles.v1.ProfilesService/ListProfiles.
 */
export async function mockListProfiles(
  page: Page,
  scenario: 'success' | 'error' = 'success',
  data = MOCK_PROFILES
): Promise<void> {
  await page.route('**/profiles.v1.ProfilesService/ListProfiles', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, data);
    } else {
      await fulfillGrpcError(route, '13', 'Failed to list profiles');
    }
  });
}

/**
 * Mocks profiles.v1.ProfilesService/SearchProfiles.
 */
export async function mockSearchProfiles(
  page: Page,
  scenario: 'success' | 'empty' | 'error' = 'success',
  profiles = MOCK_PROFILES.profiles
): Promise<void> {
  await page.route('**/profiles.v1.ProfilesService/SearchProfiles', async (route) => {
    switch (scenario) {
      case 'success':
        await fulfillGrpc(route, { profiles });
        break;
      case 'empty':
        await fulfillGrpc(route, { profiles: [] });
        break;
      case 'error':
        await fulfillGrpcError(route, '13', 'Search failed');
        break;
    }
  });
}

/**
 * Mocks admin.v1.AdminService/ListAdminUsers.
 */
export async function mockListAdminUsers(
  page: Page,
  scenario: 'success' | 'error' = 'success',
  data = MOCK_ADMIN_USERS_LIST
): Promise<void> {
  await page.route('**/admin.v1.AdminService/ListAdminUsers', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, data);
    } else {
      await fulfillGrpcError(route, '13', 'Failed to list admin users');
    }
  });
}

/**
 * Mocks admin.v1.AdminService/CreateUser.
 */
export async function mockCreateUser(
  page: Page,
  scenario: 'success' | 'duplicate' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/CreateUser', async (route) => {
    switch (scenario) {
      case 'success':
        await fulfillGrpc(route, { temporaryPassword: 'TempPass123!' });
        break;
      case 'duplicate':
        await fulfillGrpcError(route, '6', 'User with this email already exists');
        break;
      case 'error':
        await fulfillGrpcError(route, '13', 'Failed to create user');
        break;
    }
  });
}

/**
 * Mocks admin.v1.AdminService/GetUserInvestigation.
 */
export async function mockGetUserInvestigation(
  page: Page,
  scenario: 'success' | 'not_found' | 'error' = 'success',
  data = MOCK_USER_INVESTIGATION
): Promise<void> {
  await page.route('**/admin.v1.AdminService/GetUserInvestigation', async (route) => {
    switch (scenario) {
      case 'success':
        await fulfillGrpc(route, data);
        break;
      case 'not_found':
        await fulfillGrpcError(route, '5', 'User not found');
        break;
      case 'error':
        await fulfillGrpcError(route, '13', 'Failed to get user investigation');
        break;
    }
  });
}

/**
 * Mocks admin.v1.AdminService/GrantAdminRole.
 */
export async function mockGrantAdminRole(
  page: Page,
  scenario: 'success' | 'already_exists' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/GrantAdminRole', async (route) => {
    switch (scenario) {
      case 'success':
        await fulfillGrpc(route, {});
        break;
      case 'already_exists':
        await fulfillGrpcError(route, '6', 'User is already an admin');
        break;
      case 'error':
        await fulfillGrpcError(route, '13', 'Failed to grant admin role');
        break;
    }
  });
}

/**
 * Mocks admin.v1.AdminService/RevokeAdminRole.
 */
export async function mockRevokeAdminRole(
  page: Page,
  scenario: 'success' | 'self_revoke' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/RevokeAdminRole', async (route) => {
    switch (scenario) {
      case 'success':
        await fulfillGrpc(route, {});
        break;
      case 'self_revoke':
        await fulfillGrpcError(route, '9', 'Cannot revoke own admin role');
        break;
      case 'error':
        await fulfillGrpcError(route, '13', 'Failed to revoke admin role');
        break;
    }
  });
}

/**
 * Mocks admin.v1.AdminService/AdminBlockUser.
 */
export async function mockAdminBlockUser(
  page: Page,
  scenario: 'success' | 'self_block' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/AdminBlockUser', async (route) => {
    switch (scenario) {
      case 'success':
        await fulfillGrpc(route, { pausedAutomations: 3 });
        break;
      case 'self_block':
        await fulfillGrpcError(route, '9', 'Cannot block your own account');
        break;
      case 'error':
        await fulfillGrpcError(route, '13', 'Failed to block user');
        break;
    }
  });
}

/**
 * Mocks profiles.v1.ProfilesService/UnblockUser.
 */
export async function mockUnblockUser(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/profiles.v1.ProfilesService/UnblockUser', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, {});
    } else {
      await fulfillGrpcError(route, '13', 'Failed to unblock user');
    }
  });
}

/**
 * Mocks admin.v1.AdminService/ResetUserPassword.
 */
export async function mockResetUserPassword(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/ResetUserPassword', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, {
        success: true,
        email: 'pro@example.com',
        newPassword: 'NewTempPass456!',
        message: 'Password reset successfully',
      });
    } else {
      await fulfillGrpcError(route, '13', 'Failed to reset password');
    }
  });
}

/**
 * Mocks admin.v1.AdminService/AdminBulkUpdateAutomationStatus.
 */
export async function mockAdminBulkUpdateAutomationStatus(
  page: Page,
  scenario: 'success' | 'error' = 'success',
  updatedCount = 3
): Promise<void> {
  await page.route('**/admin.v1.AdminService/AdminBulkUpdateAutomationStatus', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, { updatedCount });
    } else {
      await fulfillGrpcError(route, '13', 'Failed to update automation status');
    }
  });
}

/**
 * Mocks admin.v1.AdminService/AdminSpreadAutomationSchedule.
 */
export async function mockAdminSpreadAutomationSchedule(
  page: Page,
  scenario: 'success' | 'error' = 'success',
  updatedCount = 3
): Promise<void> {
  await page.route('**/admin.v1.AdminService/AdminSpreadAutomationSchedule', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, { updatedCount });
    } else {
      await fulfillGrpcError(route, '13', 'Failed to spread automation schedule');
    }
  });
}

/**
 * Mocks profiles.v1.ProfilesService/UpdateUserPlan.
 */
export async function mockUpdateUserPlan(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/profiles.v1.ProfilesService/UpdateUserPlan', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, {});
    } else {
      await fulfillGrpcError(route, '13', 'Failed to update user plan');
    }
  });
}

// ============================================================
// Individual Mock Functions — Plans
// ============================================================

/**
 * Mocks admin.v1.AdminService/ListPlans.
 */
export async function mockListPlans(
  page: Page,
  scenario: 'success' | 'empty' | 'error' = 'success',
  data = MOCK_PLANS
): Promise<void> {
  await page.route('**/admin.v1.AdminService/ListPlans', async (route) => {
    switch (scenario) {
      case 'success':
        await fulfillGrpc(route, data);
        break;
      case 'empty':
        await fulfillGrpc(route, { plans: [] });
        break;
      case 'error':
        await fulfillGrpcError(route, '13', 'Failed to list plans');
        break;
    }
  });
}

/**
 * Mocks admin.v1.AdminService/CreatePlan.
 */
export async function mockCreatePlan(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/CreatePlan', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, { plan: { ...MOCK_PLANS.plans[1], id: 'plan-new' } });
    } else {
      await fulfillGrpcError(route, '13', 'Failed to create plan');
    }
  });
}

/**
 * Mocks admin.v1.AdminService/UpdatePlan.
 */
export async function mockUpdatePlan(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/UpdatePlan', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, { plan: MOCK_PLANS.plans[1] });
    } else {
      await fulfillGrpcError(route, '13', 'Failed to update plan');
    }
  });
}

/**
 * Mocks admin.v1.AdminService/DeletePlan.
 */
export async function mockDeletePlan(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/DeletePlan', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, {});
    } else {
      await fulfillGrpcError(route, '13', 'Failed to delete plan');
    }
  });
}

// ============================================================
// Individual Mock Functions — Categories
// ============================================================

/**
 * Mocks admin.v1.AdminService/ListCategories.
 */
export async function mockListCategories(
  page: Page,
  scenario: 'success' | 'empty' | 'error' = 'success',
  data = MOCK_CATEGORIES
): Promise<void> {
  await page.route('**/admin.v1.AdminService/ListCategories', async (route) => {
    switch (scenario) {
      case 'success':
        await fulfillGrpc(route, data);
        break;
      case 'empty':
        await fulfillGrpc(route, { categories: [] });
        break;
      case 'error':
        await fulfillGrpcError(route, '13', 'Failed to list categories');
        break;
    }
  });
}

/**
 * Mocks admin.v1.AdminService/CreateCategory.
 */
export async function mockCreateCategory(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/CreateCategory', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, {
        category: { ...MOCK_CATEGORIES.categories[1], id: 'cat-new' },
      });
    } else {
      await fulfillGrpcError(route, '13', 'Failed to create category');
    }
  });
}

/**
 * Mocks admin.v1.AdminService/UpdateCategory.
 */
export async function mockUpdateCategory(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/UpdateCategory', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, { category: MOCK_CATEGORIES.categories[0] });
    } else {
      await fulfillGrpcError(route, '13', 'Failed to update category');
    }
  });
}

/**
 * Mocks admin.v1.AdminService/DeleteCategory.
 */
export async function mockDeleteCategory(
  page: Page,
  scenario: 'success' | 'system_category' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/DeleteCategory', async (route) => {
    switch (scenario) {
      case 'success':
        await fulfillGrpc(route, {});
        break;
      case 'system_category':
        await fulfillGrpcError(route, '9', 'Cannot delete system category');
        break;
      case 'error':
        await fulfillGrpcError(route, '13', 'Failed to delete category');
        break;
    }
  });
}

/**
 * Mocks admin.v1.AdminService/CreateSubcategory.
 */
export async function mockCreateSubcategory(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/CreateSubcategory', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, {
        subcategory: {
          ...MOCK_CATEGORIES.categories[0].subcategories[0],
          id: 'sub-new',
        },
      });
    } else {
      await fulfillGrpcError(route, '13', 'Failed to create subcategory');
    }
  });
}

/**
 * Mocks admin.v1.AdminService/UpdateSubcategory.
 */
export async function mockUpdateSubcategory(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/UpdateSubcategory', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, {
        subcategory: MOCK_CATEGORIES.categories[0].subcategories[0],
      });
    } else {
      await fulfillGrpcError(route, '13', 'Failed to update subcategory');
    }
  });
}

/**
 * Mocks admin.v1.AdminService/DeleteSubcategory.
 */
export async function mockDeleteSubcategory(
  page: Page,
  scenario: 'success' | 'system_subcategory' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/DeleteSubcategory', async (route) => {
    switch (scenario) {
      case 'success':
        await fulfillGrpc(route, {});
        break;
      case 'system_subcategory':
        await fulfillGrpcError(route, '9', 'Cannot delete system subcategory');
        break;
      case 'error':
        await fulfillGrpcError(route, '13', 'Failed to delete subcategory');
        break;
    }
  });
}

// ============================================================
// Individual Mock Functions — Email Templates
// ============================================================

/**
 * Mocks admin.v1.AdminService/ListEmailTemplates.
 */
export async function mockListEmailTemplates(
  page: Page,
  scenario: 'success' | 'empty' | 'error' = 'success',
  data = MOCK_EMAIL_TEMPLATES
): Promise<void> {
  await page.route('**/admin.v1.AdminService/ListEmailTemplates', async (route) => {
    switch (scenario) {
      case 'success':
        await fulfillGrpc(route, data);
        break;
      case 'empty':
        await fulfillGrpc(route, { templates: [] });
        break;
      case 'error':
        await fulfillGrpcError(route, '13', 'Failed to list email templates');
        break;
    }
  });
}

/**
 * Mocks admin.v1.AdminService/CreateEmailTemplate.
 */
export async function mockCreateEmailTemplate(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/CreateEmailTemplate', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, {
        template: { ...MOCK_EMAIL_TEMPLATES.templates[0], id: 'tmpl-new' },
      });
    } else {
      await fulfillGrpcError(route, '13', 'Failed to create email template');
    }
  });
}

/**
 * Mocks admin.v1.AdminService/UpdateEmailTemplate.
 */
export async function mockUpdateEmailTemplate(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/admin.v1.AdminService/UpdateEmailTemplate', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, { template: MOCK_EMAIL_TEMPLATES.templates[0] });
    } else {
      await fulfillGrpcError(route, '13', 'Failed to update email template');
    }
  });
}

/**
 * Mocks admin.v1.AdminService/ToggleEmailTemplateActive.
 */
export async function mockToggleEmailTemplateActive(
  page: Page,
  scenario: 'success' | 'error' = 'success',
  isActive = false
): Promise<void> {
  await page.route('**/admin.v1.AdminService/ToggleEmailTemplateActive', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, {
        template: { ...MOCK_EMAIL_TEMPLATES.templates[0], isActive },
      });
    } else {
      await fulfillGrpcError(route, '13', 'Failed to toggle template status');
    }
  });
}

// ============================================================
// Individual Mock Functions — Settings
// ============================================================

/**
 * Mocks settings.v1.SettingsService/ListSettings.
 */
export async function mockListSettings(
  page: Page,
  scenario: 'success' | 'error' = 'success',
  data = MOCK_SETTINGS
): Promise<void> {
  await page.route('**/settings.v1.SettingsService/ListSettings', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, data);
    } else {
      await fulfillGrpcError(route, '13', 'Failed to list settings');
    }
  });
}

/**
 * Mocks settings.v1.SettingsService/UpdateSetting.
 */
export async function mockUpdateSetting(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/settings.v1.SettingsService/UpdateSetting', async (route) => {
    if (scenario === 'success') {
      await fulfillGrpc(route, {});
    } else {
      await fulfillGrpcError(route, '13', 'Failed to update setting');
    }
  });
}

// ============================================================
// Auth Helpers
// ============================================================

/**
 * Mocks the current user as admin via GetProfile.
 */
export async function mockCurrentUserAsAdmin(page: Page): Promise<void> {
  await page.route('**/profiles.v1.ProfilesService/GetProfile', async (route) => {
    await fulfillGrpc(route, {
      profile: {
        id: 'user-1',
        email: 'admin@example.com',
        plan: 'elite',
        phoneNumber: '+5511999999999',
        maxAutomations: 50,
        isBlocked: false,
        isAdmin: true,
        createdAt: new Date(Date.now() - 90 * 86400000).toISOString(),
        updatedAt: new Date().toISOString(),
      },
    });
  });
}

/**
 * Mocks access denied for non-admin users on all admin service routes.
 */
export async function mockAdminAccessDenied(page: Page): Promise<void> {
  await page.route('**/admin.v1.AdminService/**', async (route) => {
    await fulfillGrpcError(route, '7', 'Admin access required');
  });
}

// ============================================================
// Setup Composites (per tab)
// ============================================================

/**
 * Sets up all mocks needed for the Dashboard tab.
 */
export async function setupAdminDashboardMocks(page: Page): Promise<void> {
  await mockGetAdminProfileStats(page);
  await mockGetObservability(page);
  await mockGetShopeeAffiliateAudit(page);
  await mockListWebhookLogs(page);
  await mockSimulateKiwifyWebhook(page);
  await mockListSettings(page); // Dashboard uses kiwify_webhook_token setting
}

/**
 * Sets up all mocks needed for the Users tab.
 */
export async function setupAdminUsersMocks(page: Page): Promise<void> {
  await mockListProfiles(page);
  await mockSearchProfiles(page);
  await mockListAdminUsers(page);
  await mockCreateUser(page);
  await mockGetUserInvestigation(page);
  await mockGrantAdminRole(page);
  await mockRevokeAdminRole(page);
  await mockAdminBlockUser(page);
  await mockUnblockUser(page);
  await mockResetUserPassword(page);
  await mockAdminBulkUpdateAutomationStatus(page);
  await mockAdminSpreadAutomationSchedule(page);
  await mockUpdateUserPlan(page);
}

/**
 * Sets up all mocks needed for the Plans tab.
 */
export async function setupAdminPlansMocks(page: Page): Promise<void> {
  await mockListPlans(page);
  await mockCreatePlan(page);
  await mockUpdatePlan(page);
  await mockDeletePlan(page);
}

/**
 * Sets up all mocks needed for the Categories tab.
 */
export async function setupAdminCategoriesMocks(page: Page): Promise<void> {
  await mockListCategories(page);
  await mockCreateCategory(page);
  await mockUpdateCategory(page);
  await mockDeleteCategory(page);
  await mockCreateSubcategory(page);
  await mockUpdateSubcategory(page);
  await mockDeleteSubcategory(page);
}

/**
 * Sets up all mocks needed for the Team tab.
 */
export async function setupAdminTeamMocks(page: Page): Promise<void> {
  await mockListAdminUsers(page);
  await mockGrantAdminRole(page);
  await mockRevokeAdminRole(page);
  await mockSearchProfiles(page);
}

/**
 * Sets up all mocks needed for the Email Templates tab.
 */
export async function setupAdminEmailTemplatesMocks(page: Page): Promise<void> {
  await mockListEmailTemplates(page);
  await mockCreateEmailTemplate(page);
  await mockUpdateEmailTemplate(page);
  await mockToggleEmailTemplateActive(page);
}

/**
 * Sets up all mocks needed for the Settings tab.
 */
export async function setupAdminSettingsMocks(page: Page): Promise<void> {
  await mockListSettings(page);
  await mockUpdateSetting(page);
}

/**
 * Sets up ALL mocks for the full admin panel (all 7 tabs).
 */
export async function setupFullAdminMocks(page: Page): Promise<void> {
  await mockCurrentUserAsAdmin(page);
  // Dashboard
  await mockGetAdminProfileStats(page);
  await mockGetObservability(page);
  await mockGetShopeeAffiliateAudit(page);
  await mockListWebhookLogs(page);
  await mockSimulateKiwifyWebhook(page);
  // Users
  await mockListProfiles(page);
  await mockSearchProfiles(page);
  await mockListAdminUsers(page);
  await mockCreateUser(page);
  await mockGetUserInvestigation(page);
  await mockGrantAdminRole(page);
  await mockRevokeAdminRole(page);
  await mockAdminBlockUser(page);
  await mockUnblockUser(page);
  await mockResetUserPassword(page);
  await mockAdminBulkUpdateAutomationStatus(page);
  await mockAdminSpreadAutomationSchedule(page);
  await mockUpdateUserPlan(page);
  // Plans
  await mockListPlans(page);
  await mockCreatePlan(page);
  await mockUpdatePlan(page);
  await mockDeletePlan(page);
  // Categories
  await mockListCategories(page);
  await mockCreateCategory(page);
  await mockUpdateCategory(page);
  await mockDeleteCategory(page);
  await mockCreateSubcategory(page);
  await mockUpdateSubcategory(page);
  await mockDeleteSubcategory(page);
  // Email Templates
  await mockListEmailTemplates(page);
  await mockCreateEmailTemplate(page);
  await mockUpdateEmailTemplate(page);
  await mockToggleEmailTemplateActive(page);
  // Settings
  await mockListSettings(page);
  await mockUpdateSetting(page);
}
