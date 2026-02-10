import { useAuth } from '@/contexts';
import { getStats } from '@/gen/admin/v1/loadbalancer-LoadBalancerService_connectquery';
import {
  type GetStatsResponse,
  GetUserServerRequestSchema,
  LoadBalancerService,
  MigrateUserRequestSchema,
  type Server as ProtoServer,
  type LoadBalancerSummary as ProtoSummary,
} from '@/gen/admin/v1/loadbalancer_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { createConnectQueryKey, useQuery, useTransport } from '@connectrpc/connect-query';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

/**
 * Local server interface matching UI expectations.
 */
interface LocalServer {
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

/**
 * Local summary interface matching UI expectations.
 */
interface LocalSummary {
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

/**
 * Local load balancer stats interface.
 */
interface LocalLoadBalancerStats {
  servers: LocalServer[];
  summary: LocalSummary;
}

/**
 * Simple server info for user assignment.
 */
interface LocalUserServer {
  id: number;
  name: string;
  url: string;
}

interface UseLoadBalancerConnectReturn {
  /** Load balancer statistics */
  stats: LocalLoadBalancerStats | null;
  /** Whether stats are loading */
  loading: boolean;
  /** Whether a mutation is pending */
  isPending: boolean;
  /** Refetch stats */
  refetch: () => void;
  /** Get server assigned to a user */
  getUserServer: (userId: string) => Promise<LocalUserServer | null>;
  /** Migrate user to a different server */
  migrateUser: (userId: string, targetServerId: number) => Promise<void>;
}

/**
 * Converts proto Server to local format.
 */
function fromProtoServer(server: ProtoServer): LocalServer {
  return {
    serverId: server.serverId,
    serverName: server.serverName,
    url: server.url,
    currentUsers: server.currentUsers,
    maxUsers: server.maxUsers,
    availableSlots: server.availableSlots,
    usagePercent: server.usagePercent,
    status: server.status,
    wahaInstancesTotal: server.wahaInstancesTotal,
    wahaInstancesOpen: server.wahaInstancesOpen,
    wahaInstancesConnecting: server.wahaInstancesConnecting,
    wahaInstancesClose: server.wahaInstancesClose,
  };
}

/**
 * Converts proto Summary to local format.
 */
function fromProtoSummary(summary: ProtoSummary): LocalSummary {
  return {
    totalServers: summary.totalServers,
    totalCapacity: summary.totalCapacity,
    totalUsed: summary.totalUsed,
    totalAvailable: summary.totalAvailable,
    usagePercent: summary.usagePercent,
    wahaInstancesTotal: summary.wahaInstancesTotal,
    wahaInstancesOpen: summary.wahaInstancesOpen,
    wahaInstancesConnecting: summary.wahaInstancesConnecting,
    wahaInstancesClose: summary.wahaInstancesClose,
  };
}

/**
 * Converts proto GetStatsResponse to local format.
 */
function fromProtoStats(response: GetStatsResponse | undefined): LocalLoadBalancerStats | null {
  if (!response?.summary) {
    return null;
  }

  return {
    servers: response.servers.map(fromProtoServer),
    summary: fromProtoSummary(response.summary),
  };
}

/**
 * Hook for load balancer operations via ConnectRPC.
 * Provides server stats, user server lookup, and migration.
 */
export function useLoadBalancerConnect(): UseLoadBalancerConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const transport = useTransport();

  // Query for fetching load balancer stats using ConnectRPC Query
  const {
    data: statsResponse,
    isLoading: loading,
    refetch,
  } = useQuery(
    getStats,
    {},
    {
      enabled: !!user,
      staleTime: 30_000, // 30 seconds
      refetchInterval: 60_000, // Refresh every minute
    }
  );

  // Convert proto response to local format
  const stats = useMemo(() => fromProtoStats(statsResponse), [statsResponse]);

  // Get user server (imperative function, not a hook query)
  const getUserServer = async (userId: string): Promise<LocalUserServer | null> => {
    try {
      const client = createClient(LoadBalancerService, transport);
      const response = await client.getUserServer(create(GetUserServerRequestSchema, { userId }));

      if (!response.server) {
        return null;
      }

      return {
        id: response.server.serverId,
        name: response.server.serverName || `Servidor ${response.server.serverId}`,
        url: response.server.url,
      };
    } catch {
      return null;
    }
  };

  // Mutation for migrating user
  const migrateMutation = useMutation({
    mutationFn: async ({ userId, targetServerId }: { userId: string; targetServerId: number }) => {
      const client = createClient(LoadBalancerService, transport);
      return client.migrateUser(create(MigrateUserRequestSchema, { userId, targetServerId }));
    },
    onSuccess: (response, variables) => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: getStats, cardinality: 'finite' }),
      });
      toast({
        title: t('admin.users.success.migrated', { server: variables.targetServerId }),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('admin.users.errors.migrateError'),
        description: errorInfo.userMessage,
      });
    },
  });

  // Migrate user (imperative wrapper)
  const migrateUser = async (userId: string, targetServerId: number): Promise<void> => {
    await migrateMutation.mutateAsync({ userId, targetServerId });
  };

  return {
    stats,
    loading,
    isPending: migrateMutation.isPending,
    refetch: () => refetch(),
    getUserServer,
    migrateUser,
  };
}
