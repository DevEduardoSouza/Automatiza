import type { Page } from '@playwright/test';
import { test as base } from '@playwright/test';

/**
 * Test automation data for E2E tests.
 */
export const TEST_AUTOMATION = {
  name: 'E2E Test Automation',
  startTime: '08:00',
  endTime: '22:00',
  intervalMinutes: 30,
};

/**
 * Mock automation response for list endpoint.
 */
export const MOCK_AUTOMATIONS = [
  {
    id: 'test-automation-1',
    name: 'Test Automation Search',
    mode: 1, // SEARCH
    startTime: '08:00',
    endTime: '22:00',
    intervalMinutes: 30,
    status: 1, // ACTIVE
    stores: [1], // SHOPEE
    sendGroups: ['group-1'],
    categories: [],
    texts: ['iphone'],
    ctas: [],
    priority: 1, // DISCOUNT
    filterType: 1, // LIGHT
  },
  {
    id: 'test-automation-2',
    name: 'Test Automation Monitor',
    mode: 2, // MONITOR
    startTime: '09:00',
    endTime: '21:00',
    status: 2, // PAUSED
    stores: [1],
    sendGroups: ['group-2'],
    monitorGroupIds: ['group-1'],
    delaySeconds: 60,
  },
];

/**
 * Mock groups response.
 */
export const MOCK_GROUPS = [
  { id: 'group-1', name: 'Test Group 1', jid: '123@g.us', isFavorite: true },
  { id: 'group-2', name: 'Test Group 2', jid: '456@g.us', isFavorite: false },
];

/**
 * Mock list-mode automation (mode=3, store=MERCADO_LIVRE).
 */
export const MOCK_LIST_AUTOMATION = {
  id: 'test-automation-3',
  name: 'Test Automation List',
  mode: 3, // LIST
  startTime: '10:00',
  endTime: '20:00',
  intervalMinutes: 60,
  status: 1, // ACTIVE
  stores: [4], // MERCADO_LIVRE
  sendGroups: ['group-1'],
  categories: [],
  texts: [],
  ctas: [],
  priority: 0,
  filterType: 0,
  mlLinkListId: 'ml-list-1',
  listItemsPerRun: 3,
  listIncludeImage: true,
};

/**
 * Mock ML link lists response.
 */
export const MOCK_ML_LINK_LISTS = [
  { id: 'ml-list-1', name: 'Lista Eletrônicos', isActive: true },
  { id: 'ml-list-2', name: 'Lista Roupas', isActive: true },
];

/**
 * Mock Shopee preview offers response.
 */
export const MOCK_SHOPEE_PREVIEW_OFFERS = {
  offers: [
    {
      title: 'iPhone 15 Pro Max',
      price: 5499.99,
      originalPrice: 7999.0,
      discount: 31,
      imageUrl: 'https://example.com/iphone.jpg',
      productUrl: 'https://shopee.com.br/product/123',
      commission: 8.5,
    },
    {
      title: 'Samsung Galaxy S24',
      price: 3299.0,
      originalPrice: 4499.0,
      discount: 27,
      imageUrl: 'https://example.com/samsung.jpg',
      productUrl: 'https://shopee.com.br/product/456',
      commission: 7.0,
    },
  ],
};

/**
 * Mock active stores (affiliate credentials) response.
 */
export const MOCK_ACTIVE_STORES = {
  stores: [
    { store: 'shopee', isConfigured: true },
    { store: 'amazon', isConfigured: false },
    { store: 'magalu', isConfigured: false },
    { store: 'mercado_livre', isConfigured: true },
  ],
};

/**
 * Mock diagnostic result.
 */
export const MOCK_DIAGNOSTIC_SUCCESS = {
  result: {
    automationId: 'test-automation-1',
    code: 'SUCCESS',
    message: 'Automation is configured correctly',
    steps: [
      { step: 'config', ok: true, detail: 'Configuration valid' },
      { step: 'whatsapp', ok: true, detail: 'WhatsApp connected' },
      { step: 'groups', ok: true, detail: 'Groups accessible' },
      { step: 'stores', ok: true, detail: 'Stores configured' },
    ],
    offersFound: 5,
    sentTo: ['Test Group 1'],
  },
};

