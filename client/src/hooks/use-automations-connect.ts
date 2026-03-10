import { useAuth } from '@/contexts';
import {
  type Automation,
  AutomationMode,
  AutomationStatus,
  AutomationsService,
  CreateAutomationRequestSchema,
  type CreateAutomationResponse,
  DeleteAutomationRequestSchema,
  DiagnoseAutomationRequestSchema,
  type DiagnoseAutomationResponse,
  type DiagnosticResult,
  type DiagnosticStep,
  FilterType,
  GetAutomationRequestSchema,
  type GetAutomationResponse,
  ListAutomationsRequestSchema,
  type ListAutomationsResponse,
  type MonitoringTextRules,
  PriorityType,
  ResetAutomationHistoryRequestSchema,
  type ResetAutomationHistoryResponse,
  RunAutomationRequestSchema,
  type RunAutomationResponse,
  Store,
  type TextSubstitution,
  TextSubstitutionSchema,
  UpdateAutomationRequestSchema,
  type UpdateAutomationResponse,
} from '@/gen/automations/v1/automations_pb';
import { handleConnectError, isLockConflictError } from '@/lib/connect';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useMutation } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

// ============================================================
// Local Types
// ============================================================

export type LocalStoreKey = 'shopee' | 'amazon' | 'magalu' | 'ml' | 'shein' | 'aliexpress' | 'awin';
export type LocalAutomationMode = 'search' | 'monitor' | 'list' | 'offer_generate';
export type LocalPriorityType = 'discount' | 'price' | 'commission';
export type LocalFilterType = 'light' | 'heavy';
export type LocalAutomationStatus = 'active' | 'paused';

export interface LocalTextSubstitution {
  de: string;
  para: string;
}

export interface LocalMonitoringTextRules {
  substituicoes: LocalTextSubstitution[];
  remover_termos: string[];
  prefixo: string;
  sufixo: string;
  limpar_telefones: boolean;
  limpar_arrobas: boolean;
}

export interface LocalAutomation {
  id: string;
  name: string;
  mode: LocalAutomationMode;
  dispatchTextId?: string;
  monitoringTextRules?: LocalMonitoringTextRules;
  startTime: string;
  endTime: string;
  intervalMinutes: number;
  sendGroups: string[];
  monitorGroupIds?: string[];
  stores: LocalStoreKey[];
  categories: string[];
  priority: LocalPriorityType;
  filterType: LocalFilterType;
  minDiscount?: number;
  minPrice?: number;
  maxPrice?: number;
  texts: string[];
  ctas: string[];
  ctaIds: string[];
  lastRunAt?: string;
  nextRunAt?: string;
  status: LocalAutomationStatus;
  lastError?: string;
  delaySeconds?: number;
  couponId?: string;
  mlLinkListId?: string;
  listItemsPerRun?: number;
  listIncludeImage?: boolean;
}

export interface LocalDiagnosticStep {
  step: string;
  ok: boolean;
  detail: string;
  message?: string;
}

export interface LocalDiagnosticResult {
  automationId: string;
  code?: string;
  message?: string;
  steps: LocalDiagnosticStep[];
  offersFound?: number;
  sentTo: string[];
  error?: string;
}

export interface CreateAutomationParams {
  name: string;
  mode: LocalAutomationMode;
  dispatchTextId?: string;
  monitoringTextRules?: LocalMonitoringTextRules;
  startTime: string;
  endTime: string;
  intervalMinutes: number;
  sendGroups: string[];
  monitorGroupIds?: string[];
  stores: LocalStoreKey[];
  categories: string[];
  priority: LocalPriorityType;
  filterType: LocalFilterType;
  minDiscount?: number;
  minPrice?: number;
  maxPrice?: number;
  texts: string[];
  ctas: string[];
  ctaIds: string[];
  status: LocalAutomationStatus;
  delaySeconds?: number;
  couponId?: string;
  mlLinkListId?: string;
  listItemsPerRun?: number;
  listIncludeImage?: boolean;
}

export interface UpdateAutomationParams {
  id: string;
  name?: string;
  mode?: LocalAutomationMode;
  dispatchTextId?: string;
  monitoringTextRules?: LocalMonitoringTextRules;
  startTime?: string;
  endTime?: string;
  intervalMinutes?: number;
  sendGroups?: string[];
  monitorGroupIds?: string[];
  stores?: LocalStoreKey[];
  categories?: string[];
  priority?: LocalPriorityType;
  filterType?: LocalFilterType;
  minDiscount?: number;
  minPrice?: number;
  maxPrice?: number;
  texts?: string[];
  ctas?: string[];
  ctaIds?: string[];
  status?: LocalAutomationStatus;
  delaySeconds?: number;
  couponId?: string;
  mlLinkListId?: string;
  listItemsPerRun?: number;
  listIncludeImage?: boolean;
}

