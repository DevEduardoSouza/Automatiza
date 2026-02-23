import { act, renderHook, waitFor } from '@testing-library/react';
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

const mockToast = vi.fn();
vi.mock('@/hooks/use-toast', () => ({
  useToast: () => ({
    toast: mockToast,
    toasts: [],
    dismiss: vi.fn(),
  }),
}));

const mockT = (key: string, params?: Record<string, unknown>) => {
  if (params) return `${key} ${JSON.stringify(params)}`;
  return key;
};

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: mockT,
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

// ============================================================
// Mock ConnectRPC hooks
// ============================================================

const mockListAutomations = vi.fn();
const mockCreateAutomation = vi.fn();
const mockUpdateAutomation = vi.fn();
const mockDeleteAutomation = vi.fn();
const mockDiagnoseAutomation = vi.fn();
const mockRunAutomation = vi.fn();
const mockResetAutomationHistory = vi.fn();
let mockIsCreatingAutomation = false;
let mockIsUpdatingAutomation = false;

vi.mock('@/hooks/use-automations-connect', () => ({
  useAutomationsConnect: () => ({
    listAutomations: mockListAutomations,
    createAutomation: mockCreateAutomation,
    updateAutomation: mockUpdateAutomation,
    deleteAutomation: mockDeleteAutomation,
    diagnoseAutomation: mockDiagnoseAutomation,
    runAutomation: mockRunAutomation,
    resetAutomationHistory: mockResetAutomationHistory,
    isListingAutomations: false,
    isCreatingAutomation: mockIsCreatingAutomation,
    isUpdatingAutomation: mockIsUpdatingAutomation,
    isDeletingAutomation: false,
    isDiagnosingAutomation: false,
    isRunningAutomation: false,
    isResettingHistory: false,
    getAutomation: vi.fn(),
    isGettingAutomation: false,
  }),
}));

const mockToggleGroupFavorite = vi.fn();
const mockGroupsData = [
  { id: 'g1', name: 'Alpha Group', wa_group_id: '111@g.us', is_admin: true, is_favorite: false },
  { id: 'g2', name: 'Beta Group', wa_group_id: '222@g.us', is_admin: true, is_favorite: true },
  {
    id: 'g3',
    name: 'Charlie Non-Admin',
    wa_group_id: '333@g.us',
    is_admin: false,
    is_favorite: false,
  },
];

vi.mock('@/hooks/use-groups-connect', () => ({
  useGroupsConnect: () => ({
    groups: mockGroupsData,
    loading: false,
    toggleGroupFavorite: mockToggleGroupFavorite,
    refetch: vi.fn(),
  }),
}));

vi.mock('@/hooks/use-dispatch-texts-connect', () => ({
  useDispatchTextsConnect: () => ({
    dispatchTexts: [{ id: 'dt-1', name: 'Dispatch Text 1', is_active: true }],
    loading: false,
  }),
}));

const mockRefetchLists = vi.fn();
vi.mock('@/hooks/use-ml-links-connect', () => ({
  useMlLinksConnect: () => ({
    lists: [
      { id: 'ml-1', name: 'ML List 1', is_active: true },
      { id: 'ml-2', name: 'ML List Inactive', is_active: false },
    ],
    loadingLists: false,
    refetchLists: mockRefetchLists,
  }),
}));

vi.mock('@/hooks/use-profiles-connect', () => ({
  useProfilesConnect: () => ({
    limits: { max_automations: 5 },
    loading: false,
  }),
}));

vi.mock('@/hooks/use-instances-connect', () => ({
  useInstancesConnect: () => ({
    latestInstance: { status: 'connected' },
    loading: false,
  }),
}));

vi.mock('@/hooks/use-affiliate-credentials-connect', () => ({
  useAffiliateCredentialsConnect: () => ({
    activeStores: [
      { store: 'shopee', is_configured: true },
      { store: 'amazon', is_configured: false },
    ],
    loading: false,
  }),
}));

vi.mock('@/hooks/use-coupons', () => ({
  useCoupons: () => ({
    coupons: [
      { id: 'c1', code: 'SAVE10', title: 'Save 10%', store: 'shopee', is_active: true },
      { id: 'c2', code: 'OFF20', title: 'Off 20', store: 'amazon', is_active: false },
    ],
  }),
}));

const mockSearchOffers = vi.fn();
vi.mock('@/hooks/use-shopee-connect', () => ({
  useShopeeConnect: () => ({
    searchOffers: mockSearchOffers,
  }),
}));

