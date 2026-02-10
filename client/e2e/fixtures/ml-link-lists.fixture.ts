import type { Page } from '@playwright/test';
import { setupAuthMocks } from './auth.fixture';

// ============================================================
// Mock Data
// ============================================================

export const MOCK_ML_LINK_LISTS = [
  {
    id: 'list-001',
    userId: 'user-123',
    name: 'Lista de Ofertas',
    description: 'Melhores ofertas do Mercado Livre',
    isActive: true,
    createdAt: '2026-01-15T10:00:00Z',
    updatedAt: '2026-01-15T10:00:00Z',
  },
  {
    id: 'list-002',
    userId: 'user-123',
    name: 'Produtos Tech',
    description: 'Eletrônicos e gadgets',
    isActive: true,
    createdAt: '2026-01-16T10:00:00Z',
    updatedAt: '2026-01-16T10:00:00Z',
  },
];

export const MOCK_ML_LINK_LIST_ITEMS = [
  {
    id: 'item-001',
    listId: 'list-001',
    originUrl: 'https://www.mercadolivre.com.br/p/MLB12345678',
    orderIndex: 0,
    isActive: true,
    consumedAt: null,
    consumedByAutomationId: null,
    consumedByGroupId: null,
    productTitle: 'Smartphone Samsung Galaxy S24',
    productPrice: 2999.99,
    productImageUrl: 'https://http2.mlstatic.com/D_NQ_NP_123456-MLB12345678_012026-O.webp',
    metadata: '{}',
    createdAt: '2026-01-15T11:00:00Z',
    updatedAt: '2026-01-15T11:00:00Z',
  },
  {
    id: 'item-002',
    listId: 'list-001',
    originUrl: 'https://www.mercadolivre.com.br/p/MLB87654321',
    orderIndex: 1,
    isActive: true,
    consumedAt: '2026-01-20T15:00:00Z',
    consumedByAutomationId: 'auto-001',
    consumedByGroupId: 'group-001@g.us',
    productTitle: 'Fone de Ouvido JBL Tune 510BT',
    productPrice: 199.9,
    productImageUrl: 'https://http2.mlstatic.com/D_NQ_NP_654321-MLB87654321_012026-O.webp',
    metadata: '{}',
    createdAt: '2026-01-15T12:00:00Z',
    updatedAt: '2026-01-20T15:00:00Z',
  },
  {
    id: 'item-003',
    listId: 'list-001',
    originUrl: 'https://www.mercadolivre.com.br/p/MLB11223344',
    orderIndex: 2,
    isActive: false,
    consumedAt: null,
    consumedByAutomationId: null,
    consumedByGroupId: null,
    productTitle: 'Tablet Samsung Galaxy Tab A9',
    productPrice: 899.0,
    productImageUrl: 'https://http2.mlstatic.com/D_NQ_NP_112233-MLB11223344_012026-O.webp',
    metadata: '{}',
    createdAt: '2026-01-15T13:00:00Z',
    updatedAt: '2026-01-15T13:00:00Z',
  },
];

export const TEST_ML_LINK_LIST = {
  name: 'Nova Lista de Teste',
  description: 'Descrição da lista de teste',
  is_active: true,
};

export const TEST_VALID_ML_URL = 'https://www.mercadolivre.com.br/produto-teste/p/MLB99887766';
export const TEST_INVALID_URL = 'https://amazon.com.br/produto';
export const TEST_DUPLICATE_URL = MOCK_ML_LINK_LIST_ITEMS[0].originUrl;

export const MOCK_PREVIEW = {
  title: 'Produto Teste Preview',
  price: 1599.99,
  currency: 'BRL',
  imageUrl: 'https://http2.mlstatic.com/D_NQ_NP_preview-MLB99887766_012026-O.webp',
  sellerName: 'Loja Teste',
};

// ============================================================
// Mock Setup Functions
// ============================================================

/**
 * Sets up all mocks for ML Link Lists page with data.
 */
export async function setupMlLinkListsMocks(page: Page): Promise<void> {
  await setupAuthMocks(page);

  // Mock ListMlLinkLists RPC
  await page.route('**/ml_links.v1.MlLinksService/ListMlLinkLists', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        lists: MOCK_ML_LINK_LISTS,
      }),
    });
  });

  // Mock ListMlLinkListItems RPC
  await page.route('**/ml_links.v1.MlLinksService/ListMlLinkListItems', async (route) => {
    const body = await route.request().postDataJSON();
    const listId = body?.listId;

    const items = MOCK_ML_LINK_LIST_ITEMS.filter((i) => i.listId === listId);

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items,
        pagination: {
          nextPageToken: '',
          totalCount: items.length,
          hasMore: false,
        },
      }),
    });
  });
}

/**
 * Sets up mocks for empty ML Link Lists state.
 */
export async function setupEmptyMlLinkListsMocks(page: Page): Promise<void> {
  await setupAuthMocks(page);

  await page.route('**/ml_links.v1.MlLinksService/ListMlLinkLists', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ lists: [] }),
    });
  });
}

/**
 * Mocks CreateMlLinkList RPC.
 */
export async function mockCreateListApi(
  page: Page,
  result: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/ml_links.v1.MlLinksService/CreateMlLinkList', async (route) => {
    if (result === 'success') {
      const body = await route.request().postDataJSON();
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          list: {
            id: 'list-new',
            userId: 'user-123',
            name: body?.name || 'Nova Lista',
            description: body?.description || '',
            isActive: body?.isActive ?? true,
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
          },
        }),
      });
    } else {
      await route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'invalid_argument', message: 'Invalid list data' }),
      });
    }
  });
}

/**
 * Mocks UpdateMlLinkList RPC.
 */