export interface LocalRunAutomationResult {
  success: boolean;
  message?: string;
  error?: string;
}

export interface LocalResetHistoryResult {
  success: boolean;
  message?: string;
}

// ============================================================
// Hook Return Type
// ============================================================

interface UseAutomationsConnectReturn {
  listAutomations: () => Promise<LocalAutomation[]>;
  getAutomation: (id: string) => Promise<LocalAutomation>;
  createAutomation: (params: CreateAutomationParams) => Promise<LocalAutomation>;
  updateAutomation: (params: UpdateAutomationParams) => Promise<LocalAutomation>;
  deleteAutomation: (id: string) => Promise<void>;
  diagnoseAutomation: (automationId: string) => Promise<LocalDiagnosticResult>;
  runAutomation: (automationId: string) => Promise<LocalRunAutomationResult>;
  resetAutomationHistory: (automationId: string) => Promise<LocalResetHistoryResult>;

  isListingAutomations: boolean;
  isGettingAutomation: boolean;
  isCreatingAutomation: boolean;
  isUpdatingAutomation: boolean;
  isDeletingAutomation: boolean;
  isDiagnosingAutomation: boolean;
  isRunningAutomation: boolean;
  isResettingHistory: boolean;
}

// ============================================================
// Converters
// ============================================================

function toProtoStore(store: LocalStoreKey): Store {
  const map: Record<LocalStoreKey, Store> = {
    shopee: Store.SHOPEE,
    amazon: Store.AMAZON,
    magalu: Store.MAGALU,
    ml: Store.MERCADO_LIVRE,
    shein: Store.SHEIN,
    aliexpress: Store.ALIEXPRESS,
    awin: Store.AWIN,
  };
  return map[store] ?? Store.UNSPECIFIED;
}

function fromProtoStore(store: Store): LocalStoreKey {
  const map: Record<Store, LocalStoreKey> = {
    [Store.UNSPECIFIED]: 'shopee',
    [Store.SHOPEE]: 'shopee',
    [Store.AMAZON]: 'amazon',
    [Store.MAGALU]: 'magalu',
    [Store.MERCADO_LIVRE]: 'ml',
    [Store.SHEIN]: 'shein',
    [Store.ALIEXPRESS]: 'aliexpress',
    [Store.AWIN]: 'awin',
  };
  return map[store] ?? 'shopee';
}

function toProtoMode(mode: LocalAutomationMode): AutomationMode {
  const map: Record<LocalAutomationMode, AutomationMode> = {
    search: AutomationMode.SEARCH,
    monitor: AutomationMode.MONITOR,
    list: AutomationMode.LIST,
    offer_generate: AutomationMode.OFFER_GENERATE,
  };
  return map[mode] ?? AutomationMode.UNSPECIFIED;
}

function fromProtoMode(mode: AutomationMode): LocalAutomationMode {
  const map: Record<AutomationMode, LocalAutomationMode> = {
    [AutomationMode.UNSPECIFIED]: 'search',
    [AutomationMode.SEARCH]: 'search',
    [AutomationMode.MONITOR]: 'monitor',
    [AutomationMode.LIST]: 'list',
    [AutomationMode.OFFER_GENERATE]: 'offer_generate',
  };
  return map[mode] ?? 'search';
}

function toProtoPriority(priority: LocalPriorityType): PriorityType {
  const map: Record<LocalPriorityType, PriorityType> = {
    discount: PriorityType.DISCOUNT,
    price: PriorityType.PRICE,
    commission: PriorityType.COMMISSION,
  };
  return map[priority] ?? PriorityType.UNSPECIFIED;
}

function fromProtoPriority(priority: PriorityType): LocalPriorityType {
  const map: Record<PriorityType, LocalPriorityType> = {
    [PriorityType.UNSPECIFIED]: 'discount',
    [PriorityType.DISCOUNT]: 'discount',
    [PriorityType.PRICE]: 'price',
    [PriorityType.COMMISSION]: 'commission',
  };
  return map[priority] ?? 'discount';
}

function toProtoFilterType(filter: LocalFilterType): FilterType {
  const map: Record<LocalFilterType, FilterType> = {
    light: FilterType.LIGHT,
    heavy: FilterType.HEAVY,
  };
  return map[filter] ?? FilterType.UNSPECIFIED;
}

