import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useAutomationsConnect } from '../use-automations-connect';

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

vi.mock('@/contexts', () => ({
  useAuth: () => ({
    user: { id: 'user-123', email: 'test@example.com' },
    session: null,
    loading: false,
  }),
}));

// ============================================================
// Mock Transport and RPC Client
// ============================================================

const mockClientMethods: Record<string, ReturnType<typeof vi.fn>> = {
  listAutomations: vi.fn(),
  getAutomation: vi.fn(),
  createAutomation: vi.fn(),
  updateAutomation: vi.fn(),
  deleteAutomation: vi.fn(),
  diagnoseAutomation: vi.fn(),
  runAutomation: vi.fn(),
  resetAutomationHistory: vi.fn(),
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

vi.mock('@/gen/automations/v1/automations_pb', () => ({
  AutomationMode: { UNSPECIFIED: 0, SEARCH: 1, MONITOR: 2, LIST: 3 },
  AutomationStatus: { UNSPECIFIED: 0, ACTIVE: 1, PAUSED: 2 },
  AutomationsService: { typeName: 'automations.v1.AutomationsService' },
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
  PriorityType: { UNSPECIFIED: 0, DISCOUNT: 1, PRICE: 2, COMMISSION: 3 },
  FilterType: { UNSPECIFIED: 0, LIGHT: 1, HEAVY: 2 },
  CreateAutomationRequestSchema: {},
  UpdateAutomationRequestSchema: {},
  DeleteAutomationRequestSchema: {},
  GetAutomationRequestSchema: {},
  ListAutomationsRequestSchema: {},
  DiagnoseAutomationRequestSchema: {},
  RunAutomationRequestSchema: {},
  ResetAutomationHistoryRequestSchema: {},
  TextSubstitutionSchema: {},
}));

vi.mock('@bufbuild/protobuf', () => ({
  create: (_schema: unknown, data: unknown) => data,
}));

// ============================================================
// Enum references for test assertions (AFTER vi.mock)
// ============================================================

const AutomationModeEnum = {
  UNSPECIFIED: 0,
  SEARCH: 1,
  MONITOR: 2,
  LIST: 3,
} as const;

const StoreEnum = {
  UNSPECIFIED: 0,
  SHOPEE: 1,
  AMAZON: 2,
  MAGALU: 3,
  MERCADO_LIVRE: 4,
  SHEIN: 5,
  ALIEXPRESS: 6,
  AWIN: 7,
} as const;

const PriorityTypeEnum = {
  UNSPECIFIED: 0,
  DISCOUNT: 1,
  PRICE: 2,
  COMMISSION: 3,
} as const;

const FilterTypeEnum = {
  UNSPECIFIED: 0,
  LIGHT: 1,
  HEAVY: 2,
} as const;

const AutomationStatusEnum = {
  UNSPECIFIED: 0,
  ACTIVE: 1,
  PAUSED: 2,
} as const;

// ============================================================
// Mock Error Handling
// ============================================================

vi.mock('@/lib/connect-transport', () => ({
  getErrorInfo: (error: unknown) => ({
    code: 'UNKNOWN',
    message: error instanceof Error ? error.message : 'Unknown error',
    userMessage: error instanceof Error ? error.message : 'Unknown error',
    action: 'toast',
    retryable: false,
  }),
}));

vi.mock('@/lib/connect', () => ({
  handleConnectError: vi.fn(() => ({ success: false, message: 'error' })),
  isLockConflictError: vi.fn(() => false),
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
// Test Data Factories
// ============================================================

function createMockProtoAutomation(overrides: Record<string, unknown> = {}) {
  return {
    id: 'auto-1',
    name: 'Test Automation',
    mode: AutomationModeEnum.SEARCH,
    dispatchTextId: '',
    monitoringTextRules: undefined,
    startTime: '08:00',
    endTime: '22:00',
    intervalMinutes: 30,
    sendGroups: ['group1@g.us', 'group2@g.us'],
    monitorGroupIds: [],
    stores: [StoreEnum.SHOPEE],
    categories: ['Eletrônicos'],
    priority: PriorityTypeEnum.DISCOUNT,
    filterType: FilterTypeEnum.LIGHT,
    minDiscount: 10,
    minPrice: 50,
    maxPrice: 500,
    texts: ['Check this out!'],
    ctas: ['Buy now'],
    lastRunAt: '2026-01-30T10:00:00Z',
    nextRunAt: '2026-01-30T10:30:00Z',
    status: AutomationStatusEnum.ACTIVE,
    lastError: '',
    delaySeconds: 5,
    couponId: '',
    mlLinkListId: '',
    listItemsPerRun: 1,
    listIncludeImage: true,
    ...overrides,
  };
}

function createMockMonitorAutomation() {
  return createMockProtoAutomation({
    id: 'auto-monitor',
    name: 'Monitor Automation',
    mode: AutomationModeEnum.MONITOR,
    monitorGroupIds: ['source@g.us'],
    monitoringTextRules: {
      substitutions: [{ from: 'old', to: 'new' }],
      removeTerms: ['spam'],
      prefix: 'OFERTA:',
      suffix: 'Corre!',
      cleanPhones: true,
      cleanMentions: true,
    },
  });
}

function createMockListAutomation() {
  return createMockProtoAutomation({
    id: 'auto-list',
    name: 'List Automation',
    mode: AutomationModeEnum.LIST,
    stores: [StoreEnum.MERCADO_LIVRE],
    mlLinkListId: 'list-123',
    listItemsPerRun: 3,
    listIncludeImage: false,
  });
}

// ============================================================
// Helper: setup default mockUseMutation
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

// ============================================================
// Tests
// ============================================================

describe('useAutomationsConnect', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setupDefaultMutationMock();

    // Default client responses
    mockClientMethods.listAutomations.mockResolvedValue({
      automations: [createMockProtoAutomation()],
    });
    mockClientMethods.getAutomation.mockResolvedValue({
      automation: createMockProtoAutomation(),
    });
    mockClientMethods.createAutomation.mockResolvedValue({
      automation: createMockProtoAutomation({ id: 'new-auto' }),
    });
    mockClientMethods.updateAutomation.mockResolvedValue({
      automation: createMockProtoAutomation({ name: 'Updated' }),
    });
    mockClientMethods.deleteAutomation.mockResolvedValue({});
    mockClientMethods.diagnoseAutomation.mockResolvedValue({
      result: {
        automationId: 'auto-1',
        code: 'ok',
        message: 'All checks passed',
        steps: [
          { step: 'CONFIG', ok: true, detail: 'Valid', message: '' },
          { step: 'WHATSAPP', ok: true, detail: 'Connected', message: '' },
        ],
        offersFound: 0,
        sentTo: [],
        error: '',
      },
    });
    mockClientMethods.runAutomation.mockResolvedValue({
      success: true,
      message: '3 messages sent',
    });
    mockClientMethods.resetAutomationHistory.mockResolvedValue({
      success: true,
      message: 'History cleared',
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── List Automations ──────────────────────────────────────

  describe('listAutomations', () => {
    it('should list automations and convert proto to local types', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      let automations: Awaited<ReturnType<typeof result.current.listAutomations>>;
      await act(async () => {
        automations = await result.current.listAutomations();
      });

      expect(mockClientMethods.listAutomations).toHaveBeenCalled();
      expect(automations!).toHaveLength(1);
      expect(automations![0]).toMatchObject({
        id: 'auto-1',
        name: 'Test Automation',
        mode: 'search',
        stores: ['shopee'],
        priority: 'discount',
        filterType: 'light',
        status: 'active',
        intervalMinutes: 30,
        sendGroups: ['group1@g.us', 'group2@g.us'],
        categories: ['Eletrônicos'],
        listItemsPerRun: 1,
        listIncludeImage: true,
      });
    });

    it('should handle all three automation modes', async () => {
      mockClientMethods.listAutomations.mockResolvedValue({
        automations: [
          createMockProtoAutomation(),
          createMockMonitorAutomation(),
          createMockListAutomation(),
        ],
      });

      const { result } = renderHook(() => useAutomationsConnect());

      let automations: Awaited<ReturnType<typeof result.current.listAutomations>>;
      await act(async () => {
        automations = await result.current.listAutomations();
      });

      expect(automations!).toHaveLength(3);
      expect(automations![0].mode).toBe('search');
      expect(automations![1].mode).toBe('monitor');
      expect(automations![2].mode).toBe('list');
    });

    it('should convert monitoring text rules from proto', async () => {
      mockClientMethods.listAutomations.mockResolvedValue({
        automations: [createMockMonitorAutomation()],
      });

      const { result } = renderHook(() => useAutomationsConnect());

      let automations: Awaited<ReturnType<typeof result.current.listAutomations>>;
      await act(async () => {
        automations = await result.current.listAutomations();
      });

      const rules = automations![0].monitoringTextRules;
      expect(rules).toBeDefined();
      expect(rules!.substituicoes).toEqual([{ de: 'old', para: 'new' }]);
      expect(rules!.remover_termos).toEqual(['spam']);
      expect(rules!.prefixo).toBe('OFERTA:');
      expect(rules!.sufixo).toBe('Corre!');
      expect(rules!.limpar_telefones).toBe(true);
      expect(rules!.limpar_arrobas).toBe(true);
    });

    it('should convert list mode fields from proto', async () => {
      mockClientMethods.listAutomations.mockResolvedValue({
        automations: [createMockListAutomation()],
      });

      const { result } = renderHook(() => useAutomationsConnect());

      let automations: Awaited<ReturnType<typeof result.current.listAutomations>>;
      await act(async () => {
        automations = await result.current.listAutomations();
      });

      expect(automations![0].stores).toEqual(['ml']);
      expect(automations![0].mlLinkListId).toBe('list-123');
      expect(automations![0].listItemsPerRun).toBe(3);
      expect(automations![0].listIncludeImage).toBe(false);
    });
  });

  // ── Get Automation ────────────────────────────────────────

  describe('getAutomation', () => {
    it('should get single automation by ID', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      let automation: Awaited<ReturnType<typeof result.current.getAutomation>>;
      await act(async () => {
        automation = await result.current.getAutomation('auto-1');
      });

      expect(mockClientMethods.getAutomation).toHaveBeenCalled();
      expect(automation!.id).toBe('auto-1');
    });

    it('should throw when automation not found', async () => {
      mockClientMethods.getAutomation.mockResolvedValue({ automation: undefined });

      const { result } = renderHook(() => useAutomationsConnect());

      await expect(
        act(async () => {
          await result.current.getAutomation('nonexistent');
        })
      ).rejects.toThrow();
    });
  });

  // ── Create Automation ─────────────────────────────────────

  describe('createAutomation', () => {
    it('should create automation with correct proto conversion', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      await act(async () => {
        await result.current.createAutomation({
          name: 'New Automation',
          mode: 'search',
          startTime: '09:00',
          endTime: '21:00',
          intervalMinutes: 15,
          sendGroups: ['group@g.us'],
          stores: ['shopee'],
          categories: ['Moda'],
          priority: 'discount',
          filterType: 'light',
          texts: ['Check it!'],
          ctas: [],
          status: 'active',
        });
      });

      expect(mockClientMethods.createAutomation).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'New Automation',
          mode: AutomationModeEnum.SEARCH,
          stores: [StoreEnum.SHOPEE],
          priority: PriorityTypeEnum.DISCOUNT,
          filterType: FilterTypeEnum.LIGHT,
          status: AutomationStatusEnum.ACTIVE,
        })
      );
    });

    it('should convert monitoring text rules to proto format', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      await act(async () => {
        await result.current.createAutomation({
          name: 'Monitor Auto',
          mode: 'monitor',
          startTime: '08:00',
          endTime: '23:00',
          intervalMinutes: 30,
          sendGroups: ['dest@g.us'],
          monitorGroupIds: ['source@g.us'],
          stores: ['shopee'],
          categories: [],
          priority: 'discount',
          filterType: 'light',
          texts: [],
          ctas: [],
          status: 'active',
          monitoringTextRules: {
            substituicoes: [{ de: 'old', para: 'new' }],
            remover_termos: ['remove_me'],
            prefixo: 'PREFIX',
            sufixo: 'SUFFIX',
            limpar_telefones: true,
            limpar_arrobas: false,
          },
        });
      });

      expect(mockClientMethods.createAutomation).toHaveBeenCalledWith(
        expect.objectContaining({
          monitoringTextRules: {
            substitutions: [{ from: 'old', to: 'new' }],
            removeTerms: ['remove_me'],
            prefix: 'PREFIX',
            suffix: 'SUFFIX',
            cleanPhones: true,
            cleanMentions: false,
          },
        })
      );
    });

    it('should pass list mode fields correctly', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      await act(async () => {
        await result.current.createAutomation({
          name: 'List Auto',
          mode: 'list',
          startTime: '08:00',
          endTime: '23:00',
          intervalMinutes: 60,
          sendGroups: ['dest@g.us'],
          stores: ['ml'],
          categories: [],
          priority: 'discount',
          filterType: 'light',
          texts: [],
          ctas: [],
          status: 'active',
          mlLinkListId: 'list-456',
          listItemsPerRun: 5,
          listIncludeImage: true,
        });
      });

      expect(mockClientMethods.createAutomation).toHaveBeenCalledWith(
        expect.objectContaining({
          mode: AutomationModeEnum.LIST,
          stores: [StoreEnum.MERCADO_LIVRE],
          mlLinkListId: 'list-456',
          listItemsPerRun: 5,
          listIncludeImage: true,
        })
      );
    });
  });

  // ── Delete Automation ─────────────────────────────────────

  describe('deleteAutomation', () => {
    it('should delete automation by ID', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      await act(async () => {
        await result.current.deleteAutomation('auto-1');
      });

      expect(mockClientMethods.deleteAutomation).toHaveBeenCalled();
    });
  });

  // ── Diagnose Automation ───────────────────────────────────

  describe('diagnoseAutomation', () => {
    it('should return diagnostic steps', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      let diagnosis: Awaited<ReturnType<typeof result.current.diagnoseAutomation>>;
      await act(async () => {
        diagnosis = await result.current.diagnoseAutomation('auto-1');
      });

      expect(mockClientMethods.diagnoseAutomation).toHaveBeenCalled();
      expect(diagnosis!.automationId).toBe('auto-1');
      expect(diagnosis!.steps).toHaveLength(2);
      expect(diagnosis!.steps[0].step).toBe('CONFIG');
      expect(diagnosis!.steps[0].ok).toBe(true);
    });
  });

  // ── Run Automation ────────────────────────────────────────

  describe('runAutomation', () => {
    it('should run automation and return result', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      let runResult: Awaited<ReturnType<typeof result.current.runAutomation>>;
      await act(async () => {
        runResult = await result.current.runAutomation('auto-1');
      });

      expect(mockClientMethods.runAutomation).toHaveBeenCalled();
      expect(runResult!.success).toBe(true);
      expect(runResult!.message).toBe('3 messages sent');
    });
  });

  // ── Reset History ─────────────────────────────────────────

  describe('resetAutomationHistory', () => {
    it('should reset automation history', async () => {
      const { result } = renderHook(() => useAutomationsConnect());

      let resetResult: Awaited<ReturnType<typeof result.current.resetAutomationHistory>>;
      await act(async () => {
        resetResult = await result.current.resetAutomationHistory('auto-1');
      });

      expect(mockClientMethods.resetAutomationHistory).toHaveBeenCalled();
      expect(resetResult!.success).toBe(true);
    });
  });

  // ── Store Conversion ──────────────────────────────────────

  describe('store conversion parity', () => {
    it('should map all store types correctly roundtrip', async () => {
      const storeMapping: Array<[number, string]> = [
        [StoreEnum.SHOPEE, 'shopee'],
        [StoreEnum.AMAZON, 'amazon'],
        [StoreEnum.MAGALU, 'magalu'],
        [StoreEnum.MERCADO_LIVRE, 'ml'],
        [StoreEnum.SHEIN, 'shein'],
        [StoreEnum.ALIEXPRESS, 'aliexpress'],
        [StoreEnum.AWIN, 'awin'],
      ];

      for (const [protoStore, localStore] of storeMapping) {
        mockClientMethods.listAutomations.mockResolvedValue({
          automations: [createMockProtoAutomation({ stores: [protoStore] })],
        });

        const { result } = renderHook(() => useAutomationsConnect());
        let automations: Awaited<ReturnType<typeof result.current.listAutomations>>;
        await act(async () => {
          automations = await result.current.listAutomations();
        });

        expect(automations![0].stores[0]).toBe(localStore);
      }
    });
  });

  // ── Status Conversion ─────────────────────────────────────

  describe('status conversion parity', () => {
    it('should map active status', async () => {
      mockClientMethods.listAutomations.mockResolvedValue({
        automations: [createMockProtoAutomation({ status: AutomationStatusEnum.ACTIVE })],
      });

      const { result } = renderHook(() => useAutomationsConnect());
      let automations: Awaited<ReturnType<typeof result.current.listAutomations>>;
      await act(async () => {
        automations = await result.current.listAutomations();
      });

      expect(automations![0].status).toBe('active');
    });

    it('should map paused status', async () => {
      mockClientMethods.listAutomations.mockResolvedValue({
        automations: [createMockProtoAutomation({ status: AutomationStatusEnum.PAUSED })],
      });

      const { result } = renderHook(() => useAutomationsConnect());
      let automations: Awaited<ReturnType<typeof result.current.listAutomations>>;
      await act(async () => {
        automations = await result.current.listAutomations();
      });

      expect(automations![0].status).toBe('paused');
    });
  });
});
