import { useAuth } from '@/contexts';
import type { CTA, CTAPlatform, CTATypeValue } from '@/entities/cta';
import {
  CTAType,
  CTAsService,
  CreateCTARequestSchema,
  type CreateCTAResponse,
  DeleteCTARequestSchema,
  GetCTARequestSchema,
  type GetCTAResponse,
  ListCTAsRequestSchema,
  type ListCTAsResponse,
  Platform,
  type CTA as ProtoCTA,
  UpdateCTARequestSchema,
  type UpdateCTAResponse,
} from '@/gen/ctas/v1/ctas_pb';
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

export interface CreateCTAParams {
  message: string;
  platform: CTAPlatform;
  cta_type: CTATypeValue;
}

export interface UpdateCTAParams {
  id: string;
  message?: string;
  platform?: CTAPlatform;
  cta_type?: CTATypeValue;
  is_active?: boolean;
}

// ============================================================
// Hook Return Type
// ============================================================

interface UseCTAsConnectReturn {
  ctas: CTA[];
  loading: boolean;
  refetch: () => Promise<void>;

  listCTAs: () => Promise<CTA[]>;
  getCTA: (id: string) => Promise<CTA>;
  createCTA: (params: CreateCTAParams) => Promise<CTA>;
  updateCTA: (params: UpdateCTAParams) => Promise<CTA>;
  deleteCTA: (id: string) => Promise<void>;

  isListingCTAs: boolean;
  isGettingCTA: boolean;
  isCreatingCTA: boolean;
  isUpdatingCTA: boolean;
  isDeletingCTA: boolean;
}

// ============================================================
// Converters
// ============================================================

function platformToLocal(p: Platform): CTAPlatform {
  switch (p) {
    case Platform.SHOPEE:
      return 'shopee';
    case Platform.MERCADO_LIVRE:
      return 'ml';
    default:
      return 'all';
  }
}

function localPlatformToProto(p: CTAPlatform): Platform {
  switch (p) {
    case 'shopee':
      return Platform.SHOPEE;
    case 'ml':
      return Platform.MERCADO_LIVRE;
    default:
      return Platform.ALL;
  }
}

function ctaTypeToLocal(t: CTAType): CTATypeValue {
  switch (t) {
    case CTAType.NORMAL:
      return 'normal';
    case CTAType.DISCOUNT:
      return 'discount';
    default:
      return 'all';
  }
}

function localCTATypeToProto(t: CTATypeValue): CTAType {
  switch (t) {
    case 'normal':
      return CTAType.NORMAL;
    case 'discount':
      return CTAType.DISCOUNT;
    default:
      return CTAType.ALL;
  }
}

function fromProtoCTA(cta: ProtoCTA): CTA {
  return {
    id: cta.id,
    user_id: cta.userId,
    message: cta.message,
    platform: platformToLocal(cta.platform),
    cta_type: ctaTypeToLocal(cta.ctaType),
    is_active: cta.isActive,
    created_at: cta.createdAt,
    updated_at: cta.updatedAt,
  };
}

// ============================================================
// Hook Implementation
// ============================================================

/**
 * Hook for CTA operations via ConnectRPC.
 * Provides CRUD operations for call-to-action texts.
 */
