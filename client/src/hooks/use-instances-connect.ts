import { useAuth } from '@/contexts';
import {
  CheckInstanceStatusRequestSchema,
  type CheckInstanceStatusResponse,
  CleanupInvalidInstancesRequestSchema,
  type CleanupInvalidInstancesResponse,
  DeleteDisconnectedInstancesRequestSchema,
  type DeleteDisconnectedInstancesResponse,
  DeleteInstanceRequestSchema,
  GetLatestInstanceRequestSchema,
  type GetLatestInstanceResponse,
  type Instance,
  InstanceStatus,
  InstancesService,
  ListUserInstancesRequestSchema,
  type ListUserInstancesResponse,
  ReconnectInstanceRequestSchema,
  type ReconnectInstanceResponse,
  SetDefaultInstanceRequestSchema,
  type SetDefaultInstanceResponse,
  UpsertInstanceStatusRequestSchema,
  type UpsertInstanceStatusResponse,
} from '@/gen/instances/v1/instances_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

// ============================================================
// Local Types
// ============================================================

export type LocalInstanceStatus = 'pending' | 'connected' | 'disconnected';

export interface LocalInstance {
  id: string;
  user_id: string;
  instance_id: string;
  status: LocalInstanceStatus;
  is_default: boolean;
  // Optional human-readable label. Falls back to instance_id when absent.
  friendly_name?: string;
  created_at: string;
  updated_at: string;
}

// ============================================================
// Hook Return Type
// ============================================================

export interface CheckInstanceStatusResult {
  success: boolean;
  state: string;
  status: string;
}

export interface ReconnectInstanceResult {
  success: boolean;
  qrcodeBase64?: string;
  error?: string;
}

interface UseInstancesConnectReturn {
  latestInstance: LocalInstance | null;
  instances: LocalInstance[];
  loading: boolean;
  refetch: () => Promise<void>;

  getLatestInstance: () => Promise<LocalInstance | null>;
  listUserInstances: (status?: LocalInstanceStatus) => Promise<LocalInstance[]>;
  upsertInstanceStatus: (instanceId: string, status: LocalInstanceStatus) => Promise<LocalInstance>;
  cleanupInvalidInstances: () => Promise<number>;
  deleteDisconnectedInstances: () => Promise<number>;
  deleteInstance: (id: string) => Promise<void>;
  setDefaultInstance: (id: string) => Promise<LocalInstance>;
  checkInstanceStatus: (instanceName: string) => Promise<CheckInstanceStatusResult>;
  reconnectInstance: (instanceName: string) => Promise<ReconnectInstanceResult>;

  isGettingLatest: boolean;
  isListingInstances: boolean;
  isUpserting: boolean;
  isCleaning: boolean;
  isDeletingDisconnected: boolean;
  isDeletingInstance: boolean;
  isSettingDefault: boolean;
  isCheckingStatus: boolean;
  isReconnecting: boolean;
}

// ============================================================
// Converters
// ============================================================

function toProtoStatus(status: LocalInstanceStatus): InstanceStatus {
  const map: Record<LocalInstanceStatus, InstanceStatus> = {
    pending: InstanceStatus.PENDING,
    connected: InstanceStatus.CONNECTED,
    disconnected: InstanceStatus.DISCONNECTED,
  };
  return map[status] ?? InstanceStatus.UNSPECIFIED;
}

function fromProtoStatus(status: InstanceStatus): LocalInstanceStatus {
  const map: Record<InstanceStatus, LocalInstanceStatus> = {
    [InstanceStatus.UNSPECIFIED]: 'pending',
    [InstanceStatus.PENDING]: 'pending',
    [InstanceStatus.CONNECTED]: 'connected',
    [InstanceStatus.DISCONNECTED]: 'disconnected',
  };
  return map[status] ?? 'pending';
}

function fromProtoInstance(instance: Instance): LocalInstance {
  return {
    id: instance.id,
    user_id: instance.userId,
    instance_id: instance.instanceId,
    status: fromProtoStatus(instance.status),
    is_default: instance.isDefault ?? false,
    friendly_name: instance.friendlyName ?? undefined,
    created_at: instance.createdAt,
    updated_at: instance.updatedAt,
  };
}

// ============================================================
// Hook Implementation
// ============================================================

/**
 * Hook for WhatsApp instance operations via ConnectRPC.
 * Provides instance management and cleanup functionality.
 */
