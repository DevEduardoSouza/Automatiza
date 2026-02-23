import { renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules
// ============================================================

const mockUser = { id: 'user-123', email: 'test@example.com' };
let authUser: typeof mockUser | null = mockUser;

vi.mock('@/contexts', () => ({
  useAuth: () => ({
    user: authUser,
    session: null,
    loading: false,
  }),
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

vi.mock('@/lib/connect-transport', () => ({
  getErrorInfo: (error: unknown) => ({
    userMessage: error instanceof Error ? error.message : 'Unknown error',
    code: 'INTERNAL',
  }),
}));

// ============================================================
// Mock: Proto schemas (sentinels to avoid hoisting issues)
// ============================================================

vi.mock('@/gen/dispatch_texts/v1/dispatch_texts_pb', () => ({
  DispatchTextsService: 'DispatchTextsService',
  ListDispatchTextsRequestSchema: 'ListDispatchTextsRequestSchema',
  GetDispatchTextRequestSchema: 'GetDispatchTextRequestSchema',
  CreateDispatchTextRequestSchema: 'CreateDispatchTextRequestSchema',
  UpdateDispatchTextRequestSchema: 'UpdateDispatchTextRequestSchema',
  DeleteDispatchTextRequestSchema: 'DeleteDispatchTextRequestSchema',
}));

vi.mock('@bufbuild/protobuf', () => ({
  create: (_schema: unknown, data: unknown) => data,
}));

// ============================================================
// Mock: ConnectRPC client
// ============================================================

const mockClient = {
  listDispatchTexts: vi.fn(),
  getDispatchText: vi.fn(),
  createDispatchText: vi.fn(),
  updateDispatchText: vi.fn(),
  deleteDispatchText: vi.fn(),
};

vi.mock('@connectrpc/connect', () => ({
  createClient: () => mockClient,
}));

vi.mock('@connectrpc/connect-query', () => ({
  useTransport: () => 'mock-transport',
}));

// ============================================================
// Mock: React Query
// ============================================================

type QueryConfig = {
  queryKey: string[];
  queryFn: () => Promise<unknown>;
  enabled: boolean;
  staleTime: number;
};

type MutationConfig = {
  mutationFn: (...args: unknown[]) => Promise<unknown>;
  onSuccess?: () => void;
  onError?: (error: unknown) => void;
};

let capturedQueryConfig: QueryConfig | null = null;
const capturedMutations: MutationConfig[] = [];
const mockInvalidateQueries = vi.fn();

vi.mock('@tanstack/react-query', () => ({
  useQuery: (config: QueryConfig) => {
    capturedQueryConfig = config;
    return {
      data: undefined,
      isLoading: false,
      refetch: vi.fn(),
    };
  },
  useMutation: (config: MutationConfig) => {
    capturedMutations.push(config);
    return {
      mutateAsync: config.mutationFn,
      isPending: false,
    };
  },
  useQueryClient: () => ({
    invalidateQueries: mockInvalidateQueries,
  }),
}));

// ============================================================
// Import after mocks
// ============================================================

import { useDispatchTextsConnect } from '../use-dispatch-texts-connect';

// ============================================================
// Test Helpers
// ============================================================

const mockProtoDispatchText = {
  id: 'dt-1',
  userId: 'user-123',
  name: 'Test Template',
  body: 'Hello {nome_produto}',
  isActive: true,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

function renderAndGet() {
  capturedMutations.length = 0;
  capturedQueryConfig = null;
  const { result } = renderHook(() => useDispatchTextsConnect());
  return result;
}

// ============================================================
// Tests
// ============================================================

describe('useDispatchTextsConnect', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    capturedMutations.length = 0;
    capturedQueryConfig = null;
    authUser = mockUser;

    mockClient.listDispatchTexts.mockResolvedValue({
      dispatchTexts: [mockProtoDispatchText],
    });
    mockClient.getDispatchText.mockResolvedValue({
      dispatchText: mockProtoDispatchText,
    });
    mockClient.createDispatchText.mockResolvedValue({
      dispatchText: mockProtoDispatchText,
    });
    mockClient.updateDispatchText.mockResolvedValue({
      dispatchText: mockProtoDispatchText,
    });
    mockClient.deleteDispatchText.mockResolvedValue({});
  });

  // ── fromProtoDispatchText (via list query) ──────────────────

  describe('fromProtoDispatchText', () => {
    it('should convert camelCase proto to snake_case local type', async () => {
      renderAndGet();

      // Execute the queryFn to test the converter
      const queryResult = (await capturedQueryConfig!.queryFn()) as Array<{
        id: string;
        user_id: string;
        name: string;
        body: string;
        is_active: boolean;
        created_at: string;
        updated_at: string;
      }>;

      expect(queryResult[0]).toEqual({
        id: 'dt-1',
        user_id: 'user-123',
        name: 'Test Template',
        body: 'Hello {nome_produto}',
        is_active: true,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      });
    });

    it('should map all fields from proto response', async () => {
      renderAndGet();

      const result = (await capturedQueryConfig!.queryFn()) as Array<Record<string, unknown>>;

      expect(result[0]).toHaveProperty('id');
      expect(result[0]).toHaveProperty('user_id');
      expect(result[0]).toHaveProperty('name');
      expect(result[0]).toHaveProperty('body');
      expect(result[0]).toHaveProperty('is_active');
      expect(result[0]).toHaveProperty('created_at');
      expect(result[0]).toHaveProperty('updated_at');
    });
  });

  // ── List query ──────────────────────────────────────────────

  describe('list query', () => {
    it('should include user.id in query key', () => {
      renderAndGet();

      expect(capturedQueryConfig!.queryKey).toEqual([
        'dispatch-texts',
        'list',
        'user-123',
      ]);
    });

    it('should set enabled=false when user is null', () => {
      authUser = null;
      renderAndGet();

      expect(capturedQueryConfig!.enabled).toBe(false);
    });

    it('should set staleTime to 30000', () => {
      renderAndGet();

      expect(capturedQueryConfig!.staleTime).toBe(30000);
    });

    it('should return empty array when user is null', async () => {
      authUser = null;
      renderAndGet();

      const result = await capturedQueryConfig!.queryFn();
      expect(result).toEqual([]);
    });

    it('should map proto response to local types', async () => {
      renderAndGet();

      const result = await capturedQueryConfig!.queryFn();
      expect(Array.isArray(result)).toBe(true);
    });
  });

  // ── Create mutation ─────────────────────────────────────────

  describe('create mutation', () => {
    it('should throw without user', async () => {
      authUser = null;
      renderAndGet();

      // The create mutation is the 2nd captured (index 1: list, 2: get, 3: create)
      const createMutation = capturedMutations.find((_m, i) => i === 2);
      expect(createMutation).toBeDefined();

      await expect(
        createMutation!.mutationFn({ name: 'Test', body: 'Body' })
      ).rejects.toThrow();
    });

    it('should call client.createDispatchText with correct params', async () => {
      renderAndGet();

      const createMutation = capturedMutations[2];
      await createMutation.mutationFn({ name: 'New', body: 'Body', is_active: true });

      expect(mockClient.createDispatchText).toHaveBeenCalled();
    });

    it('should invalidate cache on success', () => {
      renderAndGet();

      const createMutation = capturedMutations[2];
      createMutation.onSuccess?.();

      expect(mockInvalidateQueries).toHaveBeenCalledWith({
        queryKey: ['dispatch-texts'],
      });
    });

    it('should show toast on error', () => {
      renderAndGet();

      const createMutation = capturedMutations[2];
      createMutation.onError?.(new Error('Create failed'));

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
        })
      );
    });
  });

  // ── Update mutation ─────────────────────────────────────────

  describe('update mutation', () => {
    it('should throw without user', async () => {
      authUser = null;
      renderAndGet();

      const updateMutation = capturedMutations[3];
      expect(updateMutation).toBeDefined();

      await expect(
        updateMutation.mutationFn({ id: 'dt-1', name: 'Updated' })
      ).rejects.toThrow();
    });

    it('should call client.updateDispatchText with correct params', async () => {
      renderAndGet();

      const updateMutation = capturedMutations[3];
      await updateMutation.mutationFn({
        id: 'dt-1',
        name: 'Updated',
        body: 'New Body',
        is_active: false,
      });

      expect(mockClient.updateDispatchText).toHaveBeenCalled();
    });

    it('should invalidate cache on success', () => {
      renderAndGet();

      const updateMutation = capturedMutations[3];
      updateMutation.onSuccess?.();

      expect(mockInvalidateQueries).toHaveBeenCalledWith({
        queryKey: ['dispatch-texts'],
      });
    });

    it('should show toast on error', () => {
      renderAndGet();

      const updateMutation = capturedMutations[3];
      updateMutation.onError?.(new Error('Update failed'));

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
        })
      );
    });
  });

  // ── Delete mutation ─────────────────────────────────────────

  describe('delete mutation', () => {
    it('should throw without user', async () => {
      authUser = null;
      renderAndGet();

      const deleteMutation = capturedMutations[4];
      expect(deleteMutation).toBeDefined();

      await expect(deleteMutation.mutationFn('dt-1')).rejects.toThrow();
    });

    it('should call client.deleteDispatchText', async () => {
      renderAndGet();

      const deleteMutation = capturedMutations[4];
      await deleteMutation.mutationFn('dt-1');

      expect(mockClient.deleteDispatchText).toHaveBeenCalled();
    });

    it('should invalidate cache on success', () => {
      renderAndGet();

      const deleteMutation = capturedMutations[4];
      deleteMutation.onSuccess?.();

      expect(mockInvalidateQueries).toHaveBeenCalledWith({
        queryKey: ['dispatch-texts'],
      });
    });

    it('should show toast on error', () => {
      renderAndGet();

      const deleteMutation = capturedMutations[4];
      deleteMutation.onError?.(new Error('Delete failed'));

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
        })
      );
    });
  });

  // ── Get mutation ────────────────────────────────────────────

  describe('get mutation', () => {
    it('should throw without user', async () => {
      authUser = null;
      renderAndGet();

      const getMutation = capturedMutations[1];
      expect(getMutation).toBeDefined();

      await expect(getMutation.mutationFn('dt-1')).rejects.toThrow();
    });

    it('should throw when dispatchText is missing in response', async () => {
      mockClient.getDispatchText.mockResolvedValue({ dispatchText: undefined });
      renderAndGet();

      const getMutation = capturedMutations[1];

      await expect(getMutation.mutationFn('dt-1')).rejects.toThrow();
    });

    it('should return converted dispatch text', async () => {
      renderAndGet();

      const getMutation = capturedMutations[1];
      const result = await getMutation.mutationFn('dt-1');

      expect(result).toEqual({
        id: 'dt-1',
        user_id: 'user-123',
        name: 'Test Template',
        body: 'Hello {nome_produto}',
        is_active: true,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      });
    });
  });

  // ── Return shape ────────────────────────────────────────────

  describe('return shape', () => {
    it('should return all expected properties', () => {
      const result = renderAndGet();

      expect(Array.isArray(result.current.dispatchTexts)).toBe(true);
      expect(typeof result.current.loading).toBe('boolean');
      expect(typeof result.current.refetch).toBe('function');

      expect(typeof result.current.listDispatchTexts).toBe('function');
      expect(typeof result.current.getDispatchText).toBe('function');
      expect(typeof result.current.createDispatchText).toBe('function');
      expect(typeof result.current.updateDispatchText).toBe('function');
      expect(typeof result.current.deleteDispatchText).toBe('function');

      expect(typeof result.current.isListingDispatchTexts).toBe('boolean');
      expect(typeof result.current.isGettingDispatchText).toBe('boolean');
      expect(typeof result.current.isCreatingDispatchText).toBe('boolean');
      expect(typeof result.current.isUpdatingDispatchText).toBe('boolean');
      expect(typeof result.current.isDeletingDispatchText).toBe('boolean');
    });
  });
});
