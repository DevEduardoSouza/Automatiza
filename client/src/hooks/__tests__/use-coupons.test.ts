import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules (inline values to avoid TDZ issues with vi.mock hoisting)
// ============================================================

vi.mock('@/lib/sentry', () => ({
  captureHookError: vi.fn(),
  safeClipboard: vi.fn(),
  safeLocalStorage: {
    getItem: vi.fn().mockReturnValue(null),
    setItem: vi.fn(),
    removeItem: vi.fn(),
  },
}));

const mockToast = vi.fn();
vi.mock('@/hooks/use-toast', () => ({
  useToast: () => ({ toast: mockToast, toasts: [], dismiss: vi.fn() }),
}));

const mockT = (key: string) => key;
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: mockT, i18n: { language: 'pt-BR' } }),
}));

const mockUser = { id: 'user-123', email: 'test@example.com' };
let mockUserValue: typeof mockUser | null = mockUser;
vi.mock('@/contexts', () => ({
  useAuth: () => ({ user: mockUserValue, session: null, loading: false }),
}));

// ============================================================
// Mock ConnectRPC + React Query
// ============================================================

let mockQueryData: { coupons: any[] } | undefined = { coupons: [] };
let mockQueryLoading = false;

vi.mock('@connectrpc/connect-query', () => ({
  useQuery: () => ({
    data: mockQueryData,
    isLoading: mockQueryLoading,
  }),
  useTransport: () => ({}),
  createConnectQueryKey: () => ['coupons'],
}));

const mockMutateFns: Record<string, ReturnType<typeof vi.fn>> = {
  create: vi.fn(),
  update: vi.fn(),
  delete: vi.fn(),
  toggle: vi.fn(),
};
let mockIsPending = false;
let mutationCallIndex = 0;

vi.mock('@tanstack/react-query', () => ({
  useMutation: (_opts: any) => {
    // Mutations are created in order: create, update, delete, toggleActive
    const order = ['create', 'update', 'delete', 'toggle'];
    const key = order[mutationCallIndex % order.length];
    mutationCallIndex++;
    return {
      mutate: mockMutateFns[key],
      isPending: mockIsPending,
    };
  },
  useQueryClient: () => ({
    invalidateQueries: vi.fn(),
  }),
}));

vi.mock('@connectrpc/connect', () => ({
  createClient: () => ({}),
  Code: { AlreadyExists: 6, NotFound: 5, InvalidArgument: 3 },
}));

vi.mock('@/lib/connect-transport', () => ({
  getErrorInfo: (err: any) => ({
    code: err?.code ?? 13,
    userMessage: err?.message ?? 'Unknown error',
  }),
}));

vi.mock('@bufbuild/protobuf', () => ({
  create: (_schema: any, data: any) => data ?? {},
}));

vi.mock('@/gen/coupons/v1/coupons-CouponsService_connectquery', () => ({
  listCoupons: { I: {}, O: {}, service: { typeName: 'coupons.v1.CouponsService' } },
}));

vi.mock('@/gen/coupons/v1/coupons_pb', () => ({
  CouponsService: {},
  CouponFilterSchema: {},
  CreateCouponRequestSchema: {},
  DeleteCouponRequestSchema: {},
  UpdateCouponRequestSchema: {},
  DiscountType: { FIXED: 1, PERCENT: 2 },
  Store: {
    UNSPECIFIED: 0,
    SHOPEE: 1,
    AMAZON: 2,
    MAGALU: 3,
    MERCADO_LIVRE: 4,
    SHEIN: 5,
    ALIEXPRESS: 6,
    AWIN: 7,
  },
}));

// ============================================================
// Import after mocks
// ============================================================

import { safeClipboard } from '@/lib/sentry';
import { useCoupons } from '../use-coupons';

// ============================================================
// Test Helpers
// ============================================================

const futureDate = new Date(Date.now() + 86_400_000 * 30).toISOString();
const pastDate = new Date(Date.now() - 86_400_000 * 30).toISOString();

