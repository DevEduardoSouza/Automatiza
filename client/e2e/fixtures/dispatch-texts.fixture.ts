import type { Page } from '@playwright/test';

/**
 * Dispatch text type constants for filtering.
 */
export const TEXT_TYPES = {
  ACTIVE: 'active',
  INACTIVE: 'inactive',
  ALL: 'all',
} as const;

/**
 * Mock dispatch texts matching the proto DispatchText message format.
 */
export const MOCK_DISPATCH_TEXTS = [
  {
    id: 'text-1',
    userId: 'user-1',
    name: 'CTA Shopee Principal',
    body: 'Clique no link para aproveitar esta oferta incrivel!\n\n{link_afiliado}',
    isActive: true,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  },
  {
    id: 'text-2',
    userId: 'user-1',
    name: 'Saudacao Manha',
    body: 'Bom dia! Separamos as melhores ofertas para voce.\n\n{nome_produto}\nPor: {preco_formatado}',
    isActive: true,
    createdAt: new Date(Date.now() - 86400000).toISOString(),
    updatedAt: new Date(Date.now() - 86400000).toISOString(),
  },
  {
    id: 'text-3',
    userId: 'user-1',
    name: 'Rodape Padrao',
    body: '{aviso_preco}\n\nEnvie "SAIR" para deixar de receber ofertas.',
    isActive: true,
    createdAt: new Date(Date.now() - 7 * 24 * 3600000).toISOString(),
    updatedAt: new Date(Date.now() - 3 * 24 * 3600000).toISOString(),
  },
  {
    id: 'text-4',
    userId: 'user-1',
    name: 'Mensagem Desativada',
    body: 'Este texto esta desativado e nao sera usado.',
    isActive: false,
    createdAt: new Date(Date.now() - 14 * 24 * 3600000).toISOString(),
    updatedAt: new Date(Date.now() - 7 * 24 * 3600000).toISOString(),
  },
  {
    id: 'text-5',
    userId: 'user-1',
    name: 'CTA Urgencia',
    body: 'Corra! Oferta por tempo limitado.\n\n{nome_produto}\nDe: ~{preco_original_formatado}~\nPor: *{preco_formatado}*\n\n{link_afiliado}',
    isActive: true,
    createdAt: new Date(Date.now() - 3 * 24 * 3600000).toISOString(),
    updatedAt: new Date(Date.now() - 24 * 3600000).toISOString(),
  },
];

/**
 * Test text data for creation.
 */
export const TEST_DISPATCH_TEXT = {
  name: 'E2E Test Template',
  body: 'Este e um texto de teste criado pelo E2E.\n\n{nome_produto}\nPor: {preco_formatado}\n\n{link_afiliado}',
};

/**
 * ConnectRPC service base path for dispatch texts.
 */
const SERVICE_PATH = 'dispatch_texts.v1.DispatchTextsService';

/**
 * Mocks the dispatch texts list API.
 */
export async function mockDispatchTextsListApi(
  page: Page,
  texts = MOCK_DISPATCH_TEXTS
): Promise<void> {
  await page.route(`**/${SERVICE_PATH}/ListDispatchTexts`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({ dispatchTexts: texts }),
    });
  });
}

/**
 * Mocks the create dispatch text API.
 */
export async function mockCreateDispatchTextApi(
  page: Page,
  scenario: 'success' | 'duplicate' | 'error' = 'success'
): Promise<void> {
  await page.route(`**/${SERVICE_PATH}/CreateDispatchText`, async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: { 'grpc-status': '0' },
          body: JSON.stringify({
            dispatchText: {
              id: 'new-text-id',
              userId: 'user-1',
              name: TEST_DISPATCH_TEXT.name,
              body: TEST_DISPATCH_TEXT.body,
              isActive: true,
              createdAt: new Date().toISOString(),
              updatedAt: new Date().toISOString(),
            },
          }),
        });
        break;
      case 'duplicate':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: {
            'grpc-status': '6', // ALREADY_EXISTS
            'grpc-message': 'Text with this name already exists',
          },
          body: '',
        });
        break;
      case 'error':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: {
            'grpc-status': '13', // INTERNAL
            'grpc-message': 'Failed to create dispatch text',
          },
          body: '',
        });
        break;
    }
  });
}

/**
 * Mocks the update dispatch text API.
 */
export async function mockUpdateDispatchTextApi(
  page: Page,
  scenario: 'success' | 'not_found' | 'error' = 'success'
): Promise<void> {
  await page.route(`**/${SERVICE_PATH}/UpdateDispatchText`, async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: { 'grpc-status': '0' },
          body: JSON.stringify({
            dispatchText: {
              ...MOCK_DISPATCH_TEXTS[0],
              updatedAt: new Date().toISOString(),
            },
          }),
        });
        break;
      case 'not_found':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: {
            'grpc-status': '5', // NOT_FOUND
            'grpc-message': 'Dispatch text not found',
          },
          body: '',
        });
        break;
      case 'error':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: {
            'grpc-status': '13', // INTERNAL
            'grpc-message': 'Failed to update dispatch text',
          },
          body: '',
        });
        break;
    }
  });
}

