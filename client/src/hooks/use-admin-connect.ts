import type { WebhookLog } from '@/components/admin/types/dashboard.types';
import { useAuth } from '@/contexts';
import { getObservability } from '@/gen/admin/v1/admin-AdminService_connectquery';
import {
  AdminService,
  EnsureWebhooksRequestSchema,
  EventLevel,
  GetShopeeAffiliateAuditRequestSchema,
  GetSystemEventsRequestSchema,
  ListWebhookLogsRequestSchema,
  type ObservabilityData as ProtoObsData,
  type ShopeeAuditData as ProtoShopeeData,
  type ShopeeAuditSample as ProtoShopeeSample,
  type SystemEvent as ProtoSystemEvent,
  type TopDispatchUser as ProtoTopDispatchUser,
  type TopError as ProtoTopError,
  type TopUserByGroupSlots as ProtoTopUserByGroupSlots,
  type WebhookLogEntry as ProtoWebhookLogEntry,
  ResetUserPasswordRequestSchema,
  type ResetUserPasswordResponse,
} from '@/gen/admin/v1/admin_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useQuery, useTransport } from '@connectrpc/connect-query';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

// ============================================================
// Local Types (snake_case for UI compatibility)
// ============================================================

type LocalEventLevel = 'debug' | 'info' | 'warn' | 'error';

interface LocalTopError {
  error: string;
  count: number;
}

interface LocalTopDispatchUser {
  userId: string;
  sent: number;
  error: number;
  skipped: number;
  queued: number;
}

interface LocalTopUserByGroupSlots {
  userId: string;
  automations: number;
  groupsSlots: number;
}

interface LocalObservabilityData {
  generatedAt: string;
  queue: {
    waiting: number;
    delayed: number;
    active: number;
    queued: number;
    limitPerMinute: number | null;
    workerConcurrency: number | null;
    estimatedMinutes: number | null;
  };
  loadBalancer: {
    summary: {
      totalServers: number;
      totalCapacity: number;
      totalMapped: number;
      totalAvailable: number;
      usagePercent: number;
      wahaInstancesTotal: number;
      wahaInstancesOpen: number;
      wahaInstancesConnecting: number;
      wahaInstancesClose: number;
    };
  };
  dispatchLogs: {
    windowMinutes: number;
    total: number;
    byStatus: Record<string, number>;
    topErrors: LocalTopError[];
    topUsers: LocalTopDispatchUser[];
  };
  totals: {
    usersWithAnyAutomation: number;
    usersWithActiveAutomations: number;
    automationsTotal: number;
    automationsActive: number;
    groupsTotal: number;
    avgGroupsPerAutomation: number;
  };
  topUsers: {
    byGroupsSlots: LocalTopUserByGroupSlots[];
  };
}

interface LocalShopeeAuditSample {
  user_id: string;
  email?: string | null;
  phone_number?: string | null;
  expected_app_id: string;
  expected_mmp_pid: string;
  actual_mmp_pid?: string | null;
  actual_af_siteid?: string | null;
  created_at: string;
  automation_id: string;
  automation_name: string;
  group_id: string;
  group_name?: string | null;
  status: string;
  url: string;
}

interface LocalShopeeAuditData {
  window: { days: number; sinceIso: string };
  scanned: number;
  withIdentifier: number;
  mismatches: number;
  unknown: number;
  missingCreds: number;
  samples: LocalShopeeAuditSample[];
}

interface LocalSystemEvent {
  id: string;
  created_at: string;
  user_id: string | null;
  instance_id: string | null;
  level: LocalEventLevel;
  scope: string;
  message: string;
  meta: Record<string, unknown>;
}

interface EnsureWebhooksParams {
  serverId: number;
  limit?: number;
  concurrency?: number;
  dryRun?: boolean;
}

interface EnsureWebhooksResult {
  ok: number;
  total: number;
  fail: number;
}

interface GetSystemEventsParams {
  userId?: string;
  instanceId?: string;
  limit?: number;
  sinceMinutes?: number;
}

interface LocalResetPasswordResult {
  success: boolean;
  email: string;
  newPassword?: string;
  message: string;
}