function makeProtoCoupon(overrides: Record<string, any> = {}) {
  return {
    id: 'c-1',
    userId: 'user-123',
    code: 'PROMO10',
    title: 'Promo Shopee',
    store: 1, // SHOPEE
    affiliateLink: 'https://shopee.com/aff',
    expiresAt: futureDate,
    isActive: true,
    category: 'electronics',
    discountType: 1, // FIXED
    discountValue: 10,
    maxDiscount: 50,
    minProductPrice: 100,
    couponText: 'Use this code',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

const mockCoupons = [
  makeProtoCoupon({ id: 'c-1', code: 'PROMO10', title: 'Promo Shopee', store: 1, isActive: true, expiresAt: futureDate }),
  makeProtoCoupon({ id: 'c-2', code: 'AMAZON20', title: 'Amazon Deal', store: 2, isActive: true, expiresAt: futureDate }),
  makeProtoCoupon({ id: 'c-3', code: 'MAGALU30', title: 'Magalu Oferta', store: 3, isActive: false, expiresAt: futureDate }),
  makeProtoCoupon({ id: 'c-4', code: 'EXPIRED1', title: 'Old Deal', store: 1, isActive: true, expiresAt: pastDate }),
  makeProtoCoupon({ id: 'c-5', code: 'SHEIN5', title: 'Shein Special', store: 5, isActive: true, expiresAt: null }),
  makeProtoCoupon({ id: 'c-6', code: 'AWIN15', title: 'Awin Partner', store: 7, isActive: false, expiresAt: pastDate }),
];

function renderAndGet() {
  const { result } = renderHook(() => useCoupons());
  return result;
}

// ============================================================
// Tests
// ============================================================

describe('useCoupons', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUserValue = mockUser;
    mockQueryData = { coupons: [] };
    mockQueryLoading = false;
    mockIsPending = false;
    mutationCallIndex = 0;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── A. Initial state ──────────────────────────────────────

  describe('initial state', () => {
    it('should have loading = false by default', () => {
      const result = renderAndGet();
      expect(result.current.loading).toBe(false);
    });

    it('should have isDialogOpen = false', () => {
      const result = renderAndGet();
      expect(result.current.isDialogOpen).toBe(false);
    });

    it('should have editingCoupon = null', () => {
      const result = renderAndGet();
      expect(result.current.editingCoupon).toBeNull();
    });

    it('should have filterStore = all', () => {
      const result = renderAndGet();
      expect(result.current.filterStore).toBe('all');
    });

    it('should have filterStatus = all', () => {
      const result = renderAndGet();
      expect(result.current.filterStatus).toBe('all');
    });

    it('should have searchQuery = empty string', () => {
      const result = renderAndGet();
      expect(result.current.searchQuery).toBe('');
    });

    it('should have formData equal to initialFormData', () => {
      const result = renderAndGet();
      expect(result.current.formData).toEqual({
        code: '',
        title: '',
        store: 'shopee',
        affiliate_link: '',
        expires_at: null,
        category: null,
        discount_type: 'fixed',
        discount_value: null,
        max_discount: null,
        min_product_price: null,
        coupon_text: null,
      });
    });

    it('should have isPending = false', () => {
      const result = renderAndGet();
      expect(result.current.isPending).toBe(false);
    });

    it('should have 7 store options', () => {
      const result = renderAndGet();
      expect(result.current.stores).toHaveLength(7);
      expect(result.current.stores.map((s) => s.value)).toEqual([
        'shopee',
        'amazon',
        'magalu',
        'ml',
        'shein',
        'aliexpress',
        'awin',
      ]);
    });

    it('should have 14 category options', () => {
      const result = renderAndGet();
      expect(result.current.categories).toHaveLength(14);
    });
  });

  // ── B. filteredCoupons ────────────────────────────────────

  describe('filteredCoupons', () => {
    beforeEach(() => {
      mockQueryData = { coupons: mockCoupons };
      mutationCallIndex = 0;
    });

    it('should return all coupons when no filter is applied', () => {
      const result = renderAndGet();
      expect(result.current.filteredCoupons).toHaveLength(6);
    });

    it('should search by code (case-insensitive)', () => {
      const result = renderAndGet();

      act(() => result.current.setSearchQuery('promo'));

      expect(result.current.filteredCoupons).toHaveLength(1);
      expect(result.current.filteredCoupons[0].code).toBe('PROMO10');
    });

    it('should search by title', () => {
      const result = renderAndGet();

      act(() => result.current.setSearchQuery('Amazon Deal'));

      expect(result.current.filteredCoupons).toHaveLength(1);
      expect(result.current.filteredCoupons[0].id).toBe('c-2');
    });

    it('should filter by store', () => {
      const result = renderAndGet();

      act(() => result.current.setFilterStore('shopee'));

      // c-1 (shopee, active, future) and c-4 (shopee, active, past)
      expect(result.current.filteredCoupons).toHaveLength(2);
      for (const c of result.current.filteredCoupons) {
        expect(c.store).toBe('shopee');
      }
    });

    it('should filter status active = is_active AND not expired', () => {
      const result = renderAndGet();

      act(() => result.current.setFilterStatus('active'));

      // c-1 (active+future), c-2 (active+future), c-5 (active+no expiry)
      // c-4 is active but expired, so excluded
      const ids = result.current.filteredCoupons.map((c) => c.id);
      expect(ids).toContain('c-1');
      expect(ids).toContain('c-2');
      expect(ids).toContain('c-5');
      expect(ids).not.toContain('c-4');
    });

    it('should filter status inactive = not (is_active AND not expired)', () => {
      const result = renderAndGet();

      act(() => result.current.setFilterStatus('inactive'));

      // inactive means: is_active=false AND not expired -> c-3
      // Also: c-4 (active+expired) and c-6 (inactive+expired) are expired, so excluded by inactive filter
      // Actually the code: filterStatus === 'inactive' && coupon.is_active && !isExpired(coupon) -> exclude
      // So: c-1 (active+notExpired) -> excluded; c-2 (active+notExpired) -> excluded; c-3 (inactive+notExpired) -> kept
      // c-4 (active+expired) -> kept; c-5 (active+noExpiry) -> excluded; c-6 (inactive+expired) -> kept
      const ids = result.current.filteredCoupons.map((c) => c.id);
      expect(ids).toContain('c-3');
      expect(ids).toContain('c-4');
      expect(ids).toContain('c-6');
      expect(ids).not.toContain('c-1');
    });

    it('should filter status expired = expires_at in the past', () => {
      const result = renderAndGet();

      act(() => result.current.setFilterStatus('expired'));

      // c-4 (expired), c-6 (expired)
      const ids = result.current.filteredCoupons.map((c) => c.id);
      expect(ids).toContain('c-4');
      expect(ids).toContain('c-6');
      expect(ids).not.toContain('c-1');
      expect(ids).not.toContain('c-5');
    });

    it('should combine search + store + status filters', () => {
      const result = renderAndGet();

      act(() => {
        result.current.setSearchQuery('PROMO');
        result.current.setFilterStore('shopee');
        result.current.setFilterStatus('active');
      });

      // Only c-1 matches: shopee + active + not expired + code matches PROMO
      expect(result.current.filteredCoupons).toHaveLength(1);
      expect(result.current.filteredCoupons[0].id).toBe('c-1');
    });

    it('should return empty array when search finds no results', () => {
      const result = renderAndGet();

      act(() => result.current.setSearchQuery('NONEXISTENT'));

      expect(result.current.filteredCoupons).toHaveLength(0);
    });

    it('should trim whitespace from search query', () => {
      const result = renderAndGet();

      act(() => result.current.setSearchQuery('  promo10  '));

      expect(result.current.filteredCoupons).toHaveLength(1);
      expect(result.current.filteredCoupons[0].code).toBe('PROMO10');
    });
  });

  // ── C. Dialog management ──────────────────────────────────

  describe('dialog management', () => {
    it('handleOpenDialog (create mode) opens dialog with null editingCoupon and initial form', () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenDialog());

      expect(result.current.isDialogOpen).toBe(true);
      expect(result.current.editingCoupon).toBeNull();
      expect(result.current.formData.code).toBe('');
      expect(result.current.formData.store).toBe('shopee');
    });

    it('handleOpenDialog (edit mode) pre-fills form and sets editingCoupon', () => {
      mockQueryData = { coupons: mockCoupons };
      mutationCallIndex = 0;
      const result = renderAndGet();
      const coupon = result.current.coupons[0]; // c-1

      act(() => result.current.handleOpenDialog(coupon));

      expect(result.current.isDialogOpen).toBe(true);
      expect(result.current.editingCoupon).toEqual(coupon);
      expect(result.current.formData.code).toBe(coupon.code);
      expect(result.current.formData.title).toBe(coupon.title);
      expect(result.current.formData.store).toBe(coupon.store);
    });

    it('handleCloseDialog closes dialog and resets state', () => {
      mockQueryData = { coupons: mockCoupons };
      mutationCallIndex = 0;
      const result = renderAndGet();
      const coupon = result.current.coupons[0];

      act(() => result.current.handleOpenDialog(coupon));
      expect(result.current.isDialogOpen).toBe(true);

      act(() => result.current.handleCloseDialog());

      expect(result.current.isDialogOpen).toBe(false);
      expect(result.current.editingCoupon).toBeNull();
      expect(result.current.formData.code).toBe('');
    });
  });

  // ── D. Mutations CRUD ─────────────────────────────────────

  describe('mutations', () => {
    it('handleSubmit in create mode calls create mutate', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenDialog());
      act(() =>
        result.current.setFormData((prev) => ({
          ...prev,
          code: 'NEWCODE',
          title: 'New Coupon',
        }))
      );

      const mockEvent = { preventDefault: vi.fn() } as unknown as React.FormEvent;
      await act(async () => {
        await result.current.handleSubmit(mockEvent);
      });

      expect(mockEvent.preventDefault).toHaveBeenCalled();
      expect(mockMutateFns.create).toHaveBeenCalled();
    });

    it('handleSubmit in update mode calls update mutate', async () => {
      mockQueryData = { coupons: mockCoupons };
      mutationCallIndex = 0;
      const result = renderAndGet();
      const coupon = result.current.coupons[0];

      act(() => result.current.handleOpenDialog(coupon));

      const mockEvent = { preventDefault: vi.fn() } as unknown as React.FormEvent;
      await act(async () => {
        await result.current.handleSubmit(mockEvent);
      });

      expect(mockMutateFns.update).toHaveBeenCalledWith(
        expect.objectContaining({ id: coupon.id })
      );
    });

    it('handleSubmit trims whitespace on affiliate_link', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenDialog());
      act(() =>
        result.current.setFormData((prev) => ({
          ...prev,
          code: 'CODE1',
          title: 'Title',
          affiliate_link: '  https://example.com  ',
        }))
      );

      const mockEvent = { preventDefault: vi.fn() } as unknown as React.FormEvent;
      await act(async () => {
        await result.current.handleSubmit(mockEvent);
      });

      const payload = mockMutateFns.create.mock.calls[0][0];
      expect(payload.affiliate_link).toBe('https://example.com');
    });

    it('handleSubmit nullifies empty affiliate_link', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenDialog());
      act(() =>
        result.current.setFormData((prev) => ({
          ...prev,
          code: 'CODE2',
          title: 'Title',
          affiliate_link: '   ',
        }))
      );

      const mockEvent = { preventDefault: vi.fn() } as unknown as React.FormEvent;
      await act(async () => {
        await result.current.handleSubmit(mockEvent);
      });

      const payload = mockMutateFns.create.mock.calls[0][0];
      expect(payload.affiliate_link).toBeNull();
    });

    it('handleSubmit nullifies empty coupon_text', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenDialog());
      act(() =>
        result.current.setFormData((prev) => ({
          ...prev,
          code: 'CODE3',
          title: 'Title',
          coupon_text: '   ',
        }))
      );

      const mockEvent = { preventDefault: vi.fn() } as unknown as React.FormEvent;
      await act(async () => {
        await result.current.handleSubmit(mockEvent);
      });

      const payload = mockMutateFns.create.mock.calls[0][0];
      expect(payload.coupon_text).toBeNull();
    });

    it('handleDelete with confirm=true calls delete mutate', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(true);

      const result = renderAndGet();

      await act(async () => {
        await result.current.handleDelete('c-1');
      });

      expect(mockMutateFns.delete).toHaveBeenCalledWith('c-1');
    });

    it('handleDelete with confirm=false does not call delete', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(false);

      const result = renderAndGet();

      await act(async () => {
        await result.current.handleDelete('c-1');
      });

      expect(mockMutateFns.delete).not.toHaveBeenCalled();
    });

    it('handleToggleActive deactivates an active coupon', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(true);
      mockQueryData = { coupons: mockCoupons };
      mutationCallIndex = 0;
      const result = renderAndGet();
      const activeCoupon = result.current.coupons[0]; // is_active: true

      await act(async () => {
        await result.current.handleToggleActive(activeCoupon);
      });

      expect(mockMutateFns.toggle).toHaveBeenCalledWith(
        expect.objectContaining({ id: activeCoupon.id, is_active: false })
      );
    });

    it('handleToggleActive activates an inactive coupon', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(true);
      mockQueryData = { coupons: mockCoupons };
      mutationCallIndex = 0;
      const result = renderAndGet();
      const inactiveCoupon = result.current.coupons[2]; // c-3, is_active: false

      await act(async () => {
        await result.current.handleToggleActive(inactiveCoupon);
      });

      expect(mockMutateFns.toggle).toHaveBeenCalledWith(
        expect.objectContaining({ id: inactiveCoupon.id, is_active: true })
      );
    });

    it('handleToggleActive does nothing when cancelled', async () => {
      vi.spyOn(window, 'confirm').mockReturnValue(false);
      mockQueryData = { coupons: mockCoupons };
      mutationCallIndex = 0;
      const result = renderAndGet();
      const coupon = result.current.coupons[0];

      await act(async () => {
        await result.current.handleToggleActive(coupon);
      });

      expect(mockMutateFns.toggle).not.toHaveBeenCalled();
    });
  });

  // ── E. Clipboard ──────────────────────────────────────────

  describe('clipboard', () => {
    it('handleCopyCode shows success toast when clipboard succeeds', async () => {
      vi.mocked(safeClipboard).mockResolvedValue(true);

      const result = renderAndGet();

      await act(async () => {
        result.current.handleCopyCode('PROMO10');
      });

      expect(safeClipboard).toHaveBeenCalledWith('PROMO10');
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.coupons.success.copied',
          description: 'PROMO10',
        })
      );
    });

    it('handleCopyCode shows destructive toast when clipboard fails', async () => {
      vi.mocked(safeClipboard).mockResolvedValue(false);

      const result = renderAndGet();

      await act(async () => {
        result.current.handleCopyCode('FAIL');
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
          description: 'common.clipboardUnavailable',
        })
      );
    });
  });

  // ── F. isExpired ──────────────────────────────────────────

  describe('isExpired', () => {
    it('should return false when expires_at is null', () => {
      const result = renderAndGet();
      const coupon = { expires_at: null } as any;
      expect(result.current.isExpired(coupon)).toBe(false);
    });

    it('should return false when expires_at is in the future', () => {
      const result = renderAndGet();
      const coupon = { expires_at: futureDate } as any;
      expect(result.current.isExpired(coupon)).toBe(false);
    });

    it('should return true when expires_at is in the past', () => {
      const result = renderAndGet();
      const coupon = { expires_at: pastDate } as any;
      expect(result.current.isExpired(coupon)).toBe(true);
    });
  });
});
