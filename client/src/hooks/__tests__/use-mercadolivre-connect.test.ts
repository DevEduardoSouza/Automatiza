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
  connect: vi.fn(),
  createLink: vi.fn(),
  preview: vi.fn(),
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

vi.mock('@/gen/affiliates/v1/mercadolivre_pb', () => ({
  MercadoLivreService: { typeName: 'affiliates.v1.MercadoLivreService' },
  MlConnectRequestSchema: {},
  MlCreateLinkRequestSchema: {},
  MlPreviewRequestSchema: {},
}));

vi.mock('@bufbuild/protobuf', () => ({
  create: (_schema: unknown, data: unknown) => data,
}));

// ============================================================
// Mock React Query
// ============================================================

interface MutationOptions<TData = unknown, TError = unknown, TVariables = unknown> {
  mutationFn: (variables: TVariables) => Promise<TData>;
  onSuccess?: (data: TData) => void;
  onError?: (error: TError) => void;
}

const mockUseMutation = vi.fn();

vi.mock('@tanstack/react-query', () => ({
  useMutation: (options: MutationOptions) => mockUseMutation(options),
}));

// ============================================================
// Import after mocks
// ============================================================

import { useMercadoLivreConnect } from '../use-mercadolivre-connect';
import type { LocalMlLinkUrl, LocalMlProductPreview } from '../use-mercadolivre-connect';

// ============================================================
// Test Data Factories
// ============================================================

function createMockProtoLinkUrl(overrides: Record<string, unknown> = {}) {
  return {
    longUrl: 'https://mercadolivre.com.br/product/123?aff=tag',
    shortUrl: 'https://meli.co/abc',
    ...overrides,
  };
}

function createMockProtoPreview(overrides: Record<string, unknown> = {}) {
  return {
    title: 'Samsung Galaxy S25',
    price: 3999.9,
    currency: 'BRL',
    imageUrl: 'https://img.mercadolivre.com/product.jpg',
    sellerName: 'Tech Store',
    condition: 'new',
    soldQuantity: 1500,
    ratingAverage: 4.7,
    reviewsTotal: 342,
    originalPrice: 4599.9,
    discountPercent: 13,
    installmentsText: '12x R$ 333,33',
    ...overrides,
  };
}

// ============================================================
// Helpers
// ============================================================

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
  return renderHook(() => useMercadoLivreConnect());
}

// ============================================================
// Tests
// ============================================================

