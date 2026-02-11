import { useAuth } from '@/contexts';
import {
  type ActiveStore,
  type AffiliateCredential,
  AffiliateCredentialsService,
  AffiliateStore,
  DeleteCredentialRequestSchema,
  GetCredentialRequestSchema,
  type GetCredentialResponse,
  ListActiveStoresRequestSchema,
  type ListActiveStoresResponse,
  ListCredentialsRequestSchema,
  type ListCredentialsResponse,
  UpsertCredentialRequestSchema,
  type UpsertCredentialResponse,
} from '@/gen/affiliates/v1/credentials_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { captureHookError } from '@/lib/sentry';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import * as Sentry from '@sentry/react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

// ============================================================
// Local Types
// ============================================================

export type LocalAffiliateStore =
  | 'shopee'
  | 'amazon'
  | 'magalu'
  | 'ml'
  | 'shein'
  | 'aliexpress'
  | 'awin';

export interface LocalAffiliateCredential {
  id: string;
  user_id: string;
  store: LocalAffiliateStore;
  credentials: Record<string, string>;
  auto_generate: boolean;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface LocalActiveStore {
  store: LocalAffiliateStore;
  is_configured: boolean;
  auto_generate: boolean;
}

export interface UpsertCredentialParams {
  store: LocalAffiliateStore;
  credentials: Record<string, string>;
  auto_generate?: boolean;
  is_active?: boolean;
}

// ============================================================
// Hook Return Type
// ============================================================

interface UseAffiliateCredentialsConnectReturn {
  credentials: LocalAffiliateCredential[];
  activeStores: LocalActiveStore[];
  loading: boolean;
  refetch: () => Promise<void>;

  listCredentials: () => Promise<LocalAffiliateCredential[]>;
  getCredential: (store: LocalAffiliateStore) => Promise<LocalAffiliateCredential | null>;
  upsertCredential: (params: UpsertCredentialParams) => Promise<LocalAffiliateCredential>;
  deleteCredential: (store: LocalAffiliateStore) => Promise<void>;
  listActiveStores: () => Promise<LocalActiveStore[]>;

  isListingCredentials: boolean;
  isGettingCredential: boolean;
  isUpsertingCredential: boolean;
  isDeletingCredential: boolean;
  isListingActiveStores: boolean;
}

// ============================================================
// Converters
// ============================================================

function toProtoStore(store: LocalAffiliateStore): AffiliateStore {
  const map: Record<LocalAffiliateStore, AffiliateStore> = {
    shopee: AffiliateStore.SHOPEE,
    amazon: AffiliateStore.AMAZON,
    magalu: AffiliateStore.MAGALU,
    ml: AffiliateStore.MERCADO_LIVRE,
    shein: AffiliateStore.SHEIN,
    aliexpress: AffiliateStore.ALIEXPRESS,
    awin: AffiliateStore.AWIN,
  };
  return map[store] ?? AffiliateStore.UNSPECIFIED;
}

function fromProtoStore(store: AffiliateStore): LocalAffiliateStore {
  const map: Record<AffiliateStore, LocalAffiliateStore> = {
    [AffiliateStore.UNSPECIFIED]: 'shopee',
    [AffiliateStore.SHOPEE]: 'shopee',
    [AffiliateStore.AMAZON]: 'amazon',
    [AffiliateStore.MAGALU]: 'magalu',
    [AffiliateStore.MERCADO_LIVRE]: 'ml',
    [AffiliateStore.SHEIN]: 'shein',
    [AffiliateStore.ALIEXPRESS]: 'aliexpress',
    [AffiliateStore.AWIN]: 'awin',
  };
  return map[store] ?? 'shopee';
}

function fromProtoCredential(credential: AffiliateCredential): LocalAffiliateCredential {
  let parsedCredentials: Record<string, string> = {};
  try {
    parsedCredentials = JSON.parse(credential.credentials || '{}');
  } catch (error: unknown) {
    Sentry.captureMessage(
      `Failed to parse affiliate credential JSON for store=${credential.store}`,
      'warning'
    );
    captureHookError(error, 'useAffiliateCredentialsConnect', 'fromProtoCredential');
    parsedCredentials = {};
  }

  return {
    id: credential.id,
    user_id: credential.userId,
    store: fromProtoStore(credential.store),
    credentials: parsedCredentials,
    auto_generate: credential.autoGenerate,
    is_active: credential.isActive,
    created_at: credential.createdAt,
    updated_at: credential.updatedAt,
  };
}

function fromProtoActiveStore(store: ActiveStore): LocalActiveStore {
  return {
    store: fromProtoStore(store.store),
    is_configured: store.isConfigured,
    auto_generate: store.autoGenerate,
  };
}

// ============================================================
// Hook Implementation
// ============================================================

/**
 * Hook for affiliate credentials operations via ConnectRPC.
 * Provides CRUD operations for managing affiliate store credentials.
 */
export function useAffiliateCredentialsConnect(): UseAffiliateCredentialsConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();
  const queryClient = useQueryClient();

  // ============================================================
  // List Credentials Query
  // ============================================================

  const {
    data: credentialsData,
    isLoading: loadingCredentials,
    refetch: refetchCredentials,
  } = useQuery({
    queryKey: ['affiliate-credentials', 'list', user?.id],
    queryFn: async (): Promise<LocalAffiliateCredential[]> => {
      if (!user?.id) return [];
      const client = createClient(AffiliateCredentialsService, transport);
      const response: ListCredentialsResponse = await client.listCredentials(
        create(ListCredentialsRequestSchema, {})
      );
      return response.credentials.map(fromProtoCredential);
    },
    enabled: !!user?.id,
    staleTime: 60000,
  });