/**
 * Mocks the automations list API.
 */
export async function mockAutomationsListApi(
  page: Page,
  automations = MOCK_AUTOMATIONS
): Promise<void> {
  await page.route('**/automations.v1.AutomationsService/ListAutomations', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify({ automations })),
    });
  });
}

/**
 * Mocks the create automation API.
 */
export async function mockCreateAutomationApi(
  page: Page,
  scenario: 'success' | 'limit_reached' | 'error' = 'success'
): Promise<void> {
  await page.route('**/automations.v1.AutomationsService/CreateAutomation', async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(
            JSON.stringify({
              automation: {
                id: 'new-automation-id',
                name: 'E2E Test Automation',
                mode: 1,
                status: 1,
                startTime: '08:00',
                endTime: '22:00',
                intervalMinutes: 30,
                stores: [1],
                sendGroups: ['group-1'],
              },
            })
          ),
        });
        break;
      case 'limit_reached':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: {
            'grpc-status': '8', // RESOURCE_EXHAUSTED
            'grpc-message': 'Automation limit reached',
          },
          body: Buffer.from([]),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'Internal server error' }),
        });
        break;
    }
  });
}

/**
 * Mocks the run automation API.
 */
export async function mockRunAutomationApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/automations.v1.AutomationsService/RunAutomation', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(
          JSON.stringify({
            success: true,
            message: 'Automation executed successfully',
          })
        ),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to run automation',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the diagnose automation API.
 */
export async function mockDiagnoseAutomationApi(
  page: Page,
  scenario: 'success' | 'issues' | 'error' = 'success'
): Promise<void> {
  await page.route('**/automations.v1.AutomationsService/DiagnoseAutomation', async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify(MOCK_DIAGNOSTIC_SUCCESS)),
        });
        break;
      case 'issues':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(
            JSON.stringify({
              result: {
                automationId: 'test-automation-1',
                code: 'WARNING',
                message: 'Automation has configuration issues',
                steps: [
                  { step: 'config', ok: true, detail: 'Configuration valid' },
                  {
                    step: 'whatsapp',
                    ok: false,
                    detail: 'WhatsApp disconnected',
                    message: 'Please reconnect WhatsApp',
                  },
                  { step: 'groups', ok: true, detail: 'Groups accessible' },
                ],
                offersFound: 0,
                sentTo: [],
              },
            })
          ),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'Internal server error' }),
        });
        break;
    }
  });
}

/**
 * Mocks the reset automation history API.
 */
export async function mockResetHistoryApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/automations.v1.AutomationsService/ResetAutomationHistory', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(
          JSON.stringify({
            success: true,
            message: 'History reset successfully',
          })
        ),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to reset history',
        },
        body: Buffer.from([]),
      });
    }
  });
}

/**
 * Mocks the delete automation API.
 */
export async function mockDeleteAutomationApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/automations.v1.AutomationsService/DeleteAutomation', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from([]),
      });
    } else {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Failed to delete automation' }),
      });
    }
  });
}

/**
 * Mocks the update automation API (for status toggle).
 */
export async function mockUpdateAutomationApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/automations.v1.AutomationsService/UpdateAutomation', async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(
          JSON.stringify({
            automation: MOCK_AUTOMATIONS[0],
          })
        ),
      });
    } else {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Failed to update automation' }),
      });
    }
  });
}

/**
 * Mocks the groups list API.
 */
export async function mockGroupsApi(page: Page): Promise<void> {
  await page.route('**/groups.v1.GroupsService/ListGroups', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify({ groups: MOCK_GROUPS })),
    });
  });
}

/**
 * Mocks the profile/limits API.
 */
