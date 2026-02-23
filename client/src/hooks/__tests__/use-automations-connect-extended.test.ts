import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules
// ============================================================

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

// ============================================================
// Mock @bufbuild/protobuf and @connectrpc
// ============================================================

const mockClientMethods = {
  listAutomations: vi.fn(),
  getAutomation: vi.fn(),
  createAutomation: vi.fn(),
  updateAutomation: vi.fn(),
  deleteAutomation: vi.fn(),
  diagnoseAutomation: vi.fn(),
  runAutomation: vi.fn(),
  resetAutomationHistory: vi.fn(),
};

vi.mock('@bufbuild/protobuf', () => ({
  create: vi.fn((_schema: unknown, data: unknown) => data),
}));

vi.mock('@connectrpc/connect', () => ({
  createClient: vi.fn(() => mockClientMethods),
}));

vi.mock('@connectrpc/connect-query', () => ({
  useTransport: vi.fn(() => ({})),
}));

vi.mock('@/lib/connect', () => ({
  handleConnectError: vi.fn((_error: unknown, _t: unknown, _opts: unknown) => ({
    userMessage: 'Lock conflict',
  })),
  isLockConflictError: vi.fn(() => false),
}));

vi.mock('@/lib/connect-transport', () => ({
  getErrorInfo: vi.fn((error: unknown) => ({
    userMessage: error instanceof Error ? error.message : 'Unknown error',
  })),
}));

// We need to mock useMutation to return a mock mutateAsync
const mutationResults = new Map<string, { mutateAsync: ReturnType<typeof vi.fn>; isPending: boolean }>();

vi.mock('@tanstack/react-query', () => ({
  useMutation: vi.fn((options: { mutationFn: (...args: unknown[]) => Promise<unknown> }) => {
    const mutateAsync = vi.fn((...args: unknown[]) => options.mutationFn(...args));
    return {
      mutateAsync,
      isPending: false,
      isError: false,
      isSuccess: false,
      data: undefined,
      error: null,
    };
  }),
}));

// ============================================================
// Import after mocks
// ============================================================

import { useAutomationsConnect } from '../use-automations-connect';
import { renderHook, act, waitFor } from '@testing-library/react';

// ============================================================
// Tests
// ============================================================