export async function mockUpdateListApi(
  page: Page,
  result: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/ml_links.v1.MlLinksService/UpdateMlLinkList', async (route) => {
    if (result === 'success') {
      const body = await route.request().postDataJSON();
      const existing = MOCK_ML_LINK_LISTS.find((l) => l.id === body?.id);
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          list: {
            ...existing,
            ...body,
            updatedAt: new Date().toISOString(),
          },
        }),
      });
    } else {
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'not_found', message: 'List not found' }),
      });
    }
  });
}

/**
 * Mocks DeleteMlLinkList RPC.
 */
export async function mockDeleteListApi(
  page: Page,
  result: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/ml_links.v1.MlLinksService/DeleteMlLinkList', async (route) => {
    if (result === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({}),
      });
    } else {
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'not_found', message: 'List not found' }),
      });
    }
  });
}

/**
 * Mocks CreateMlLinkListItem RPC.
 */
export async function mockCreateItemApi(
  page: Page,
  result: 'success' | 'error' | 'duplicate' = 'success'
): Promise<void> {
  await page.route('**/ml_links.v1.MlLinksService/CreateMlLinkListItem', async (route) => {
    if (result === 'success') {
      const body = await route.request().postDataJSON();
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          item: {
            id: 'item-new',
            listId: body?.listId,
            originUrl: body?.originUrl,
            orderIndex: body?.orderIndex ?? 0,
            isActive: body?.isActive ?? true,
            consumedAt: null,
            productTitle: body?.productTitle || null,
            productPrice: body?.productPrice || null,
            productImageUrl: body?.productImageUrl || null,
            metadata: body?.metadata || '{}',
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
          },
        }),
      });
    } else if (result === 'duplicate') {
      await route.fulfill({
        status: 409,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'already_exists', message: 'URL_ALREADY_EXISTS' }),
      });
    } else {
      await route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'invalid_argument', message: 'INVALID_MERCADOLIVRE_URL' }),
      });
    }
  });
}

/**
 * Mocks BulkCreateMlLinkListItems RPC.
 */
export async function mockBulkCreateItemsApi(
  page: Page,
  result: 'success' | 'partial' | 'error' = 'success'
): Promise<void> {
  await page.route('**/ml_links.v1.MlLinksService/BulkCreateMlLinkListItems', async (route) => {
    const body = await route.request().postDataJSON();
    const urls = body?.originUrls || [];

    if (result === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          createdCount: urls.length,
          skippedCount: 0,
          skippedUrls: [],
          items: urls.map((url: string, i: number) => ({
            id: `item-bulk-${i}`,
            listId: body?.listId,
            originUrl: url,
            orderIndex: (body?.baseOrderIndex ?? 0) + i,
            isActive: true,
            consumedAt: null,
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
          })),
        }),
      });
    } else if (result === 'partial') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          createdCount: 1,
          skippedCount: urls.length - 1,
          skippedUrls: urls.slice(1),
          items: [
            {
              id: 'item-bulk-0',
              listId: body?.listId,
              originUrl: urls[0],
              orderIndex: body?.baseOrderIndex ?? 0,
              isActive: true,
              consumedAt: null,
              createdAt: new Date().toISOString(),
              updatedAt: new Date().toISOString(),
            },
          ],
        }),
      });
    } else {
      await route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'invalid_argument', message: 'No valid URLs provided' }),
      });
    }
  });
}

/**
 * Mocks UpdateMlLinkListItem RPC.
 */
export async function mockUpdateItemApi(
  page: Page,
  result: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/ml_links.v1.MlLinksService/UpdateMlLinkListItem', async (route) => {
    if (result === 'success') {
      const body = await route.request().postDataJSON();
      const existing = MOCK_ML_LINK_LIST_ITEMS.find((i) => i.id === body?.id);
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          item: {
            ...existing,
            isActive: body?.isActive ?? existing?.isActive,
            orderIndex: body?.orderIndex ?? existing?.orderIndex,
            updatedAt: new Date().toISOString(),
          },
        }),
      });
    } else {
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'not_found', message: 'Item not found' }),
      });
    }
  });
}

/**
 * Mocks DeleteMlLinkListItem RPC.
 */
export async function mockDeleteItemApi(
  page: Page,
  result: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/ml_links.v1.MlLinksService/DeleteMlLinkListItem', async (route) => {
    if (result === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({}),
      });
    } else {
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'not_found', message: 'Item not found' }),
      });
    }
  });
}

/**
 * Mocks ResetConsumedItems RPC.
 */
export async function mockResetConsumedApi(
  page: Page,
  result: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/ml_links.v1.MlLinksService/ResetConsumedItems', async (route) => {
    if (result === 'success') {
      const consumedCount = MOCK_ML_LINK_LIST_ITEMS.filter((i) => i.consumedAt !== null).length;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ resetCount: consumedCount }),
      });
    } else {
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'not_found', message: 'List not found' }),
      });
    }
  });
}

/**
 * Mocks ReorderItems RPC.
 */
export async function mockReorderItemsApi(
  page: Page,
  result: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/ml_links.v1.MlLinksService/ReorderItems', async (route) => {
    if (result === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({}),
      });
    } else {
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'not_found', message: 'List not found' }),
      });
    }
  });
}

/**
 * Mocks MercadoLivre Preview RPC.
 */
export async function mockPreviewApi(
  page: Page,
  result: 'success' | 'error' = 'success'
): Promise<void> {
  await page.route('**/affiliates.v1.MercadoLivreService/Preview', async (route) => {
    if (result === 'success') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ preview: MOCK_PREVIEW }),
      });
    } else {
      await route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'not_found', message: 'Product not found' }),
      });
    }
  });
}
