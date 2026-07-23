import { type Page, expect, test } from '@playwright/test';
import { setupAuthMocks } from './fixtures/auth.fixture';

/**
 * E2E das regras de cupom por CATEGORIA oficial da Shopee (migração 075).
 *
 * Dois grupos com um ID COMPARTILHADO (100002) — é o caso que a UI precisa
 * tratar direito: marcar um grupo adiciona a união dos IDs, desmarcar
 * remove os IDs daquele grupo mesmo que outro também os use, e a marcação
 * dos demais é recalculada por cobertura.
 *
 * proto int64 viaja como STRING no JSON do Connect; as asserções abaixo
 * comparam strings de propósito.
 */

const GROUP_A = { name: 'Beleza', ids: ['100001', '100002'] };
const GROUP_B = { name: 'Eletrônicos', ids: ['100002', '100003'] };

const GENERIC_RULE = {
  id: 'rule-generic',
  userId: 'user-123',
  couponId: '',
  platform: 'shopee',
  minPrice: 0,
  priority: 0,
  isActive: true,
  officialOnly: false,
  message: 'Cupom generico',
  createdAt: '2026-07-01T00:00:00Z',
  updatedAt: '2026-07-01T00:00:00Z',
  productCategoryIds: [] as string[],
};

const CATEGORY_RULE = {
  ...GENERIC_RULE,
  id: 'rule-category',
  message: 'Cupom de categoria',
  productCategoryIds: ['100001', '100002', '100003'],
};

type Captured = { body: Record<string, unknown> | null };

async function setupRulesMocks(
  page: Page,
  opts: {
    rules?: unknown[];
    categories?: 'ok' | 'error' | 'error-then-ok';
    created?: Captured;
    updated?: Captured;
    /** Coletor do guard: qualquer RPC não mockado é registrado aqui. */
    problems?: string[];
  } = {}
) {
  const rules = opts.rules ?? [GENERIC_RULE];

  // Registrado PRIMEIRO de propósito: no Playwright a última rota casada
  // vence, então este catch-all só atende o que ninguém mais mockou — e aí
  // o teste falha por request não mockado em vez de estourar 500 do proxy
  // do vite (que não tem backend nesta suíte).
  await page.route('**/connect/**', (route) => {
    opts.problems?.push(`RPC não mockado: ${route.request().url()}`);
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({}),
    });
  });

  // App shell da página (fora do escopo desta feature, mas precisa existir
  // para a rota renderizar sem erro).
  await page.route('**/settings.v1.SettingsService/GetSettings', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({}) })
  );
  await page.route('**/profiles.v1.ProfilesService/GetProfile', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({}) })
  );

  await page.route('**/coupons.v1.CouponsService/ListCouponRules', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ rules }),
    })
  );

  let categoryCalls = 0;
  await page.route('**/affiliates.v1.ShopeeService/ListCategories', (route) => {
    categoryCalls++;
    const fail =
      opts.categories === 'error' || (opts.categories === 'error-then-ok' && categoryCalls === 1);
    if (fail) {
      return route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'internal', message: 'failed to list categories' }),
      });
    }
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ groups: [GROUP_A, GROUP_B] }),
    });
  });

  await page.route('**/coupons.v1.CouponsService/CreateCouponRule', async (route) => {
    if (opts.created) {
      opts.created.body = JSON.parse(route.request().postData() ?? '{}');
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ rule: { ...CATEGORY_RULE, id: 'rule-new' } }),
    });
  });

  await page.route('**/coupons.v1.CouponsService/UpdateCouponRule', async (route) => {
    if (opts.updated) {
      opts.updated.body = JSON.parse(route.request().postData() ?? '{}');
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ rule: CATEGORY_RULE }),
    });
  });

  // Re-registra o auth POR ÚLTIMO: no Playwright a rota mais recente vence,
  // e o catch-all acima engoliria o GetCurrentUser do beforeEach.
  await setupAuthMocks(page);
}

/** Falha o teste em erro de console e em qualquer request não mockado. */
function guardPage(page: Page) {
  const problems: string[] = [];
  page.on('console', (msg) => {
    if (msg.type() === 'error') problems.push(`console: ${msg.text()}`);
  });
  page.on('requestfailed', (req) => {
    problems.push(`request falhou: ${req.url()}`);
  });
  return problems;
}

async function openNewRuleDialog(page: Page) {
  await page.goto('/coupons/rules');
  await page.getByRole('button', { name: 'Adicionar Regra' }).click();
  await expect(page.getByTestId('rule-categories-section')).toBeVisible();
}

async function fillMessage(page: Page, text: string) {
  await page.locator('#rule-message').fill(text);
}

function categoryCheckbox(page: Page, name: string) {
  return page.locator(`#rule-cat-${name}`);
}