  // ============================================================
  // List Active Stores Query
  // ============================================================

  const {
    data: activeStoresData,
    isLoading: loadingActiveStores,
    refetch: refetchActiveStores,
  } = useQuery({
    queryKey: ['affiliate-credentials', 'active-stores', user?.id],
    queryFn: async (): Promise<LocalActiveStore[]> => {
      if (!user?.id) return [];
      const client = createClient(AffiliateCredentialsService, transport);
      const response: ListActiveStoresResponse = await client.listActiveStores(
        create(ListActiveStoresRequestSchema, {})
      );
      return response.stores.map(fromProtoActiveStore);
    },
    enabled: !!user?.id,
    staleTime: 60000,
  });

  const credentials = credentialsData ?? [];
  const activeStores = activeStoresData ?? [];
  const loading = loadingCredentials || loadingActiveStores;

  const refetch = useCallback(async () => {
    await Promise.all([refetchCredentials(), refetchActiveStores()]);
  }, [refetchCredentials, refetchActiveStores]);

  // ============================================================
  // List Credentials Mutation
  // ============================================================

  const listCredentialsMutation = useMutation({
    mutationFn: async (): Promise<LocalAffiliateCredential[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AffiliateCredentialsService, transport);
      const response: ListCredentialsResponse = await client.listCredentials(
        create(ListCredentialsRequestSchema, {})
      );

      return response.credentials.map(fromProtoCredential);
    },
    onError: (error: unknown) => {
      captureHookError(error, 'useAffiliateCredentialsConnect', 'listCredentials');
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Get Credential
  // ============================================================

  const getCredentialMutation = useMutation({
    mutationFn: async (store: LocalAffiliateStore): Promise<LocalAffiliateCredential | null> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AffiliateCredentialsService, transport);
      const response: GetCredentialResponse = await client.getCredential(
        create(GetCredentialRequestSchema, { store: toProtoStore(store) })
      );

      return response.credential ? fromProtoCredential(response.credential) : null;
    },
    onError: (error: unknown) => {
      captureHookError(error, 'useAffiliateCredentialsConnect', 'getCredential');
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Upsert Credential
  // ============================================================

  const upsertCredentialMutation = useMutation({
    mutationFn: async (params: UpsertCredentialParams): Promise<LocalAffiliateCredential> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AffiliateCredentialsService, transport);
      const response: UpsertCredentialResponse = await client.upsertCredential(
        create(UpsertCredentialRequestSchema, {
          store: toProtoStore(params.store),
          credentials: JSON.stringify(params.credentials),
          autoGenerate: params.auto_generate,
          isActive: params.is_active,
        })
      );

      if (!response.credential) {
        throw new Error(t('pages.affiliatePrograms.errors.saveFailed'));
      }

      return fromProtoCredential(response.credential);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['affiliate-credentials'] });
      toast({
        title: t('pages.affiliatePrograms.success.saved'),
      });
    },
    onError: (error: unknown) => {
      captureHookError(error, 'useAffiliateCredentialsConnect', 'upsertCredential');
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Delete Credential
  // ============================================================

  const deleteCredentialMutation = useMutation({
    mutationFn: async (store: LocalAffiliateStore): Promise<void> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AffiliateCredentialsService, transport);
      await client.deleteCredential(
        create(DeleteCredentialRequestSchema, { store: toProtoStore(store) })
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['affiliate-credentials'] });
      toast({
        title: t('pages.affiliatePrograms.success.deleted'),
      });
    },
    onError: (error: unknown) => {
      captureHookError(error, 'useAffiliateCredentialsConnect', 'deleteCredential');
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // List Active Stores Mutation
  // ============================================================

  const listActiveStoresMutation = useMutation({
    mutationFn: async (): Promise<LocalActiveStore[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AffiliateCredentialsService, transport);
      const response: ListActiveStoresResponse = await client.listActiveStores(
        create(ListActiveStoresRequestSchema, {})
      );

      return response.stores.map(fromProtoActiveStore);
    },
    onError: (error: unknown) => {
      captureHookError(error, 'useAffiliateCredentialsConnect', 'listActiveStores');
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

  const listCredentials = useCallback(async (): Promise<LocalAffiliateCredential[]> => {
    return listCredentialsMutation.mutateAsync();
  }, [listCredentialsMutation]);

  const getCredential = useCallback(
    async (store: LocalAffiliateStore): Promise<LocalAffiliateCredential | null> => {
      return getCredentialMutation.mutateAsync(store);
    },
    [getCredentialMutation]
  );

  const upsertCredential = useCallback(
    async (params: UpsertCredentialParams): Promise<LocalAffiliateCredential> => {
      return upsertCredentialMutation.mutateAsync(params);
    },
    [upsertCredentialMutation]
  );

  const deleteCredential = useCallback(
    async (store: LocalAffiliateStore): Promise<void> => {
      return deleteCredentialMutation.mutateAsync(store);
    },
    [deleteCredentialMutation]
  );

  const listActiveStores = useCallback(async (): Promise<LocalActiveStore[]> => {
    return listActiveStoresMutation.mutateAsync();
  }, [listActiveStoresMutation]);

  return {
    credentials,
    activeStores,
    loading,
    refetch,

    listCredentials,
    getCredential,
    upsertCredential,
    deleteCredential,
    listActiveStores,

    isListingCredentials: listCredentialsMutation.isPending,
    isGettingCredential: getCredentialMutation.isPending,
    isUpsertingCredential: upsertCredentialMutation.isPending,
    isDeletingCredential: deleteCredentialMutation.isPending,
    isListingActiveStores: listActiveStoresMutation.isPending,
  };
}
