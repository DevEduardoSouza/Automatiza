/**
 * Types for the DashboardTab component and useDashboardTab hook.
 * Manages admin dashboard statistics, observability, and audit data.
 */

export interface DashboardStats {
  totalUsers: number;
  activeUsers: number;
  inactiveUsers: number;
  planStats: { plan: string; count: number }[];
  growthData: { date: string; users: number }[];
}

export interface DashboardTabProps {
  onStatsUpdate?: (stats: DashboardStats) => void;
}

export interface WebhookLog {
  id: string;
  email: string;
  event: string;
  product: string | null;
  processed_at: string;
}

export interface LoadBalancerServer {
  serverId: number;
  serverName: string;
  url: string;
  currentUsers: number;
  maxUsers: number;
  availableSlots: number;
  usagePercent: number;
  status: string;
  wahaInstancesTotal?: number;
  wahaInstancesOpen?: number;
  wahaInstancesConnecting?: number;
  wahaInstancesClose?: number;
}

export interface LoadBalancerSummary {
  totalServers: number;
  totalCapacity: number;
  totalUsed: number;
  totalAvailable: number;
  usagePercent: number;
  wahaInstancesTotal?: number;
  wahaInstancesOpen?: number;
  wahaInstancesConnecting?: number;
  wahaInstancesClose?: number;
}

export interface LoadBalancerStats {
  servers: LoadBalancerServer[];
  summary: LoadBalancerSummary;
}

export type LoadBalancerStatsResponse = {
  success: boolean;
  data?: LoadBalancerStats;
};

export interface ObservabilityData {
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
    topErrors: Array<{ error: string; count: number }>;
    topUsers: Array<{
      userId: string;
      sent: number;
      error: number;
      skipped: number;
      queued: number;
    }>;
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
    byGroupsSlots: Array<{
      userId: string;
      automations: number;
      groupsSlots: number;
    }>;
  };
}

export type ObservabilityResponse = {
  success: boolean;
  data?: ObservabilityData;
  cached?: boolean;
};

export interface ShopeeAuditSample {
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

export interface ShopeeAuditData {
  window: { days: number; sinceIso: string };
  scanned: number;
  withIdentifier: number;
  mismatches: number;
  unknown: number;
  missingCreds: number;
  samples: ShopeeAuditSample[];
}

export type ShopeeAffiliateAuditResponse = {
  success: boolean;
  data?: ShopeeAuditData;
};

export interface ShopeeAuditSectionProps {
  audit: ShopeeAuditData | null;
  auditLoading: boolean;
  auditDays: number;
  onAuditDaysChange: (days: number) => void;
  onRefresh: (days?: number) => void;
}

export interface UseDashboardTabReturn {
  stats: DashboardStats;
  webhookLogs: WebhookLog[];
  lbStats: LoadBalancerStats | null;
  lbLoading: boolean;
  obs: ObservabilityData | null;
  obsLoading: boolean;
  audit: ShopeeAuditData | null;
  auditLoading: boolean;
  auditDays: number;
  setAuditDays: (days: number) => void;
  ensureWebhookLoading: boolean;
  fetchLoadBalancerStats: () => Promise<void>;
  forceWebhooksServer2: () => Promise<void>;
  fetchObservability: () => Promise<void>;
  fetchShopeeAffiliateAudit: (days?: number) => Promise<void>;
  handleSimulateWebhook: (email: string, event: string, product: string) => Promise<void>;
}