for (const variant of [
  { label: 'desktop', viewport: null },
  { label: 'mobile', viewport: { width: 390, height: 844 } },
] as const) {
  test.describe(`Regras de cupom por categoria (${variant.label})`, () => {
    test.beforeEach(async ({ page }) => {
      if (variant.viewport) await page.setViewportSize(variant.viewport);
      await setupAuthMocks(page);
    });

    test('seleciona grupo, envia união única em bigint e mostra a contagem', async ({ page }) => {
      const problems = guardPage(page);
      const created: Captured = { body: null };
      await setupRulesMocks(page, { created, problems });

      await openNewRuleDialog(page);
      await expect(page.getByTestId('rule-categories-summary')).toHaveText('Todas as categorias');

      // No viewport mobile (390x844) a grade, a ação de limpar e o botão de
      // salvar precisam continuar visíveis e operáveis — os cliques abaixo
      // só passam com o elemento realmente acionável.
      await expect(categoryCheckbox(page, GROUP_A.name)).toBeVisible();
      await expect(page.getByRole('button', { name: 'Todas as categorias' })).toBeVisible();

      // Marcar os dois grupos: 100002 é compartilhado e não pode duplicar.
      await categoryCheckbox(page, GROUP_A.name).click();
      await categoryCheckbox(page, GROUP_B.name).click();
      await expect(page.getByTestId('rule-categories-summary')).toHaveText('3 IDs de categoria');

      // Desmarcar Eletrônicos remove 100002 e 100003 — inclusive o
      // compartilhado — então Beleza deixa de estar coberto.
      await categoryCheckbox(page, GROUP_B.name).click();
      await expect(page.getByTestId('rule-categories-summary')).toHaveText('1 IDs de categoria');
      await expect(categoryCheckbox(page, GROUP_A.name)).not.toBeChecked();

      // Remarcar Eletrônicos devolve 100002+100003, e Beleza volta a ficar
      // coberto sozinho (o compartilhado voltou) — cobertura é derivada.
      await categoryCheckbox(page, GROUP_B.name).click();
      await expect(page.getByTestId('rule-categories-summary')).toHaveText('3 IDs de categoria');
      await expect(categoryCheckbox(page, GROUP_A.name)).toBeChecked();

      await fillMessage(page, 'Cupom de categoria');
      await page.getByRole('button', { name: 'Criar' }).click();

      await expect.poll(() => created.body).not.toBeNull();
      expect(created.body?.productCategoryIds).toEqual(['100001', '100002', '100003']);
      expect(created.body?.platform).toBe('shopee');

      expect(problems).toEqual([]);
    });

    test('editar sem mexer nas categorias reenvia a lista, sem clear', async ({ page }) => {
      const problems = guardPage(page);
      const updated: Captured = { body: null };
      await setupRulesMocks(page, { rules: [CATEGORY_RULE], updated, problems });

      await page.goto('/coupons/rules');
      await expect(page.getByText('3 IDs')).toBeVisible();

      await page.getByRole('button', { name: 'Editar' }).first().click();
      await expect(page.getByTestId('rule-categories-summary')).toHaveText('3 IDs de categoria');

      await page.getByRole('button', { name: 'Salvar' }).click();

      await expect.poll(() => updated.body).not.toBeNull();
      expect(updated.body?.productCategoryIds).toEqual(['100001', '100002', '100003']);
      expect(updated.body?.clearProductCategoryIds).toBeFalsy();

      expect(problems).toEqual([]);
    });

    test('"Todas as categorias" limpa e envia clear=true', async ({ page }) => {
      const problems = guardPage(page);
      const updated: Captured = { body: null };
      await setupRulesMocks(page, { rules: [CATEGORY_RULE], updated, problems });

      await page.goto('/coupons/rules');
      await page.getByRole('button', { name: 'Editar' }).first().click();

      await page.getByRole('button', { name: 'Todas as categorias' }).click();
      await expect(page.getByTestId('rule-categories-summary')).toHaveText('Todas as categorias');

      await page.getByRole('button', { name: 'Salvar' }).click();

      await expect.poll(() => updated.body).not.toBeNull();
      expect(updated.body?.productCategoryIds ?? []).toEqual([]);
      expect(updated.body?.clearProductCategoryIds).toBe(true);

      expect(problems).toEqual([]);
    });

    test('trocar para Mercado Livre esconde a seção e limpa categorias + loja oficial', async ({
      page,
    }) => {
      const problems = guardPage(page);
      const updated: Captured = { body: null };
      await setupRulesMocks(page, { rules: [CATEGORY_RULE], updated, problems });

      await page.goto('/coupons/rules');
      await page.getByRole('button', { name: 'Editar' }).first().click();
      await expect(page.getByTestId('rule-categories-section')).toBeVisible();

      await page.locator('#rule-platform').click();
      await page.getByRole('option', { name: 'Mercado Livre' }).click();
      await expect(page.getByTestId('rule-categories-section')).toHaveCount(0);

      await page.getByRole('button', { name: 'Salvar' }).click();

      await expect.poll(() => updated.body).not.toBeNull();
      expect(updated.body?.platform).toBe('mercado_livre');
      expect(updated.body?.productCategoryIds ?? []).toEqual([]);
      expect(updated.body?.clearProductCategoryIds).toBe(true);
      expect(updated.body?.officialOnly).toBeFalsy();

      expect(problems).toEqual([]);
    });

    test('falha do catálogo mostra retry, preserva IDs e permite salvar genérica', async ({
      page,
    }) => {
      const updated: Captured = { body: null };
      await setupRulesMocks(page, {
        rules: [CATEGORY_RULE],
        categories: 'error-then-ok',
        updated,
      });

      await page.goto('/coupons/rules');
      await page.getByRole('button', { name: 'Editar' }).first().click();

      const alert = page.getByRole('alert').filter({ hasText: 'categorias da Shopee' });
      await expect(alert).toBeVisible();
      // Falha do catálogo NÃO é catálogo vazio: os IDs da regra continuam lá.
      await expect(page.getByTestId('rule-categories-summary')).toHaveText('3 IDs de categoria');
      // E a listagem de regras continua de pé.
      await expect(page.getByRole('button', { name: 'Salvar' })).toBeEnabled();

      await alert.getByRole('button', { name: 'Tentar novamente' }).click();
      await expect(categoryCheckbox(page, GROUP_A.name)).toBeVisible();
      await expect(page.getByTestId('rule-categories-summary')).toHaveText('3 IDs de categoria');
    });
  });
}
