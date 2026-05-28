import { useAuth } from '@/contexts';
import { PaginationRequestSchema } from '@/gen/common/v1/pagination_pb';
import {
  BulkCreateMlLinkListItemsRequestSchema,
  type BulkCreateMlLinkListItemsResponse,
  CreateMlLinkListItemRequestSchema,
  type CreateMlLinkListItemResponse,
  CreateMlLinkListRequestSchema,
  type CreateMlLinkListResponse,
  DeleteMlLinkListItemRequestSchema,
  DeleteMlLinkListRequestSchema,
  GetMlLinkListRequestSchema,
  type GetMlLinkListResponse,
  ListMlLinkListItemsRequestSchema,
  type ListMlLinkListItemsResponse,
  ListMlLinkListsRequestSchema,
  type ListMlLinkListsResponse,
  type MlLinkList,
  type MlLinkListItem,
  MlLinksService,
  ReorderItemsRequestSchema,
  ResetConsumedItemsRequestSchema,
  type ResetConsumedItemsResponse,
  UpdateMlLinkListItemRequestSchema,
  type UpdateMlLinkListItemResponse,
  UpdateMlLinkListRequestSchema,
  type UpdateMlLinkListResponse,
} from '@/gen/ml_links/v1/ml_links_pb';
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

export interface LocalMlLinkList {
  id: string;
  user_id: string;
  name: string;
  description?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface LocalMlLinkListItem {
  id: string;
  list_id: string;
  origin_url: string;
  order_index: number;
  is_active: boolean;
  consumed_at?: string;
  consumed_by_automation_id?: string;
  consumed_by_group_id?: string;
  product_title?: string;
  product_price?: number;
  product_image_url?: string;
  metadata?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateMlLinkListParams {
  name: string;
  description?: string;
  is_active?: boolean;
}

export interface UpdateMlLinkListParams {
  id: string;
  name?: string;
  description?: string;
  is_active?: boolean;
}

export interface CreateMlLinkListItemParams {
  list_id: string;
  origin_url: string;
  order_index?: number;
  is_active?: boolean;
  product_title?: string;
  product_price?: number;
  product_image_url?: string;
  metadata?: string;
}

export interface UpdateMlLinkListItemParams {
  id: string;
  is_active?: boolean;
  order_index?: number;
}

export interface PaginatedItemsParams {
  listId: string;
  pageSize?: number;
  pageToken?: string;
}

export interface PaginatedItemsResult {
  items: LocalMlLinkListItem[];
  nextPageToken: string;
  totalCount: number;
  hasMore: boolean;
}

// ============================================================
// Hook Return Type
// ============================================================

interface UseMlLinksConnectReturn {
  lists: LocalMlLinkList[];
  loadingLists: boolean;
  refetchLists: () => Promise<void>;

  listMlLinkLists: () => Promise<LocalMlLinkList[]>;
  getMlLinkList: (id: string) => Promise<LocalMlLinkList>;
  createMlLinkList: (params: CreateMlLinkListParams) => Promise<LocalMlLinkList>;
  updateMlLinkList: (params: UpdateMlLinkListParams) => Promise<LocalMlLinkList>;
  deleteMlLinkList: (id: string) => Promise<void>;

  listMlLinkListItems: (listId: string) => Promise<LocalMlLinkListItem[]>;
  listMlLinkListItemsPaginated: (params: PaginatedItemsParams) => Promise<PaginatedItemsResult>;
  createMlLinkListItem: (params: CreateMlLinkListItemParams) => Promise<LocalMlLinkListItem>;
  bulkCreateMlLinkListItems: (
    listId: string,
    originUrls: string[],
    baseOrderIndex?: number
  ) => Promise<{
    createdCount: number;
    skippedCount: number;
    skippedUrls: string[];
    items: LocalMlLinkListItem[];
  }>;
  updateMlLinkListItem: (params: UpdateMlLinkListItemParams) => Promise<LocalMlLinkListItem>;
  deleteMlLinkListItem: (id: string) => Promise<void>;
  resetConsumedItems: (listId: string) => Promise<number>;
  reorderItems: (listId: string, orderMap: Record<string, number>) => Promise<void>;

  isListingLists: boolean;
  isGettingList: boolean;
  isCreatingList: boolean;
  isUpdatingList: boolean;
  isDeletingList: boolean;
  isListingItems: boolean;
  isListingItemsPaginated: boolean;
  isCreatingItem: boolean;
  isBulkCreatingItems: boolean;
  isUpdatingItem: boolean;
  isDeletingItem: boolean;
  isResettingConsumed: boolean;
  isReordering: boolean;
}

// ============================================================
// Converters
// ============================================================

function fromProtoMlLinkList(list: MlLinkList): LocalMlLinkList {
  return {
    id: list.id,
    user_id: list.userId,
    name: list.name,
    description: list.description,
    is_active: list.isActive,
    created_at: list.createdAt,
    updated_at: list.updatedAt,
  };
}

function fromProtoMlLinkListItem(item: MlLinkListItem): LocalMlLinkListItem {
  return {
    id: item.id,
    list_id: item.listId,
    origin_url: item.originUrl,
    order_index: item.orderIndex,
    is_active: item.isActive,
    consumed_at: item.consumedAt,
    consumed_by_automation_id: item.consumedByAutomationId,
    consumed_by_group_id: item.consumedByGroupId,
    product_title: item.productTitle,
    product_price: item.productPrice,
    product_image_url: item.productImageUrl,
    metadata: item.metadata,
    created_at: item.createdAt,
    updated_at: item.updatedAt,
  };
}

// ============================================================
// Hook Implementation
// ============================================================

/**
 * Hook for ML link list operations via ConnectRPC.
 * Provides CRUD operations for Mercado Livre link lists and items.
 */
export function useMlLinksConnect(): UseMlLinksConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();
  const queryClient = useQueryClient();

  // ============================================================
  // List ML Link Lists Query
  // ============================================================

  const {
    data: listsData,
    isLoading: loadingLists,
    refetch: refetchListsQuery,
  } = useQuery({
    queryKey: ['ml-link-lists', 'list', user?.id],
    queryFn: async (): Promise<LocalMlLinkList[]> => {
      if (!user?.id) return [];
      const client = createClient(MlLinksService, transport);
      const response: ListMlLinkListsResponse = await client.listMlLinkLists(
        create(ListMlLinkListsRequestSchema, {})
      );
      return response.lists.map(fromProtoMlLinkList);
    },
    enabled: !!user?.id,
    staleTime: 30000,
  });

  const lists = listsData ?? [];

  const refetchLists = useCallback(async () => {
    await refetchListsQuery();
  }, [refetchListsQuery]);

  // ============================================================
  // List ML Link Lists Mutation
  // ============================================================

  const listListsMutation = useMutation({
    mutationFn: async (): Promise<LocalMlLinkList[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MlLinksService, transport);
      const response: ListMlLinkListsResponse = await client.listMlLinkLists(
        create(ListMlLinkListsRequestSchema, {})
      );

      return response.lists.map(fromProtoMlLinkList);
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
  // Get ML Link List
  // ============================================================

  const getListMutation = useMutation({
    mutationFn: async (id: string): Promise<LocalMlLinkList> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MlLinksService, transport);
      const response: GetMlLinkListResponse = await client.getMlLinkList(
        create(GetMlLinkListRequestSchema, { id })
      );

      if (!response.list) {
        throw new Error(t('pages.mlLinkLists.errors.notFound'));
      }

      return fromProtoMlLinkList(response.list);
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
  // Create ML Link List
  // ============================================================

  const createListMutation = useMutation({
    mutationFn: async (params: CreateMlLinkListParams): Promise<LocalMlLinkList> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MlLinksService, transport);
      const response: CreateMlLinkListResponse = await client.createMlLinkList(
        create(CreateMlLinkListRequestSchema, {
          name: params.name,
          description: params.description,
          isActive: params.is_active,
        })
      );

      if (!response.list) {
        throw new Error(t('pages.mlLinkLists.errors.createFailed'));
      }

      return fromProtoMlLinkList(response.list);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['ml-link-lists'] });
      toast({
        title: t('pages.mlLinkLists.success.created'),
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
  // Update ML Link List
  // ============================================================

  const updateListMutation = useMutation({
    mutationFn: async (params: UpdateMlLinkListParams): Promise<LocalMlLinkList> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MlLinksService, transport);
      const response: UpdateMlLinkListResponse = await client.updateMlLinkList(
        create(UpdateMlLinkListRequestSchema, {
          id: params.id,
          name: params.name,
          description: params.description,
          isActive: params.is_active,
        })
      );

      if (!response.list) {
        throw new Error(t('pages.mlLinkLists.errors.updateFailed'));
      }

      return fromProtoMlLinkList(response.list);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['ml-link-lists'] });
      toast({
        title: t('pages.mlLinkLists.success.updated'),
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
  // Delete ML Link List
  // ============================================================

  const deleteListMutation = useMutation({
    mutationFn: async (id: string): Promise<void> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MlLinksService, transport);
      await client.deleteMlLinkList(create(DeleteMlLinkListRequestSchema, { id }));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['ml-link-lists'] });
      toast({
        title: t('pages.mlLinkLists.success.deleted'),
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
  // List ML Link List Items
  // ============================================================

  const listItemsMutation = useMutation({
    mutationFn: async (listId: string): Promise<LocalMlLinkListItem[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MlLinksService, transport);
      const response: ListMlLinkListItemsResponse = await client.listMlLinkListItems(
        create(ListMlLinkListItemsRequestSchema, { listId })
      );

      return response.items.map(fromProtoMlLinkListItem);
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
  // List ML Link List Items (Paginated)
  // ============================================================

  const listItemsPaginatedMutation = useMutation({
    mutationFn: async (params: PaginatedItemsParams): Promise<PaginatedItemsResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MlLinksService, transport);
      const response: ListMlLinkListItemsResponse = await client.listMlLinkListItems(
        create(ListMlLinkListItemsRequestSchema, {
          listId: params.listId,
          pagination: create(PaginationRequestSchema, {
            pageSize: params.pageSize ?? 20,
            pageToken: params.pageToken ?? '',
          }),
        })
      );

      return {
        items: response.items.map(fromProtoMlLinkListItem),
        nextPageToken: response.pagination?.nextPageToken ?? '',
        totalCount: response.pagination?.totalCount ?? 0,
        hasMore: response.pagination?.hasMore ?? false,
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
  // Create ML Link List Item
  // ============================================================

  const createItemMutation = useMutation({
    mutationFn: async (params: CreateMlLinkListItemParams): Promise<LocalMlLinkListItem> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MlLinksService, transport);
      const response: CreateMlLinkListItemResponse = await client.createMlLinkListItem(
        create(CreateMlLinkListItemRequestSchema, {
          listId: params.list_id,
          originUrl: params.origin_url,
          orderIndex: params.order_index,
          isActive: params.is_active,
          productTitle: params.product_title,
          productPrice: params.product_price,
          productImageUrl: params.product_image_url,
          metadata: params.metadata,
        })
      );

      if (!response.item) {
        throw new Error(t('pages.mlLinkLists.errors.addLinkFailed'));
      }

      return fromProtoMlLinkListItem(response.item);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['ml-link-lists'] });
      toast({
        title: t('pages.mlLinkLists.success.linkAdded'),
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
  // Bulk Create ML Link List Items
  // ============================================================

  const bulkCreateItemsMutation = useMutation({
    mutationFn: async (params: {
      listId: string;
      originUrls: string[];
      baseOrderIndex?: number;
    }): Promise<{ createdCount: number; items: LocalMlLinkListItem[] }> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MlLinksService, transport);
      const response: BulkCreateMlLinkListItemsResponse = await client.bulkCreateMlLinkListItems(
        create(BulkCreateMlLinkListItemsRequestSchema, {
          listId: params.listId,
          originUrls: params.originUrls,
          baseOrderIndex: params.baseOrderIndex,
        })
      );

      return {
        createdCount: response.createdCount,
        skippedCount: response.skippedCount,
        skippedUrls: response.skippedUrls,
        items: response.items.map(fromProtoMlLinkListItem),
      };
    },
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['ml-link-lists'] });
      toast({
        title: t('pages.mlLinkLists.success.imported'),
        description: t('pages.mlLinkLists.success.importedMessage', {
          count: result.createdCount,
        }),
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
  // Update ML Link List Item
  // ============================================================

  const updateItemMutation = useMutation({
    mutationFn: async (params: UpdateMlLinkListItemParams): Promise<LocalMlLinkListItem> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MlLinksService, transport);
      const response: UpdateMlLinkListItemResponse = await client.updateMlLinkListItem(
        create(UpdateMlLinkListItemRequestSchema, {
          id: params.id,
          isActive: params.is_active,
          orderIndex: params.order_index,
        })
      );

      if (!response.item) {
        throw new Error(t('pages.mlLinkLists.errors.updateItemFailed'));
      }

      return fromProtoMlLinkListItem(response.item);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['ml-link-lists'] });
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
  // Delete ML Link List Item
  // ============================================================

  const deleteItemMutation = useMutation({
    mutationFn: async (id: string): Promise<void> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MlLinksService, transport);
      await client.deleteMlLinkListItem(create(DeleteMlLinkListItemRequestSchema, { id }));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['ml-link-lists'] });
      toast({
        title: t('pages.mlLinkLists.success.linkRemoved'),
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
  // Reset Consumed Items
  // ============================================================

  const resetConsumedMutation = useMutation({
    mutationFn: async (listId: string): Promise<number> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MlLinksService, transport);
      const response: ResetConsumedItemsResponse = await client.resetConsumedItems(
        create(ResetConsumedItemsRequestSchema, { listId })
      );

      return response.resetCount;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['ml-link-lists'] });
      toast({
        title: t('pages.mlLinkLists.success.sentReset'),
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
  // Reorder Items
  // ============================================================

  const reorderMutation = useMutation({
    mutationFn: async (params: {
      listId: string;
      orderMap: Record<string, number>;
    }): Promise<void> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MlLinksService, transport);
      await client.reorderItems(
        create(ReorderItemsRequestSchema, {
          listId: params.listId,
          orderMap: params.orderMap,
        })
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['ml-link-lists'] });
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

  const listMlLinkLists = useCallback(async (): Promise<LocalMlLinkList[]> => {
    return listListsMutation.mutateAsync();
  }, [listListsMutation]);

  const getMlLinkList = useCallback(
    async (id: string): Promise<LocalMlLinkList> => {
      return getListMutation.mutateAsync(id);
    },
    [getListMutation]
  );

  const createMlLinkList = useCallback(
    async (params: CreateMlLinkListParams): Promise<LocalMlLinkList> => {
      return createListMutation.mutateAsync(params);
    },
    [createListMutation]
  );

  const updateMlLinkList = useCallback(
    async (params: UpdateMlLinkListParams): Promise<LocalMlLinkList> => {
      return updateListMutation.mutateAsync(params);
    },
    [updateListMutation]
  );

  const deleteMlLinkList = useCallback(
    async (id: string): Promise<void> => {
      return deleteListMutation.mutateAsync(id);
    },
    [deleteListMutation]
  );

  const listMlLinkListItems = useCallback(
    async (listId: string): Promise<LocalMlLinkListItem[]> => {
      return listItemsMutation.mutateAsync(listId);
    },
    [listItemsMutation]
  );

  const listMlLinkListItemsPaginated = useCallback(
    async (params: PaginatedItemsParams): Promise<PaginatedItemsResult> => {
      return listItemsPaginatedMutation.mutateAsync(params);
    },
    [listItemsPaginatedMutation]
  );

  const createMlLinkListItem = useCallback(
    async (params: CreateMlLinkListItemParams): Promise<LocalMlLinkListItem> => {
      return createItemMutation.mutateAsync(params);
    },
    [createItemMutation]
  );

  const bulkCreateMlLinkListItems = useCallback(
    async (
      listId: string,
      originUrls: string[],
      baseOrderIndex?: number
    ): Promise<{
      createdCount: number;
      skippedCount: number;
      skippedUrls: string[];
      items: LocalMlLinkListItem[];
    }> => {
      return bulkCreateItemsMutation.mutateAsync({ listId, originUrls, baseOrderIndex });
    },
    [bulkCreateItemsMutation]
  );

  const updateMlLinkListItem = useCallback(
    async (params: UpdateMlLinkListItemParams): Promise<LocalMlLinkListItem> => {
      return updateItemMutation.mutateAsync(params);
    },
    [updateItemMutation]
  );

  const deleteMlLinkListItem = useCallback(
    async (id: string): Promise<void> => {
      return deleteItemMutation.mutateAsync(id);
    },
    [deleteItemMutation]
  );

  const resetConsumedItems = useCallback(
    async (listId: string): Promise<number> => {
      return resetConsumedMutation.mutateAsync(listId);
    },
    [resetConsumedMutation]
  );

  const reorderItems = useCallback(
    async (listId: string, orderMap: Record<string, number>): Promise<void> => {
      return reorderMutation.mutateAsync({ listId, orderMap });
    },
    [reorderMutation]
  );

  return {
    lists,
    loadingLists,
    refetchLists,

    listMlLinkLists,
    getMlLinkList,
    createMlLinkList,
    updateMlLinkList,
    deleteMlLinkList,

    listMlLinkListItems,
    listMlLinkListItemsPaginated,
    createMlLinkListItem,
    bulkCreateMlLinkListItems,
    updateMlLinkListItem,
    deleteMlLinkListItem,
    resetConsumedItems,
    reorderItems,

    isListingLists: listListsMutation.isPending,
    isGettingList: getListMutation.isPending,
    isCreatingList: createListMutation.isPending,
    isUpdatingList: updateListMutation.isPending,
    isDeletingList: deleteListMutation.isPending,
    isListingItems: listItemsMutation.isPending,
    isListingItemsPaginated: listItemsPaginatedMutation.isPending,
    isCreatingItem: createItemMutation.isPending,
    isBulkCreatingItems: bulkCreateItemsMutation.isPending,
    isUpdatingItem: updateItemMutation.isPending,
    isDeletingItem: deleteItemMutation.isPending,
    isResettingConsumed: resetConsumedMutation.isPending,
    isReordering: reorderMutation.isPending,
  };
}
