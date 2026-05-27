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
const mockI18n = { language: 'pt-BR' };

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: mockT,
    i18n: mockI18n,
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
    isCreatingAutomation: false,
    isUpdatingAutomation: false,
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

import { fromLocalAutomation, toLocalMonitoringRules, useAutomations } from '../use-automations';
import type { LocalAutomation } from '../use-automations-connect';

// ============================================================
// Test Helpers
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

/** Flush microtask queue */
const flushPromises = () =>
  act(async () => {
    /* await act flushes pending microtasks */
  });

/** Renders the hook and waits for initial loading to complete */
async function renderAndWait() {
  let hookResult: ReturnType<typeof renderHook<ReturnType<typeof useAutomations>>>;

  await act(async () => {
    hookResult = renderHook(() => useAutomations());
  });

  // Flush remaining microtasks (loadAutomations promise resolution + state updates)
  await flushPromises();

  // Wait for loading to become false
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

describe('use-automations', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUserValue = mockUser;
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
    vi.spyOn(globalThis, 'confirm').mockReturnValue(true);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── Pure function: fromLocalAutomation ────────────────────

  describe('fromLocalAutomation', () => {
    it('should convert search mode LocalAutomation to Automation', () => {
      const local = createLocalAutomation({ mode: 'search' });
      const result = fromLocalAutomation(local);

      expect(result.id).toBe('auto-1');
      expect(result.mode).toBe('search');
      expect(result.type).toBe('search');
      expect(result.stores).toEqual(['shopee']);
      expect(result.interval_minutes).toBe(30);
    });

    it('should convert monitor mode with type=monitoring', () => {
      const local = createLocalAutomation({ mode: 'monitor' });
      const result = fromLocalAutomation(local);

      expect(result.mode).toBe('monitor');
      expect(result.type).toBe('monitoring');
    });

    it('should convert list mode with type=list', () => {
      const local = createLocalAutomation({ mode: 'list' });
      const result = fromLocalAutomation(local);

      expect(result.mode).toBe('list');
      expect(result.type).toBe('list');
    });

    it('should map null optional fields correctly', () => {
      const local = createLocalAutomation({
        dispatchTextId: undefined,
        monitoringTextRules: undefined,
        minDiscount: undefined,
        minPrice: undefined,
        maxPrice: undefined,
        couponId: undefined,
        mlLinkListId: undefined,
      });
      const result = fromLocalAutomation(local);

      expect(result.dispatch_text_id).toBeNull();
      expect(result.monitoring_text_rules).toBeNull();
      expect(result.min_discount).toBeNull();
      expect(result.min_price).toBeNull();
      expect(result.max_price).toBeNull();
      expect(result.coupon_id).toBeNull();
      expect(result.ml_link_list_id).toBeNull();
    });

    it('should clamp interval_minutes via clampIntervalMinutes', () => {
      const local = createLocalAutomation({ intervalMinutes: 1 });
      const result = fromLocalAutomation(local);

      expect(result.interval_minutes).toBe(3);
    });

    it('should convert monitoringTextRules when present', () => {
      const local = createLocalAutomation({
        mode: 'monitor',
        monitoringTextRules: {
          substituicoes: [{ de: 'old', para: 'new' }],
          remover_termos: ['spam'],
          prefixo: 'PREFIX:',
          sufixo: ':SUFFIX',
          limpar_telefones: true,
          limpar_arrobas: false,
        },
      });
      const result = fromLocalAutomation(local);

      expect(result.monitoring_text_rules).toEqual({
        substituicoes: [{ de: 'old', para: 'new' }],
        remover_termos: ['spam'],
        prefixo: 'PREFIX:',
        sufixo: ':SUFFIX',
        limpar_telefones: true,
        limpar_arrobas: false,
      });
    });

    it('should convert list mode fields', () => {
      const local = createLocalAutomation({
        mode: 'list',
        mlLinkListId: 'list-123',
        listItemsPerRun: 5,
        listIncludeImage: false,
      });
      const result = fromLocalAutomation(local);

      expect(result.ml_link_list_id).toBe('list-123');
      expect(result.list_items_per_run).toBe(5);
      expect(result.list_include_image).toBe(false);
    });
  });

  // ── Pure function: toLocalMonitoringRules ──────────────────

  describe('toLocalMonitoringRules', () => {
    it('should return undefined for null input', () => {
      expect(toLocalMonitoringRules(null)).toBeUndefined();
    });

    it('should return undefined for undefined input', () => {
      expect(toLocalMonitoringRules(undefined)).toBeUndefined();
    });

    it('should convert full rules correctly', () => {
      const result = toLocalMonitoringRules({
        substituicoes: [{ de: 'a', para: 'b' }],
        remover_termos: ['x'],
        prefixo: 'P',
        sufixo: 'S',
        limpar_telefones: true,
        limpar_arrobas: true,
      });

      expect(result).toEqual({
        substituicoes: [{ de: 'a', para: 'b' }],
        remover_termos: ['x'],
        prefixo: 'P',
        sufixo: 'S',
        limpar_telefones: true,
        limpar_arrobas: true,
      });
    });

    it('should default empty fields when null/undefined in input', () => {
      const result = toLocalMonitoringRules({
        substituicoes: [{ de: 'a', para: 'b' }],
        remover_termos: ['x'],
        prefixo: '',
        sufixo: '',
        limpar_telefones: false,
        limpar_arrobas: false,
      });

      expect(result!.prefixo).toBe('');
      expect(result!.sufixo).toBe('');
      expect(result!.limpar_telefones).toBe(false);
      expect(result!.limpar_arrobas).toBe(false);
    });
  });

  // ── Hook: initial state & data loading ────────────────────

  describe('initial state and data loading', () => {
    it('should call listAutomations on mount when user exists', async () => {
      await renderAndWait();
      expect(mockListAutomations).toHaveBeenCalledTimes(1);
    });

    it('should NOT call listAutomations when user is null', async () => {
      mockUserValue = null;

      await act(async () => {
        renderHook(() => useAutomations());
      });

      await flushPromises();

      expect(mockListAutomations).not.toHaveBeenCalled();
    });

    it('should return loading=false after fetch completes', async () => {
      const { result } = await renderAndWait();
      expect(result.current.loading).toBe(false);
    });
  });

  // ── Hook: derived state ────────────────────────────────────

  describe('derived state', () => {
    it('should compute limitReached when automations >= maxAutomations', async () => {
      const fiveAutomations = Array.from({ length: 5 }, (_, i) =>
        createLocalAutomation({ id: `auto-${i}` })
      );
      mockListAutomations.mockResolvedValue(fiveAutomations);

      const { result } = await renderAndWait();

      expect(result.current.limitReached).toBe(true);
      expect(result.current.nearLimit).toBe(true);
    });

    it('should filter groups to admin-only and sort by favorite then name', async () => {
      const { result } = await renderAndWait();

      // Should exclude non-admin (g3)
      expect(result.current.groups).toHaveLength(2);
      // Favorites first (g2 Beta), then alphabetical (g1 Alpha)
      expect(result.current.groups[0].name).toBe('Beta Group');
      expect(result.current.groups[1].name).toBe('Alpha Group');
    });

    it('should reflect whatsappConnected from instance status', async () => {
      const { result } = await renderAndWait();
      expect(result.current.whatsappConnected).toBe(true);
    });

    it('should filter active stores to configured only', async () => {
      const { result } = await renderAndWait();
      expect(result.current.activeStores).toEqual(['shopee']);
    });

    it('should filter active coupons only', async () => {
      const { result } = await renderAndWait();
      expect(result.current.availableCoupons).toHaveLength(1);
      expect(result.current.availableCoupons[0].code).toBe('SAVE10');
    });

    it('should filter active ML link lists only', async () => {
      const { result } = await renderAndWait();
      expect(result.current.mlLinkLists).toHaveLength(1);
      expect(result.current.mlLinkLists[0].name).toBe('ML List 1');
    });

    it('should return maxAutomations from profile limits', async () => {
      const { result } = await renderAndWait();
      expect(result.current.maxAutomations).toBe(5);
    });
  });

  // ── Hook: handleSave validation ────────────────────────────

  describe('handleSave validation', () => {
    it('should reject when name is empty', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.setFormData({
          name: '',
          send_groups: ['group@g.us'],
          stores: ['shopee'],
          categories: ['Eletrônicos'],
          mode: 'search',
        });
      });

      await act(async () => {
        await result.current.handleSave();
      });

      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
      expect(mockCreateAutomation).not.toHaveBeenCalled();
    });

    it('should reject when sendGroups is empty', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.setFormData({
          name: 'Test',
          send_groups: [],
          stores: ['shopee'],
          categories: ['Eletrônicos'],
          mode: 'search',
        });
      });

      await act(async () => {
        await result.current.handleSave();
      });

      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
      expect(mockCreateAutomation).not.toHaveBeenCalled();
    });

    it('should reject when stores is empty', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.setFormData({
          name: 'Test',
          send_groups: ['group@g.us'],
          stores: [],
          categories: ['Eletrônicos'],
          mode: 'search',
        });
      });

      await act(async () => {
        await result.current.handleSave();
      });

      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
      expect(mockCreateAutomation).not.toHaveBeenCalled();
    });

    it('should reject search mode when categories is empty', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.setFormData({
          name: 'Test',
          send_groups: ['group@g.us'],
          stores: ['shopee'],
          categories: [],
          mode: 'search',
        });
      });

      await act(async () => {
        await result.current.handleSave();
      });

      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
      expect(mockCreateAutomation).not.toHaveBeenCalled();
    });

    it('should reject monitor mode when monitorGroupIds is empty', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.setFormData({
          name: 'Test',
          send_groups: ['group@g.us'],
          stores: ['shopee'],
          categories: [],
          mode: 'monitor',
          monitor_groups: [],
        });
      });

      await act(async () => {
        await result.current.handleSave();
      });

      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
      expect(mockCreateAutomation).not.toHaveBeenCalled();
    });

    it('should reject monitor mode when monitor groups overlap with send groups', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.setFormData({
          name: 'Test',
          send_groups: ['group@g.us'],
          stores: ['shopee'],
          categories: [],
          mode: 'monitor',
          monitor_groups: ['group@g.us'],
        });
      });

      await act(async () => {
        await result.current.handleSave();
      });

      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
      expect(mockCreateAutomation).not.toHaveBeenCalled();
    });

    it('should reject creation when limit reached', async () => {
      const fiveAutomations = Array.from({ length: 5 }, (_, i) =>
        createLocalAutomation({ id: `auto-${i}` })
      );
      mockListAutomations.mockResolvedValue(fiveAutomations);

      const { result } = await renderAndWait();

      act(() => {
        result.current.setFormData({
          name: 'Test',
          send_groups: ['group@g.us'],
          stores: ['shopee'],
          categories: ['Eletrônicos'],
          mode: 'search',
        });
      });

      await act(async () => {
        await result.current.handleSave();
      });

      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
      expect(mockCreateAutomation).not.toHaveBeenCalled();
    });
  });

  // ── Hook: handleSave create/update ─────────────────────────

  describe('handleSave create/update', () => {
    it('should call createAutomation with correct params for new automation', async () => {
      mockListAutomations.mockResolvedValue([]);

      const { result } = await renderAndWait();

      act(() => {
        result.current.setFormData({
          name: 'New Auto',
          send_groups: ['group@g.us'],
          stores: ['shopee'],
          categories: ['Eletrônicos'],
          mode: 'search',
          start_time: '09:00',
          end_time: '21:00',
          interval_minutes: 15,
          priority: 'discount',
          filter_type: 'light',
          texts: ['Hello!'],
          ctas: [],
          status: 'active',
        });
      });

      await act(async () => {
        await result.current.handleSave();
      });

      expect(mockCreateAutomation).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'New Auto',
          mode: 'search',
          stores: ['shopee'],
          categories: ['Eletrônicos'],
        })
      );
    });

    it('should enforce GENERAL_CATEGORY exclusivity in search mode', async () => {
      mockListAutomations.mockResolvedValue([]);

      const { result } = await renderAndWait();

      act(() => {
        result.current.setFormData({
          name: 'General Auto',
          send_groups: ['group@g.us'],
          stores: ['shopee'],
          categories: ['Ofertas gerais', 'Eletrônicos'],
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

      expect(mockCreateAutomation).toHaveBeenCalledWith(
        expect.objectContaining({
          categories: ['Ofertas gerais'],
        })
      );
    });

    it('should force stores=["ml"] for list mode', async () => {
      mockListAutomations.mockResolvedValue([]);

      const { result } = await renderAndWait();

      act(() => {
        result.current.setFormData({
          name: 'List Auto',
          send_groups: ['group@g.us'],
          stores: ['shopee'],
          categories: [],
          mode: 'list',
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

      expect(mockCreateAutomation).toHaveBeenCalledWith(
        expect.objectContaining({
          stores: ['ml'],
        })
      );
    });

    it('should call updateAutomation for editing', async () => {
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

      act(() => {
        result.current.setFormData((prev) => ({ ...prev, name: 'Updated Name' }));
      });

      await act(async () => {
        await result.current.handleSave();
      });

      expect(mockUpdateAutomation).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'auto-1',
          name: 'Updated Name',
        })
      );
    });

    it('should close dialog and reset form on successful save', async () => {
      mockListAutomations.mockResolvedValue([]);

      const { result } = await renderAndWait();

      act(() => {
        result.current.setDialogOpen(true);
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

      expect(result.current.dialogOpen).toBe(false);
      expect(result.current.editingId).toBeNull();
    });
  });

  // ── Hook: actions ──────────────────────────────────────────

  describe('actions', () => {
    it('should toggle status from active to paused', async () => {
      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.toggleStatus('auto-1', 'active');
      });

      expect(mockUpdateAutomation).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'auto-1',
          status: 'paused',
        })
      );
    });

    it('should toggle status from paused to active', async () => {
      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.toggleStatus('auto-1', 'paused');
      });

      expect(mockUpdateAutomation).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'auto-1',
          status: 'active',
        })
      );
    });

    it('should call deleteAutomation when confirmed', async () => {
      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handleDelete('auto-1');
      });

      expect(mockDeleteAutomation).toHaveBeenCalledWith('auto-1');
    });

    it('should NOT call deleteAutomation when confirm returns false', async () => {
      vi.spyOn(globalThis, 'confirm').mockReturnValue(false);

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handleDelete('auto-1');
      });

      expect(mockDeleteAutomation).not.toHaveBeenCalled();
    });

    it('should populate formData when openEdit is called', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.openEdit({
          id: 'auto-1',
          name: 'Edit Me',
          mode: 'search',
          type: 'search',
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

      expect(result.current.editingId).toBe('auto-1');
      expect(result.current.dialogOpen).toBe(true);
      expect(result.current.formData.name).toBe('Edit Me');
    });
  });

  // ── Hook: text management ──────────────────────────────────

  describe('text management', () => {
    it('should addText up to 3 texts', async () => {
      const { result } = await renderAndWait();

      // Default form has texts: ['']
      act(() => result.current.addText());
      expect(result.current.formData.texts).toHaveLength(2);

      act(() => result.current.addText());
      expect(result.current.formData.texts).toHaveLength(3);

      // Should not add beyond 3
      act(() => result.current.addText());
      expect(result.current.formData.texts).toHaveLength(3);
    });

    it('should updateText at specific index', async () => {
      const { result } = await renderAndWait();

      act(() => result.current.updateText(0, 'Updated text'));
      expect(result.current.formData.texts![0]).toBe('Updated text');
    });

    it('should removeText at specific index', async () => {
      const { result } = await renderAndWait();

      act(() => result.current.addText());
      act(() => result.current.updateText(1, 'Second'));

      const lengthBefore = result.current.formData.texts!.length;
      act(() => result.current.removeText(0));
      expect(result.current.formData.texts!.length).toBe(lengthBefore - 1);
    });
  });

  // ── Hook: handleRunNow ─────────────────────────────────────

  describe('handleRunNow', () => {
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

    it('should call diagnose then run on success code', async () => {
      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handleRunNow(testAutomation);
      });

      expect(mockDiagnoseAutomation).toHaveBeenCalledWith('auto-1');
      expect(mockRunAutomation).toHaveBeenCalledWith('auto-1');
    });

    it('should show error toast when diagnostic code is not success', async () => {
      mockDiagnoseAutomation.mockResolvedValue({
        automationId: 'auto-1',
        code: 'WHATSAPP_DISCONNECTED',
        message: 'WhatsApp is disconnected',
        steps: [{ step: 'WHATSAPP', ok: false, detail: 'Disconnected', message: '' }],
        offersFound: 0,
        sentTo: [],
        error: '',
      });

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handleRunNow(testAutomation);
      });

      expect(mockDiagnoseAutomation).toHaveBeenCalledWith('auto-1');
      expect(mockRunAutomation).not.toHaveBeenCalled();
      expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
    });

    it('should handle ENVIO_PRONTO code as success', async () => {
      mockDiagnoseAutomation.mockResolvedValue({
        automationId: 'auto-1',
        code: 'ENVIO_PRONTO',
        message: 'Ready to send',
        steps: [{ step: 'CONFIG', ok: true, detail: 'Valid', message: '' }],
        offersFound: 3,
        sentTo: [],
        error: '',
      });

      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handleRunNow(testAutomation);
      });

      expect(mockRunAutomation).toHaveBeenCalledWith('auto-1');
    });

    it('should clear runningId after execution finishes', async () => {
      const { result } = await renderAndWait();

      await act(async () => {
        await result.current.handleRunNow(testAutomation);
      });

      expect(result.current.runningId).toBeNull();
    });
  });

  // ── Hook: resetForm ────────────────────────────────────────

  describe('resetForm', () => {
    it('should reset editingId and formData', async () => {
      const { result } = await renderAndWait();

      act(() => {
        result.current.openEdit({
          id: 'auto-1',
          name: 'Test',
          mode: 'search',
          start_time: '08:00',
          end_time: '22:00',
          interval_minutes: 30,
          send_groups: [],
          stores: ['shopee'],
          categories: [],
          priority: 'discount',
          filter_type: 'light',
          texts: [],
          ctas: [],
          status: 'active',
        });
      });

      expect(result.current.editingId).toBe('auto-1');

      act(() => result.current.resetForm());

      expect(result.current.editingId).toBeNull();
      expect(result.current.formData.name).toBeUndefined();
      expect(result.current.formData.mode).toBe('search');
    });
  });

  // ── Hook: return shape ─────────────────────────────────────

  describe('return shape', () => {
    it('should return all expected properties and functions', async () => {
      const { result } = await renderAndWait();

      expect(Array.isArray(result.current.automations)).toBe(true);
      expect(typeof result.current.maxAutomations).toBe('number');
      expect(Array.isArray(result.current.groups)).toBe(true);
      expect(Array.isArray(result.current.allGroups)).toBe(true);
      expect(typeof result.current.loading).toBe('boolean');
      expect(typeof result.current.dialogOpen).toBe('boolean');
      expect(typeof result.current.whatsappConnected).toBe('boolean');
      expect(Array.isArray(result.current.activeStores)).toBe(true);
      expect(typeof result.current.limitReached).toBe('boolean');
      expect(typeof result.current.nearLimit).toBe('boolean');

      expect(typeof result.current.handleSave).toBe('function');
      expect(typeof result.current.toggleStatus).toBe('function');
      expect(typeof result.current.handleDelete).toBe('function');
      expect(typeof result.current.openEdit).toBe('function');
      expect(typeof result.current.resetForm).toBe('function');
      expect(typeof result.current.handleRunNow).toBe('function');
      expect(typeof result.current.handleResetHistory).toBe('function');
      expect(typeof result.current.addText).toBe('function');
      expect(typeof result.current.updateText).toBe('function');
      expect(typeof result.current.removeText).toBe('function');
      expect(typeof result.current.toggleFavorite).toBe('function');
      expect(typeof result.current.handlePreviewOffers).toBe('function');
      expect(typeof result.current.suggestTexts).toBe('function');
      expect(typeof result.current.loadMlLinkLists).toBe('function');
    });
  });
});