/**
 * Mocks the delete dispatch text API.
 */
export async function mockDeleteDispatchTextApi(
  page: Page,
  scenario: 'success' | 'in_use' | 'error' = 'success'
): Promise<void> {
  await page.route(`**/${SERVICE_PATH}/DeleteDispatchText`, async (route) => {
    switch (scenario) {
      case 'success':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: { 'grpc-status': '0' },
          body: JSON.stringify({}),
        });
        break;
      case 'in_use':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: {
            'grpc-status': '9', // FAILED_PRECONDITION
            'grpc-message': 'Text is in use by automations',
          },
          body: '',
        });
        break;
      case 'error':
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          headers: {
            'grpc-status': '13', // INTERNAL
            'grpc-message': 'Failed to delete dispatch text',
          },
          body: '',
        });
        break;
    }
  });
}

/**
 * Sets up all common mocks for dispatch texts tests.
 */
export async function setupDispatchTextsMocks(page: Page): Promise<void> {
  await mockDispatchTextsListApi(page);
  await mockCreateDispatchTextApi(page);
  await mockUpdateDispatchTextApi(page);
  await mockDeleteDispatchTextApi(page);
}

/**
 * Sets up mocks for empty dispatch texts state.
 */
export async function setupEmptyDispatchTextsMocks(page: Page): Promise<void> {
  await mockDispatchTextsListApi(page, []);
  await mockCreateDispatchTextApi(page);
}

/**
 * Mocks the toggle dispatch text status API.
 */
export async function mockToggleDispatchTextStatusApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route(`**/${SERVICE_PATH}/UpdateDispatchText`, async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({
          dispatchText: {
            ...MOCK_DISPATCH_TEXTS[0],
            isActive: !MOCK_DISPATCH_TEXTS[0].isActive,
            updatedAt: new Date().toISOString(),
          },
        }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to toggle status',
        },
        body: '',
      });
    }
  });
}

/**
 * Mocks the duplicate dispatch text API.
 */
export async function mockDuplicateDispatchTextApi(
  page: Page,
  scenario: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route(`**/${SERVICE_PATH}/CreateDispatchText`, async (route) => {
    if (scenario === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: { 'grpc-status': '0' },
        body: JSON.stringify({
          dispatchText: {
            ...MOCK_DISPATCH_TEXTS[0],
            id: 'duplicated-text-id',
            name: `${MOCK_DISPATCH_TEXTS[0].name} (copia)`,
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
          },
        }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13',
          'grpc-message': 'Failed to duplicate dispatch text',
        },
        body: '',
      });
    }
  });
}

/**
 * Sets up mocks for filtered dispatch texts.
 */
export async function setupFilteredDispatchTextsMocks(
  page: Page,
  filter: 'active' | 'inactive' = 'active'
): Promise<void> {
  const filtered = MOCK_DISPATCH_TEXTS.filter((t) =>
    filter === 'active' ? t.isActive : !t.isActive
  );
  await mockDispatchTextsListApi(page, filtered);
  await mockCreateDispatchTextApi(page);
  await mockUpdateDispatchTextApi(page);
  await mockDeleteDispatchTextApi(page);
}

/**
 * Helper to simulate network delay for any route.
 */
export async function mockWithDelay(
  page: Page,
  urlPattern: string,
  delayMs: number,
  response: { status: number; body: string; headers?: Record<string, string> }
): Promise<void> {
  await page.route(urlPattern, async (route) => {
    await new Promise((resolve) => setTimeout(resolve, delayMs));
    await route.fulfill({
      status: response.status,
      contentType: 'application/json',
      headers: { 'grpc-status': '0', ...response.headers },
      body: response.body,
    });
  });
}

/**
 * Mocks the list API with a network error.
 */
export async function mockDispatchTextsListApiNetworkError(page: Page): Promise<void> {
  await page.route(`**/${SERVICE_PATH}/ListDispatchTexts`, async (route) => {
    await route.abort('connectionrefused');
  });
}

/**
 * Mocks the list API with a slow response.
 */
export async function mockDispatchTextsListApiSlow(
  page: Page,
  delayMs = 3000
): Promise<void> {
  await page.route(`**/${SERVICE_PATH}/ListDispatchTexts`, async (route) => {
    await new Promise((resolve) => setTimeout(resolve, delayMs));
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'grpc-status': '0' },
      body: JSON.stringify({ dispatchTexts: MOCK_DISPATCH_TEXTS }),
    });
  });
}
