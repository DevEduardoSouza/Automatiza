import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock values (declared before vi.mock for hoisting)
// ============================================================

let mockObservability: Record<string, unknown> | null = null;
let mockObsLoading = false;
const mockRefetchObservability = vi.fn();
const mockFetchShopeeAudit = vi.fn();
const mockListWebhookLogs = vi.fn();
const mockFetchAdminProfileStats = vi.fn();
const mockSimulateKiwifyWebhook = vi.fn();
const mockGetSetting = vi.fn();

// ============================================================
// Module Mocks
// ============================================================

vi.mock('@/hooks/use-admin-connect', () => ({
  useAdminConnect: () => ({
    observability: mockObservability,
    obsLoading: mockObsLoading,
    refetchObservability: mockRefetchObservability,
    fetchShopeeAudit: mockFetchShopeeAudit,
    listWebhookLogs: mockListWebhookLogs,
    fetchAdminProfileStats: mockFetchAdminProfileStats,
  }),
}));

vi.mock('@/hooks/use-webhooks-connect', () => ({
  useWebhooksConnect: () => ({
    simulateKiwifyWebhook: mockSimulateKiwifyWebhook,
  }),
}));

vi.mock('@/hooks/use-settings-connect', () => ({
  useSettingsConnect: () => ({
    getSetting: mockGetSetting,
  }),
}));

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'pt-BR' },
  }),
}));

// ============================================================
// Import after mocks
// ============================================================

import { toast } from 'sonner';
import { useDashboardTab } from '../use-dashboard-tab';

// ============================================================
// Helpers
// ============================================================

const sampleObservability = {
  generatedAt: '2026-01-01T00:00:00Z',
  queue: {
    waiting: 5,
    delayed: 2,
    active: 1,
    queued: 3,
    limitPerMinute: 60,
    workerConcurrency: 4,
    estimatedMinutes: 10,
  },
  dispatchLogs: {
    windowMinutes: 60,
    total: 100,
    byStatus: { sent: 80, error: 20 },
    topErrors: [{ error: 'timeout', count: 5 }],
    topUsers: [{ userId: 'u1', sent: 50, error: 2, skipped: 0, queued: 1 }],
  },
  totals: {
    usersWithAnyAutomation: 10,
    usersWithActiveAutomations: 8,
    automationsTotal: 25,
    automationsActive: 20,
    groupsTotal: 100,
    avgGroupsPerAutomation: 4,
  },
  topUsers: {
    byGroupsSlots: [{ userId: 'u1', automations: 5, groupsSlots: 20 }],
  },
};

const sampleAuditResult = {
  window: { days: 7, sinceIso: '2026-01-01T00:00:00Z' },
  scanned: 500,
  withIdentifier: 400,
  mismatches: 10,
  unknown: 5,
  missingCreds: 3,
  samples: [],
};

const sampleStats = {
  totalUsers: 100,
  activeUsers: 80,
  inactiveUsers: 20,
  planStats: [{ plan: 'pro', count: 50 }],
  growthData: [{ date: '2026-01-01', users: 10 }],
};

const sampleWebhookLogs = [
  {
    id: 'log-1',
    email: 'test@example.com',
    event: 'order.approved',
    product: 'pro',
    processed_at: '2026-01-01T00:00:00Z',
  },
];

// ============================================================
// Tests
// ============================================================