export function useCTAsConnect(): UseCTAsConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();
  const queryClient = useQueryClient();

  // ============================================================
  // List CTAs Query
  // ============================================================

  const {
    data: ctasData,
    isLoading: loading,
    refetch: refetchQuery,
  } = useQuery({
    queryKey: ['ctas', 'list', user?.id],
    queryFn: async (): Promise<CTA[]> => {
      if (!user?.id) return [];
      const client = createClient(CTAsService, transport);
      const response: ListCTAsResponse = await client.listCTAs(create(ListCTAsRequestSchema, {}));
      return response.ctas.map(fromProtoCTA);
    },
    enabled: !!user?.id,
    staleTime: 30000,
  });

  const ctas = ctasData ?? [];

  const refetch = useCallback(async () => {
    await refetchQuery();
  }, [refetchQuery]);

  // ============================================================
  // List CTAs Mutation
  // ============================================================

  const listMutation = useMutation({
    mutationFn: async (): Promise<CTA[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(CTAsService, transport);
      const response: ListCTAsResponse = await client.listCTAs(create(ListCTAsRequestSchema, {}));

      return response.ctas.map(fromProtoCTA);
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
  // Get CTA
  // ============================================================

  const getMutation = useMutation({
    mutationFn: async (id: string): Promise<CTA> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(CTAsService, transport);
      const response: GetCTAResponse = await client.getCTA(create(GetCTARequestSchema, { id }));

      if (!response.cta) {
        throw new Error(t('pages.ctas.errors.notFound'));
      }

      return fromProtoCTA(response.cta);
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
  // Create CTA
  // ============================================================

  const createMutation = useMutation({
    mutationFn: async (params: CreateCTAParams): Promise<CTA> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(CTAsService, transport);
      const response: CreateCTAResponse = await client.createCTA(
        create(CreateCTARequestSchema, {
          message: params.message,
          platform: localPlatformToProto(params.platform),
          ctaType: localCTATypeToProto(params.cta_type),
        })
      );

      if (!response.cta) {
        throw new Error(t('pages.ctas.errors.createFailed'));
      }

      return fromProtoCTA(response.cta);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['ctas'] });
      toast({
        title: t('pages.ctas.success.created'),
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
  // Update CTA
  // ============================================================

  const updateMutation = useMutation({
    mutationFn: async (params: UpdateCTAParams): Promise<CTA> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(CTAsService, transport);
      const response: UpdateCTAResponse = await client.updateCTA(
        create(UpdateCTARequestSchema, {
          id: params.id,
          message: params.message,
          platform: params.platform != null ? localPlatformToProto(params.platform) : undefined,
          ctaType: params.cta_type != null ? localCTATypeToProto(params.cta_type) : undefined,
          isActive: params.is_active,
        })
      );

      if (!response.cta) {
        throw new Error(t('pages.ctas.errors.updateFailed'));
      }

      return fromProtoCTA(response.cta);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['ctas'] });
      toast({
        title: t('pages.ctas.success.updated'),
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
  // Delete CTA
  // ============================================================

  const deleteMutation = useMutation({
    mutationFn: async (id: string): Promise<void> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(CTAsService, transport);
      await client.deleteCTA(create(DeleteCTARequestSchema, { id }));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['ctas'] });
      toast({
        title: t('pages.ctas.success.deleted'),
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

  const listCTAs = useCallback(async (): Promise<CTA[]> => {
    return listMutation.mutateAsync();
  }, [listMutation]);

  const getCTA = useCallback(
    async (id: string): Promise<CTA> => {
      return getMutation.mutateAsync(id);
    },
    [getMutation]
  );

  const createCTA = useCallback(
    async (params: CreateCTAParams): Promise<CTA> => {
      return createMutation.mutateAsync(params);
    },
    [createMutation]
  );

  const updateCTA = useCallback(
    async (params: UpdateCTAParams): Promise<CTA> => {
      return updateMutation.mutateAsync(params);
    },
    [updateMutation]
  );

  const deleteCTA = useCallback(
    async (id: string): Promise<void> => {
      return deleteMutation.mutateAsync(id);
    },
    [deleteMutation]
  );

  return {
    ctas,
    loading,
    refetch,

    listCTAs,
    getCTA,
    createCTA,
    updateCTA,
    deleteCTA,

    isListingCTAs: listMutation.isPending,
    isGettingCTA: getMutation.isPending,
    isCreatingCTA: createMutation.isPending,
    isUpdatingCTA: updateMutation.isPending,
    isDeletingCTA: deleteMutation.isPending,
  };
}
