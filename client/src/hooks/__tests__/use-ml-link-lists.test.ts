import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock All Heavy Dependencies First
// ============================================================

// Mock proto imports to prevent memory-heavy resolution
vi.mock('@/gen/ml_links/v1/ml_links_pb', () => ({}));
vi.mock('@/gen/common/v1/pagination_pb', () => ({}));
vi.mock('@bufbuild/protobuf', () => ({ create: vi.fn((_, data) => data) }));
vi.mock('@connectrpc/connect', () => ({ createClient: vi.fn() }));
vi.mock('@connectrpc/connect-query', () => ({ useTransport: () => ({}) }));
vi.mock('@tanstack/react-query', () => ({
  useQuery: () => ({ data: undefined, isLoading: false, refetch: vi.fn() }),
  useMutation: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useQueryClient: () => ({ invalidateQueries: vi.fn() }),
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

vi.mock('@/contexts', () => ({
  useAuth: () => ({
    user: { id: 'user-123', email: 'test@example.com' },
    session: null,
    loading: false,
  }),
}));

vi.mock('@/lib/connect-transport', () => ({
  getErrorInfo: () => ({ userMessage: 'Error' }),
}));

// Mock connect hook - use vi.hoisted for stable references
const mockFunctions = vi.hoisted(() => ({
  refetchLists: vi.fn(),
  listMlLinkListItemsPaginated: vi.fn(),
  createMlLinkList: vi.fn(),
  updateMlLinkList: vi.fn(),
  deleteMlLinkList: vi.fn(),
  createMlLinkListItem: vi.fn(),
  bulkCreateMlLinkListItems: vi.fn(),
  updateMlLinkListItem: vi.fn(),
  deleteMlLinkListItem: vi.fn(),
  resetConsumedItems: vi.fn(),
  reorderItems: vi.fn(),
}));

const mockLists = vi.hoisted(
  () =>
    [] as Array<{
      id: string;
      user_id: string;
      name: string;
      description?: string;
      is_active: boolean;
      created_at: string;
      updated_at: string;
    }>
);

vi.mock('@/hooks/use-ml-links-connect', () => ({
  useMlLinksConnect: () => ({
    lists: mockLists,
    loadingLists: false,
    ...mockFunctions,
  }),
}));

const mockMlPreview = vi.fn();

vi.mock('@/hooks/use-mercadolivre-connect', () => ({
  useMercadoLivreConnect: () => ({
    preview: mockMlPreview,
  }),
}));

// Import hook after mocks
import { useMlLinkLists } from '../use-ml-link-lists';

// ============================================================
// Test Helpers
// ============================================================

function createMockList(overrides: Record<string, unknown> = {}) {
  return {
    id: 'list-001',
    user_id: 'user-123',
    name: 'Test List',
    description: 'Test Description',
    is_active: true,
    created_at: '2026-01-15T10:00:00Z',
    updated_at: '2026-01-15T10:00:00Z',
    ...overrides,
  };
}

function createMockItem(overrides: Record<string, unknown> = {}) {
  return {
    id: 'item-001',
    list_id: 'list-001',
    origin_url: 'https://www.mercadolivre.com.br/p/MLB12345678',
    order_index: 0,
    is_active: true,
    consumed_at: undefined,
    product_title: 'Test Product',
    product_price: 199.99,
    product_image_url: 'https://example.com/image.jpg',
    metadata: '{}',
    created_at: '2026-01-15T10:00:00Z',
    updated_at: '2026-01-15T10:00:00Z',
    ...overrides,
  };
}

// ============================================================
// Tests
// ============================================================

describe('useMlLinkLists', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockLists.length = 0;

    mockFunctions.refetchLists.mockResolvedValue(undefined);
    mockFunctions.listMlLinkListItemsPaginated.mockResolvedValue({
      items: [],
      nextPageToken: '',
      totalCount: 0,
      hasMore: false,
    });
    mockFunctions.createMlLinkList.mockResolvedValue(createMockList({ id: 'list-new' }));
    mockFunctions.updateMlLinkList.mockResolvedValue(createMockList({ name: 'Updated' }));
    mockFunctions.deleteMlLinkList.mockResolvedValue(undefined);
    mockFunctions.createMlLinkListItem.mockResolvedValue(createMockItem({ id: 'item-new' }));
    mockFunctions.bulkCreateMlLinkListItems.mockResolvedValue({ createdCount: 3, items: [] });
    mockFunctions.updateMlLinkListItem.mockResolvedValue(createMockItem({ is_active: false }));
    mockFunctions.deleteMlLinkListItem.mockResolvedValue(undefined);
    mockFunctions.resetConsumedItems.mockResolvedValue(5);
    mockFunctions.reorderItems.mockResolvedValue(undefined);
    mockMlPreview.mockResolvedValue({
      title: 'Preview Title',
      price: 299.99,
      imageUrl: 'https://example.com/preview.jpg',
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('initial state', () => {
    it('should initialize with empty lists', () => {
      const { result } = renderHook(() => useMlLinkLists());
      expect(result.current.lists).toEqual([]);
      expect(result.current.selectedListId).toBe('');
      expect(result.current.items).toEqual([]);
    });

    it('should expose loading states', () => {
      const { result } = renderHook(() => useMlLinkLists());
      expect(result.current.loadingLists).toBe(false);
      expect(result.current.loadingItems).toBe(false);
    });
  });

  describe('loadLists', () => {
    it('should call refetchLists', async () => {
      const { result } = renderHook(() => useMlLinkLists());
      await act(async () => {
        await result.current.loadLists();
      });
      expect(mockFunctions.refetchLists).toHaveBeenCalled();
    });
  });

  describe('list CRUD', () => {
    it('should open create dialog', () => {
      const { result } = renderHook(() => useMlLinkLists());
      act(() => result.current.openCreateList());
      expect(result.current.listDialogOpen).toBe(true);
      expect(result.current.editingList).toBeNull();
    });

    it('should save new list', async () => {
      const { result } = renderHook(() => useMlLinkLists());

      act(() => {
        result.current.openCreateList();
        result.current.setListForm({
          name: 'New List',
          description: 'Description',
          is_active: true,
        });
      });

      await act(async () => {
        await result.current.saveList();
      });

      expect(mockFunctions.createMlLinkList).toHaveBeenCalledWith({
        name: 'New List',
        description: 'Description',
        is_active: true,
      });
      expect(result.current.listDialogOpen).toBe(false);
    });

    it('should require name to save', async () => {
      const { result } = renderHook(() => useMlLinkLists());

      act(() => {
        result.current.openCreateList();
        result.current.setListForm({ name: '', description: '', is_active: true });
      });

      await act(async () => {
        await result.current.saveList();
      });

      expect(mockFunctions.createMlLinkList).not.toHaveBeenCalled();
      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
    });

    it('should open edit dialog', () => {
      mockLists.push(createMockList({ id: 'list-001', name: 'Test' }));
      const { result } = renderHook(() => useMlLinkLists());

      act(() => result.current.openEditList(result.current.lists[0]));

      expect(result.current.listDialogOpen).toBe(true);
      expect(result.current.editingList).not.toBeNull();
      expect(result.current.listForm.name).toBe('Test');
    });

    it('should delete list', async () => {
      mockLists.push(createMockList({ id: 'list-001' }));
      const { result } = renderHook(() => useMlLinkLists());

      await waitFor(() => expect(result.current.selectedListId).toBe('list-001'));

      act(() => result.current.requestDeleteList(result.current.lists[0]));
      expect(result.current.deleteListTarget).not.toBeNull();

      await act(async () => {
        await result.current.confirmDeleteList();
      });

      expect(mockFunctions.deleteMlLinkList).toHaveBeenCalledWith('list-001');
      expect(result.current.deleteListTarget).toBeNull();
    });
  });

  describe('addOneLink', () => {
    it('should add valid ML URL', async () => {
      mockLists.push(createMockList({ id: 'list-001' }));
      const { result } = renderHook(() => useMlLinkLists());

      await waitFor(() => expect(result.current.selectedListId).toBe('list-001'));

      act(() => result.current.setSingleUrl('https://www.mercadolivre.com.br/p/MLB12345678'));

      await act(async () => {
        await result.current.addOneLink();
      });

      expect(mockFunctions.createMlLinkListItem).toHaveBeenCalledWith(
        expect.objectContaining({
          list_id: 'list-001',
          origin_url: expect.stringContaining('mercadolivre'),
        })
      );
      expect(result.current.singleUrl).toBe('');
    });

    it('should accept meli.la short link', async () => {
      mockLists.push(createMockList({ id: 'list-001' }));
      const { result } = renderHook(() => useMlLinkLists());

      await waitFor(() => expect(result.current.selectedListId).toBe('list-001'));

      act(() => result.current.setSingleUrl('https://meli.la/28fhqrh'));

      await act(async () => {
        await result.current.addOneLink();
      });

      expect(mockFunctions.createMlLinkListItem).toHaveBeenCalledWith(
        expect.objectContaining({
          list_id: 'list-001',
          origin_url: 'https://meli.la/28fhqrh',
        })
      );
    });

    it('should reject invalid URL', async () => {
      mockLists.push(createMockList({ id: 'list-001' }));
      const { result } = renderHook(() => useMlLinkLists());

      await waitFor(() => expect(result.current.selectedListId).toBe('list-001'));

      act(() => result.current.setSingleUrl('https://amazon.com.br/product'));

      await act(async () => {
        await result.current.addOneLink();
      });

      expect(mockFunctions.createMlLinkListItem).not.toHaveBeenCalled();
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.mlLinkLists.errors.invalidLink',
        })
      );
    });

    it('should normalize URL by removing hash', async () => {
      mockLists.push(createMockList({ id: 'list-001' }));
      const { result } = renderHook(() => useMlLinkLists());

      await waitFor(() => expect(result.current.selectedListId).toBe('list-001'));

      act(() =>
        result.current.setSingleUrl('https://www.mercadolivre.com.br/p/MLB12345678#section')
      );

      await act(async () => {
        await result.current.addOneLink();
      });

      expect(mockFunctions.createMlLinkListItem).toHaveBeenCalledWith(
        expect.objectContaining({
          origin_url: 'https://www.mercadolivre.com.br/p/MLB12345678',
        })
      );
    });
  });

  describe('importBulk', () => {
    it('should import multiple valid URLs', async () => {
      mockLists.push(createMockList({ id: 'list-001' }));
      const { result } = renderHook(() => useMlLinkLists());

      await waitFor(() => expect(result.current.selectedListId).toBe('list-001'));

      const urls = [
        'https://www.mercadolivre.com.br/p/MLB11111111',
        'https://www.mercadolivre.com.br/p/MLB22222222',
      ].join('\n');

      act(() => result.current.setBulkText(urls));

      await act(async () => {
        await result.current.importBulk();
      });

      expect(mockFunctions.bulkCreateMlLinkListItems).toHaveBeenCalled();
      expect(result.current.bulkText).toBe('');
    });

    it('should deduplicate URLs', async () => {
      mockLists.push(createMockList({ id: 'list-001' }));
      const { result } = renderHook(() => useMlLinkLists());

      await waitFor(() => expect(result.current.selectedListId).toBe('list-001'));

      const urls = [
        'https://www.mercadolivre.com.br/p/MLB11111111',
        'https://www.mercadolivre.com.br/p/MLB11111111',
        'https://www.mercadolivre.com.br/p/MLB22222222',
      ].join('\n');

      act(() => result.current.setBulkText(urls));

      await act(async () => {
        await result.current.importBulk();
      });

      const [, urlsArg] = mockFunctions.bulkCreateMlLinkListItems.mock.calls[0];
      expect(urlsArg).toHaveLength(2);
    });

    it('should reject when no valid URLs', async () => {
      mockLists.push(createMockList({ id: 'list-001' }));
      const { result } = renderHook(() => useMlLinkLists());

      await waitFor(() => expect(result.current.selectedListId).toBe('list-001'));

      act(() => result.current.setBulkText('https://amazon.com/product'));

      await act(async () => {
        await result.current.importBulk();
      });

      expect(mockFunctions.bulkCreateMlLinkListItems).not.toHaveBeenCalled();
      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
    });
  });

  describe('item operations', () => {
    it('should toggle item active status', async () => {
      mockLists.push(createMockList({ id: 'list-001' }));
      mockFunctions.listMlLinkListItemsPaginated.mockResolvedValue({
        items: [createMockItem({ id: 'item-001', is_active: true })],
        nextPageToken: '',
        totalCount: 1,
        hasMore: false,
      });

      const { result } = renderHook(() => useMlLinkLists());

      await waitFor(() => expect(result.current.items).toHaveLength(1));

      await act(async () => {
        await result.current.toggleItemActive(result.current.items[0]);
      });

      expect(mockFunctions.updateMlLinkListItem).toHaveBeenCalledWith({
        id: 'item-001',
        is_active: false,
      });
    });

    it('should delete item', async () => {
      mockLists.push(createMockList({ id: 'list-001' }));
      mockFunctions.listMlLinkListItemsPaginated.mockResolvedValue({
        items: [createMockItem({ id: 'item-001' })],
        nextPageToken: '',
        totalCount: 1,
        hasMore: false,
      });

      const { result } = renderHook(() => useMlLinkLists());

      await waitFor(() => expect(result.current.items).toHaveLength(1));

      act(() => result.current.requestDeleteItem(result.current.items[0]));
      expect(result.current.deleteItemTarget).not.toBeNull();

      await act(async () => {
        await result.current.confirmDeleteItem();
      });

      expect(mockFunctions.deleteMlLinkListItem).toHaveBeenCalledWith('item-001');
    });
  });

  describe('reorder', () => {
    it('should swap order indices when moving up', async () => {
      mockLists.push(createMockList({ id: 'list-001' }));
      mockFunctions.listMlLinkListItemsPaginated.mockResolvedValue({
        items: [
          createMockItem({ id: 'item-001', order_index: 0 }),
          createMockItem({ id: 'item-002', order_index: 1 }),
        ],
        nextPageToken: '',
        totalCount: 2,
        hasMore: false,
      });

      const { result } = renderHook(() => useMlLinkLists());

      await waitFor(() => expect(result.current.items).toHaveLength(2));

      await act(async () => {
        await result.current.moveItem(result.current.items[1], 'up');
      });

      expect(mockFunctions.reorderItems).toHaveBeenCalledWith('list-001', {
        'item-002': 0,
        'item-001': 1,
      });
    });

    it('should not move first item up', async () => {
      mockLists.push(createMockList({ id: 'list-001' }));
      mockFunctions.listMlLinkListItemsPaginated.mockResolvedValue({
        items: [createMockItem({ id: 'item-001', order_index: 0 })],
        nextPageToken: '',
        totalCount: 1,
        hasMore: false,
      });

      const { result } = renderHook(() => useMlLinkLists());

      await waitFor(() => expect(result.current.items).toHaveLength(1));

      await act(async () => {
        await result.current.moveItem(result.current.items[0], 'up');
      });

      expect(mockFunctions.reorderItems).not.toHaveBeenCalled();
    });
  });

  describe('resetConsumed', () => {
    it('should reset consumed items', async () => {
      mockLists.push(createMockList({ id: 'list-001' }));
      const { result } = renderHook(() => useMlLinkLists());

      await waitFor(() => expect(result.current.selectedListId).toBe('list-001'));

      act(() => result.current.requestResetConsumed());
      expect(result.current.resetConsumedDialogOpen).toBe(true);

      await act(async () => {
        await result.current.confirmResetConsumed();
      });

      expect(mockFunctions.resetConsumedItems).toHaveBeenCalledWith('list-001');
      expect(result.current.resetConsumedDialogOpen).toBe(false);
    });
  });

  describe('copyToClipboard', () => {
    it('should copy text and show toast', async () => {
      const mockWriteText = vi.fn().mockResolvedValue(undefined);
      Object.assign(navigator, { clipboard: { writeText: mockWriteText } });

      const { result } = renderHook(() => useMlLinkLists());

      await act(async () => {
        await result.current.copyToClipboard('https://example.com');
      });

      expect(mockWriteText).toHaveBeenCalledWith('https://example.com');
      expect(mockToast).toHaveBeenCalledWith({
        title: 'pages.mlLinkLists.success.copied',
      });
    });
  });

  describe('error handling', () => {
    it('should show toast on save error', async () => {
      mockFunctions.createMlLinkList.mockRejectedValue(new Error('Save failed'));

      const { result } = renderHook(() => useMlLinkLists());

      act(() => {
        result.current.openCreateList();
        result.current.setListForm({ name: 'Test', description: '', is_active: true });
      });

      await act(async () => {
        await result.current.saveList();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.mlLinkLists.errors.save',
        })
      );
    });

    it('should show toast on add link error', async () => {
      mockLists.push(createMockList({ id: 'list-001' }));
      mockFunctions.createMlLinkListItem.mockRejectedValue(new Error('URL_ALREADY_EXISTS'));

      const { result } = renderHook(() => useMlLinkLists());

      await waitFor(() => expect(result.current.selectedListId).toBe('list-001'));

      act(() => result.current.setSingleUrl('https://www.mercadolivre.com.br/p/MLB12345678'));

      await act(async () => {
        await result.current.addOneLink();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'pages.mlLinkLists.errors.addLink',
        })
      );
    });
  });

  describe('computed values', () => {
    it('should compute availableCount correctly', async () => {
      mockLists.push(createMockList({ id: 'list-001' }));
      mockFunctions.listMlLinkListItemsPaginated.mockResolvedValue({
        items: [
          createMockItem({ id: '1', is_active: true, consumed_at: undefined }),
          createMockItem({ id: '2', is_active: true, consumed_at: '2026-01-20T15:00:00Z' }),
          createMockItem({ id: '3', is_active: false, consumed_at: undefined }),
          createMockItem({ id: '4', is_active: true, consumed_at: undefined }),
        ],
        nextPageToken: '',
        totalCount: 4,
        hasMore: false,
      });

      const { result } = renderHook(() => useMlLinkLists());

      await waitFor(() => expect(result.current.availableCount).toBe(2));
    });
  });
});
