import type { Page } from '@playwright/test';

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
            updatedAt: new Date().toISOString(),
          },
        }),
      });
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        headers: {
          'grpc-status': '13', // INTERNAL
          'grpc-message': 'Failed to update dispatch text',
        },
        body: '',
      });
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
