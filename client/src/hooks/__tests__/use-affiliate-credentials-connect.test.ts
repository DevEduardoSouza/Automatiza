import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules (inline values to avoid TDZ issues with vi.mock hoisting)
// ============================================================

vi.mock('@/lib/sentry', () => ({
  captureHookError: vi.fn(),
  safeLocalStorage: {
    getItem: vi.fn().mockReturnValue(null),
    setItem: vi.fn(),
    removeItem: vi.fn(),
  },
}));

vi.mock('@sentry/react', () => ({
  captureMessage: vi.fn(),
}));

const mockToast = vi.fn();
vi.mock('@/hooks/use-toast', () => ({
  useToast: () => ({
    toast: mockToast,
    toasts: [],
    dismiss: vi.fn(),
  }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'pt-BR' },
  }),
}));

const mockUser = { id: 'user-123', email: 'test@example.com' };
let mockUserValue: typeof mockUser | null = mockUser;

vi.mock('@/contexts', () => ({
  useAuth: () => ({
    user: mockUserValue,
    session: null,
    loading: false,
  }),
}));

vi.mock('@/lib/connect-transport', () => ({
  getErrorInfo: (error: unknown) => ({
    code: 'UNKNOWN',
    message: error instanceof Error ? error.message : 'Unknown error',
    userMessage: error instanceof Error ? error.message : 'Unknown error',
    action: 'toast',
    retryable: false,
  }),
}));

// ============================================================
// Mock Transport and RPC Client
// ============================================================

const mockClientMethods: Record<string, ReturnType<typeof vi.fn>> = {
  listCredentials: vi.fn(),
  getCredential: vi.fn(),
  upsertCredential: vi.fn(),
  deleteCredential: vi.fn(),
  listActiveStores: vi.fn(),
};

vi.mock('@connectrpc/connect', () => ({
  createClient: () => mockClientMethods,
}));

vi.mock('@connectrpc/connect-query', () => ({
  useTransport: () => ({}),
}));

// ============================================================
// Mock Proto Modules (generated code)
// ============================================================

const AffiliateStoreEnum = {
  UNSPECIFIED: 0,
  SHOPEE: 1,
  AMAZON: 2,
  MAGALU: 3,
  MERCADO_LIVRE: 4,
  SHEIN: 5,
  ALIEXPRESS: 6,
  AWIN: 7,
} as const;

vi.mock('@/gen/affiliates/v1/credentials_pb', () => ({
  AffiliateStore: {
    UNSPECIFIED: 0,
    SHOPEE: 1,
    AMAZON: 2,
    MAGALU: 3,
    MERCADO_LIVRE: 4,
    SHEIN: 5,
    ALIEXPRESS: 6,
    AWIN: 7,
  },
  AffiliateCredentialsService: { typeName: 'affiliates.v1.AffiliateCredentialsService' },
  ListCredentialsRequestSchema: {},
  GetCredentialRequestSchema: {},
  UpsertCredentialRequestSchema: {},
  DeleteCredentialRequestSchema: {},
  ListActiveStoresRequestSchema: {},
}));

vi.mock('@bufbuild/protobuf', () => ({
  create: (_schema: unknown, data: unknown) => data,
}));

// ============================================================
// Mock React Query (useQuery + useMutation)
// ============================================================

interface MutationOptions<TData = unknown, TError = unknown, TVariables = unknown> {
  mutationFn: (variables: TVariables) => Promise<TData>;
  onSuccess?: (data: TData) => void;
  onError?: (error: TError) => void;
}

interface QueryOptions {
  queryKey: unknown[];
  queryFn: () => Promise<unknown>;
  enabled?: boolean;
  staleTime?: number;
}

const mockUseMutation = vi.fn();
const mockUseQuery = vi.fn();
const mockInvalidateQueries = vi.fn();
const mockQueryClient = { invalidateQueries: mockInvalidateQueries };

vi.mock('@tanstack/react-query', () => ({
  useMutation: (options: MutationOptions) => mockUseMutation(options),
  useQuery: (options: QueryOptions) => mockUseQuery(options),
  useQueryClient: () => mockQueryClient,
}));

// ============================================================
// Import after mocks
// ============================================================

import { useAffiliateCredentialsConnect } from '../use-affiliate-credentials-connect';
import type {
  LocalActiveStore,
  LocalAffiliateCredential,
  LocalAffiliateStore,
} from '../use-affiliate-credentials-connect';

// ============================================================
// Test Data Factories
// ============================================================

function createMockProtoCredential(overrides: Record<string, unknown> = {}) {
  return {
    id: 'cred-1',
    userId: 'user-123',
    store: AffiliateStoreEnum.SHOPEE,
    credentials: JSON.stringify({ appId: 'abc', secret: 'xyz' }),
    autoGenerate: true,
    isActive: true,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-02T00:00:00Z',
    ...overrides,
  };
}