function fromProtoFilterType(filter: FilterType): LocalFilterType {
  const map: Record<FilterType, LocalFilterType> = {
    [FilterType.UNSPECIFIED]: 'light',
    [FilterType.LIGHT]: 'light',
    [FilterType.HEAVY]: 'heavy',
  };
  return map[filter] ?? 'light';
}

function toProtoStatus(status: LocalAutomationStatus): AutomationStatus {
  const map: Record<LocalAutomationStatus, AutomationStatus> = {
    active: AutomationStatus.ACTIVE,
    paused: AutomationStatus.PAUSED,
  };
  return map[status] ?? AutomationStatus.UNSPECIFIED;
}

function fromProtoStatus(status: AutomationStatus): LocalAutomationStatus {
  const map: Record<AutomationStatus, LocalAutomationStatus> = {
    [AutomationStatus.UNSPECIFIED]: 'paused',
    [AutomationStatus.ACTIVE]: 'active',
    [AutomationStatus.PAUSED]: 'paused',
  };
  return map[status] ?? 'paused';
}

function toProtoTextSubstitutions(subs: LocalTextSubstitution[]): TextSubstitution[] {
  return subs.map((s) =>
    create(TextSubstitutionSchema, {
      from: s.de,
      to: s.para,
    })
  );
}

function fromProtoTextSubstitutions(subs: TextSubstitution[]): LocalTextSubstitution[] {
  return subs.map((s) => ({
    de: s.from,
    para: s.to,
  }));
}

function fromProtoMonitoringTextRules(
  rules: MonitoringTextRules | undefined
): LocalMonitoringTextRules | undefined {
  if (!rules) return undefined;
  return {
    substituicoes: fromProtoTextSubstitutions(rules.substitutions),
    remover_termos: [...rules.removeTerms],
    prefixo: rules.prefix,
    sufixo: rules.suffix,
    limpar_telefones: rules.cleanPhones,
    limpar_arrobas: rules.cleanMentions,
  };
}

function fromProtoAutomation(automation: Automation): LocalAutomation {
  return {
    id: automation.id,
    name: automation.name,
    mode: fromProtoMode(automation.mode),
    dispatchTextId: automation.dispatchTextId,
    monitoringTextRules: fromProtoMonitoringTextRules(automation.monitoringTextRules),
    startTime: automation.startTime,
    endTime: automation.endTime,
    intervalMinutes: automation.intervalMinutes,
    sendGroups: [...automation.sendGroups],
    monitorGroupIds:
      automation.monitorGroupIds.length > 0 ? [...automation.monitorGroupIds] : undefined,
    stores: automation.stores.map(fromProtoStore),
    categories: [...automation.categories],
    priority: fromProtoPriority(automation.priority),
    filterType: fromProtoFilterType(automation.filterType),
    minDiscount: automation.minDiscount,
    minPrice: automation.minPrice,
    maxPrice: automation.maxPrice,
    texts: [...automation.texts],
    ctas: [...automation.ctas],
    ctaIds: [...(automation.ctaIds ?? [])],
    lastRunAt: automation.lastRunAt,
    nextRunAt: automation.nextRunAt,
    status: fromProtoStatus(automation.status),
    lastError: automation.lastError,
    delaySeconds: automation.delaySeconds,
    couponId: automation.couponId,
    mlLinkListId: automation.mlLinkListId,
    listItemsPerRun: automation.listItemsPerRun,
    listIncludeImage: automation.listIncludeImage,
  };
}

function fromProtoDiagnosticStep(step: DiagnosticStep): LocalDiagnosticStep {
  return {
    step: step.step,
    ok: step.ok,
    detail: step.detail,
    message: step.message,
  };
}

function fromProtoDiagnosticResult(result: DiagnosticResult): LocalDiagnosticResult {
  return {
    automationId: result.automationId,
    code: result.code,
    message: result.message,
    steps: result.steps.map(fromProtoDiagnosticStep),
    offersFound: result.offersFound,
    sentTo: [...result.sentTo],
    error: result.error,
  };
}

// ============================================================
// Hook Implementation
// ============================================================

/**
 * Hook for automation operations via ConnectRPC.
 * Provides CRUD operations, diagnostics, and manual execution.
 */
