import { renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules
// ============================================================

const mockUser = { id: 'user-123', email: 'admin@example.com' };
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

vi.mock('@/gen/admin/v1/admin_pb', () => ({
  AdminService: 'AdminService',
  EnsureWebhooksRequestSchema: 'EnsureWebhooksRequestSchema',
  GetAdminProfileStatsRequestSchema: 'GetAdminProfileStatsRequestSchema',
  GetShopeeAffiliateAuditRequestSchema: 'GetShopeeAffiliateAuditRequestSchema',
  GetSystemEventsRequestSchema: 'GetSystemEventsRequestSchema',
  ListWebhookLogsRequestSchema: 'ListWebhookLogsRequestSchema',
  ResetUserPasswordRequestSchema: 'ResetUserPasswordRequestSchema',
  EventLevel: { DEBUG: 0, INFO: 1, WARN: 2, ERROR: 3 },
}));

vi.mock('@/gen/admin/v1/admin-AdminService_connectquery', () => ({
  getObservability: 'getObservability',
}));

vi.mock('@bufbuild/protobuf', () => ({
  create: (_schema: unknown, data: unknown) => data,
}));

// ============================================================
// Mock: ConnectRPC client
// ============================================================

const mockClient = {
  getShopeeAffiliateAudit: vi.fn(),
  ensureWebhooks: vi.fn(),
  getSystemEvents: vi.fn(),
  resetUserPassword: vi.fn(),
  listWebhookLogs: vi.fn(),
  getAdminProfileStats: vi.fn(),
};

vi.mock('@connectrpc/connect', () => ({
  createClient: () => mockClient,
}));

// ============================================================
// Mock: ConnectRPC Query
// ============================================================

type ConnectQueryConfig = {
  enabled?: boolean;
  staleTime?: number;
  refetchInterval?: number;
};

let capturedConnectQueryConfig: ConnectQueryConfig | null = null;
const mockRefetch = vi.fn();
let mockObsResponse: Record<string, unknown> | undefined = undefined;

vi.mock('@connectrpc/connect-query', () => ({
  useTransport: () => 'mock-transport',
  useQuery: (_descriptor: unknown, _input: unknown, config: ConnectQueryConfig) => {
    capturedConnectQueryConfig = config;
    return {
      data: mockObsResponse,
      isLoading: false,
      refetch: mockRefetch,
    };
  },
}));

// ============================================================
// Mock: React Query
// ============================================================

type MutationConfig = {
  mutationFn: (...args: unknown[]) => Promise<unknown>;
  onSuccess?: (result: unknown) => void;
  onError?: (error: unknown) => void;
};

const capturedMutations: MutationConfig[] = [];
let pendingMutationIndex = -1;
const mockInvalidateQueries = vi.fn();

vi.mock('@tanstack/react-query', () => ({
  useMutation: (config: MutationConfig) => {
    const idx = capturedMutations.length;
    capturedMutations.push(config);
    return {
      mutateAsync: config.mutationFn,
      isPending: idx === pendingMutationIndex,
    };
  },
  useQueryClient: () => ({
    invalidateQueries: mockInvalidateQueries,
  }),
}));

// ============================================================
// Import after mocks
// ============================================================

import { useAdminConnect } from '../use-admin-connect';

// ============================================================
// Test Data
// ============================================================

const mockProtoObsData = {
  generatedAt: '2026-02-22T00:00:00Z',
  queue: {
    waiting: 5,
    delayed: 2,
    active: 1,
    queued: 8,
    limitPerMinute: 100,
    workerConcurrency: 4,
    estimatedMinutes: 3,
  },
  dispatchLogs: {
    windowMinutes: 60,
    total: 500,
    byStatus: { sent: 400, error: 50, skipped: 50 },
    topErrors: [{ error: 'timeout', count: 20 }],
    topUsers: [{ userId: 'u-1', sent: 100, error: 5, skipped: 2, queued: 10 }],
  },
  totals: {
    usersWithAnyAutomation: 10,
    usersWithActiveAutomations: 8,
    automationsTotal: 25,
    automationsActive: 20,
    groupsTotal: 50,
    avgGroupsPerAutomation: 2,
  },
  topUsers: {
    byGroupsSlots: [{ userId: 'u-1', automations: 5, groupsSlots: 15 }],
  },
};

const mockProtoShopeeAuditResponse = {
  data: {
    window: { days: 7, sinceIso: '2026-02-15T00:00:00Z' },
    scanned: 1000,
    withIdentifier: 800,
    mismatches: 10,
    unknown: 5,
    missingCreds: 3,
    samples: [
      {
        userId: 'u-1',
        email: 'user@test.com',
        phoneNumber: '+5511999999999',
        expectedAppId: 'app-123',
        expectedMmpPid: 'mmp-456',
        actualMmpPid: 'mmp-789',
        actualAfSiteid: 'af-001',
        createdAt: '2026-02-20T00:00:00Z',
        automationId: 'auto-1',
        automationName: 'Shopee Auto',
        groupId: 'g-1',
        groupName: 'Test Group',
        status: 'mismatch',
        url: 'https://shopee.com.br/product/123',
      },
    ],
  },
};

const mockProtoSystemEventsResponse = {
  events: [
    {
      id: 'ev-1',
      createdAt: '2026-02-22T01:00:00Z',
      userId: 'u-1',
      instanceId: 'inst-1',
      level: 3, // ERROR
      scope: 'dispatch',
      message: 'Failed to send message',
      meta: { groupId: 'g-1' },
    },
  ],
};

const mockResetPasswordResponse = {
  success: true,
  email: 'user@test.com',
  newPassword: 'temp-pass-123',
  message: 'Password reset successfully',
};

const mockWebhookLogsResponse = {
  logs: [
    {
      id: 'wl-1',
      email: 'buyer@test.com',
      eventType: 'order_paid',
      plan: 'premium',
      createdAt: '2026-02-22T02:00:00Z',
    },
  ],
};

const mockProfileStatsResponse = {
  stats: {
    totalUsers: 100,
    activeUsers: 80,
    inactiveUsers: 20,
    planStats: [
      { plan: 'free', count: 50 },
      { plan: 'pro', count: 30 },
    ],
    growthData: [
      { date: '2026-02-01', users: 90 },
      { date: '2026-02-15', users: 100 },
    ],
  },
};

// ============================================================
// Test Helpers
// ============================================================

function renderAndGet(options?: { pendingIdx?: number }) {
  capturedMutations.length = 0;
  capturedConnectQueryConfig = null;
  pendingMutationIndex = options?.pendingIdx ?? -1;
  const { result } = renderHook(() => useAdminConnect());
  return result;
}

// ============================================================
// Tests
// ============================================================

describe('useAdminConnect', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    capturedMutations.length = 0;
    capturedConnectQueryConfig = null;
    pendingMutationIndex = -1;
    authUser = mockUser;
    mockObsResponse = { data: mockProtoObsData, cached: true };

    mockClient.getShopeeAffiliateAudit.mockResolvedValue(mockProtoShopeeAuditResponse);
    mockClient.ensureWebhooks.mockResolvedValue({ ok: 8, total: 10, fail: 2 });
    mockClient.getSystemEvents.mockResolvedValue(mockProtoSystemEventsResponse);
    mockClient.resetUserPassword.mockResolvedValue(mockResetPasswordResponse);
    mockClient.listWebhookLogs.mockResolvedValue(mockWebhookLogsResponse);
    mockClient.getAdminProfileStats.mockResolvedValue(mockProfileStatsResponse);
  });

  // ── Proto conversion: fromProtoObservability ──────────────

  describe('fromProtoObservability', () => {
    it('should convert full proto observability data to local format', () => {
      const result = renderAndGet();

      expect(result.current.observability).toEqual({
        generatedAt: '2026-02-22T00:00:00Z',
        queue: {
          waiting: 5,
          delayed: 2,
          active: 1,
          queued: 8,
          limitPerMinute: 100,
          workerConcurrency: 4,
          estimatedMinutes: 3,
        },
        dispatchLogs: {
          windowMinutes: 60,
          total: 500,
          byStatus: { sent: 400, error: 50, skipped: 50 },
          topErrors: [{ error: 'timeout', count: 20 }],
          topUsers: [{ userId: 'u-1', sent: 100, error: 5, skipped: 2, queued: 10 }],
        },
        totals: {
          usersWithAnyAutomation: 10,
          usersWithActiveAutomations: 8,
          automationsTotal: 25,
          automationsActive: 20,
          groupsTotal: 50,
          avgGroupsPerAutomation: 2,
        },
        topUsers: {
          byGroupsSlots: [{ userId: 'u-1', automations: 5, groupsSlots: 15 }],
        },
      });
    });

    it('should return null when proto data is undefined', () => {
      mockObsResponse = { data: undefined };
      const result = renderAndGet();

      expect(result.current.observability).toBeNull();
    });

    it('should handle missing nested fields with defaults', () => {
      mockObsResponse = {
        data: {
          generatedAt: '2026-02-22T00:00:00Z',
          queue: undefined,
          dispatchLogs: undefined,
          totals: undefined,
          topUsers: undefined,
        },
      };
      const result = renderAndGet();

      expect(result.current.observability).toEqual({
        generatedAt: '2026-02-22T00:00:00Z',
        queue: {
          waiting: 0,
          delayed: 0,
          active: 0,
          queued: 0,
          limitPerMinute: null,
          workerConcurrency: null,
          estimatedMinutes: null,
        },
        dispatchLogs: {
          windowMinutes: 0,
          total: 0,
          byStatus: {},
          topErrors: [],
          topUsers: [],
        },
        totals: {
          usersWithAnyAutomation: 0,
          usersWithActiveAutomations: 0,
          automationsTotal: 0,
          automationsActive: 0,
          groupsTotal: 0,
          avgGroupsPerAutomation: 0,
        },
        topUsers: {
          byGroupsSlots: [],
        },
      });
    });

    it('should read obsCached from response', () => {
      mockObsResponse = { data: mockProtoObsData, cached: true };
      const result = renderAndGet();

      expect(result.current.obsCached).toBe(true);
    });

    it('should default obsCached to false when not present', () => {
      mockObsResponse = { data: mockProtoObsData };
      const result = renderAndGet();

      expect(result.current.obsCached).toBe(false);
    });
  });

  // ── Query config ──────────────────────────────────────────

  describe('observability query config', () => {
    it('should set enabled=true when user exists', () => {
      renderAndGet();

      expect(capturedConnectQueryConfig!.enabled).toBe(true);
    });

    it('should set enabled=false when user is null', () => {
      authUser = null;
      renderAndGet();

      expect(capturedConnectQueryConfig!.enabled).toBe(false);
    });

    it('should set staleTime to 30000', () => {
      renderAndGet();

      expect(capturedConnectQueryConfig!.staleTime).toBe(30000);
    });

    it('should set refetchInterval to 60000', () => {
      renderAndGet();

      expect(capturedConnectQueryConfig!.refetchInterval).toBe(60000);
    });

    it('should call refetch when refetchObservability is invoked', () => {
      const result = renderAndGet();
      result.current.refetchObservability();

      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });
  });

  // ── fetchShopeeAudit ─────────────────────────────────────

  describe('fetchShopeeAudit', () => {
    it('should return mapped audit data on success', async () => {
      const result = renderAndGet();
      const audit = await result.current.fetchShopeeAudit({ days: 7 });

      expect(audit).toEqual({
        window: { days: 7, sinceIso: '2026-02-15T00:00:00Z' },
        scanned: 1000,
        withIdentifier: 800,
        mismatches: 10,
        unknown: 5,
        missingCreds: 3,
        samples: [
          {
            user_id: 'u-1',
            email: 'user@test.com',
            phone_number: '+5511999999999',
            expected_app_id: 'app-123',
            expected_mmp_pid: 'mmp-456',
            actual_mmp_pid: 'mmp-789',
            actual_af_siteid: 'af-001',
            created_at: '2026-02-20T00:00:00Z',
            automation_id: 'auto-1',
            automation_name: 'Shopee Auto',
            group_id: 'g-1',
            group_name: 'Test Group',
            status: 'mismatch',
            url: 'https://shopee.com.br/product/123',
          },
        ],
      });
    });

    it('should return null when audit data has no window', async () => {
      mockClient.getShopeeAffiliateAudit.mockResolvedValue({ data: {} });
      const result = renderAndGet();
      const audit = await result.current.fetchShopeeAudit();

      expect(audit).toBeNull();
    });

    it('should return null on error', async () => {
      mockClient.getShopeeAffiliateAudit.mockRejectedValue(new Error('Audit failed'));
      const result = renderAndGet();
      const audit = await result.current.fetchShopeeAudit();

      expect(audit).toBeNull();
    });

    it('should call client with default params when none provided', async () => {
      const result = renderAndGet();
      await result.current.fetchShopeeAudit();

      expect(mockClient.getShopeeAffiliateAudit).toHaveBeenCalledWith({
        days: 7,
        limit: 25,
        maxScan: 5000,
      });
    });

    it('should call client with custom params', async () => {
      const result = renderAndGet();
      await result.current.fetchShopeeAudit({ days: 30, limit: 50, maxScan: 10000 });

      expect(mockClient.getShopeeAffiliateAudit).toHaveBeenCalledWith({
        days: 30,
        limit: 50,
        maxScan: 10000,
      });
    });
  });

  // ── ensureWebhooks ────────────────────────────────────────

  describe('ensureWebhooks', () => {
    it('should return ok/total/fail on success', async () => {
      const result = renderAndGet();
      const webhookResult = await result.current.ensureWebhooks({
        serverId: 1,
        limit: 50,
        concurrency: 5,
        dryRun: false,
      });

      expect(webhookResult).toEqual({ ok: 8, total: 10, fail: 2 });
    });

    it('should show success toast', async () => {
      const result = renderAndGet();
      await result.current.ensureWebhooks({ serverId: 1 });

      // Trigger onSuccess manually via captured mutation
      const ensureMutation = capturedMutations[1];
      ensureMutation.onSuccess?.({ ok: 8, total: 10, fail: 2 });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'Webhooks: ok 8/10 • fail 2',
      });
    });

    it('should show error toast on failure', () => {
      renderAndGet();

      const ensureMutation = capturedMutations[1];
      ensureMutation.onError?.(new Error('Webhook setup failed'));

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'common.error',
          description: 'Webhook setup failed',
        })
      );
    });

    it('should pass params to client', async () => {
      const result = renderAndGet();
      await result.current.ensureWebhooks({
        serverId: 2,
        limit: 100,
        concurrency: 10,
        dryRun: true,
      });

      expect(mockClient.ensureWebhooks).toHaveBeenCalledWith({
        serverId: 2,
        limit: 100,
        concurrency: 10,
        dryRun: true,
      });
    });
  });

  // ── fetchSystemEvents ─────────────────────────────────────

  describe('fetchSystemEvents', () => {
    it('should return mapped events on success', async () => {
      const result = renderAndGet();
      const events = await result.current.fetchSystemEvents({ limit: 50 });

      expect(events).toEqual([
        {
          id: 'ev-1',
          created_at: '2026-02-22T01:00:00Z',
          user_id: 'u-1',
          instance_id: 'inst-1',
          level: 'error',
          scope: 'dispatch',
          message: 'Failed to send message',
          meta: { groupId: 'g-1' },
        },
      ]);
    });

    it('should return empty array on error', async () => {
      mockClient.getSystemEvents.mockRejectedValue(new Error('Events failed'));
      const result = renderAndGet();
      const events = await result.current.fetchSystemEvents({ limit: 10 });

      expect(events).toEqual([]);
    });

    it('should use default params when not provided', async () => {
      const result = renderAndGet();
      await result.current.fetchSystemEvents({});

      expect(mockClient.getSystemEvents).toHaveBeenCalledWith(
        expect.objectContaining({
          limit: 100,
          sinceMinutes: 60,
        })
      );
    });

    it('should pass custom userId and instanceId', async () => {
      const result = renderAndGet();
      await result.current.fetchSystemEvents({
        userId: 'u-2',
        instanceId: 'inst-2',
        limit: 25,
        sinceMinutes: 30,
      });

      expect(mockClient.getSystemEvents).toHaveBeenCalledWith({
        userId: 'u-2',
        instanceId: 'inst-2',
        limit: 25,
        sinceMinutes: 30,
      });
    });

    it('should handle events with null userId and instanceId', async () => {
      mockClient.getSystemEvents.mockResolvedValue({
        events: [
          {
            id: 'ev-2',
            createdAt: '2026-02-22T03:00:00Z',
            userId: undefined,
            instanceId: undefined,
            level: 1, // INFO
            scope: 'system',
            message: 'Health check OK',
            meta: {},
          },
        ],
      });
      const result = renderAndGet();
      const events = await result.current.fetchSystemEvents({});

      expect(events[0].user_id).toBeNull();
      expect(events[0].instance_id).toBeNull();
      expect(events[0].level).toBe('info');
    });
  });

  // ── resetUserPassword ─────────────────────────────────────

  describe('resetUserPassword', () => {
    it('should return reset result on success', async () => {
      const result = renderAndGet();
      const resetResult = await result.current.resetUserPassword('u-1');

      expect(resetResult).toEqual({
        success: true,
        email: 'user@test.com',
        newPassword: 'temp-pass-123',
        message: 'Password reset successfully',
      });
    });

    it('should show success toast with email and temp password', () => {
      renderAndGet();

      const resetMutation = capturedMutations[3];
      resetMutation.onSuccess?.({
        success: true,
        email: 'user@test.com',
        newPassword: 'temp-pass-123',
        message: 'Password reset successfully',
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'admin.users.success.passwordReset',
        description: 'admin.users.success.tempPassword',
      });
    });

    it('should show message when newPassword is absent', () => {
      renderAndGet();

      const resetMutation = capturedMutations[3];
      resetMutation.onSuccess?.({
        success: true,
        email: 'user@test.com',
        newPassword: undefined,
        message: 'Reset email sent',
      });

      expect(mockToast).toHaveBeenCalledWith({
        title: 'admin.users.success.passwordReset',
        description: 'Reset email sent',
      });
    });

    it('should show error toast on failure', () => {
      renderAndGet();

      const resetMutation = capturedMutations[3];
      resetMutation.onError?.(new Error('User not found'));

      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: 'admin.users.errors.resetPasswordError',
          description: 'User not found',
        })
      );
    });

    it('should call client with userId', async () => {
      const result = renderAndGet();
      await result.current.resetUserPassword('u-42');

      expect(mockClient.resetUserPassword).toHaveBeenCalledWith({ userId: 'u-42' });
    });
  });

  // ── listWebhookLogs ───────────────────────────────────────

  describe('listWebhookLogs', () => {
    it('should return mapped webhook logs on success', async () => {
      const result = renderAndGet();
      const logs = await result.current.listWebhookLogs();

      expect(logs).toEqual([
        {
          id: 'wl-1',
          email: 'buyer@test.com',
          event: 'order_paid',
          product: 'premium',
          processed_at: '2026-02-22T02:00:00Z',
        },
      ]);
    });

    it('should return empty array on error', async () => {
      mockClient.listWebhookLogs.mockRejectedValue(new Error('Logs failed'));
      const result = renderAndGet();
      const logs = await result.current.listWebhookLogs();

      expect(logs).toEqual([]);
    });

    it('should use default params when none provided', async () => {
      const result = renderAndGet();
      await result.current.listWebhookLogs();

      expect(mockClient.listWebhookLogs).toHaveBeenCalledWith({
        limit: 10,
        source: 'kiwify',
      });
    });

    it('should pass custom params', async () => {
      const result = renderAndGet();
      await result.current.listWebhookLogs({ limit: 50, source: 'stripe' });

      expect(mockClient.listWebhookLogs).toHaveBeenCalledWith({
        limit: 50,
        source: 'stripe',
      });
    });

    it('should handle entries with missing optional fields', async () => {
      mockClient.listWebhookLogs.mockResolvedValue({
        logs: [
          {
            id: 'wl-2',
            email: undefined,
            eventType: undefined,
            plan: undefined,
            createdAt: '2026-02-22T03:00:00Z',
          },
        ],
      });
      const result = renderAndGet();
      const logs = await result.current.listWebhookLogs();

      expect(logs[0]).toEqual({
        id: 'wl-2',
        email: '',
        event: '',
        product: null,
        processed_at: '2026-02-22T03:00:00Z',
      });
    });
  });

  // ── fetchAdminProfileStats ────────────────────────────────

  describe('fetchAdminProfileStats', () => {
    it('should return stats with planStats and growthData on success', async () => {
      const result = renderAndGet();
      const stats = await result.current.fetchAdminProfileStats();

      expect(stats).toEqual({
        totalUsers: 100,
        activeUsers: 80,
        inactiveUsers: 20,
        planStats: [
          { plan: 'free', count: 50 },
          { plan: 'pro', count: 30 },
        ],
        growthData: [
          { date: '2026-02-01', users: 90 },
          { date: '2026-02-15', users: 100 },
        ],
      });
    });

    it('should return null when stats is missing in response', async () => {
      mockClient.getAdminProfileStats.mockResolvedValue({ stats: undefined });
      const result = renderAndGet();
      const stats = await result.current.fetchAdminProfileStats();

      expect(stats).toBeNull();
    });

    it('should return null on error', async () => {
      mockClient.getAdminProfileStats.mockRejectedValue(new Error('Stats failed'));
      const result = renderAndGet();
      const stats = await result.current.fetchAdminProfileStats();

      expect(stats).toBeNull();
    });
  });

  // ── isPending ─────────────────────────────────────────────

  describe('isPending', () => {
    it('should be false when no mutations are pending', () => {
      const result = renderAndGet();

      expect(result.current.isPending).toBe(false);
    });

    it('should be true when shopeeAudit mutation is pending', () => {
      const result = renderAndGet({ pendingIdx: 0 });

      expect(result.current.isPending).toBe(true);
    });

    it('should be true when ensureWebhooks mutation is pending', () => {
      const result = renderAndGet({ pendingIdx: 1 });

      expect(result.current.isPending).toBe(true);
    });

    it('should be true when systemEvents mutation is pending', () => {
      const result = renderAndGet({ pendingIdx: 2 });

      expect(result.current.isPending).toBe(true);
    });

    it('should be true when resetPassword mutation is pending', () => {
      const result = renderAndGet({ pendingIdx: 3 });

      expect(result.current.isPending).toBe(true);
    });

    it('should be true when webhookLogs mutation is pending', () => {
      const result = renderAndGet({ pendingIdx: 4 });

      expect(result.current.isPending).toBe(true);
    });

    it('should be true when profileStats mutation is pending', () => {
      const result = renderAndGet({ pendingIdx: 5 });

      expect(result.current.isPending).toBe(true);
    });
  });

  // ── isResettingPassword ───────────────────────────────────

  describe('isResettingPassword', () => {
    it('should be false when resetPassword is not pending', () => {
      const result = renderAndGet();

      expect(result.current.isResettingPassword).toBe(false);
    });

    it('should be true when resetPassword mutation is pending', () => {
      const result = renderAndGet({ pendingIdx: 3 });

      expect(result.current.isResettingPassword).toBe(true);
    });
  });

  // ── Return shape ──────────────────────────────────────────

  describe('return shape', () => {
    it('should return all expected properties', () => {
      const result = renderAndGet();

      expect(typeof result.current.obsLoading).toBe('boolean');
      expect(typeof result.current.obsCached).toBe('boolean');
      expect(typeof result.current.refetchObservability).toBe('function');
      expect(typeof result.current.isPending).toBe('boolean');
      expect(typeof result.current.fetchShopeeAudit).toBe('function');
      expect(typeof result.current.ensureWebhooks).toBe('function');
      expect(typeof result.current.fetchSystemEvents).toBe('function');
      expect(typeof result.current.resetUserPassword).toBe('function');
      expect(typeof result.current.isResettingPassword).toBe('boolean');
      expect(typeof result.current.listWebhookLogs).toBe('function');
      expect(typeof result.current.fetchAdminProfileStats).toBe('function');
    });
  });

  // ── Proto conversion: fromProtoWebhookLogEntry ────────────

  describe('fromProtoWebhookLogEntry', () => {
    it('should map email fallback to empty string', async () => {
      mockClient.listWebhookLogs.mockResolvedValue({
        logs: [
          {
            id: 'wl-3',
            email: '',
            eventType: 'refund',
            plan: null,
            createdAt: '2026-01-01T00:00:00Z',
          },
        ],
      });
      const result = renderAndGet();
      const logs = await result.current.listWebhookLogs();

      expect(logs[0].email).toBe('');
      expect(logs[0].product).toBeNull();
    });
  });

  // ── Proto conversion: fromProtoSystemEvent ────────────────

  describe('fromProtoSystemEvent event levels', () => {
    it('should map DEBUG level', async () => {
      mockClient.getSystemEvents.mockResolvedValue({
        events: [
          {
            id: 'ev-d',
            createdAt: '',
            userId: '',
            instanceId: '',
            level: 0,
            scope: '',
            message: '',
            meta: {},
          },
        ],
      });
      const result = renderAndGet();
      const events = await result.current.fetchSystemEvents({});

      expect(events[0].level).toBe('debug');
    });

    it('should map INFO level', async () => {
      mockClient.getSystemEvents.mockResolvedValue({
        events: [
          {
            id: 'ev-i',
            createdAt: '',
            userId: '',
            instanceId: '',
            level: 1,
            scope: '',
            message: '',
            meta: {},
          },
        ],
      });
      const result = renderAndGet();
      const events = await result.current.fetchSystemEvents({});

      expect(events[0].level).toBe('info');
    });

    it('should map WARN level', async () => {
      mockClient.getSystemEvents.mockResolvedValue({
        events: [
          {
            id: 'ev-w',
            createdAt: '',
            userId: '',
            instanceId: '',
            level: 2,
            scope: '',
            message: '',
            meta: {},
          },
        ],
      });
      const result = renderAndGet();
      const events = await result.current.fetchSystemEvents({});

      expect(events[0].level).toBe('warn');
    });

    it('should map ERROR level', async () => {
      mockClient.getSystemEvents.mockResolvedValue({
        events: [
          {
            id: 'ev-e',
            createdAt: '',
            userId: '',
            instanceId: '',
            level: 3,
            scope: '',
            message: '',
            meta: {},
          },
        ],
      });
      const result = renderAndGet();
      const events = await result.current.fetchSystemEvents({});

      expect(events[0].level).toBe('error');
    });

    it('should default unknown level to info', async () => {
      mockClient.getSystemEvents.mockResolvedValue({
        events: [
          {
            id: 'ev-u',
            createdAt: '',
            userId: '',
            instanceId: '',
            level: 99,
            scope: '',
            message: '',
            meta: {},
          },
        ],
      });
      const result = renderAndGet();
      const events = await result.current.fetchSystemEvents({});

      expect(events[0].level).toBe('info');
    });
  });
});