function createMockProtoActiveStore(overrides: Record<string, unknown> = {}) {
  return {
    store: AffiliateStoreEnum.SHOPEE,
    isConfigured: true,
    autoGenerate: false,
    ...overrides,
  };
}

// ============================================================
// Helpers
// ============================================================

let credentialsQueryFn: (() => Promise<unknown>) | null = null;
let activeStoresQueryFn: (() => Promise<unknown>) | null = null;
let credentialsQueryData: unknown = undefined;
let activeStoresQueryData: unknown = undefined;
let loadingCredentials = false;
let loadingActiveStores = false;

const mockRefetchCredentials = vi.fn().mockResolvedValue(undefined);
const mockRefetchActiveStores = vi.fn().mockResolvedValue(undefined);

function setupDefaultQueryMock() {
  let queryCallIndex = 0;
  mockUseQuery.mockImplementation((options: QueryOptions) => {
    const index = queryCallIndex++;
    if (index === 0) {
      // First useQuery call: credentials
      credentialsQueryFn = options.queryFn as () => Promise<unknown>;
      return {
        data: credentialsQueryData,
        isLoading: loadingCredentials,
        refetch: mockRefetchCredentials,
      };
    }
    // Second useQuery call: active stores
    activeStoresQueryFn = options.queryFn as () => Promise<unknown>;
    return {
      data: activeStoresQueryData,
      isLoading: loadingActiveStores,
      refetch: mockRefetchActiveStores,
    };
  });
}

function setupDefaultMutationMock() {
  mockUseMutation.mockImplementation((options: MutationOptions) => ({
    mutateAsync: async (variables: unknown) => {
      try {
        const result = await options.mutationFn(variables);
        if (options.onSuccess) {
          options.onSuccess(result);
        }
        return result;
      } catch (err) {
        if (options.onError) {
          options.onError(err);
        }
        throw err;
      }
    },
    isPending: false,
  }));
}

function renderAndGetHook() {
  return renderHook(() => useAffiliateCredentialsConnect());
}

// ============================================================
// Tests
// ============================================================

