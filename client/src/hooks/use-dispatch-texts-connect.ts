import { useAuth } from '@/contexts';
import {
  CreateDispatchTextRequestSchema,
  type CreateDispatchTextResponse,
  DeleteDispatchTextRequestSchema,
  type DispatchText,
  DispatchTextsService,
  GetDispatchTextRequestSchema,
  type GetDispatchTextResponse,
  ListDispatchTextsRequestSchema,
  type ListDispatchTextsResponse,
  UpdateDispatchTextRequestSchema,
  type UpdateDispatchTextResponse,
} from '@/gen/dispatch_texts/v1/dispatch_texts_pb';
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

export interface LocalDispatchText {
  id: string;
  user_id: string;
  name: string;
  body: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateDispatchTextParams {
  name: string;
  body: string;
  is_active?: boolean;
}

export interface UpdateDispatchTextParams {
  id: string;
  name?: string;
  body?: string;
  is_active?: boolean;
}

// ============================================================
// Hook Return Type
// ============================================================

interface UseDispatchTextsConnectReturn {
  dispatchTexts: LocalDispatchText[];
  loading: boolean;
  refetch: () => Promise<void>;

  listDispatchTexts: () => Promise<LocalDispatchText[]>;
  getDispatchText: (id: string) => Promise<LocalDispatchText>;
  createDispatchText: (params: CreateDispatchTextParams) => Promise<LocalDispatchText>;
  updateDispatchText: (params: UpdateDispatchTextParams) => Promise<LocalDispatchText>;
  deleteDispatchText: (id: string) => Promise<void>;

  isListingDispatchTexts: boolean;
  isGettingDispatchText: boolean;
  isCreatingDispatchText: boolean;
  isUpdatingDispatchText: boolean;
  isDeletingDispatchText: boolean;
}

// ============================================================
// Converters
// ============================================================

function fromProtoDispatchText(dispatchText: DispatchText): LocalDispatchText {
  return {
    id: dispatchText.id,
    user_id: dispatchText.userId,
    name: dispatchText.name,
    body: dispatchText.body,
    is_active: dispatchText.isActive,
    created_at: dispatchText.createdAt,
    updated_at: dispatchText.updatedAt,
  };
}

// ============================================================
// Hook Implementation
// ============================================================

/**
 * Hook for dispatch text operations via ConnectRPC.
 * Provides CRUD operations for dispatch text templates.
 */
export function useDispatchTextsConnect(): UseDispatchTextsConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();
  const queryClient = useQueryClient();

  // ============================================================
  // List Dispatch Texts Query
  // ============================================================

  const {
    data: dispatchTextsData,
    isLoading: loading,
    refetch: refetchQuery,
  } = useQuery({
    queryKey: ['dispatch-texts', 'list', user?.id],
    queryFn: async (): Promise<LocalDispatchText[]> => {
      if (!user?.id) return [];
      const client = createClient(DispatchTextsService, transport);
      const response: ListDispatchTextsResponse = await client.listDispatchTexts(
        create(ListDispatchTextsRequestSchema, {})
      );
      return response.dispatchTexts.map(fromProtoDispatchText);
    },
    enabled: !!user?.id,
    staleTime: 30000,
  });

  const dispatchTexts = dispatchTextsData ?? [];

  const refetch = useCallback(async () => {
    await refetchQuery();
  }, [refetchQuery]);

  // ============================================================
  // List Dispatch Texts Mutation
  // ============================================================

  const listMutation = useMutation({
    mutationFn: async (): Promise<LocalDispatchText[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(DispatchTextsService, transport);
      const response: ListDispatchTextsResponse = await client.listDispatchTexts(
        create(ListDispatchTextsRequestSchema, {})
      );

      return response.dispatchTexts.map(fromProtoDispatchText);
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
  // Get Dispatch Text
  // ============================================================

  const getMutation = useMutation({
    mutationFn: async (id: string): Promise<LocalDispatchText> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(DispatchTextsService, transport);
      const response: GetDispatchTextResponse = await client.getDispatchText(
        create(GetDispatchTextRequestSchema, { id })
      );

      if (!response.dispatchText) {
        throw new Error(t('pages.dispatchTexts.errors.notFound'));
      }

      return fromProtoDispatchText(response.dispatchText);
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
  // Create Dispatch Text
  // ============================================================

  const createMutation = useMutation({
    mutationFn: async (params: CreateDispatchTextParams): Promise<LocalDispatchText> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(DispatchTextsService, transport);
      const response: CreateDispatchTextResponse = await client.createDispatchText(
        create(CreateDispatchTextRequestSchema, {
          name: params.name,
          body: params.body,
          isActive: params.is_active,
        })
      );

      if (!response.dispatchText) {
        throw new Error(t('pages.dispatchTexts.errors.createFailed'));
      }

      return fromProtoDispatchText(response.dispatchText);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dispatch-texts'] });
      toast({
        title: t('pages.dispatchTexts.success.created'),
      });
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
  // Update Dispatch Text
  // ============================================================

  const updateMutation = useMutation({
    mutationFn: async (params: UpdateDispatchTextParams): Promise<LocalDispatchText> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(DispatchTextsService, transport);
      const response: UpdateDispatchTextResponse = await client.updateDispatchText(
        create(UpdateDispatchTextRequestSchema, {
          id: params.id,
          name: params.name,
          body: params.body,
          isActive: params.is_active,
        })
      );

      if (!response.dispatchText) {
        throw new Error(t('pages.dispatchTexts.errors.updateFailed'));
      }

      return fromProtoDispatchText(response.dispatchText);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dispatch-texts'] });
      toast({
        title: t('pages.dispatchTexts.success.updated'),
      });
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
  // Delete Dispatch Text
  // ============================================================

  const deleteMutation = useMutation({
    mutationFn: async (id: string): Promise<void> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(DispatchTextsService, transport);
      await client.deleteDispatchText(create(DeleteDispatchTextRequestSchema, { id }));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dispatch-texts'] });
      toast({
        title: t('pages.dispatchTexts.success.deleted'),
      });
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

  const listDispatchTexts = useCallback(async (): Promise<LocalDispatchText[]> => {
    return listMutation.mutateAsync();
  }, [listMutation]);

  const getDispatchText = useCallback(
    async (id: string): Promise<LocalDispatchText> => {
      return getMutation.mutateAsync(id);
    },
    [getMutation]
  );

  const createDispatchText = useCallback(
    async (params: CreateDispatchTextParams): Promise<LocalDispatchText> => {
      return createMutation.mutateAsync(params);
    },
    [createMutation]
  );

  const updateDispatchText = useCallback(
    async (params: UpdateDispatchTextParams): Promise<LocalDispatchText> => {
      return updateMutation.mutateAsync(params);
    },
    [updateMutation]
  );

  const deleteDispatchText = useCallback(
    async (id: string): Promise<void> => {
      return deleteMutation.mutateAsync(id);
    },
    [deleteMutation]
  );

  return {
    dispatchTexts,
    loading,
    refetch,

    listDispatchTexts,
    getDispatchText,
    createDispatchText,
    updateDispatchText,
    deleteDispatchText,

    isListingDispatchTexts: listMutation.isPending,
    isGettingDispatchText: getMutation.isPending,
    isCreatingDispatchText: createMutation.isPending,
    isUpdatingDispatchText: updateMutation.isPending,
    isDeletingDispatchText: deleteMutation.isPending,
  };
}