interface UseAdminConnectReturn {
  /** Observability data */
  observability: LocalObservabilityData | null;
  /** Whether observability is loading */
  obsLoading: boolean;
  /** Whether observability is from cache */
  obsCached: boolean;
  /** Refetch observability */
  refetchObservability: () => void;
  /** Whether any mutation is pending */
  isPending: boolean;
  /** Fetch Shopee affiliate audit data */
  fetchShopeeAudit: (params?: {
    days?: number;
    limit?: number;
    maxScan?: number;
  }) => Promise<LocalShopeeAuditData | null>;
  /** Ensure webhooks for a server */
  ensureWebhooks: (params: EnsureWebhooksParams) => Promise<EnsureWebhooksResult>;
  /** Fetch system events */
  fetchSystemEvents: (params: GetSystemEventsParams) => Promise<LocalSystemEvent[]>;
  /** Reset a user's password (admin only) */
  resetUserPassword: (userId: string) => Promise<LocalResetPasswordResult>;
  /** Whether password reset is in progress */
  isResettingPassword: boolean;
  /** List webhook logs */
  listWebhookLogs: (params?: { limit?: number; source?: string }) => Promise<WebhookLog[]>;
}

// ============================================================
// Conversion Functions
// ============================================================

function fromProtoEventLevel(level: EventLevel): LocalEventLevel {
  switch (level) {
    case EventLevel.DEBUG:
      return 'debug';
    case EventLevel.INFO:
      return 'info';
    case EventLevel.WARN:
      return 'warn';
    case EventLevel.ERROR:
      return 'error';
    default:
      return 'info';
  }
}

function fromProtoTopError(e: ProtoTopError): LocalTopError {
  return { error: e.error, count: e.count };
}

function fromProtoTopDispatchUser(u: ProtoTopDispatchUser): LocalTopDispatchUser {
  return {
    userId: u.userId,
    sent: u.sent,
    error: u.error,
    skipped: u.skipped,
    queued: u.queued,
  };
}

function fromProtoTopUserByGroupSlots(u: ProtoTopUserByGroupSlots): LocalTopUserByGroupSlots {
  return {
    userId: u.userId,
    automations: u.automations,
    groupsSlots: u.groupsSlots,
  };
}

function fromProtoObservability(data: ProtoObsData | undefined): LocalObservabilityData | null {
  if (!data) return null;

  return {
    generatedAt: data.generatedAt,
    queue: {
      waiting: data.queue?.waiting ?? 0,
      delayed: data.queue?.delayed ?? 0,
      active: data.queue?.active ?? 0,
      queued: data.queue?.queued ?? 0,
      limitPerMinute: data.queue?.limitPerMinute ?? null,
      workerConcurrency: data.queue?.workerConcurrency ?? null,
      estimatedMinutes: data.queue?.estimatedMinutes ?? null,
    },
    loadBalancer: {
      summary: {
        totalServers: data.loadBalancer?.summary?.totalServers ?? 0,
        totalCapacity: data.loadBalancer?.summary?.totalCapacity ?? 0,
        totalMapped: data.loadBalancer?.summary?.totalMapped ?? 0,
        totalAvailable: data.loadBalancer?.summary?.totalAvailable ?? 0,
        usagePercent: data.loadBalancer?.summary?.usagePercent ?? 0,
        wahaInstancesTotal: data.loadBalancer?.summary?.wahaInstancesTotal ?? 0,
        wahaInstancesOpen: data.loadBalancer?.summary?.wahaInstancesOpen ?? 0,
        wahaInstancesConnecting: data.loadBalancer?.summary?.wahaInstancesConnecting ?? 0,
        wahaInstancesClose: data.loadBalancer?.summary?.wahaInstancesClose ?? 0,
      },
    },
    dispatchLogs: {
      windowMinutes: data.dispatchLogs?.windowMinutes ?? 0,
      total: data.dispatchLogs?.total ?? 0,
      byStatus: data.dispatchLogs?.byStatus ?? {},
      topErrors: (data.dispatchLogs?.topErrors ?? []).map(fromProtoTopError),
      topUsers: (data.dispatchLogs?.topUsers ?? []).map(fromProtoTopDispatchUser),
    },
    totals: {
      usersWithAnyAutomation: data.totals?.usersWithAnyAutomation ?? 0,
      usersWithActiveAutomations: data.totals?.usersWithActiveAutomations ?? 0,
      automationsTotal: data.totals?.automationsTotal ?? 0,
      automationsActive: data.totals?.automationsActive ?? 0,
      groupsTotal: data.totals?.groupsTotal ?? 0,
      avgGroupsPerAutomation: data.totals?.avgGroupsPerAutomation ?? 0,
    },
    topUsers: {
      byGroupsSlots: (data.topUsers?.byGroupsSlots ?? []).map(fromProtoTopUserByGroupSlots),
    },
  };
}