describe('useAffiliateCredentialsConnect', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUserValue = mockUser;
    credentialsQueryData = undefined;
    activeStoresQueryData = undefined;
    loadingCredentials = false;
    loadingActiveStores = false;
    credentialsQueryFn = null;
    activeStoresQueryFn = null;

    setupDefaultQueryMock();
    setupDefaultMutationMock();

    // Default client responses
    mockClientMethods.listCredentials.mockResolvedValue({
      credentials: [createMockProtoCredential()],
    });
    mockClientMethods.getCredential.mockResolvedValue({
      credential: createMockProtoCredential(),
    });
    mockClientMethods.upsertCredential.mockResolvedValue({
      credential: createMockProtoCredential(),
    });
    mockClientMethods.deleteCredential.mockResolvedValue({});
    mockClientMethods.listActiveStores.mockResolvedValue({
      stores: [createMockProtoActiveStore()],
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── toProtoStore - via upsertCredential ─────────────────────

  describe('toProtoStore - via upsertCredential', () => {
    it('should convert shopee to AffiliateStore.SHOPEE', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.upsertCredential({
          store: 'shopee',
          credentials: { appId: '123' },
          auto_generate: true,
          is_active: true,
        });
      });

      expect(mockClientMethods.upsertCredential).toHaveBeenCalledWith(
        expect.objectContaining({ store: AffiliateStoreEnum.SHOPEE })
      );
    });

    it('should convert amazon to AffiliateStore.AMAZON', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.upsertCredential({
          store: 'amazon',
          credentials: { accessKey: 'abc' },
        });
      });

      expect(mockClientMethods.upsertCredential).toHaveBeenCalledWith(
        expect.objectContaining({ store: AffiliateStoreEnum.AMAZON })
      );
    });

    it('should convert magalu to AffiliateStore.MAGALU', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.upsertCredential({
          store: 'magalu',
          credentials: { token: 'xyz' },
        });
      });

      expect(mockClientMethods.upsertCredential).toHaveBeenCalledWith(
        expect.objectContaining({ store: AffiliateStoreEnum.MAGALU })
      );
    });

    it('should convert ml to AffiliateStore.MERCADO_LIVRE', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.upsertCredential({
          store: 'ml',
          credentials: { tag: 'my-tag' },
        });
      });

      expect(mockClientMethods.upsertCredential).toHaveBeenCalledWith(
        expect.objectContaining({ store: AffiliateStoreEnum.MERCADO_LIVRE })
      );
    });

    it('should convert shein to AffiliateStore.SHEIN', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.upsertCredential({
          store: 'shein',
          credentials: { apiKey: 'key' },
        });
      });

      expect(mockClientMethods.upsertCredential).toHaveBeenCalledWith(
        expect.objectContaining({ store: AffiliateStoreEnum.SHEIN })
      );
    });

    it('should convert aliexpress to AffiliateStore.ALIEXPRESS', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.upsertCredential({
          store: 'aliexpress',
          credentials: { apiKey: 'ali-key' },
        });
      });

      expect(mockClientMethods.upsertCredential).toHaveBeenCalledWith(
        expect.objectContaining({ store: AffiliateStoreEnum.ALIEXPRESS })
      );
    });

    it('should convert awin to AffiliateStore.AWIN', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.upsertCredential({
          store: 'awin',
          credentials: { publisherId: '1234' },
        });
      });

      expect(mockClientMethods.upsertCredential).toHaveBeenCalledWith(
        expect.objectContaining({ store: AffiliateStoreEnum.AWIN })
      );
    });
  });

  // ── fromProtoStore - via query data ─────────────────────────

  describe('fromProtoStore - via query data', () => {
    it('should convert SHOPEE proto store to local shopee', () => {
      credentialsQueryData = [
        {
          id: 'cred-1',
          user_id: 'user-123',
          store: 'shopee',
          credentials: { appId: 'abc' },
          auto_generate: true,
          is_active: true,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-02T00:00:00Z',
        },
      ];
      const { result } = renderAndGetHook();

      expect(result.current.credentials[0].store).toBe('shopee');
    });

    it('should convert AMAZON proto store to local amazon via query function', async () => {
      mockClientMethods.listCredentials.mockResolvedValue({
        credentials: [createMockProtoCredential({ store: AffiliateStoreEnum.AMAZON })],
      });

      renderAndGetHook();

      expect(credentialsQueryFn).toBeDefined();
      const data = (await credentialsQueryFn!()) as LocalAffiliateCredential[];
      expect(data[0].store).toBe('amazon');
    });

    it('should convert MAGALU proto store to local magalu via query function', async () => {
      mockClientMethods.listCredentials.mockResolvedValue({
        credentials: [createMockProtoCredential({ store: AffiliateStoreEnum.MAGALU })],
      });

      renderAndGetHook();
      const data = (await credentialsQueryFn!()) as LocalAffiliateCredential[];
      expect(data[0].store).toBe('magalu');
    });

    it('should convert MERCADO_LIVRE proto store to local ml via query function', async () => {
      mockClientMethods.listCredentials.mockResolvedValue({
        credentials: [createMockProtoCredential({ store: AffiliateStoreEnum.MERCADO_LIVRE })],
      });

      renderAndGetHook();
      const data = (await credentialsQueryFn!()) as LocalAffiliateCredential[];
      expect(data[0].store).toBe('ml');
    });

    it('should convert SHEIN proto store to local shein via query function', async () => {
      mockClientMethods.listCredentials.mockResolvedValue({
        credentials: [createMockProtoCredential({ store: AffiliateStoreEnum.SHEIN })],
      });

      renderAndGetHook();
      const data = (await credentialsQueryFn!()) as LocalAffiliateCredential[];
      expect(data[0].store).toBe('shein');
    });

    it('should convert ALIEXPRESS proto store to local aliexpress via query function', async () => {
      mockClientMethods.listCredentials.mockResolvedValue({
        credentials: [createMockProtoCredential({ store: AffiliateStoreEnum.ALIEXPRESS })],
      });

      renderAndGetHook();
      const data = (await credentialsQueryFn!()) as LocalAffiliateCredential[];
      expect(data[0].store).toBe('aliexpress');
    });

    it('should convert AWIN proto store to local awin via query function', async () => {
      mockClientMethods.listCredentials.mockResolvedValue({
        credentials: [createMockProtoCredential({ store: AffiliateStoreEnum.AWIN })],
      });

      renderAndGetHook();
      const data = (await credentialsQueryFn!()) as LocalAffiliateCredential[];
      expect(data[0].store).toBe('awin');
    });

    it('should fallback UNSPECIFIED proto store to shopee', async () => {
      mockClientMethods.listCredentials.mockResolvedValue({
        credentials: [createMockProtoCredential({ store: AffiliateStoreEnum.UNSPECIFIED })],
      });

      renderAndGetHook();
      const data = (await credentialsQueryFn!()) as LocalAffiliateCredential[];
      expect(data[0].store).toBe('shopee');
    });
  });

  // ── fromProtoCredential - via query data ────────────────────

  describe('fromProtoCredential - via query data', () => {
    it('should map all proto credential fields correctly', async () => {
      const protoCred = createMockProtoCredential({
        id: 'cred-99',
        userId: 'user-456',
        store: AffiliateStoreEnum.AMAZON,
        credentials: JSON.stringify({ accessKey: 'k1', secretKey: 'k2' }),
        autoGenerate: false,
        isActive: true,
        createdAt: '2026-02-10T12:00:00Z',
        updatedAt: '2026-02-11T14:00:00Z',
      });

      mockClientMethods.listCredentials.mockResolvedValue({
        credentials: [protoCred],
      });

      renderAndGetHook();
      const data = (await credentialsQueryFn!()) as LocalAffiliateCredential[];

      expect(data[0]).toEqual({
        id: 'cred-99',
        user_id: 'user-456',
        store: 'amazon',
        credentials: { accessKey: 'k1', secretKey: 'k2' },
        auto_generate: false,
        is_active: true,
        created_at: '2026-02-10T12:00:00Z',
        updated_at: '2026-02-11T14:00:00Z',
      });
    });

    it('should parse valid JSON credentials string', async () => {
      mockClientMethods.listCredentials.mockResolvedValue({
        credentials: [
          createMockProtoCredential({
            credentials: JSON.stringify({ apiKey: 'my-key', apiSecret: 'my-secret' }),
          }),
        ],
      });

      renderAndGetHook();
      const data = (await credentialsQueryFn!()) as LocalAffiliateCredential[];

      expect(data[0].credentials).toEqual({ apiKey: 'my-key', apiSecret: 'my-secret' });
    });

    it('should fallback to empty object for invalid JSON credentials', async () => {
      mockClientMethods.listCredentials.mockResolvedValue({
        credentials: [
          createMockProtoCredential({
            credentials: 'not-valid-json{{{',
          }),
        ],
      });

      renderAndGetHook();
      const data = (await credentialsQueryFn!()) as LocalAffiliateCredential[];

      expect(data[0].credentials).toEqual({});
    });

    it('should call Sentry.captureMessage on JSON parse failure', async () => {
      const Sentry = await import('@sentry/react');

      mockClientMethods.listCredentials.mockResolvedValue({
        credentials: [
          createMockProtoCredential({
            credentials: '<<<invalid>>>',
            store: AffiliateStoreEnum.MAGALU,
          }),
        ],
      });

      renderAndGetHook();
      await credentialsQueryFn!();

      expect(Sentry.captureMessage).toHaveBeenCalledWith(
        expect.stringContaining('Failed to parse affiliate credential JSON'),
        'warning'
      );
    });

    it('should call captureHookError on JSON parse failure', async () => {
      const { captureHookError } = await import('@/lib/sentry');

      mockClientMethods.listCredentials.mockResolvedValue({
        credentials: [createMockProtoCredential({ credentials: '{bad' })],
      });

      renderAndGetHook();
      await credentialsQueryFn!();

      expect(captureHookError).toHaveBeenCalledWith(
        expect.any(SyntaxError),
        'useAffiliateCredentialsConnect',
        'fromProtoCredential'
      );
    });

    it('should parse empty string credentials as empty object', async () => {
      mockClientMethods.listCredentials.mockResolvedValue({
        credentials: [createMockProtoCredential({ credentials: '' })],
      });

      renderAndGetHook();
      const data = (await credentialsQueryFn!()) as LocalAffiliateCredential[];

      // JSON.parse('{}') from the fallback '' || '{}'
      expect(data[0].credentials).toEqual({});
    });

    it('should handle multiple credentials in list', async () => {
      mockClientMethods.listCredentials.mockResolvedValue({
        credentials: [
          createMockProtoCredential({ id: 'c1', store: AffiliateStoreEnum.SHOPEE }),
          createMockProtoCredential({ id: 'c2', store: AffiliateStoreEnum.AMAZON }),
          createMockProtoCredential({ id: 'c3', store: AffiliateStoreEnum.MAGALU }),
        ],
      });

      renderAndGetHook();
      const data = (await credentialsQueryFn!()) as LocalAffiliateCredential[];

      expect(data).toHaveLength(3);
      expect(data[0].id).toBe('c1');
      expect(data[1].id).toBe('c2');
      expect(data[2].id).toBe('c3');
    });
  });

  // ── fromProtoActiveStore - via active stores query ──────────

  describe('fromProtoActiveStore - via active stores query', () => {
    it('should map isConfigured and autoGenerate correctly', async () => {
      mockClientMethods.listActiveStores.mockResolvedValue({
        stores: [
          createMockProtoActiveStore({
            store: AffiliateStoreEnum.SHOPEE,
            isConfigured: true,
            autoGenerate: true,
          }),
        ],
      });

      renderAndGetHook();
      const data = (await activeStoresQueryFn!()) as LocalActiveStore[];

      expect(data[0]).toEqual({
        store: 'shopee',
        is_configured: true,
        auto_generate: true,
      });
    });

    it('should map unconfigured store correctly', async () => {
      mockClientMethods.listActiveStores.mockResolvedValue({
        stores: [
          createMockProtoActiveStore({
            store: AffiliateStoreEnum.AMAZON,
            isConfigured: false,
            autoGenerate: false,
          }),
        ],
      });

      renderAndGetHook();
      const data = (await activeStoresQueryFn!()) as LocalActiveStore[];

      expect(data[0]).toEqual({
        store: 'amazon',
        is_configured: false,
        auto_generate: false,
      });
    });

    it('should handle multiple active stores', async () => {
      mockClientMethods.listActiveStores.mockResolvedValue({
        stores: [
          createMockProtoActiveStore({ store: AffiliateStoreEnum.SHOPEE, isConfigured: true }),
          createMockProtoActiveStore({ store: AffiliateStoreEnum.AMAZON, isConfigured: false }),
          createMockProtoActiveStore({ store: AffiliateStoreEnum.ALIEXPRESS, isConfigured: true }),
        ],
      });

      renderAndGetHook();
      const data = (await activeStoresQueryFn!()) as LocalActiveStore[];

      expect(data).toHaveLength(3);
      expect(data[0].store).toBe('shopee');
      expect(data[1].store).toBe('amazon');
      expect(data[2].store).toBe('aliexpress');
    });
  });

  // ── Queries ─────────────────────────────────────────────────

  describe('queries', () => {
    it('should return empty array for credentials when user is null', () => {
      mockUserValue = null;
      const { result } = renderAndGetHook();

      expect(result.current.credentials).toEqual([]);
    });

    it('should return empty array for activeStores when user is null', () => {
      mockUserValue = null;
      const { result } = renderAndGetHook();

      expect(result.current.activeStores).toEqual([]);
    });

    it('should not enable query when user.id is null', () => {
      mockUserValue = null;
      renderAndGetHook();

      // useQuery is called but the queryFn should guard against null user
      expect(mockUseQuery).toHaveBeenCalled();
      const firstCallOptions = mockUseQuery.mock.calls[0][0] as QueryOptions;
      expect(firstCallOptions.enabled).toBe(false);
    });

    it('should enable query when user.id exists', () => {
      renderAndGetHook();

      const firstCallOptions = mockUseQuery.mock.calls[0][0] as QueryOptions;
      expect(firstCallOptions.enabled).toBe(true);
    });

    it('should return credentials from query data', () => {
      const mockCredentials: LocalAffiliateCredential[] = [
        {
          id: 'cred-1',
          user_id: 'user-123',
          store: 'shopee' as LocalAffiliateStore,
          credentials: { appId: 'abc' },
          auto_generate: true,
          is_active: true,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-02T00:00:00Z',
        },
      ];
      credentialsQueryData = mockCredentials;

      const { result } = renderAndGetHook();

      expect(result.current.credentials).toEqual(mockCredentials);
    });

    it('should return activeStores from query data', () => {
      const mockStores: LocalActiveStore[] = [
        { store: 'shopee' as LocalAffiliateStore, is_configured: true, auto_generate: false },
        { store: 'amazon' as LocalAffiliateStore, is_configured: false, auto_generate: false },
      ];
      activeStoresQueryData = mockStores;

      const { result } = renderAndGetHook();

      expect(result.current.activeStores).toEqual(mockStores);
    });

    it('should set loading=true when credentials query is loading', () => {
      loadingCredentials = true;
      const { result } = renderAndGetHook();

      expect(result.current.loading).toBe(true);
    });

    it('should set loading=true when active stores query is loading', () => {
      loadingActiveStores = true;
      const { result } = renderAndGetHook();

      expect(result.current.loading).toBe(true);
    });

    it('should set loading=false when both queries finished', () => {
      loadingCredentials = false;
      loadingActiveStores = false;
      const { result } = renderAndGetHook();

      expect(result.current.loading).toBe(false);
    });

    it('should return empty credentials when query data is undefined', () => {
      credentialsQueryData = undefined;
      const { result } = renderAndGetHook();

      expect(result.current.credentials).toEqual([]);
    });

    it('should return empty activeStores when query data is undefined', () => {
      activeStoresQueryData = undefined;
      const { result } = renderAndGetHook();

      expect(result.current.activeStores).toEqual([]);
    });

    it('should return empty credentials from queryFn when user is null', async () => {
      mockUserValue = null;
      renderAndGetHook();

      if (credentialsQueryFn) {
        const data = await credentialsQueryFn();
        expect(data).toEqual([]);
      }
    });

    it('should return empty activeStores from queryFn when user is null', async () => {
      mockUserValue = null;
      renderAndGetHook();

      if (activeStoresQueryFn) {
        const data = await activeStoresQueryFn();
        expect(data).toEqual([]);
      }
    });

    it('should include user.id in queryKey', () => {
      renderAndGetHook();

      const firstCallOptions = mockUseQuery.mock.calls[0][0] as QueryOptions;
      expect(firstCallOptions.queryKey).toContain('user-123');
    });

    it('should set staleTime on queries', () => {
      renderAndGetHook();

      const firstCallOptions = mockUseQuery.mock.calls[0][0] as QueryOptions;
      expect(firstCallOptions.staleTime).toBe(60000);
    });
  });

  // ── refetch ─────────────────────────────────────────────────

  describe('refetch', () => {
    it('should call both refetchCredentials and refetchActiveStores', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.refetch();
      });

      expect(mockRefetchCredentials).toHaveBeenCalled();
      expect(mockRefetchActiveStores).toHaveBeenCalled();
    });
  });

  // ── upsertCredential ───────────────────────────────────────

  describe('upsertCredential', () => {
    it('should send store, credentials JSON, autoGenerate, isActive', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.upsertCredential({
          store: 'shopee',
          credentials: { appId: 'id-123', secret: 'sec-456' },
          auto_generate: true,
          is_active: true,
        });
      });

      expect(mockClientMethods.upsertCredential).toHaveBeenCalledWith(
        expect.objectContaining({
          store: AffiliateStoreEnum.SHOPEE,
          credentials: JSON.stringify({ appId: 'id-123', secret: 'sec-456' }),
          autoGenerate: true,
          isActive: true,
        })
      );
    });

    it('should return fromProtoCredential of response', async () => {
      mockClientMethods.upsertCredential.mockResolvedValue({
        credential: createMockProtoCredential({
          id: 'cred-new',
          store: AffiliateStoreEnum.AMAZON,
        }),
      });

      const { result } = renderAndGetHook();

      let upserted: LocalAffiliateCredential | undefined;
      await act(async () => {
        upserted = await result.current.upsertCredential({
          store: 'amazon',
          credentials: { key: 'val' },
        });
      });

      expect(upserted).toBeDefined();
      expect(upserted!.id).toBe('cred-new');
      expect(upserted!.store).toBe('amazon');
    });

    it('should throw saveFailed when response has no credential', async () => {
      mockClientMethods.upsertCredential.mockResolvedValue({
        credential: undefined,
      });

      const { result } = renderAndGetHook();

      await expect(
        act(async () => {
          await result.current.upsertCredential({
            store: 'shopee',
            credentials: { appId: 'abc' },
          });
        })
      ).rejects.toThrow('pages.affiliatePrograms.errors.saveFailed');
    });

    it('should throw notAuthenticated when user is null', async () => {
      mockUserValue = null;
      const { result } = renderAndGetHook();

      await expect(
        act(async () => {
          await result.current.upsertCredential({
            store: 'shopee',
            credentials: { appId: 'abc' },
          });
        })
      ).rejects.toThrow('common.errors.notAuthenticated');
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.upsertCredential({
          store: 'shopee',
          credentials: { appId: 'abc' },
        });
      });

      expect(mockInvalidateQueries).toHaveBeenCalledWith({
        queryKey: ['affiliate-credentials'],
      });
    });

    it('should show success toast on upsert', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.upsertCredential({
          store: 'shopee',
          credentials: { appId: 'abc' },
        });
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.affiliatePrograms.success.saved',
        })
      );
    });

    it('should show destructive toast on error', async () => {
      mockClientMethods.upsertCredential.mockRejectedValue(new Error('Network error'));

      const { result } = renderAndGetHook();

      await act(async () => {
        try {
          await result.current.upsertCredential({
            store: 'shopee',
            credentials: { appId: 'abc' },
          });
        } catch {
          // Expected error
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive' })
      );
    });
  });

  // ── deleteCredential ────────────────────────────────────────

  describe('deleteCredential', () => {
    it('should call with correct store enum', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.deleteCredential('amazon');
      });

      expect(mockClientMethods.deleteCredential).toHaveBeenCalledWith(
        expect.objectContaining({ store: AffiliateStoreEnum.AMAZON })
      );
    });

    it('should call with shopee store enum', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.deleteCredential('shopee');
      });

      expect(mockClientMethods.deleteCredential).toHaveBeenCalledWith(
        expect.objectContaining({ store: AffiliateStoreEnum.SHOPEE })
      );
    });

    it('should throw notAuthenticated when user is null', async () => {
      mockUserValue = null;
      const { result } = renderAndGetHook();

      await expect(
        act(async () => {
          await result.current.deleteCredential('shopee');
        })
      ).rejects.toThrow('common.errors.notAuthenticated');
    });

    it('should invalidate queries on success', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.deleteCredential('shopee');
      });

      expect(mockInvalidateQueries).toHaveBeenCalledWith({
        queryKey: ['affiliate-credentials'],
      });
    });

    it('should show success toast on delete', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.deleteCredential('shopee');
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.affiliatePrograms.success.deleted',
        })
      );
    });

    it('should show destructive toast on error', async () => {
      mockClientMethods.deleteCredential.mockRejectedValue(new Error('Delete failed'));

      const { result } = renderAndGetHook();

      await act(async () => {
        try {
          await result.current.deleteCredential('shopee');
        } catch {
          // Expected error
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive' })
      );
    });
  });

  // ── listCredentials mutation ────────────────────────────────

  describe('listCredentials mutation', () => {
    it('should list credentials and return local types', async () => {
      const { result } = renderAndGetHook();

      let credentials: LocalAffiliateCredential[] | undefined;
      await act(async () => {
        credentials = await result.current.listCredentials();
      });

      expect(mockClientMethods.listCredentials).toHaveBeenCalled();
      expect(credentials).toHaveLength(1);
      expect(credentials![0].store).toBe('shopee');
    });

    it('should throw notAuthenticated when user is null', async () => {
      mockUserValue = null;
      const { result } = renderAndGetHook();

      await expect(
        act(async () => {
          await result.current.listCredentials();
        })
      ).rejects.toThrow('common.errors.notAuthenticated');
    });

    it('should show destructive toast on error', async () => {
      mockClientMethods.listCredentials.mockRejectedValue(new Error('List failed'));

      const { result } = renderAndGetHook();

      await act(async () => {
        try {
          await result.current.listCredentials();
        } catch {
          // Expected error
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive' })
      );
    });
  });

  // ── getCredential mutation ──────────────────────────────────

  describe('getCredential mutation', () => {
    it('should get credential for a specific store', async () => {
      const { result } = renderAndGetHook();

      let credential: LocalAffiliateCredential | null | undefined;
      await act(async () => {
        credential = await result.current.getCredential('shopee');
      });

      expect(mockClientMethods.getCredential).toHaveBeenCalledWith(
        expect.objectContaining({ store: AffiliateStoreEnum.SHOPEE })
      );
      expect(credential).toBeDefined();
      expect(credential!.store).toBe('shopee');
    });

    it('should return null when response has no credential', async () => {
      mockClientMethods.getCredential.mockResolvedValue({
        credential: undefined,
      });

      const { result } = renderAndGetHook();

      let credential: LocalAffiliateCredential | null | undefined;
      await act(async () => {
        credential = await result.current.getCredential('shopee');
      });

      expect(credential).toBeNull();
    });

    it('should throw notAuthenticated when user is null', async () => {
      mockUserValue = null;
      const { result } = renderAndGetHook();

      await expect(
        act(async () => {
          await result.current.getCredential('shopee');
        })
      ).rejects.toThrow('common.errors.notAuthenticated');
    });

    it('should show destructive toast on error', async () => {
      mockClientMethods.getCredential.mockRejectedValue(new Error('Get failed'));

      const { result } = renderAndGetHook();

      await act(async () => {
        try {
          await result.current.getCredential('shopee');
        } catch {
          // Expected error
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive' })
      );
    });
  });

  // ── listActiveStores mutation ───────────────────────────────

  describe('listActiveStores mutation', () => {
    it('should map response via fromProtoActiveStore', async () => {
      mockClientMethods.listActiveStores.mockResolvedValue({
        stores: [
          createMockProtoActiveStore({ store: AffiliateStoreEnum.SHOPEE, isConfigured: true }),
          createMockProtoActiveStore({ store: AffiliateStoreEnum.AMAZON, isConfigured: false }),
        ],
      });

      const { result } = renderAndGetHook();

      let stores: LocalActiveStore[] | undefined;
      await act(async () => {
        stores = await result.current.listActiveStores();
      });

      expect(stores).toHaveLength(2);
      expect(stores![0]).toEqual({
        store: 'shopee',
        is_configured: true,
        auto_generate: false,
      });
      expect(stores![1]).toEqual({
        store: 'amazon',
        is_configured: false,
        auto_generate: false,
      });
    });

    it('should throw notAuthenticated when user is null', async () => {
      mockUserValue = null;
      const { result } = renderAndGetHook();

      await expect(
        act(async () => {
          await result.current.listActiveStores();
        })
      ).rejects.toThrow('common.errors.notAuthenticated');
    });

    it('should show destructive toast on error', async () => {
      mockClientMethods.listActiveStores.mockRejectedValue(new Error('List stores failed'));

      const { result } = renderAndGetHook();

      await act(async () => {
        try {
          await result.current.listActiveStores();
        } catch {
          // Expected error
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ variant: 'destructive' })
      );
    });
  });

  // ── Loading states ──────────────────────────────────────────

  describe('loading states', () => {
    it('should expose isListingCredentials from mutation', () => {
      let isPendingCallIndex = 0;
      mockUseMutation.mockImplementation((options: MutationOptions) => {
        const index = isPendingCallIndex++;
        return {
          mutateAsync: async (variables: unknown) => {
            const result = await options.mutationFn(variables);
            if (options.onSuccess) options.onSuccess(result);
            return result;
          },
          isPending: index === 0, // first mutation = listCredentials
        };
      });

      const { result } = renderAndGetHook();
      expect(result.current.isListingCredentials).toBe(true);
    });

    it('should expose isGettingCredential from mutation', () => {
      let isPendingCallIndex = 0;
      mockUseMutation.mockImplementation((options: MutationOptions) => {
        const index = isPendingCallIndex++;
        return {
          mutateAsync: async (variables: unknown) => {
            const result = await options.mutationFn(variables);
            if (options.onSuccess) options.onSuccess(result);
            return result;
          },
          isPending: index === 1, // second mutation = getCredential
        };
      });

      const { result } = renderAndGetHook();
      expect(result.current.isGettingCredential).toBe(true);
    });

    it('should expose isUpsertingCredential from mutation', () => {
      let isPendingCallIndex = 0;
      mockUseMutation.mockImplementation((options: MutationOptions) => {
        const index = isPendingCallIndex++;
        return {
          mutateAsync: async (variables: unknown) => {
            const result = await options.mutationFn(variables);
            if (options.onSuccess) options.onSuccess(result);
            return result;
          },
          isPending: index === 2, // third mutation = upsertCredential
        };
      });

      const { result } = renderAndGetHook();
      expect(result.current.isUpsertingCredential).toBe(true);
    });

    it('should expose isDeletingCredential from mutation', () => {
      let isPendingCallIndex = 0;
      mockUseMutation.mockImplementation((options: MutationOptions) => {
        const index = isPendingCallIndex++;
        return {
          mutateAsync: async (variables: unknown) => {
            const result = await options.mutationFn(variables);
            if (options.onSuccess) options.onSuccess(result);
            return result;
          },
          isPending: index === 3, // fourth mutation = deleteCredential
        };
      });

      const { result } = renderAndGetHook();
      expect(result.current.isDeletingCredential).toBe(true);
    });

    it('should expose isListingActiveStores from mutation', () => {
      let isPendingCallIndex = 0;
      mockUseMutation.mockImplementation((options: MutationOptions) => {
        const index = isPendingCallIndex++;
        return {
          mutateAsync: async (variables: unknown) => {
            const result = await options.mutationFn(variables);
            if (options.onSuccess) options.onSuccess(result);
            return result;
          },
          isPending: index === 4, // fifth mutation = listActiveStores
        };
      });

      const { result } = renderAndGetHook();
      expect(result.current.isListingActiveStores).toBe(true);
    });

    it('should have all isPending=false by default', () => {
      const { result } = renderAndGetHook();

      expect(result.current.isListingCredentials).toBe(false);
      expect(result.current.isGettingCredential).toBe(false);
      expect(result.current.isUpsertingCredential).toBe(false);
      expect(result.current.isDeletingCredential).toBe(false);
      expect(result.current.isListingActiveStores).toBe(false);
    });
  });

  // ── Return shape ────────────────────────────────────────────

  describe('return shape', () => {
    it('should return all expected properties and functions', () => {
      const { result } = renderAndGetHook();

      expect(Array.isArray(result.current.credentials)).toBe(true);
      expect(Array.isArray(result.current.activeStores)).toBe(true);
      expect(typeof result.current.loading).toBe('boolean');
      expect(typeof result.current.refetch).toBe('function');
      expect(typeof result.current.listCredentials).toBe('function');
      expect(typeof result.current.getCredential).toBe('function');
      expect(typeof result.current.upsertCredential).toBe('function');
      expect(typeof result.current.deleteCredential).toBe('function');
      expect(typeof result.current.listActiveStores).toBe('function');
      expect(typeof result.current.isListingCredentials).toBe('boolean');
      expect(typeof result.current.isGettingCredential).toBe('boolean');
      expect(typeof result.current.isUpsertingCredential).toBe('boolean');
      expect(typeof result.current.isDeletingCredential).toBe('boolean');
      expect(typeof result.current.isListingActiveStores).toBe('boolean');
    });
  });
});