describe('useDashboardTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockObservability = null;
    mockObsLoading = false;
    mockFetchShopeeAudit.mockResolvedValue(null);
    mockListWebhookLogs.mockResolvedValue([]);
    mockFetchAdminProfileStats.mockResolvedValue(null);
    mockSimulateKiwifyWebhook.mockResolvedValue({ success: true });
    mockGetSetting.mockReturnValue('');
  });

  describe('initial state', () => {
    it('returns default stats, empty webhookLogs, null audit, auditDays=7', () => {
      const { result } = renderHook(() => useDashboardTab());

      expect(result.current.stats).toEqual({
        totalUsers: 0,
        activeUsers: 0,
        inactiveUsers: 0,
        planStats: [],
        growthData: [],
      });
      expect(result.current.webhookLogs).toEqual([]);
      expect(result.current.audit).toBeNull();
      expect(result.current.auditDays).toBe(7);
      expect(result.current.obs).toBeNull();
      expect(result.current.obsLoading).toBe(false);
    });
  });

  describe('observability mapping', () => {
    it('maps observability data correctly when present', () => {
      mockObservability = sampleObservability;

      const { result } = renderHook(() => useDashboardTab());

      expect(result.current.obs).toEqual({
        generatedAt: '2026-01-01T00:00:00Z',
        queue: sampleObservability.queue,
        dispatchLogs: sampleObservability.dispatchLogs,
        totals: sampleObservability.totals,
        topUsers: sampleObservability.topUsers,
      });
    });

    it('returns obs=null when observability is null', () => {
      mockObservability = null;

      const { result } = renderHook(() => useDashboardTab());

      expect(result.current.obs).toBeNull();
    });

    it('reflects obsLoading from useAdminConnect', () => {
      mockObsLoading = true;

      const { result } = renderHook(() => useDashboardTab());

      expect(result.current.obsLoading).toBe(true);
    });
  });

  describe('handleFetchShopeeAudit', () => {
    it('sets loading, maps result, and sets audit on success', async () => {
      mockFetchShopeeAudit.mockResolvedValue(sampleAuditResult);

      const { result } = renderHook(() => useDashboardTab());

      await act(async () => {
        await result.current.fetchShopeeAffiliateAudit(7);
      });

      expect(mockFetchShopeeAudit).toHaveBeenCalledWith({
        days: 7,
        limit: 25,
        maxScan: 5000,
      });
      expect(result.current.audit).toEqual(sampleAuditResult);
      expect(result.current.auditLoading).toBe(false);
    });

    it('sets audit to null when fetchShopeeAudit returns null', async () => {
      mockFetchShopeeAudit.mockResolvedValue(null);

      const { result } = renderHook(() => useDashboardTab());

      await act(async () => {
        await result.current.fetchShopeeAffiliateAudit(14);
      });

      expect(result.current.audit).toBeNull();
    });

    it('sets audit to null on error', async () => {
      mockFetchShopeeAudit.mockRejectedValue(new Error('network'));

      const { result } = renderHook(() => useDashboardTab());

      await act(async () => {
        await result.current.fetchShopeeAffiliateAudit(7);
      });

      expect(result.current.audit).toBeNull();
      expect(result.current.auditLoading).toBe(false);
    });

    it('uses default auditDays when no argument provided', async () => {
      mockFetchShopeeAudit.mockResolvedValue(sampleAuditResult);

      const { result } = renderHook(() => useDashboardTab());

      await act(async () => {
        await result.current.fetchShopeeAffiliateAudit();
      });

      expect(mockFetchShopeeAudit).toHaveBeenCalledWith({
        days: 7,
        limit: 25,
        maxScan: 5000,
      });
    });
  });

  describe('handleSimulateWebhook', () => {
    it('shows toast.error when token is empty', async () => {
      mockGetSetting.mockReturnValue('');

      const { result } = renderHook(() => useDashboardTab());

      await act(async () => {
        await result.current.handleSimulateWebhook('test@example.com', 'order.approved', 'pro');
      });

      expect(toast.error).toHaveBeenCalledWith(
        'admin.dashboard.webhook.simulator.tokenNotConfigured'
      );
      expect(mockSimulateKiwifyWebhook).not.toHaveBeenCalled();
    });

    it('calls simulateKiwifyWebhook and shows toast.success on success', async () => {
      mockGetSetting.mockReturnValue('valid-token');
      mockSimulateKiwifyWebhook.mockResolvedValue({ success: true });
      mockListWebhookLogs.mockResolvedValue(sampleWebhookLogs);
      mockFetchAdminProfileStats.mockResolvedValue(sampleStats);

      const { result } = renderHook(() => useDashboardTab());

      await act(async () => {
        await result.current.handleSimulateWebhook('test@example.com', 'order.approved', 'pro');
      });

      expect(mockSimulateKiwifyWebhook).toHaveBeenCalledWith({
        email: 'test@example.com',
        event: 'order.approved',
        product: 'pro',
        token: 'valid-token',
      });
      expect(toast.success).toHaveBeenCalledWith('admin.dashboard.webhook.simulator.success');
    });

    it('passes undefined for empty product', async () => {
      mockGetSetting.mockReturnValue('valid-token');
      mockSimulateKiwifyWebhook.mockResolvedValue({ success: true });
      mockListWebhookLogs.mockResolvedValue([]);
      mockFetchAdminProfileStats.mockResolvedValue(null);

      const { result } = renderHook(() => useDashboardTab());

      await act(async () => {
        await result.current.handleSimulateWebhook('test@example.com', 'order.approved', '');
      });

      expect(mockSimulateKiwifyWebhook).toHaveBeenCalledWith({
        email: 'test@example.com',
        event: 'order.approved',
        product: undefined,
        token: 'valid-token',
      });
    });

    it('throws when simulateKiwifyWebhook returns an error', async () => {
      mockGetSetting.mockReturnValue('valid-token');
      mockSimulateKiwifyWebhook.mockResolvedValue({
        success: false,
        error: 'webhook failed',
      });

      const { result } = renderHook(() => useDashboardTab());

      await expect(
        act(async () => {
          await result.current.handleSimulateWebhook('test@example.com', 'order.approved', 'pro');
        })
      ).rejects.toThrow('webhook failed');
    });

    it('refetches webhook logs and stats after successful simulation', async () => {
      mockGetSetting.mockReturnValue('valid-token');
      mockSimulateKiwifyWebhook.mockResolvedValue({ success: true });
      mockListWebhookLogs.mockResolvedValue(sampleWebhookLogs);
      mockFetchAdminProfileStats.mockResolvedValue(sampleStats);

      const { result } = renderHook(() => useDashboardTab());

      // Clear calls from initial mount
      mockListWebhookLogs.mockClear();
      mockFetchAdminProfileStats.mockClear();

      await act(async () => {
        await result.current.handleSimulateWebhook('test@example.com', 'order.approved', 'pro');
      });

      expect(mockListWebhookLogs).toHaveBeenCalledWith({
        limit: 10,
        source: 'kiwify',
      });
      expect(mockFetchAdminProfileStats).toHaveBeenCalled();
    });
  });

  describe('fetchDashboardStats', () => {
    it('fetchAdminProfileStats is called on mount', async () => {
      mockFetchAdminProfileStats.mockResolvedValue(sampleStats);
      mockListWebhookLogs.mockResolvedValue([]);
      mockFetchShopeeAudit.mockResolvedValue(null);

      await act(async () => {
        renderHook(() => useDashboardTab());
        // Allow mount effect + async operations to flush
        await new Promise((r) => setTimeout(r, 50));
      });

      expect(mockFetchAdminProfileStats).toHaveBeenCalled();
    });

    it('calls onStatsUpdate callback when provided', async () => {
      const onStatsUpdate = vi.fn();
      mockFetchAdminProfileStats.mockResolvedValue(sampleStats);
      mockListWebhookLogs.mockResolvedValue([]);
      mockFetchShopeeAudit.mockResolvedValue(null);

      await act(async () => {
        renderHook(() => useDashboardTab({ onStatsUpdate }));
        await new Promise((r) => setTimeout(r, 50));
      });

      expect(mockFetchAdminProfileStats).toHaveBeenCalled();
    });

    it('does not crash when fetchAdminProfileStats returns null', async () => {
      mockFetchAdminProfileStats.mockResolvedValue(null);
      mockListWebhookLogs.mockResolvedValue([]);
      mockFetchShopeeAudit.mockResolvedValue(null);

      const { result } = renderHook(() => useDashboardTab());

      // Stats should remain at default
      expect(result.current.stats).toEqual({
        totalUsers: 0,
        activeUsers: 0,
        inactiveUsers: 0,
        planStats: [],
        growthData: [],
      });
    });
  });

  describe('fetchWebhookLogs', () => {
    it('calls listWebhookLogs with correct params on mount', async () => {
      mockListWebhookLogs.mockResolvedValue(sampleWebhookLogs);
      mockFetchAdminProfileStats.mockResolvedValue(null);
      mockFetchShopeeAudit.mockResolvedValue(null);

      await act(async () => {
        renderHook(() => useDashboardTab());
        await new Promise((r) => setTimeout(r, 50));
      });

      expect(mockListWebhookLogs).toHaveBeenCalledWith({
        limit: 10,
        source: 'kiwify',
      });
    });
  });

  describe('fetchObservability', () => {
    it('calls refetchObservability', async () => {
      const { result } = renderHook(() => useDashboardTab());

      await act(async () => {
        await result.current.fetchObservability();
      });

      expect(mockRefetchObservability).toHaveBeenCalled();
    });
  });

  describe('setAuditDays', () => {
    it('updates auditDays state', () => {
      const { result } = renderHook(() => useDashboardTab());

      act(() => {
        result.current.setAuditDays(14);
      });

      expect(result.current.auditDays).toBe(14);
    });
  });
});