function fromProtoShopeeAuditSample(s: ProtoShopeeSample): LocalShopeeAuditSample {
  return {
    user_id: s.userId,
    email: s.email ?? null,
    phone_number: s.phoneNumber ?? null,
    expected_app_id: s.expectedAppId,
    expected_mmp_pid: s.expectedMmpPid,
    actual_mmp_pid: s.actualMmpPid ?? null,
    actual_af_siteid: s.actualAfSiteid ?? null,
    created_at: s.createdAt,
    automation_id: s.automationId,
    automation_name: s.automationName,
    group_id: s.groupId,
    group_name: s.groupName ?? null,
    status: s.status,
    url: s.url,
  };
}

function fromProtoShopeeAudit(data: ProtoShopeeData | undefined): LocalShopeeAuditData | null {
  if (!data?.window) return null;

  return {
    window: { days: data.window.days, sinceIso: data.window.sinceIso },
    scanned: data.scanned,
    withIdentifier: data.withIdentifier,
    mismatches: data.mismatches,
    unknown: data.unknown,
    missingCreds: data.missingCreds,
    samples: data.samples.map(fromProtoShopeeAuditSample),
  };
}

function fromProtoSystemEvent(e: ProtoSystemEvent): LocalSystemEvent {
  return {
    id: e.id,
    created_at: e.createdAt,
    user_id: e.userId ?? null,
    instance_id: e.instanceId ?? null,
    level: fromProtoEventLevel(e.level),
    scope: e.scope,
    message: e.message,
    meta: (e.meta as Record<string, unknown>) ?? {},
  };
}

function fromProtoWebhookLogEntry(entry: ProtoWebhookLogEntry): WebhookLog {
  return {
    id: entry.id,
    email: entry.email ?? '',
    event: entry.eventType ?? '',
    product: entry.plan ?? null,
    processed_at: entry.createdAt,
  };
}

// ============================================================
// Hook
// ============================================================

/**
 * Hook for admin operations via ConnectRPC.
 * Provides observability, shopee audit, webhooks, and system events.
 */