const mockSuggestOfferText = vi.fn();
vi.mock('@/hooks/use-ai-tools-connect', () => ({
  useAiToolsConnect: () => ({
    suggestOfferText: mockSuggestOfferText,
  }),
}));

// ============================================================
// Import after mocks
// ============================================================

import type { LocalAutomation } from '../use-automations-connect';
import { useAutomations } from '../use-automations';

// ============================================================
// Helpers
// ============================================================

function createLocalAutomation(overrides: Partial<LocalAutomation> = {}): LocalAutomation {
  return {
    id: 'auto-1',
    name: 'Test Automation',
    mode: 'search',
    startTime: '08:00',
    endTime: '22:00',
    intervalMinutes: 30,
    sendGroups: ['group1@g.us'],
    stores: ['shopee'],
    categories: ['Eletrônicos'],
    priority: 'discount',
    filterType: 'light',
    texts: ['Check this out!'],
    ctas: [],
    status: 'active',
    ...overrides,
  };
}

const flushPromises = () =>
  act(async () => {
    /* await act flushes pending microtasks */
  });

async function renderAndWait() {
  let hookResult: ReturnType<typeof renderHook<ReturnType<typeof useAutomations>>>;

  await act(async () => {
    hookResult = renderHook(() => useAutomations());
  });

  await flushPromises();

  await waitFor(
    () => {
      expect(hookResult!.result.current.loading).toBe(false);
    },
    { timeout: 3000 }
  );

  return hookResult!;
}

// ============================================================
// Tests
// ============================================================