describe('useMercadoLivreConnect', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUserValue = mockUser;
    setupDefaultMutationMock();

    // Default client responses
    mockClientMethods.connect.mockResolvedValue({
      success: true,
      message: 'Connected successfully',
    });
    mockClientMethods.createLink.mockResolvedValue({
      urls: [createMockProtoLinkUrl()],
    });
    mockClientMethods.preview.mockResolvedValue({
      preview: createMockProtoPreview(),
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── fromProtoLinkUrl - via createLink ───────────────────────

  describe('fromProtoLinkUrl - via createLink', () => {
    it('should convert proto link URL with longUrl and shortUrl', async () => {
      mockClientMethods.createLink.mockResolvedValue({
        urls: [
          {
            longUrl: 'https://mercadolivre.com.br/product/456?aff=mytag',
            shortUrl: 'https://meli.co/xyz',
          },
        ],
      });

      const { result } = renderAndGetHook();

      let links: LocalMlLinkUrl[] | undefined;
      await act(async () => {
        links = await result.current.createLink('https://mercadolivre.com.br/product/456');
      });

      expect(links).toHaveLength(1);
      expect(links![0].longUrl).toBe('https://mercadolivre.com.br/product/456?aff=mytag');
      expect(links![0].shortUrl).toBe('https://meli.co/xyz');
    });

    it('should handle undefined longUrl', async () => {
      mockClientMethods.createLink.mockResolvedValue({
        urls: [{ longUrl: undefined, shortUrl: 'https://meli.co/abc' }],
      });

      const { result } = renderAndGetHook();

      let links: LocalMlLinkUrl[] | undefined;
      await act(async () => {
        links = await result.current.createLink('https://mercadolivre.com.br/p/1');
      });

      expect(links![0].longUrl).toBeUndefined();
      expect(links![0].shortUrl).toBe('https://meli.co/abc');
    });

    it('should handle undefined shortUrl', async () => {
      mockClientMethods.createLink.mockResolvedValue({
        urls: [{ longUrl: 'https://mercadolivre.com.br/p/1?aff=x', shortUrl: undefined }],
      });

      const { result } = renderAndGetHook();

      let links: LocalMlLinkUrl[] | undefined;
      await act(async () => {
        links = await result.current.createLink('https://mercadolivre.com.br/p/1');
      });

      expect(links![0].longUrl).toBe('https://mercadolivre.com.br/p/1?aff=x');
      expect(links![0].shortUrl).toBeUndefined();
    });

    it('should handle multiple URLs in response', async () => {
      mockClientMethods.createLink.mockResolvedValue({
        urls: [
          createMockProtoLinkUrl({ longUrl: 'https://ml.com/a', shortUrl: 'https://meli.co/a' }),
          createMockProtoLinkUrl({ longUrl: 'https://ml.com/b', shortUrl: 'https://meli.co/b' }),
        ],
      });

      const { result } = renderAndGetHook();

      let links: LocalMlLinkUrl[] | undefined;
      await act(async () => {
        links = await result.current.createLink('https://mercadolivre.com.br/p/1');
      });

      expect(links).toHaveLength(2);
      expect(links![0].shortUrl).toBe('https://meli.co/a');
      expect(links![1].shortUrl).toBe('https://meli.co/b');
    });
  });

  // ── fromProtoPreview - via preview ──────────────────────────

  describe('fromProtoPreview - via preview', () => {
    it('should return null for undefined preview', async () => {
      mockClientMethods.preview.mockResolvedValue({
        preview: undefined,
      });

      const { result } = renderAndGetHook();

      let previewResult: LocalMlProductPreview | null | undefined;
      await act(async () => {
        previewResult = await result.current.preview('https://mercadolivre.com.br/p/1');
      });

      expect(previewResult).toBeNull();
    });

    it('should convert all fields correctly', async () => {
      const protoPreview = createMockProtoPreview();
      mockClientMethods.preview.mockResolvedValue({ preview: protoPreview });

      const { result } = renderAndGetHook();

      let previewResult: LocalMlProductPreview | null | undefined;
      await act(async () => {
        previewResult = await result.current.preview('https://mercadolivre.com.br/p/1');
      });

      expect(previewResult).toEqual({
        title: 'Samsung Galaxy S25',
        price: 3999.9,
        currency: 'BRL',
        imageUrl: 'https://img.mercadolivre.com/product.jpg',
        sellerName: 'Tech Store',
        condition: 'new',
        soldQuantity: 1500,
        ratingAverage: 4.7,
        reviewsTotal: 342,
        originalPrice: 4599.9,
        discountPercent: 13,
        installmentsText: '12x R$ 333,33',
      });
    });

    it('should handle undefined optional fields gracefully', async () => {
      mockClientMethods.preview.mockResolvedValue({
        preview: {
          title: 'Simple Product',
          price: undefined,
          currency: undefined,
          imageUrl: undefined,
          sellerName: undefined,
          condition: undefined,
          soldQuantity: undefined,
          ratingAverage: undefined,
          reviewsTotal: undefined,
          originalPrice: undefined,
          discountPercent: undefined,
          installmentsText: undefined,
        },
      });

      const { result } = renderAndGetHook();

      let previewResult: LocalMlProductPreview | null | undefined;
      await act(async () => {
        previewResult = await result.current.preview('https://mercadolivre.com.br/p/1');
      });

      expect(previewResult).toBeDefined();
      expect(previewResult!.title).toBe('Simple Product');
      expect(previewResult!.price).toBeUndefined();
      expect(previewResult!.currency).toBeUndefined();
      expect(previewResult!.imageUrl).toBeUndefined();
      expect(previewResult!.sellerName).toBeUndefined();
      expect(previewResult!.condition).toBeUndefined();
      expect(previewResult!.soldQuantity).toBeUndefined();
      expect(previewResult!.ratingAverage).toBeUndefined();
      expect(previewResult!.reviewsTotal).toBeUndefined();
      expect(previewResult!.originalPrice).toBeUndefined();
      expect(previewResult!.discountPercent).toBeUndefined();
      expect(previewResult!.installmentsText).toBeUndefined();
    });

    it('should preserve numeric zero values', async () => {
      mockClientMethods.preview.mockResolvedValue({
        preview: createMockProtoPreview({
          price: 0,
          soldQuantity: 0,
          ratingAverage: 0,
          reviewsTotal: 0,
          discountPercent: 0,
        }),
      });

      const { result } = renderAndGetHook();

      let previewResult: LocalMlProductPreview | null | undefined;
      await act(async () => {
        previewResult = await result.current.preview('https://mercadolivre.com.br/p/1');
      });

      expect(previewResult!.price).toBe(0);
      expect(previewResult!.soldQuantity).toBe(0);
      expect(previewResult!.ratingAverage).toBe(0);
      expect(previewResult!.reviewsTotal).toBe(0);
      expect(previewResult!.discountPercent).toBe(0);
    });
  });

  // ── connect mutation ────────────────────────────────────────

  describe('connect mutation', () => {
    it('should throw when user.id is null', async () => {
      mockUserValue = null;
      const { result } = renderAndGetHook();

      await expect(
        act(async () => {
          await result.current.connect({ tag: 'my-tag' });
        })
      ).rejects.toThrow('common.errors.notAuthenticated');
    });

    it('should send userId, tag, cookie, csrfToken, autoGenerate', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.connect({
          tag: 'my-tag',
          cookie: 'session-cookie',
          csrfToken: 'csrf-123',
          autoGenerate: true,
        });
      });

      expect(mockClientMethods.connect).toHaveBeenCalledWith(
        expect.objectContaining({
          userId: 'user-123',
          tag: 'my-tag',
          cookie: 'session-cookie',
          csrfToken: 'csrf-123',
          autoGenerate: true,
        })
      );
    });

    it('should return success and message', async () => {
      mockClientMethods.connect.mockResolvedValue({
        success: true,
        message: 'Connected OK',
      });

      const { result } = renderAndGetHook();

      let connectResult: { success: boolean; message?: string } | undefined;
      await act(async () => {
        connectResult = await result.current.connect({ tag: 'my-tag' });
      });

      expect(connectResult).toEqual({ success: true, message: 'Connected OK' });
    });

    it('should return failure response', async () => {
      mockClientMethods.connect.mockResolvedValue({
        success: false,
        message: 'Invalid credentials',
      });

      const { result } = renderAndGetHook();

      let connectResult: { success: boolean; message?: string } | undefined;
      await act(async () => {
        connectResult = await result.current.connect({ tag: 'bad-tag' });
      });

      expect(connectResult).toEqual({ success: false, message: 'Invalid credentials' });
    });

    it('should show success toast when data.success is true', async () => {
      mockClientMethods.connect.mockResolvedValue({
        success: true,
        message: 'Connected!',
      });

      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.connect({ tag: 'tag-1' });
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'common.success',
          description: 'Connected!',
        })
      );
    });

    it('should use fallback description when message is undefined on success', async () => {
      mockClientMethods.connect.mockResolvedValue({
        success: true,
        message: undefined,
      });

      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.connect({ tag: 'tag-1' });
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'common.success',
          description: 'pages.affiliatePrograms.ml.connected',
        })
      );
    });

    it('should not show success toast when data.success is false', async () => {
      mockClientMethods.connect.mockResolvedValue({
        success: false,
        message: 'Failed',
      });

      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.connect({ tag: 'tag-1' });
      });

      // Toast should NOT be called with success pattern
      expect(mockToast).not.toHaveBeenCalledWith(
        expect.objectContaining({ title: 'common.success' })
      );
    });

    it('should show destructive toast on error', async () => {
      mockClientMethods.connect.mockRejectedValue(new Error('Network error'));

      const { result } = renderAndGetHook();

      await act(async () => {
        try {
          await result.current.connect({ tag: 'tag-1' });
        } catch {
          // Expected error
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
        })
      );
    });

    it('should send optional params as undefined when not provided', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.connect({ tag: 'tag-only' });
      });

      expect(mockClientMethods.connect).toHaveBeenCalledWith(
        expect.objectContaining({
          userId: 'user-123',
          tag: 'tag-only',
          cookie: undefined,
          csrfToken: undefined,
          autoGenerate: undefined,
        })
      );
    });
  });

  // ── createLink mutation ─────────────────────────────────────

  describe('createLink mutation', () => {
    it('should throw when user.id is null', async () => {
      mockUserValue = null;
      const { result } = renderAndGetHook();

      await expect(
        act(async () => {
          await result.current.createLink('https://ml.com/p/1');
        })
      ).rejects.toThrow('common.errors.notAuthenticated');
    });

    it('should send userId and originUrl', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.createLink('https://mercadolivre.com.br/p/MLB123');
      });

      expect(mockClientMethods.createLink).toHaveBeenCalledWith(
        expect.objectContaining({
          userId: 'user-123',
          originUrl: 'https://mercadolivre.com.br/p/MLB123',
        })
      );
    });

    it('should return array of LocalMlLinkUrl', async () => {
      mockClientMethods.createLink.mockResolvedValue({
        urls: [
          createMockProtoLinkUrl({
            longUrl: 'https://ml.com/p/1?aff=tag',
            shortUrl: 'https://meli.co/short',
          }),
        ],
      });

      const { result } = renderAndGetHook();

      let links: LocalMlLinkUrl[] | undefined;
      await act(async () => {
        links = await result.current.createLink('https://ml.com/p/1');
      });

      expect(links).toHaveLength(1);
      expect(links![0]).toEqual({
        longUrl: 'https://ml.com/p/1?aff=tag',
        shortUrl: 'https://meli.co/short',
      });
    });

    it('should return empty array when no urls in response', async () => {
      mockClientMethods.createLink.mockResolvedValue({ urls: [] });

      const { result } = renderAndGetHook();

      let links: LocalMlLinkUrl[] | undefined;
      await act(async () => {
        links = await result.current.createLink('https://ml.com/p/1');
      });

      expect(links).toEqual([]);
    });

    it('should show destructive toast on error', async () => {
      mockClientMethods.createLink.mockRejectedValue(new Error('Link creation failed'));

      const { result } = renderAndGetHook();

      await act(async () => {
        try {
          await result.current.createLink('https://ml.com/p/1');
        } catch {
          // Expected error
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
        })
      );
    });
  });

  // ── preview mutation ────────────────────────────────────────

  describe('preview mutation', () => {
    it('should throw when user.id is null', async () => {
      mockUserValue = null;
      const { result } = renderAndGetHook();

      await expect(
        act(async () => {
          await result.current.preview('https://ml.com/p/1');
        })
      ).rejects.toThrow('common.errors.notAuthenticated');
    });

    it('should send userId and originUrl', async () => {
      const { result } = renderAndGetHook();

      await act(async () => {
        await result.current.preview('https://mercadolivre.com.br/p/MLB456');
      });

      expect(mockClientMethods.preview).toHaveBeenCalledWith(
        expect.objectContaining({
          userId: 'user-123',
          originUrl: 'https://mercadolivre.com.br/p/MLB456',
        })
      );
    });

    it('should return LocalMlProductPreview', async () => {
      mockClientMethods.preview.mockResolvedValue({
        preview: createMockProtoPreview({ title: 'Test Product', price: 199.99 }),
      });

      const { result } = renderAndGetHook();

      let previewResult: LocalMlProductPreview | null | undefined;
      await act(async () => {
        previewResult = await result.current.preview('https://ml.com/p/1');
      });

      expect(previewResult).toBeDefined();
      expect(previewResult!.title).toBe('Test Product');
      expect(previewResult!.price).toBe(199.99);
    });

    it('should return null when preview is undefined in response', async () => {
      mockClientMethods.preview.mockResolvedValue({ preview: undefined });

      const { result } = renderAndGetHook();

      let previewResult: LocalMlProductPreview | null | undefined;
      await act(async () => {
        previewResult = await result.current.preview('https://ml.com/p/1');
      });

      expect(previewResult).toBeNull();
    });

    it('should show destructive toast on error', async () => {
      mockClientMethods.preview.mockRejectedValue(new Error('Preview failed'));

      const { result } = renderAndGetHook();

      await act(async () => {
        try {
          await result.current.preview('https://ml.com/p/1');
        } catch {
          // Expected error
        }
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
        })
      );
    });
  });

  // ── Loading states ──────────────────────────────────────────

  describe('loading states', () => {
    it('should expose isConnecting from mutation', () => {
      let isPendingCallIndex = 0;
      mockUseMutation.mockImplementation((options: MutationOptions) => {
        const index = isPendingCallIndex++;
        return {
          mutateAsync: async (variables: unknown) => {
            const res = await options.mutationFn(variables);
            if (options.onSuccess) options.onSuccess(res);
            return res;
          },
          isPending: index === 0, // first mutation = connect
        };
      });

      const { result } = renderAndGetHook();
      expect(result.current.isConnecting).toBe(true);
      expect(result.current.isCreatingLink).toBe(false);
      expect(result.current.isPreviewing).toBe(false);
    });

    it('should expose isCreatingLink from mutation', () => {
      let isPendingCallIndex = 0;
      mockUseMutation.mockImplementation((options: MutationOptions) => {
        const index = isPendingCallIndex++;
        return {
          mutateAsync: async (variables: unknown) => {
            const res = await options.mutationFn(variables);
            if (options.onSuccess) options.onSuccess(res);
            return res;
          },
          isPending: index === 1, // second mutation = createLink
        };
      });

      const { result } = renderAndGetHook();
      expect(result.current.isConnecting).toBe(false);
      expect(result.current.isCreatingLink).toBe(true);
      expect(result.current.isPreviewing).toBe(false);
    });

    it('should expose isPreviewing from mutation', () => {
      let isPendingCallIndex = 0;
      mockUseMutation.mockImplementation((options: MutationOptions) => {
        const index = isPendingCallIndex++;
        return {
          mutateAsync: async (variables: unknown) => {
            const res = await options.mutationFn(variables);
            if (options.onSuccess) options.onSuccess(res);
            return res;
          },
          isPending: index === 2, // third mutation = preview
        };
      });

      const { result } = renderAndGetHook();
      expect(result.current.isConnecting).toBe(false);
      expect(result.current.isCreatingLink).toBe(false);
      expect(result.current.isPreviewing).toBe(true);
    });

    it('should have all isPending=false by default', () => {
      const { result } = renderAndGetHook();

      expect(result.current.isConnecting).toBe(false);
      expect(result.current.isCreatingLink).toBe(false);
      expect(result.current.isPreviewing).toBe(false);
    });
  });

  // ── Return shape ────────────────────────────────────────────

  describe('return shape', () => {
    it('should return all expected properties and functions', () => {
      const { result } = renderAndGetHook();

      expect(typeof result.current.connect).toBe('function');
      expect(typeof result.current.createLink).toBe('function');
      expect(typeof result.current.preview).toBe('function');
      expect(typeof result.current.isConnecting).toBe('boolean');
      expect(typeof result.current.isCreatingLink).toBe('boolean');
      expect(typeof result.current.isPreviewing).toBe('boolean');
    });
  });
});
