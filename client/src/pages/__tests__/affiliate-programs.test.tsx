import { render, screen, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules
// ============================================================

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'pt-BR' },
  }),
}));

vi.mock('@/components/error-boundary', () => ({
  ErrorBoundary: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock('react-router', () => ({
  Link: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
}));

// ============================================================
// Mock useAffiliatePrograms — a página é uma casca sobre este hook.
// ============================================================

const STORE_CONFIGS = [
  {
    key: 'shopee',
    name: 'Shopee',
    fields: [
      { key: 'appId', label: 'App ID', type: 'text', placeholder: '12345' },
      { key: 'password', label: 'Senha', type: 'password', placeholder: '***' },
    ],
  },
  {
    key: 'amazon',
    name: 'Amazon Associates',
    fields: [
      { key: 'tag', label: 'Tag de afiliado', type: 'text', placeholder: 'seusite-20' },
      {
        key: 'clientId',
        label: 'Client ID (opcional)',
        type: 'text',
        placeholder: '',
        optional: true,
      },
      {
        key: 'clientSecret',
        label: 'Client Secret (opcional)',
        type: 'password',
        placeholder: '',
        optional: true,
      },
    ],
  },
  {
    key: 'magalu',
    name: 'Magazine Luiza',
    fields: [{ key: 'partnerStore', label: 'Loja Parceiro', type: 'text', placeholder: '' }],
  },
  {
    key: 'aliexpress',
    name: 'AliExpress',
    fields: [
      { key: 'trackingId', label: 'Tracking ID', type: 'text', placeholder: 'default' },
      { key: 'appKey', label: 'App Key', type: 'text', placeholder: '12345' },
      { key: 'appSecret', label: 'App Secret', type: 'password', placeholder: '' },
    ],
  },
  {
    key: 'awin',
    name: 'Awin',
    fields: [
      { key: 'awinId', label: 'Awin ID', type: 'text', placeholder: '123457' },
      { key: 'oauth2Token', label: 'Awin OAuth2 Token', type: 'password', placeholder: '' },
    ],
  },
];

const hookReturn = {
  credentials: {},
  loading: false,
  saving: null,
  testing: null,
  mlLastTest: null,
  storeConfigs: STORE_CONFIGS,
  enabledStores: ['shopee', 'amazon', 'magalu', 'aliexpress', 'awin'],
  awinBrands: ['Netshoes'],
  updateCredentialField: vi.fn(),
  toggleAutoGenerate: vi.fn(),
  toggleBrand: vi.fn(),
  handleSave: vi.fn(),
  handleTest: vi.fn(),
  getStoreDescription: (key: string) => `desc.${key}`,
  formatSoldQuantity: () => null,
  decodeMlToken: vi.fn(),
};

vi.mock('@/hooks', () => ({
  useAffiliatePrograms: () => hookReturn,
}));

import AffiliateProgramsPage from '../affiliate-programs';

describe('/affiliate-programs — o que o usuário vê', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // A entrega pedida: Amazon, Magalu, AliExpress e Awin deixam de exibir
  // "Em breve" e passam a exibir formulário de credencial.
  it('renderiza formulário para as lojas habilitadas', () => {
    render(<AffiliateProgramsPage />);

    const cards = screen.getAllByTestId('store-card');
    const stores = cards.map((c) => c.getAttribute('data-store'));

    expect(stores).toEqual(['shopee', 'amazon', 'magalu', 'aliexpress', 'awin']);
  });

  it('o card da Magalu pede só a loja parceira', () => {
    render(<AffiliateProgramsPage />);

    const magalu = screen
      .getAllByTestId('store-card')
      .find((c) => c.getAttribute('data-store') === 'magalu');
    expect(magalu).toBeDefined();
    expect(within(magalu as HTMLElement).getByLabelText('Loja Parceiro')).toBeDefined();
  });

  it('o card da Amazon pede a tag e NÃO pede mais Access Key / Secret Key', () => {
    render(<AffiliateProgramsPage />);

    const amazon = screen
      .getAllByTestId('store-card')
      .find((c) => c.getAttribute('data-store') === 'amazon');
    expect(amazon).toBeDefined();

    const card = within(amazon as HTMLElement);
    expect(card.getByLabelText('Tag de afiliado')).toBeDefined();
    expect(card.getByLabelText('Client ID (opcional)')).toBeDefined();
    // Credenciais da PA-API 5.0, desligada em 15/05/2026.
    expect(card.queryByLabelText('Access Key')).toBeNull();
    expect(card.queryByLabelText('Secret Key')).toBeNull();
  });

  it('o card do AliExpress pede o App Secret (sem ele não há como assinar)', () => {
    render(<AffiliateProgramsPage />);

    const ali = screen
      .getAllByTestId('store-card')
      .find((c) => c.getAttribute('data-store') === 'aliexpress');
    expect(within(ali as HTMLElement).getByLabelText('App Secret')).toBeDefined();
  });

  // Com a Magalu ligada, toda loja do catálogo tem conversor: nenhum card fica
  // em "Em breve". O ComingSoonCard segue vivo para a próxima loja que entrar.
  it('nenhuma loja continua em "Em breve"', () => {
    render(<AffiliateProgramsPage />);

    expect(screen.queryAllByTestId('coming-soon-card')).toHaveLength(0);
  });
});