describe('use-automations edge cases', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUserValue = mockUser;
    mockIsCreatingAutomation = false;
    mockIsUpdatingAutomation = false;
    mockListAutomations.mockResolvedValue([createLocalAutomation()]);
    mockCreateAutomation.mockResolvedValue(createLocalAutomation({ id: 'new-auto' }));
    mockUpdateAutomation.mockResolvedValue(createLocalAutomation({ name: 'Updated' }));
    mockDeleteAutomation.mockResolvedValue(undefined);
    mockDiagnoseAutomation.mockResolvedValue({
      automationId: 'auto-1',
      code: 'SENT_OK',
      message: 'All checks passed',
      steps: [
        { step: 'CONFIG', ok: true, detail: 'Valid', message: '' },
        { step: 'WHATSAPP', ok: true, detail: 'Connected', message: '' },
      ],
      offersFound: 5,
      sentTo: ['group1'],
      error: '',
    });
    mockRunAutomation.mockResolvedValue({ success: true, message: '3 messages sent' });
    mockResetAutomationHistory.mockResolvedValue({ success: true, message: 'History cleared' });
    mockSearchOffers.mockResolvedValue([]);
    mockSuggestOfferText.mockResolvedValue([]);
    mockToggleGroupFavorite.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── confirmDelete ──────────────────────────────────────────

  describe('confirmDelete', () => {
    it('should be a no-op when deleteTargetId is null', async () => {
      const { result } = await renderAndWait();

      // deleteTargetId is null by default
      await act(async () => {
        await result.current.confirmDelete();
      });

      expect(mockDeleteAutomation).not.toHaveBeenCalled();
    });

    it('should show error toast when API fails', async () => {
      mockDeleteAutomation.mockRejectedValue(new Error('Server error'));

      const { result } = await renderAndWait();

      act(() => {
        result.current.setDeleteTargetId('auto-1');
      });

      await act(async () => {
        await result.current.confirmDelete();
      });

      expect(mockDeleteAutomation).toHaveBeenCalledWith('auto-1');
      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
      // Should clear deleteTargetId after error
      expect(result.current.deleteTargetId).toBeNull();
    });

    it('should show success toast and reload on success', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.setDeleteTargetId('auto-1');
      });

      await act(async () => {
        await result.current.confirmDelete();
      });

      expect(mockDeleteAutomation).toHaveBeenCalledWith('auto-1');
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.offerAutomations.success.deleted',
        })
      );
      // Should reload automations
      expect(mockListAutomations).toHaveBeenCalledTimes(2); // initial + reload
      expect(result.current.deleteTargetId).toBeNull();
    });
  });

  // ── confirmResetHistory ────────────────────────────────────

  describe('confirmResetHistory', () => {
    it('should be a no-op when resetTargetId is null', async () => {
      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.confirmResetHistory();
      });

      expect(mockResetAutomationHistory).not.toHaveBeenCalled();
    });

    it('should set resettingId lifecycle and show success toast', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.setResetTarget('auto-1', 'Test Auto');
      });

      expect(result.current.resetTargetId).toBe('auto-1');
      expect(result.current.resetTargetName).toBe('Test Auto');

      await act(async () => {
        await result.current.confirmResetHistory();
      });

      expect(mockResetAutomationHistory).toHaveBeenCalledWith('auto-1');
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.offerAutomations.success.historyReset',
        })
      );
      // Should clear state after
      expect(result.current.resettingId).toBeNull();
      expect(result.current.resetTargetId).toBeNull();
      expect(result.current.resetTargetName).toBeNull();
    });

    it('should show error toast and clear state on API failure', async () => {
      mockResetAutomationHistory.mockRejectedValue(new Error('Reset failed'));

      const { result } = await renderAndWait();

      act(() => {
        result.current.setResetTarget('auto-1', 'Test Auto');
      });

      await act(async () => {
        await result.current.confirmResetHistory();
      });

      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
      expect(result.current.resettingId).toBeNull();
      expect(result.current.resetTargetId).toBeNull();
      expect(result.current.resetTargetName).toBeNull();
    });
  });

  // ── handleResetHistory / setResetTarget ────────────────────

  describe('handleResetHistory / setResetTarget', () => {
    it('should set resetTargetId and resetTargetName via handleResetHistory', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.handleResetHistory('auto-1', 'My Auto');
      });

      expect(result.current.resetTargetId).toBe('auto-1');
      expect(result.current.resetTargetName).toBe('My Auto');
    });

    it('should clear via setResetTarget(null)', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.setResetTarget('auto-1', 'Name');
      });

      expect(result.current.resetTargetId).toBe('auto-1');

      act(() => {
        result.current.setResetTarget(null);
      });

      expect(result.current.resetTargetId).toBeNull();
      expect(result.current.resetTargetName).toBeNull();
    });
  });

  // ── handlePreviewOffers ────────────────────────────────────

  describe('handlePreviewOffers', () => {
    const shopeeAutomation = {
      id: 'auto-1',
      name: 'Test',
      mode: 'search' as const,
      start_time: '08:00',
      end_time: '22:00',
      interval_minutes: 30,
      send_groups: ['g@g.us'],
      stores: ['shopee' as const],
      categories: ['Eletrônicos'],
      priority: 'discount' as const,
      filter_type: 'light' as const,
      texts: [],
      ctas: [],
      status: 'active' as const,
    };

    it('should reject non-Shopee automations', async () => {
      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handlePreviewOffers({
          ...shopeeAutomation,
          stores: ['amazon'],
        });
      });

      expect(mockSearchOffers).not.toHaveBeenCalled();
      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
    });

    it('should map offers on success', async () => {
      mockSearchOffers.mockResolvedValue([
        {
          title: 'iPhone 15',
          originalPrice: 7999,
          finalPrice: 5499,
          discountPercent: 31,
          imageUrl: 'https://example.com/img.jpg',
          productUrl: 'https://shopee.com/product/1',
          storeName: 'Apple Store',
          commissionRate: 8.5,
        },
      ]);

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handlePreviewOffers(shopeeAutomation);
      });

      expect(result.current.previewOffers).toHaveLength(1);
      expect(result.current.previewOffers[0].title).toBe('iPhone 15');
      expect(result.current.previewDialogOpen).toBe(true);
      expect(result.current.loadingPreview).toBe(false);
    });

    it('should show toast when result is empty', async () => {
      mockSearchOffers.mockResolvedValue([]);

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handlePreviewOffers(shopeeAutomation);
      });

      expect(result.current.previewOffers).toHaveLength(0);
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.offerAutomations.errors.noOffersFound',
        })
      );
    });

    it('should show error toast on API failure', async () => {
      mockSearchOffers.mockRejectedValue(new Error('API down'));

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handlePreviewOffers(shopeeAutomation);
      });

      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
      expect(result.current.loadingPreview).toBe(false);
    });
  });

  // ── suggestTexts ───────────────────────────────────────────

  describe('suggestTexts', () => {
    it('should populate texts on success (truncated to 3)', async () => {
      mockSuggestOfferText.mockResolvedValue([
        'Text 1',
        'Text 2',
        'Text 3',
        'Text 4 should be dropped',
      ]);

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.suggestTexts();
      });

      expect(result.current.formData.texts).toEqual(['Text 1', 'Text 2', 'Text 3']);
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.offerAutomations.success.textsSuggested',
        })
      );
    });

    it('should not update formData when empty array returned', async () => {
      mockSuggestOfferText.mockResolvedValue([]);

      const { result } = await renderAndWait();

      const textsBefore = result.current.formData.texts;

      await act(async () => {
        await result.current.suggestTexts();
      });

      // Texts should remain unchanged (the if guard prevents update)
      expect(result.current.formData.texts).toEqual(textsBefore);
    });

    it('should show error toast on failure', async () => {
      mockSuggestOfferText.mockRejectedValue(new Error('AI service unavailable'));

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.suggestTexts();
      });

      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
    });
  });

  // ── toggleFavorite ─────────────────────────────────────────

  describe('toggleFavorite', () => {
    it('should toggle favorite on success', async () => {
      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.toggleFavorite('g1');
      });

      expect(mockToggleGroupFavorite).toHaveBeenCalledWith('g1', true); // g1 is_favorite=false -> toggle to true
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.offerAutomations.success.addedFavorite',
        })
      );
    });

    it('should be no-op when group not found', async () => {
      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.toggleFavorite('nonexistent-id');
      });

      expect(mockToggleGroupFavorite).not.toHaveBeenCalled();
    });

    it('should show error toast on failure', async () => {
      mockToggleGroupFavorite.mockRejectedValue(new Error('Network error'));

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.toggleFavorite('g1');
      });

      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
    });
  });

  // ── loadMlLinkLists ────────────────────────────────────────

  describe('loadMlLinkLists', () => {
    it('should call refetchLists', async () => {
      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.loadMlLinkLists();
      });

      expect(mockRefetchLists).toHaveBeenCalled();
    });
  });

  // ── handleSave API errors ──────────────────────────────────

  describe('handleSave API errors', () => {
    it('should show error toast when createAutomation fails', async () => {
      mockListAutomations.mockResolvedValue([]);
      mockCreateAutomation.mockRejectedValue(new Error('Create failed'));

      const { result } = await renderAndWait();

      act(() => {
        result.current.setFormData({
          name: 'New Auto',
          send_groups: ['group@g.us'],
          stores: ['shopee'],
          categories: ['Eletrônicos'],
          mode: 'search',
          priority: 'discount',
          filter_type: 'light',
          texts: [],
          ctas: [],
          status: 'active',
        });
      });

      await act(async () => {
        await result.current.handleSave();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.offerAutomations.errors.save',
          variant: 'destructive',
        })
      );
    });

    it('should show error toast when updateAutomation fails', async () => {
      mockUpdateAutomation.mockRejectedValue(new Error('Update failed'));

      const { result } = await renderAndWait();

      act(() => {
        result.current.openEdit({
          id: 'auto-1',
          name: 'Test',
          mode: 'search',
          start_time: '08:00',
          end_time: '22:00',
          interval_minutes: 30,
          send_groups: ['group@g.us'],
          stores: ['shopee'],
          categories: ['Eletrônicos'],
          priority: 'discount',
          filter_type: 'light',
          texts: ['Hello'],
          ctas: [],
          status: 'active',
        });
      });

      await act(async () => {
        await result.current.handleSave();
      });

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.offerAutomations.errors.save',
          variant: 'destructive',
        })
      );
    });
  });

  // ── loadAutomations error ──────────────────────────────────

  describe('loadAutomations error', () => {
    it('should show toast when loadAutomations fails', async () => {
      mockListAutomations.mockRejectedValue(new Error('Network error'));

      let hookResult: ReturnType<typeof renderHook<ReturnType<typeof useAutomations>>>;

      await act(async () => {
        hookResult = renderHook(() => useAutomations());
      });

      await flushPromises();

      await waitFor(
        () => {
          expect(hookResult!.result.current.loading).toBe(false);
        },
        { timeout: 3000 }
      );

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'pages.offerAutomations.errors.loadAutomations',
          variant: 'destructive',
        })
      );
    });
  });

  // ── monitor delay unit auto-detection ──────────────────────

  describe('monitor delay unit auto-detection', () => {
    it('should detect minutes when delay_seconds >= 60 and divisible by 60', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.openEdit({
          id: 'auto-1',
          name: 'Monitor',
          mode: 'monitor',
          type: 'monitoring',
          start_time: '08:00',
          end_time: '22:00',
          interval_minutes: 30,
          send_groups: ['g@g.us'],
          monitor_groups: ['m@g.us'],
          stores: ['shopee'],
          categories: [],
          priority: 'discount',
          filter_type: 'light',
          texts: [],
          ctas: [],
          status: 'active',
          delay_seconds: 120,
        });
      });

      await waitFor(() => {
        expect(result.current.monitorDelayUnit).toBe('minutes');
        expect(result.current.monitorDelayInput).toBe('2');
      });
    });

    it('should detect seconds when delay_seconds < 60', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.openEdit({
          id: 'auto-1',
          name: 'Monitor',
          mode: 'monitor',
          type: 'monitoring',
          start_time: '08:00',
          end_time: '22:00',
          interval_minutes: 30,
          send_groups: ['g@g.us'],
          monitor_groups: ['m@g.us'],
          stores: ['shopee'],
          categories: [],
          priority: 'discount',
          filter_type: 'light',
          texts: [],
          ctas: [],
          status: 'active',
          delay_seconds: 45,
        });
      });

      await waitFor(() => {
        expect(result.current.monitorDelayUnit).toBe('seconds');
        expect(result.current.monitorDelayInput).toBe('45');
      });
    });

    it('should detect seconds when delay_seconds is not divisible by 60', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.openEdit({
          id: 'auto-1',
          name: 'Monitor',
          mode: 'monitor',
          type: 'monitoring',
          start_time: '08:00',
          end_time: '22:00',
          interval_minutes: 30,
          send_groups: ['g@g.us'],
          monitor_groups: ['m@g.us'],
          stores: ['shopee'],
          categories: [],
          priority: 'discount',
          filter_type: 'light',
          texts: [],
          ctas: [],
          status: 'active',
          delay_seconds: 90,
        });
      });

      await waitFor(() => {
        expect(result.current.monitorDelayUnit).toBe('seconds');
        expect(result.current.monitorDelayInput).toBe('90');
      });
    });
  });

  // ── openEdit refinement state ──────────────────────────────

  describe('openEdit refinement state', () => {
    it('should open eletronicos refinement when subcategory present', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.openEdit({
          id: 'auto-1',
          name: 'Eletronicos Auto',
          mode: 'search',
          start_time: '08:00',
          end_time: '22:00',
          interval_minutes: 30,
          send_groups: ['g@g.us'],
          stores: ['shopee'],
          categories: ['Eletrônicos - Computadores (Notebooks)'],
          priority: 'discount',
          filter_type: 'light',
          texts: [],
          ctas: [],
          status: 'active',
        });
      });

      expect(result.current.refinementOpen.eletronicos).toBe(true);
      expect(result.current.refinementOpen.moda).toBe(false);
    });

    it('should open moda refinement when moda subcategory present', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.openEdit({
          id: 'auto-1',
          name: 'Moda Auto',
          mode: 'search',
          start_time: '08:00',
          end_time: '22:00',
          interval_minutes: 30,
          send_groups: ['g@g.us'],
          stores: ['shopee'],
          categories: ['Moda Feminina - Vestidos'],
          priority: 'discount',
          filter_type: 'light',
          texts: [],
          ctas: [],
          status: 'active',
        });
      });

      expect(result.current.refinementOpen.moda).toBe(true);
      expect(result.current.refinementOpen.eletronicos).toBe(false);
    });
  });

  // ── default form data ──────────────────────────────────────

  describe('default form data', () => {
    it('should have correct defaults after resetForm', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.openEdit({
          id: 'auto-1',
          name: 'Test',
          mode: 'monitor',
          start_time: '10:00',
          end_time: '20:00',
          interval_minutes: 60,
          send_groups: ['g@g.us'],
          stores: ['shopee'],
          categories: [],
          priority: 'commission',
          filter_type: 'heavy',
          texts: ['text'],
          ctas: [],
          status: 'paused',
        });
      });

      expect(result.current.editingId).toBe('auto-1');

      act(() => result.current.resetForm());

      expect(result.current.editingId).toBeNull();
      expect(result.current.formData.mode).toBe('search');
      expect(result.current.formData.start_time).toBe('00:01');
      expect(result.current.formData.end_time).toBe('23:59');
      expect(result.current.formData.interval_minutes).toBe(30);
      expect(result.current.formData.priority).toBe('discount');
      expect(result.current.formData.filter_type).toBe('light');
      expect(result.current.formData.status).toBe('active');
      expect(result.current.formData.delay_seconds).toBe(10);
      expect(result.current.formData.list_items_per_run).toBe(1);
      expect(result.current.formData.list_include_image).toBe(true);
    });
  });

  // ── handleSave mode-specific params ────────────────────────

  describe('handleSave mode-specific params', () => {
    it('should include monitoringTextRules for monitor mode', async () => {
      mockListAutomations.mockResolvedValue([]);

      const { result } = await renderAndWait();

      act(() => {
        result.current.setFormData({
          name: 'Monitor Auto',
          send_groups: ['g@g.us'],
          stores: ['shopee'],
          categories: [],
          mode: 'monitor',
          monitor_groups: ['m@g.us'],
          priority: 'discount',
          filter_type: 'light',
          texts: [],
          ctas: [],
          status: 'active',
          monitoring_text_rules: {
            substituicoes: [{ de: 'old', para: 'new' }],
            remover_termos: ['spam'],
            prefixo: 'PREFIX',
            sufixo: 'SUFFIX',
            limpar_telefones: true,
            limpar_arrobas: false,
          },
        });
      });

      await act(async () => {
        await result.current.handleSave();
      });

      expect(mockCreateAutomation).toHaveBeenCalledWith(
        expect.objectContaining({
          mode: 'monitor',
          monitoringTextRules: expect.objectContaining({
            substituicoes: [{ de: 'old', para: 'new' }],
            remover_termos: ['spam'],
            prefixo: 'PREFIX',
          }),
        })
      );
    });

    it('should force priority=discount for monitor mode', async () => {
      mockListAutomations.mockResolvedValue([]);

      const { result } = await renderAndWait();

      act(() => {
        result.current.setFormData({
          name: 'Monitor Auto',
          send_groups: ['g@g.us'],
          stores: ['shopee'],
          categories: [],
          mode: 'monitor',
          monitor_groups: ['m@g.us'],
          priority: 'commission',
          filter_type: 'heavy',
          texts: [],
          ctas: [],
          status: 'active',
        });
      });

      await act(async () => {
        await result.current.handleSave();
      });

      expect(mockCreateAutomation).toHaveBeenCalledWith(
        expect.objectContaining({
          priority: 'discount',
          filterType: 'light',
        })
      );
    });

    it('should clamp listItemsPerRun between 1 and 20 for list mode', async () => {
      mockListAutomations.mockResolvedValue([]);

      const { result } = await renderAndWait();

      act(() => {
        result.current.setFormData({
          name: 'List Auto',
          send_groups: ['g@g.us'],
          stores: ['shopee'],
          categories: [],
          mode: 'list',
          priority: 'discount',
          filter_type: 'light',
          texts: [],
          ctas: [],
          status: 'active',
          list_items_per_run: 25,
          list_include_image: true,
        });
      });

      await act(async () => {
        await result.current.handleSave();
      });

      expect(mockCreateAutomation).toHaveBeenCalledWith(
        expect.objectContaining({
          listItemsPerRun: 20,
          listIncludeImage: true,
          stores: ['ml'],
        })
      );
    });
  });

  // ── handleRunNow exception handling ────────────────────────

  describe('handleRunNow exception handling', () => {
    const testAutomation = {
      id: 'auto-1',
      name: 'Test',
      mode: 'search' as const,
      start_time: '08:00',
      end_time: '22:00',
      interval_minutes: 30,
      send_groups: ['g@g.us'],
      stores: ['shopee' as const],
      categories: ['Eletrônicos'],
      priority: 'discount' as const,
      filter_type: 'light' as const,
      texts: [] as string[],
      ctas: [] as string[],
      status: 'active' as const,
    };

    it('should show error toast when diagnose throws', async () => {
      mockDiagnoseAutomation.mockRejectedValue(new Error('Diagnose failed'));

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handleRunNow(testAutomation);
      });

      expect(mockRunAutomation).not.toHaveBeenCalled();
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          description: 'Diagnose failed',
        })
      );
      expect(result.current.runningId).toBeNull();
    });

    it('should show error toast when run throws after successful diagnose', async () => {
      mockRunAutomation.mockRejectedValue(new Error('Run failed'));

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handleRunNow(testAutomation);
      });

      expect(mockDiagnoseAutomation).toHaveBeenCalledWith('auto-1');
      expect(mockRunAutomation).toHaveBeenCalledWith('auto-1');
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          description: 'Run failed',
        })
      );
      expect(result.current.runningId).toBeNull();
    });
  });
});
