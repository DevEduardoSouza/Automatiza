import { useAuth } from '@/contexts';
import {
  MercadoLivreService,
  MlConnectRequestSchema,
  type MlConnectResponse,
  MlCreateLinkRequestSchema,
  type MlCreateLinkResponse,
  type MlLinkUrl,
  MlPreviewRequestSchema,
  type MlPreviewResponse,
  type MlProductPreview,
} from '@/gen/affiliates/v1/mercadolivre_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useMutation } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

/**
 * Local ML link URL interface.
 */
export interface LocalMlLinkUrl {
  longUrl?: string;
  shortUrl?: string;
}

/**
 * Local ML product preview interface.
 */
export interface LocalMlProductPreview {
  title?: string;
  price?: number;
  currency?: string;
  imageUrl?: string;
  sellerName?: string;
  condition?: string;
  soldQuantity?: number;
  ratingAverage?: number;
  reviewsTotal?: number;
  originalPrice?: number;
  discountPercent?: number;
  installmentsText?: string;
}

/**
 * Connect request parameters.
 */
export interface MlConnectParams {
  tag: string;
  cookie?: string;
  csrfToken?: string;
  autoGenerate?: boolean;
}

interface UseMercadoLivreConnectReturn {
  /** Connect/configure ML affiliate account */
  connect: (params: MlConnectParams) => Promise<{ success: boolean; message?: string }>;
  /** Create affiliate link for a product URL */
  createLink: (originUrl: string) => Promise<LocalMlLinkUrl[]>;
  /** Preview product information from URL */
  preview: (originUrl: string) => Promise<LocalMlProductPreview | null>;
  /** Whether connect is in progress */
  isConnecting: boolean;
  /** Whether link creation is in progress */
  isCreatingLink: boolean;
  /** Whether preview is in progress */
  isPreviewing: boolean;
}

/**
 * Converts proto MlLinkUrl to local format.
 */
function fromProtoLinkUrl(url: MlLinkUrl): LocalMlLinkUrl {
  return {
    longUrl: url.longUrl,
    shortUrl: url.shortUrl,
  };
}

/**
 * Converts proto MlProductPreview to local format.
 */
function fromProtoPreview(preview: MlProductPreview | undefined): LocalMlProductPreview | null {
  if (!preview) {
    return null;
  }
  return {
    title: preview.title,
    price: preview.price,
    currency: preview.currency,
    imageUrl: preview.imageUrl,
    sellerName: preview.sellerName,
    condition: preview.condition,
    soldQuantity: preview.soldQuantity,
    ratingAverage: preview.ratingAverage,
    reviewsTotal: preview.reviewsTotal,
    originalPrice: preview.originalPrice,
    discountPercent: preview.discountPercent,
    installmentsText: preview.installmentsText,
  };
}

/**
 * Hook for Mercado Livre affiliate operations via ConnectRPC.
 * Provides connect, create link, and preview functionality.
 */
export function useMercadoLivreConnect(): UseMercadoLivreConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();

  // Mutation for connecting ML account
  const connectMutation = useMutation({
    mutationFn: async (
      params: MlConnectParams
    ): Promise<{ success: boolean; message?: string }> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MercadoLivreService, transport);
      const response: MlConnectResponse = await client.connect(
        create(MlConnectRequestSchema, {
          userId: user.id,
          tag: params.tag,
          cookie: params.cookie,
          csrfToken: params.csrfToken,
          autoGenerate: params.autoGenerate,
        })
      );

      return {
        success: response.success,
        message: response.message,
      };
    },
    onSuccess: (data) => {
      if (data.success) {
        toast({
          title: t('common.success'),
          description: data.message ?? t('pages.affiliatePrograms.ml.connected'),
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

  // Mutation for creating affiliate link
  const createLinkMutation = useMutation({
    mutationFn: async (originUrl: string): Promise<LocalMlLinkUrl[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MercadoLivreService, transport);
      const response: MlCreateLinkResponse = await client.createLink(
        create(MlCreateLinkRequestSchema, {
          userId: user.id,
          originUrl,
        })
      );

      return response.urls.map(fromProtoLinkUrl);
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

  // Mutation for previewing product
  const previewMutation = useMutation({
    mutationFn: async (originUrl: string): Promise<LocalMlProductPreview | null> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(MercadoLivreService, transport);
      const response: MlPreviewResponse = await client.preview(
        create(MlPreviewRequestSchema, {
          userId: user.id,
          originUrl,
        })
      );

      return fromProtoPreview(response.preview);
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

  const connect = useCallback(
    async (params: MlConnectParams): Promise<{ success: boolean; message?: string }> => {
      return connectMutation.mutateAsync(params);
    },
    [connectMutation]
  );

  const createLink = useCallback(
    async (originUrl: string): Promise<LocalMlLinkUrl[]> => {
      return createLinkMutation.mutateAsync(originUrl);
    },
    [createLinkMutation]
  );

  const preview = useCallback(
    async (originUrl: string): Promise<LocalMlProductPreview | null> => {
      return previewMutation.mutateAsync(originUrl);
    },
    [previewMutation]
  );

  return {
    connect,
    createLink,
    preview,
    isConnecting: connectMutation.isPending,
    isCreatingLink: createLinkMutation.isPending,
    isPreviewing: previewMutation.isPending,
  };
}