export function useInstancesConnect(): UseInstancesConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();
  const queryClient = useQueryClient();

  // ============================================================
  // Get Latest Instance Query
  // ============================================================

  const {
    data: latestInstanceData,
    isLoading: loadingLatest,
    refetch: refetchLatest,
  } = useQuery({
    queryKey: ['instances', 'latest', user?.id],
    queryFn: async (): Promise<LocalInstance | null> => {
      if (!user?.id) return null;
      const client = createClient(InstancesService, transport);
      const response: GetLatestInstanceResponse = await client.getLatestInstance(
        create(GetLatestInstanceRequestSchema, {})
      );
      return response.instance ? fromProtoInstance(response.instance) : null;
    },
    enabled: !!user?.id,
    staleTime: 10000,
    refetchInterval: 30000,
  });

  // ============================================================
  // List User Instances Query
  // ============================================================

  const {
    data: instancesData,
    isLoading: loadingInstances,
    refetch: refetchInstances,
  } = useQuery({
    queryKey: ['instances', 'list', user?.id],
    queryFn: async (): Promise<LocalInstance[]> => {
      if (!user?.id) return [];
      const client = createClient(InstancesService, transport);
      const response: ListUserInstancesResponse = await client.listUserInstances(
        create(ListUserInstancesRequestSchema, {})
      );
      return response.instances.map(fromProtoInstance);
    },
    enabled: !!user?.id,
    staleTime: 10000,
  });

  const latestInstance = latestInstanceData ?? null;
  const instances = instancesData ?? [];
  const loading = loadingLatest || loadingInstances;

  const refetch = useCallback(async () => {
    await Promise.all([refetchLatest(), refetchInstances()]);
  }, [refetchLatest, refetchInstances]);

  // ============================================================
  // Get Latest Instance Mutation
  // ============================================================

  const getLatestMutation = useMutation({
    mutationFn: async (): Promise<LocalInstance | null> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(InstancesService, transport);
      const response: GetLatestInstanceResponse = await client.getLatestInstance(
        create(GetLatestInstanceRequestSchema, {})
      );

      return response.instance ? fromProtoInstance(response.instance) : null;
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
  // List User Instances Mutation
  // ============================================================

  const listInstancesMutation = useMutation({
    mutationFn: async (status?: LocalInstanceStatus): Promise<LocalInstance[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(InstancesService, transport);
      const response: ListUserInstancesResponse = await client.listUserInstances(
        create(ListUserInstancesRequestSchema, {
          status: status ? toProtoStatus(status) : undefined,
        })
      );

      return response.instances.map(fromProtoInstance);
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
  // Upsert Instance Status
  // ============================================================

  const upsertMutation = useMutation({
    mutationFn: async (params: {
      instanceId: string;
      status: LocalInstanceStatus;
    }): Promise<LocalInstance> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(InstancesService, transport);
      const response: UpsertInstanceStatusResponse = await client.upsertInstanceStatus(
        create(UpsertInstanceStatusRequestSchema, {
          instanceId: params.instanceId,
          status: toProtoStatus(params.status),
        })
      );

      if (!response.instance) {
        throw new Error(t('pages.whatsapp.errors.upsertFailed'));
      }

      return fromProtoInstance(response.instance);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['instances'] });
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
  // Cleanup Invalid Instances
  // ============================================================

  const cleanupMutation = useMutation({
    mutationFn: async (): Promise<number> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(InstancesService, transport);
      const response: CleanupInvalidInstancesResponse = await client.cleanupInvalidInstances(
        create(CleanupInvalidInstancesRequestSchema, {})
      );

      return response.deletedCount;
    },
    onSuccess: (deletedCount) => {
      queryClient.invalidateQueries({ queryKey: ['instances'] });
      if (deletedCount > 0) {
        toast({
          title: t('pages.whatsapp.success.cleaned'),
          description: t('pages.whatsapp.success.cleanedMessage', { count: deletedCount }),
        });
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
  // Delete Disconnected Instances
  // ============================================================

  const deleteDisconnectedMutation = useMutation({
    mutationFn: async (): Promise<number> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(InstancesService, transport);
      const response: DeleteDisconnectedInstancesResponse =
        await client.deleteDisconnectedInstances(
          create(DeleteDisconnectedInstancesRequestSchema, {})
        );

      return response.deletedCount;
    },
    onSuccess: (deletedCount) => {
      queryClient.invalidateQueries({ queryKey: ['instances'] });
      if (deletedCount > 0) {
        toast({
          title: t('pages.whatsapp.success.cleaned'),
          description: t('pages.whatsapp.success.cleanedMessage', { count: deletedCount }),
        });
      } else {
        toast({
          title: t('pages.whatsapp.success.nothingToClean'),
          description: t('pages.whatsapp.success.nothingToCleanMessage'),
        });
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
  // Delete Instance
  // ============================================================

  const deleteInstanceMutation = useMutation({
    mutationFn: async (id: string): Promise<void> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(InstancesService, transport);
      await client.deleteInstance(create(DeleteInstanceRequestSchema, { id }));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['instances'] });
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
  // Set Default Instance
  // ============================================================

  const setDefaultMutation = useMutation({
    mutationFn: async (id: string): Promise<LocalInstance> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(InstancesService, transport);
      const response: SetDefaultInstanceResponse = await client.setDefaultInstance(
        create(SetDefaultInstanceRequestSchema, { id })
      );

      if (!response.instance) {
        throw new Error(t('pages.whatsapp.errors.upsertFailed'));
      }

      return fromProtoInstance(response.instance);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['instances'] });
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
  // Check Instance Status (WAHA)
  // ============================================================

  const checkStatusMutation = useMutation({
    mutationFn: async (instanceName: string): Promise<CheckInstanceStatusResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(InstancesService, transport);
      const response: CheckInstanceStatusResponse = await client.checkInstanceStatus(
        create(CheckInstanceStatusRequestSchema, { instanceName })
      );

      return {
        success: response.success,
        state: response.state,
        status: response.status,
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
  // Reconnect Instance (WAHA)
  // ============================================================

  const reconnectMutation = useMutation({
    mutationFn: async (instanceName: string): Promise<ReconnectInstanceResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(InstancesService, transport);
      const response: ReconnectInstanceResponse = await client.reconnectInstance(
        create(ReconnectInstanceRequestSchema, { instanceName })
      );

      return {
        success: response.success,
        qrcodeBase64: response.qrcodeBase64,
        error: response.error,
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

  const getLatestInstance = useCallback(async (): Promise<LocalInstance | null> => {
    return getLatestMutation.mutateAsync();
  }, [getLatestMutation]);

  const listUserInstances = useCallback(
    async (status?: LocalInstanceStatus): Promise<LocalInstance[]> => {
      return listInstancesMutation.mutateAsync(status);
    },
    [listInstancesMutation]
  );

  const upsertInstanceStatus = useCallback(
    async (instanceId: string, status: LocalInstanceStatus): Promise<LocalInstance> => {
      return upsertMutation.mutateAsync({ instanceId, status });
    },
    [upsertMutation]
  );

  const cleanupInvalidInstances = useCallback(async (): Promise<number> => {
    return cleanupMutation.mutateAsync();
  }, [cleanupMutation]);

  const deleteDisconnectedInstances = useCallback(async (): Promise<number> => {
    return deleteDisconnectedMutation.mutateAsync();
  }, [deleteDisconnectedMutation]);

  const deleteInstance = useCallback(
    async (id: string): Promise<void> => {
      return deleteInstanceMutation.mutateAsync(id);
    },
    [deleteInstanceMutation]
  );

  const setDefaultInstance = useCallback(
    async (id: string): Promise<LocalInstance> => {
      return setDefaultMutation.mutateAsync(id);
    },
    [setDefaultMutation]
  );

  const checkInstanceStatus = useCallback(
    async (instanceName: string): Promise<CheckInstanceStatusResult> => {
      return checkStatusMutation.mutateAsync(instanceName);
    },
    [checkStatusMutation]
  );

  const reconnectInstance = useCallback(
    async (instanceName: string): Promise<ReconnectInstanceResult> => {
      return reconnectMutation.mutateAsync(instanceName);
    },
    [reconnectMutation]
  );

  return {
    latestInstance,
    instances,
    loading,
    refetch,

    getLatestInstance,
    listUserInstances,
    upsertInstanceStatus,
    cleanupInvalidInstances,
    deleteDisconnectedInstances,
    deleteInstance,
    setDefaultInstance,
    checkInstanceStatus,
    reconnectInstance,

    isGettingLatest: getLatestMutation.isPending,
    isListingInstances: listInstancesMutation.isPending,
    isUpserting: upsertMutation.isPending,
    isCleaning: cleanupMutation.isPending,
    isDeletingDisconnected: deleteDisconnectedMutation.isPending,
    isDeletingInstance: deleteInstanceMutation.isPending,
    isSettingDefault: setDefaultMutation.isPending,
    isCheckingStatus: checkStatusMutation.isPending,
    isReconnecting: reconnectMutation.isPending,
  };
}