export async function mockLimitsApi(
  page: Page,
  options: { maxAutomations?: number; currentCount?: number } = {}
): Promise<void> {
  const { maxAutomations = 5, currentCount = 2 } = options;

  await page.route('**/profiles.v1.ProfilesService/GetLimits', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(
        JSON.stringify({
          maxAutomations,
          currentAutomations: currentCount,
          maxGroups: 10,
          currentGroups: 2,
        })
      ),
    });
  });
}

/**
 * Mocks the WhatsApp status API.
 */
export async function mockWhatsAppStatusApi(page: Page, connected = true): Promise<void> {
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
 * Mocks the ML link lists API.
 */
export async function mockMlLinkListsApi(page: Page): Promise<void> {
  await page.route('**/ml_links.v1.MlLinksService/ListMlLinkLists', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(JSON.stringify({ lists: MOCK_ML_LINK_LISTS })),
    });
  });
}

/**
 * Mocks the Shopee preview offers API.
 */
export async function mockShopeePreviewApi(
  page: Page,
  scenario: 'success' | 'empty' | 'error' = 'success'
): Promise<void> {
  await page.route('**/shopee.v1.ShopeeService/SearchOffers', async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify(MOCK_SHOPEE_PREVIEW_OFFERS)),
        });
        break;
      case 'empty':
        await route.fulfill({
          status: 200,
          contentType: 'application/grpc-web+proto',
          headers: { 'grpc-status': '0' },
          body: Buffer.from(JSON.stringify({ offers: [] })),
        });
        break;
      case 'error':
        await route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'Shopee API error' }),
        });
        break;
    }
  });
}

/**
 * Mocks the run automation API with conflict (ALREADY_EXISTS) response.
 */
export async function mockConflictRunApi(page: Page): Promise<void> {
  await page.route('**/automations.v1.AutomationsService/RunAutomation', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: {
        'grpc-status': '6', // ALREADY_EXISTS
        'grpc-message': 'Automation is already running',
      },
      body: Buffer.from([]),
    });
  });
}

/**
 * Mocks the dispatch texts API.
 */
export async function mockDispatchTextsApi(page: Page): Promise<void> {
  await page.route('**/dispatch_texts.v1.DispatchTextsService/ListDispatchTexts', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/grpc-web+proto',
      headers: { 'grpc-status': '0' },
      body: Buffer.from(
        JSON.stringify({
          dispatchTexts: [
            { id: 'dt-1', name: 'Texto Promo Shopee', isActive: true },
            { id: 'dt-2', name: 'Texto Black Friday', isActive: true },
          ],
        })
      ),
    });
  });
}

/**
 * Mocks the affiliate credentials (active stores) API.
 */
export async function mockAffiliateCredentialsApi(page: Page): Promise<void> {
  await page.route(
    '**/affiliate_credentials.v1.AffiliateCredentialsService/GetActiveStores',
    async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/grpc-web+proto',
        headers: { 'grpc-status': '0' },
        body: Buffer.from(JSON.stringify(MOCK_ACTIVE_STORES)),
      });
    }
  );
}

/**
 * Sets up all common mocks for automation tests.
 */
export async function setupAutomationMocks(page: Page): Promise<void> {
  await mockAutomationsListApi(page);
  await mockGroupsApi(page);
  await mockLimitsApi(page);
  await mockWhatsAppStatusApi(page);
  await mockCreateAutomationApi(page);
  await mockUpdateAutomationApi(page);
  await mockDeleteAutomationApi(page);
  await mockRunAutomationApi(page);
  await mockDiagnoseAutomationApi(page);
  await mockResetHistoryApi(page);
  await mockMlLinkListsApi(page);
  await mockDispatchTextsApi(page);
  await mockAffiliateCredentialsApi(page);
  await mockShopeePreviewApi(page);
}

/**
 * Extended test fixture with automations page.
 */
export const test = base.extend<{
  automationsPage: Page;
}>({
  automationsPage: async ({ page }, use) => {
    await setupAutomationMocks(page);
    await use(page);
  },
});

export { expect } from '@playwright/test';