describe('use-automations-connect extended', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUserValue = mockUser;

    // Default mock responses
    mockClientMethods.listAutomations.mockResolvedValue({
      automations: [
        {
          id: 'auto-1',
          name: 'Test Auto',
          mode: 1, // SEARCH
          startTime: '08:00',
          endTime: '22:00',
          intervalMinutes: 30,
          sendGroups: ['g1'],
          monitorGroupIds: [],
          stores: [1], // SHOPEE
          categories: ['Eletrônicos'],
          priority: 1, // DISCOUNT
          filterType: 1, // LIGHT
          texts: ['hello'],
          ctas: [],
          status: 1, // ACTIVE
          minDiscount: 0,
          minPrice: 0,
          maxPrice: 0,
        },
      ],
    });

    mockClientMethods.getAutomation.mockResolvedValue({
      automation: {
        id: 'auto-1',
        name: 'Test Auto',
        mode: 1,
        startTime: '08:00',
        endTime: '22:00',
        intervalMinutes: 30,
        sendGroups: ['g1'],
        monitorGroupIds: [],
        stores: [1],
        categories: [],
        priority: 1,
        filterType: 1,
        texts: [],
        ctas: [],
        status: 1,
      },
    });

    mockClientMethods.createAutomation.mockResolvedValue({
      automation: {
        id: 'new-auto',
        name: 'New Auto',
        mode: 1,
        startTime: '08:00',
        endTime: '22:00',
        intervalMinutes: 30,
        sendGroups: ['g1'],
        monitorGroupIds: [],
        stores: [1],
        categories: [],
        priority: 1,
        filterType: 1,
        texts: [],
        ctas: [],
        status: 1,
      },
    });

    mockClientMethods.updateAutomation.mockResolvedValue({
      automation: {
        id: 'auto-1',
        name: 'Updated',
        mode: 1,
        startTime: '08:00',
        endTime: '22:00',
        intervalMinutes: 30,
        sendGroups: ['g1'],
        monitorGroupIds: [],
        stores: [1],
        categories: [],
        priority: 1,
        filterType: 1,
        texts: [],
        ctas: [],
        status: 1,
      },
    });

    mockClientMethods.deleteAutomation.mockResolvedValue({});

    mockClientMethods.diagnoseAutomation.mockResolvedValue({
      result: {
        automationId: 'auto-1',
        code: 'SENT_OK',
        message: 'OK',
        steps: [{ step: 'CONFIG', ok: true, detail: 'Valid' }],
        offersFound: 5,
        sentTo: ['g1'],
        error: '',
      },
    });

    mockClientMethods.runAutomation.mockResolvedValue({
      success: true,
      message: 'Executed',
      error: '',
    });

    mockClientMethods.resetAutomationHistory.mockResolvedValue({
      success: true,
      message: 'Reset done',
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── Auth guard ─────────────────────────────────────────────

  describe('auth guard', () => {
    it('should throw notAuthenticated when user is null for listAutomations', async () => {
      mockUserValue = null;

      const { result } = renderHook(() => useAutomationsConnect());

      await expect(
        act(async () => {
          await result.current.listAutomations();
        })
      ).rejects.toThrow('common.errors.notAuthenticated');
    });

    it('should throw notAuthenticated when user is null for createAutomation', async () => {
      mockUserValue = null;

      const { result } = renderHook(() => useAutomationsConnect());

      await expect(
        act(async () => {
          await result.current.createAutomation({
            name: 'Test',
            mode: 'search',
            startTime: '08:00',
            endTime: '22:00',
            intervalMinutes: 30,
            sendGroups: ['g1'],
            stores: ['shopee'],
            categories: [],
            priority: 'discount',
            filterType: 'light',
            texts: [],
            ctas: [],
            status: 'active',
          });
        })
      ).rejects.toThrow('common.errors.notAuthenticated');
    });

    it('should throw notAuthenticated when user is null for deleteAutomation', async () => {
      mockUserValue = null;

      const { result } = renderHook(() => useAutomationsConnect());

      await expect(
        act(async () => {
          await result.current.deleteAutomation('auto-1');
        })
      ).rejects.toThrow('common.errors.notAuthenticated');
    });
  });

  // ── listAutomations ────────────────────────────────────────

  describe('listAutomations', () => {
    it('should return mapped LocalAutomation array', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      let automations: Awaited<ReturnType<typeof result.current.listAutomations>>;

      await act(async () => {
        automations = await result.current.listAutomations();
      });

      expect(automations!).toHaveLength(1);
      expect(automations![0].id).toBe('auto-1');
      expect(automations![0].mode).toBe('search');
      expect(automations![0].priority).toBe('discount');
      expect(automations![0].filterType).toBe('light');
      expect(automations![0].status).toBe('active');
    });

    it('should throw when API fails', async () => {
      mockClientMethods.listAutomations.mockRejectedValue(new Error('API down'));

      const { result } = renderHook(() => useAutomationsConnect());

      await expect(
        act(async () => {
          await result.current.listAutomations();
        })
      ).rejects.toThrow('API down');
    });
  });

  // ── getAutomation ──────────────────────────────────────────

  describe('getAutomation', () => {
    it('should return mapped LocalAutomation', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      let automation: Awaited<ReturnType<typeof result.current.getAutomation>>;

      await act(async () => {
        automation = await result.current.getAutomation('auto-1');
      });

      expect(automation!.id).toBe('auto-1');
    });

    it('should throw when automation not in response', async () => {
      mockClientMethods.getAutomation.mockResolvedValue({ automation: undefined });

      const { result } = renderHook(() => useAutomationsConnect());

      await expect(
        act(async () => {
          await result.current.getAutomation('missing');
        })
      ).rejects.toThrow('automations.errors.notFound');
    });
  });

  // ── createAutomation ───────────────────────────────────────

  describe('createAutomation', () => {
    it('should throw when response has no automation', async () => {
      mockClientMethods.createAutomation.mockResolvedValue({ automation: undefined });

      const { result } = renderHook(() => useAutomationsConnect());

      await expect(
        act(async () => {
          await result.current.createAutomation({
            name: 'Test',
            mode: 'search',
            startTime: '08:00',
            endTime: '22:00',
            intervalMinutes: 30,
            sendGroups: ['g1'],
            stores: ['shopee'],
            categories: [],
            priority: 'discount',
            filterType: 'light',
            texts: [],
            ctas: [],
            status: 'active',
          });
        })
      ).rejects.toThrow('automations.errors.createFailed');
    });

    it('should pass monitoringTextRules to proto for monitor mode', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      await act(async () => {
        await result.current.createAutomation({
          name: 'Monitor',
          mode: 'monitor',
          startTime: '08:00',
          endTime: '22:00',
          intervalMinutes: 30,
          sendGroups: ['g1'],
          monitorGroupIds: ['m1'],
          stores: ['shopee'],
          categories: [],
          priority: 'discount',
          filterType: 'light',
          texts: [],
          ctas: [],
          status: 'active',
          monitoringTextRules: {
            substituicoes: [{ de: 'a', para: 'b' }],
            remover_termos: ['spam'],
            prefixo: 'P',
            sufixo: 'S',
            limpar_telefones: true,
            limpar_arrobas: false,
          },
        });
      });

      expect(mockClientMethods.createAutomation).toHaveBeenCalled();
    });
  });

  // ── updateAutomation ───────────────────────────────────────

  describe('updateAutomation', () => {
    it('should handle partial update (only status)', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      await act(async () => {
        await result.current.updateAutomation({
          id: 'auto-1',
          status: 'paused',
        });
      });

      expect(mockClientMethods.updateAutomation).toHaveBeenCalled();
    });

    it('should throw when response has no automation', async () => {
      mockClientMethods.updateAutomation.mockResolvedValue({ automation: undefined });

      const { result } = renderHook(() => useAutomationsConnect());

      await expect(
        act(async () => {
          await result.current.updateAutomation({ id: 'auto-1', name: 'Updated' });
        })
      ).rejects.toThrow('automations.errors.updateFailed');
    });
  });

  // ── diagnoseAutomation ─────────────────────────────────────

  describe('diagnoseAutomation', () => {
    it('should return mapped diagnostic result', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      let diagResult: Awaited<ReturnType<typeof result.current.diagnoseAutomation>>;

      await act(async () => {
        diagResult = await result.current.diagnoseAutomation('auto-1');
      });

      expect(diagResult!.automationId).toBe('auto-1');
      expect(diagResult!.code).toBe('SENT_OK');
      expect(diagResult!.steps).toHaveLength(1);
    });

    it('should throw when result is missing', async () => {
      mockClientMethods.diagnoseAutomation.mockResolvedValue({ result: undefined });

      const { result } = renderHook(() => useAutomationsConnect());

      await expect(
        act(async () => {
          await result.current.diagnoseAutomation('auto-1');
        })
      ).rejects.toThrow('automations.errors.diagnoseFailed');
    });
  });

  // ── runAutomation ──────────────────────────────────────────

  describe('runAutomation', () => {
    it('should return run result', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      let runResult: Awaited<ReturnType<typeof result.current.runAutomation>>;

      await act(async () => {
        runResult = await result.current.runAutomation('auto-1');
      });

      expect(runResult!.success).toBe(true);
      expect(runResult!.message).toBe('Executed');
    });
  });

  // ── resetAutomationHistory ─────────────────────────────────

  describe('resetAutomationHistory', () => {
    it('should return reset result', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      let resetResult: Awaited<ReturnType<typeof result.current.resetAutomationHistory>>;

      await act(async () => {
        resetResult = await result.current.resetAutomationHistory('auto-1');
      });

      expect(resetResult!.success).toBe(true);
      expect(resetResult!.message).toBe('Reset done');
    });

    it('should throw notAuthenticated when user is null', async () => {
      mockUserValue = null;

      const { result } = renderHook(() => useAutomationsConnect());

      await expect(
        act(async () => {
          await result.current.resetAutomationHistory('auto-1');
        })
      ).rejects.toThrow('common.errors.notAuthenticated');
    });
  });

  // ── Mode/Priority/Filter/Status enum conversion ────────────

  describe('enum conversion via roundtrip', () => {
    it('should handle MONITOR mode correctly', async () => {
      mockClientMethods.listAutomations.mockResolvedValue({
        automations: [
          {
            id: 'auto-m',
            name: 'Monitor',
            mode: 2, // MONITOR
            startTime: '08:00',
            endTime: '22:00',
            intervalMinutes: 30,
            sendGroups: ['g1'],
            monitorGroupIds: ['m1'],
            stores: [1],
            categories: [],
            priority: 2, // PRICE
            filterType: 2, // HEAVY
            texts: [],
            ctas: [],
            status: 2, // PAUSED
          },
        ],
      });

      const { result } = renderHook(() => useAutomationsConnect());

      let automations: Awaited<ReturnType<typeof result.current.listAutomations>>;

      await act(async () => {
        automations = await result.current.listAutomations();
      });

      expect(automations![0].mode).toBe('monitor');
      expect(automations![0].priority).toBe('price');
      expect(automations![0].filterType).toBe('heavy');
      expect(automations![0].status).toBe('paused');
    });

    it('should handle LIST mode correctly', async () => {
      mockClientMethods.listAutomations.mockResolvedValue({
        automations: [
          {
            id: 'auto-l',
            name: 'List',
            mode: 3, // LIST
            startTime: '08:00',
            endTime: '22:00',
            intervalMinutes: 60,
            sendGroups: ['g1'],
            monitorGroupIds: [],
            stores: [4], // MERCADO_LIVRE
            categories: [],
            priority: 3, // COMMISSION
            filterType: 1,
            texts: [],
            ctas: [],
            status: 1,
            mlLinkListId: 'ml-1',
            listItemsPerRun: 5,
            listIncludeImage: true,
          },
        ],
      });

      const { result } = renderHook(() => useAutomationsConnect());

      let automations: Awaited<ReturnType<typeof result.current.listAutomations>>;

      await act(async () => {
        automations = await result.current.listAutomations();
      });

      expect(automations![0].mode).toBe('list');
      expect(automations![0].priority).toBe('commission');
      expect(automations![0].stores).toContain('ml');
    });

    it('should handle UNSPECIFIED enums with defaults', async () => {
      mockClientMethods.listAutomations.mockResolvedValue({
        automations: [
          {
            id: 'auto-u',
            name: 'Unspecified',
            mode: 0, // UNSPECIFIED
            startTime: '08:00',
            endTime: '22:00',
            intervalMinutes: 30,
            sendGroups: [],
            monitorGroupIds: [],
            stores: [0], // UNSPECIFIED
            categories: [],
            priority: 0, // UNSPECIFIED
            filterType: 0, // UNSPECIFIED
            texts: [],
            ctas: [],
            status: 0, // UNSPECIFIED
          },
        ],
      });

      const { result } = renderHook(() => useAutomationsConnect());

      let automations: Awaited<ReturnType<typeof result.current.listAutomations>>;

      await act(async () => {
        automations = await result.current.listAutomations();
      });

      expect(automations![0].mode).toBe('search'); // UNSPECIFIED -> search
      expect(automations![0].priority).toBe('discount'); // UNSPECIFIED -> discount
      expect(automations![0].filterType).toBe('light'); // UNSPECIFIED -> light
      expect(automations![0].status).toBe('paused'); // UNSPECIFIED -> paused
    });
  });

  // ── MonitoringTextRules conversion ─────────────────────────

  describe('monitoringTextRules conversion', () => {
    it('should convert undefined rules to undefined', async () => {
      mockClientMethods.listAutomations.mockResolvedValue({
        automations: [
          {
            id: 'auto-no-rules',
            name: 'No Rules',
            mode: 2,
            startTime: '08:00',
            endTime: '22:00',
            intervalMinutes: 30,
            sendGroups: ['g1'],
            monitorGroupIds: ['m1'],
            stores: [1],
            categories: [],
            priority: 1,
            filterType: 1,
            texts: [],
            ctas: [],
            status: 1,
            monitoringTextRules: undefined,
          },
        ],
      });

      const { result } = renderHook(() => useAutomationsConnect());

      let automations: Awaited<ReturnType<typeof result.current.listAutomations>>;

      await act(async () => {
        automations = await result.current.listAutomations();
      });

      expect(automations![0].monitoringTextRules).toBeUndefined();
    });

    it('should convert rules with empty substitutions', async () => {
      mockClientMethods.listAutomations.mockResolvedValue({
        automations: [
          {
            id: 'auto-empty-rules',
            name: 'Empty Rules',
            mode: 2,
            startTime: '08:00',
            endTime: '22:00',
            intervalMinutes: 30,
            sendGroups: ['g1'],
            monitorGroupIds: ['m1'],
            stores: [1],
            categories: [],
            priority: 1,
            filterType: 1,
            texts: [],
            ctas: [],
            status: 1,
            monitoringTextRules: {
              substitutions: [],
              removeTerms: [],
              prefix: '',
              suffix: '',
              cleanPhones: false,
              cleanMentions: false,
            },
          },
        ],
      });

      const { result } = renderHook(() => useAutomationsConnect());

      let automations: Awaited<ReturnType<typeof result.current.listAutomations>>;

      await act(async () => {
        automations = await result.current.listAutomations();
      });

      const rules = automations![0].monitoringTextRules;
      expect(rules).toBeDefined();
      expect(rules!.substituicoes).toEqual([]);
      expect(rules!.remover_termos).toEqual([]);
      expect(rules!.prefixo).toBe('');
      expect(rules!.sufixo).toBe('');
      expect(rules!.limpar_telefones).toBe(false);
      expect(rules!.limpar_arrobas).toBe(false);
    });
  });
});