export function useAdminConnect(): UseAdminConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const transport = useTransport();

  // Query for fetching observability data using ConnectRPC Query
  const {
    data: obsResponse,
    isLoading: obsLoading,
    refetch: refetchObs,
  } = useQuery(
    getObservability,
    {},
    {
      enabled: !!user,
      staleTime: 30_000, // 30 seconds
      refetchInterval: 60_000, // Refresh every minute
    }
  );

  // Convert proto response to local format
  const observability = useMemo(() => fromProtoObservability(obsResponse?.data), [obsResponse]);
  const obsCached = obsResponse?.cached ?? false;

  // Mutation for fetching Shopee audit (not a query because it has params)
  const shopeeAuditMutation = useMutation({
    mutationFn: async (params?: { days?: number; limit?: number; maxScan?: number }) => {
      const client = createClient(AdminService, transport);
      const response = await client.getShopeeAffiliateAudit(
        create(GetShopeeAffiliateAuditRequestSchema, {
          days: params?.days ?? 7,
          limit: params?.limit ?? 25,
          maxScan: params?.maxScan ?? 5000,
        })
      );
      return fromProtoShopeeAudit(response.data);
    },
  });

  // Mutation for ensuring webhooks
  const ensureWebhooksMutation = useMutation({
    mutationFn: async (params: EnsureWebhooksParams) => {
      const client = createClient(AdminService, transport);
      const response = await client.ensureWebhooks(
        create(EnsureWebhooksRequestSchema, {
          serverId: params.serverId,
          limit: params.limit,
          concurrency: params.concurrency,
          dryRun: params.dryRun,
        })
      );
      return { ok: response.ok, total: response.total, fail: response.fail };
    },
    onSuccess: (result) => {
      toast({
        title: t('admin.dashboard.loadBalancer.forceWebhooksSuccess', {
          ok: result.ok,
          total: result.total,
          fail: result.fail,
        }),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('admin.dashboard.loadBalancer.forceWebhooksError'),
        description: errorInfo.userMessage,
      });
    },
  });

  // Mutation for fetching system events
  const systemEventsMutation = useMutation({
    mutationFn: async (params: GetSystemEventsParams) => {
      const client = createClient(AdminService, transport);
      const response = await client.getSystemEvents(
        create(GetSystemEventsRequestSchema, {
          userId: params.userId,
          instanceId: params.instanceId,
          limit: params.limit ?? 100,
          sinceMinutes: params.sinceMinutes ?? 60,
        })
      );
      return response.events.map(fromProtoSystemEvent);
    },
  });

  // Mutation for resetting user password
  const resetPasswordMutation = useMutation({
    mutationFn: async (userId: string): Promise<LocalResetPasswordResult> => {
      const client = createClient(AdminService, transport);
      const response: ResetUserPasswordResponse = await client.resetUserPassword(
        create(ResetUserPasswordRequestSchema, { userId })
      );
      return {
        success: response.success,
        email: response.email,
        newPassword: response.newPassword,
        message: response.message,
      };
    },
    onSuccess: (result) => {
      toast({
        title: t('admin.users.success.passwordReset', { email: result.email }),
        description: result.newPassword
          ? t('admin.users.success.tempPassword', { password: result.newPassword })
          : result.message,
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('admin.users.errors.resetPasswordError'),
        description: errorInfo.userMessage,
      });
    },
  });

  // Mutation for listing webhook logs
  const webhookLogsMutation = useMutation({
    mutationFn: async (params?: { limit?: number; source?: string }) => {
      const client = createClient(AdminService, transport);
      const response = await client.listWebhookLogs(
        create(ListWebhookLogsRequestSchema, {
          limit: params?.limit ?? 10,
          source: params?.source ?? 'kiwify',
        })
      );
      return response.logs.map(fromProtoWebhookLogEntry);
    },
  });

  // Imperative wrappers
  const fetchShopeeAudit = async (params?: {
    days?: number;
    limit?: number;
    maxScan?: number;
  }): Promise<LocalShopeeAuditData | null> => {
    try {
      return await shopeeAuditMutation.mutateAsync(params);
    } catch {
      return null;
    }
  };

  const ensureWebhooks = async (params: EnsureWebhooksParams): Promise<EnsureWebhooksResult> => {
    return ensureWebhooksMutation.mutateAsync(params);
  };

  const fetchSystemEvents = async (params: GetSystemEventsParams): Promise<LocalSystemEvent[]> => {
    try {
      return await systemEventsMutation.mutateAsync(params);
    } catch {
      return [];
    }
  };

  const resetUserPassword = async (userId: string): Promise<LocalResetPasswordResult> => {
    return resetPasswordMutation.mutateAsync(userId);
  };

  const listWebhookLogs = async (params?: { limit?: number; source?: string }): Promise<
    WebhookLog[]
  > => {
    try {
      return await webhookLogsMutation.mutateAsync(params);
    } catch {
      return [];
    }
  };

  const isPending =
    shopeeAuditMutation.isPending ||
    ensureWebhooksMutation.isPending ||
    systemEventsMutation.isPending ||
    resetPasswordMutation.isPending ||
    webhookLogsMutation.isPending;

  return {
    observability,
    obsLoading,
    obsCached,
    refetchObservability: () => refetchObs(),
    isPending,
    fetchShopeeAudit,
    ensureWebhooks,
    fetchSystemEvents,
    resetUserPassword,
    isResettingPassword: resetPasswordMutation.isPending,
    listWebhookLogs,
  };
}