export function useAutomationsConnect(): UseAutomationsConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();

  // ============================================================
  // List Automations
  // ============================================================

  const listAutomationsMutation = useMutation({
    mutationFn: async (): Promise<LocalAutomation[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AutomationsService, transport);
      const response: ListAutomationsResponse = await client.listAutomations(
        create(ListAutomationsRequestSchema, {})
      );

      return response.automations.map(fromProtoAutomation);
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Get Automation
  // ============================================================

  const getAutomationMutation = useMutation({
    mutationFn: async (id: string): Promise<LocalAutomation> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AutomationsService, transport);
      const response: GetAutomationResponse = await client.getAutomation(
        create(GetAutomationRequestSchema, { id })
      );

      if (!response.automation) {
        throw new Error(t('automations.errors.notFound'));
      }

      return fromProtoAutomation(response.automation);
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Create Automation
  // ============================================================

  const createAutomationMutation = useMutation({
    mutationFn: async (params: CreateAutomationParams): Promise<LocalAutomation> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AutomationsService, transport);
      const response: CreateAutomationResponse = await client.createAutomation(
        create(CreateAutomationRequestSchema, {
          name: params.name,
          mode: toProtoMode(params.mode),
          dispatchTextId: params.dispatchTextId,
          monitoringTextRules: params.monitoringTextRules
            ? {
                substitutions: toProtoTextSubstitutions(params.monitoringTextRules.substituicoes),
                removeTerms: params.monitoringTextRules.remover_termos,
                prefix: params.monitoringTextRules.prefixo,
                suffix: params.monitoringTextRules.sufixo,
                cleanPhones: params.monitoringTextRules.limpar_telefones,
                cleanMentions: params.monitoringTextRules.limpar_arrobas,
              }
            : undefined,
          startTime: params.startTime,
          endTime: params.endTime,
          intervalMinutes: params.intervalMinutes,
          sendGroups: params.sendGroups,
          monitorGroupIds: params.monitorGroupIds ?? [],
          stores: params.stores.map(toProtoStore),
          categories: params.categories,
          priority: toProtoPriority(params.priority),
          filterType: toProtoFilterType(params.filterType),
          minDiscount: params.minDiscount,
          minPrice: params.minPrice,
          maxPrice: params.maxPrice,
          texts: params.texts,
          ctas: params.ctas,
          ctaIds: params.ctaIds,
          status: toProtoStatus(params.status),
          delaySeconds: params.delaySeconds,
          couponId: params.couponId,
          mlLinkListId: params.mlLinkListId,
          listItemsPerRun: params.listItemsPerRun,
          listIncludeImage: params.listIncludeImage,
        })
      );

      if (!response.automation) {
        throw new Error(t('automations.errors.createFailed'));
      }

      return fromProtoAutomation(response.automation);
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Update Automation
  // ============================================================

  const updateAutomationMutation = useMutation({
    mutationFn: async (params: UpdateAutomationParams): Promise<LocalAutomation> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AutomationsService, transport);
      const response: UpdateAutomationResponse = await client.updateAutomation(
        create(UpdateAutomationRequestSchema, {
          id: params.id,
          name: params.name,
          mode: params.mode ? toProtoMode(params.mode) : undefined,
          dispatchTextId: params.dispatchTextId,
          monitoringTextRules: params.monitoringTextRules
            ? {
                substitutions: toProtoTextSubstitutions(params.monitoringTextRules.substituicoes),
                removeTerms: params.monitoringTextRules.remover_termos,
                prefix: params.monitoringTextRules.prefixo,
                suffix: params.monitoringTextRules.sufixo,
                cleanPhones: params.monitoringTextRules.limpar_telefones,
                cleanMentions: params.monitoringTextRules.limpar_arrobas,
              }
            : undefined,
          startTime: params.startTime,
          endTime: params.endTime,
          intervalMinutes: params.intervalMinutes,
          sendGroups: params.sendGroups ?? [],
          monitorGroupIds: params.monitorGroupIds ?? [],
          stores: params.stores?.map(toProtoStore) ?? [],
          categories: params.categories ?? [],
          priority: params.priority ? toProtoPriority(params.priority) : undefined,
          filterType: params.filterType ? toProtoFilterType(params.filterType) : undefined,
          minDiscount: params.minDiscount,
          minPrice: params.minPrice,
          maxPrice: params.maxPrice,
          texts: params.texts ?? [],
          ctas: params.ctas ?? [],
          ctaIds: params.ctaIds ?? [],
          status: params.status ? toProtoStatus(params.status) : undefined,
          delaySeconds: params.delaySeconds,
          couponId: params.couponId,
          mlLinkListId: params.mlLinkListId,
          listItemsPerRun: params.listItemsPerRun,
          listIncludeImage: params.listIncludeImage,
        })
      );

      if (!response.automation) {
        throw new Error(t('automations.errors.updateFailed'));
      }

      return fromProtoAutomation(response.automation);
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Delete Automation
  // ============================================================

  const deleteAutomationMutation = useMutation({
    mutationFn: async (id: string): Promise<void> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AutomationsService, transport);
      await client.deleteAutomation(create(DeleteAutomationRequestSchema, { id }));
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Diagnose Automation (120s timeout)
  // ============================================================

  const diagnoseAutomationMutation = useMutation({
    mutationFn: async (automationId: string): Promise<LocalDiagnosticResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AutomationsService, transport);

      // Create AbortController with 120s timeout for complex automations
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 120000);

      try {
        const response: DiagnoseAutomationResponse = await client.diagnoseAutomation(
          create(DiagnoseAutomationRequestSchema, { automationId }),
          { signal: controller.signal }
        );

        if (!response.result) {
          throw new Error(t('automations.errors.diagnoseFailed'));
        }

        return fromProtoDiagnosticResult(response.result);
      } finally {
        clearTimeout(timeoutId);
      }
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Run Automation
  // ============================================================

  const runAutomationMutation = useMutation({
    mutationFn: async (automationId: string): Promise<LocalRunAutomationResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AutomationsService, transport);
      const response: RunAutomationResponse = await client.runAutomation(
        create(RunAutomationRequestSchema, { automationId })
      );

      return {
        success: response.success,
        message: response.message,
        error: response.error,
      };
    },
    onError: (error: unknown) => {
      // Special handling for lock conflicts (409/AlreadyExists)
      if (isLockConflictError(error)) {
        const errorResult = handleConnectError(error, t, { operation: 'run' });
        toast({
          variant: 'destructive',
          title: t('common.error'),
          description: errorResult.userMessage,
          duration: 5000,
        });
        return;
      }

      // Default error handling for other errors
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Reset Automation History
  // ============================================================

  const resetHistoryMutation = useMutation({
    mutationFn: async (automationId: string): Promise<LocalResetHistoryResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AutomationsService, transport);
      const response: ResetAutomationHistoryResponse = await client.resetAutomationHistory(
        create(ResetAutomationHistoryRequestSchema, { automationId })
      );

      return {
        success: response.success,
        message: response.message,
      };
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Callback Wrappers
  // ============================================================

  // Use mutateAsync directly (stable reference from TanStack Query v5)
  // instead of the full mutation object which changes on every render.
  const listAutomations = useCallback(async (): Promise<LocalAutomation[]> => {
    return listAutomationsMutation.mutateAsync();
  }, [listAutomationsMutation.mutateAsync]);

  const getAutomation = useCallback(
    async (id: string): Promise<LocalAutomation> => {
      return getAutomationMutation.mutateAsync(id);
    },
    [getAutomationMutation.mutateAsync]
  );

  const createAutomation = useCallback(
    async (params: CreateAutomationParams): Promise<LocalAutomation> => {
      return createAutomationMutation.mutateAsync(params);
    },
    [createAutomationMutation.mutateAsync]
  );

  const updateAutomation = useCallback(
    async (params: UpdateAutomationParams): Promise<LocalAutomation> => {
      return updateAutomationMutation.mutateAsync(params);
    },
    [updateAutomationMutation.mutateAsync]
  );

  const deleteAutomation = useCallback(
    async (id: string): Promise<void> => {
      return deleteAutomationMutation.mutateAsync(id);
    },
    [deleteAutomationMutation.mutateAsync]
  );

  const diagnoseAutomation = useCallback(
    async (automationId: string): Promise<LocalDiagnosticResult> => {
      return diagnoseAutomationMutation.mutateAsync(automationId);
    },
    [diagnoseAutomationMutation.mutateAsync]
  );

  const runAutomation = useCallback(
    async (automationId: string): Promise<LocalRunAutomationResult> => {
      return runAutomationMutation.mutateAsync(automationId);
    },
    [runAutomationMutation.mutateAsync]
  );

  const resetAutomationHistory = useCallback(
    async (automationId: string): Promise<LocalResetHistoryResult> => {
      return resetHistoryMutation.mutateAsync(automationId);
    },
    [resetHistoryMutation.mutateAsync]
  );

  return {
    listAutomations,
    getAutomation,
    createAutomation,
    updateAutomation,
    deleteAutomation,
    diagnoseAutomation,
    runAutomation,
    resetAutomationHistory,

    isListingAutomations: listAutomationsMutation.isPending,
    isGettingAutomation: getAutomationMutation.isPending,
    isCreatingAutomation: createAutomationMutation.isPending,
    isUpdatingAutomation: updateAutomationMutation.isPending,
    isDeletingAutomation: deleteAutomationMutation.isPending,
    isDiagnosingAutomation: diagnoseAutomationMutation.isPending,
    isRunningAutomation: runAutomationMutation.isPending,
    isResettingHistory: resetHistoryMutation.isPending,
  };
}
