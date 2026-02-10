import { useAuth } from '@/contexts';
import {
  GenerateOfferImageRequestSchema,
  type GenerateOfferImageResponse,
  GenerateOfferPostImageRequestSchema,
  type GenerateOfferPostImageResponse,
  ImagesService,
  PostImageLayout,
  PostImageTemplate,
} from '@/gen/images/v1/images_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useMutation } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

// ============================================================
// Local Types
// ============================================================

/**
 * Parameters for generating an offer image.
 */
export interface GenerateOfferImageParams {
  title: string;
  price: number;
  discount?: number;
  imageUrl?: string;
  store?: string;
}

/**
 * Result of offer image generation.
 */
export interface LocalGenerateOfferImageResult {
  success: boolean;
  imageUrl?: string;
  imageBase64?: string;
  error?: string;
}

/**
 * Layout types for post images.
 */
export type LocalPostImageLayout = 'standard' | 'compact' | 'wide';

/**
 * Template types for post images.
 */
export type LocalPostImageTemplate = 'default' | 'promotional' | 'flash_sale';

/**
 * Parameters for generating an offer post image.
 */
export interface GenerateOfferPostImageParams {
  productImageUrl: string;
  topText: string;
  bottomText?: string;
  backgroundColor?: string;
  layout?: LocalPostImageLayout;
  template?: LocalPostImageTemplate;
}

/**
 * Result of offer post image generation.
 */
export interface LocalGenerateOfferPostImageResult {
  success: boolean;
  imageUrl?: string;
  imageBase64?: string;
  error?: string;
}

// ============================================================
// Hook Return Type
// ============================================================

interface UseImagesConnectReturn {
  generateOfferImage: (params: GenerateOfferImageParams) => Promise<LocalGenerateOfferImageResult>;
  generateOfferPostImage: (
    params: GenerateOfferPostImageParams
  ) => Promise<LocalGenerateOfferPostImageResult>;
  isGeneratingOfferImage: boolean;
  isGeneratingOfferPostImage: boolean;
}

// ============================================================
// Converters
// ============================================================

function toProtoLayout(layout: LocalPostImageLayout | undefined): PostImageLayout {
  if (!layout) return PostImageLayout.UNSPECIFIED;
  const map: Record<LocalPostImageLayout, PostImageLayout> = {
    standard: PostImageLayout.STANDARD,
    compact: PostImageLayout.COMPACT,
    wide: PostImageLayout.WIDE,
  };
  return map[layout] ?? PostImageLayout.UNSPECIFIED;
}

function toProtoTemplate(template: LocalPostImageTemplate | undefined): PostImageTemplate {
  if (!template) return PostImageTemplate.UNSPECIFIED;
  const map: Record<LocalPostImageTemplate, PostImageTemplate> = {
    default: PostImageTemplate.DEFAULT,
    promotional: PostImageTemplate.PROMOTIONAL,
    flash_sale: PostImageTemplate.FLASH_SALE,
  };
  return map[template] ?? PostImageTemplate.UNSPECIFIED;
}

function fromProtoOfferImageResponse(
  response: GenerateOfferImageResponse
): LocalGenerateOfferImageResult {
  return {
    success: response.success,
    imageUrl: response.imageUrl,
    imageBase64: response.imageBase64,
    error: response.error,
  };
}

function fromProtoOfferPostImageResponse(
  response: GenerateOfferPostImageResponse
): LocalGenerateOfferPostImageResult {
  return {
    success: response.success,
    imageUrl: response.imageUrl,
    imageBase64: response.imageBase64,
    error: response.error,
  };
}

// ============================================================
// Hook Implementation
// ============================================================

/**
 * Hook for image generation operations via ConnectRPC.
 */
export function useImagesConnect(): UseImagesConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();

  // ============================================================
  // Generate Offer Image
  // ============================================================

  const generateOfferImageMutation = useMutation({
    mutationFn: async (
      params: GenerateOfferImageParams
    ): Promise<LocalGenerateOfferImageResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(ImagesService, transport);
      const response: GenerateOfferImageResponse = await client.generateOfferImage(
        create(GenerateOfferImageRequestSchema, {
          title: params.title,
          price: params.price,
          discount: params.discount,
          imageUrl: params.imageUrl,
          store: params.store,
        })
      );

      return fromProtoOfferImageResponse(response);
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
  // Generate Offer Post Image
  // ============================================================

  const generateOfferPostImageMutation = useMutation({
    mutationFn: async (
      params: GenerateOfferPostImageParams
    ): Promise<LocalGenerateOfferPostImageResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(ImagesService, transport);
      const response: GenerateOfferPostImageResponse = await client.generateOfferPostImage(
        create(GenerateOfferPostImageRequestSchema, {
          productImageUrl: params.productImageUrl,
          topText: params.topText,
          bottomText: params.bottomText,
          backgroundColor: params.backgroundColor,
          layout: toProtoLayout(params.layout),
          template: toProtoTemplate(params.template),
        })
      );

      return fromProtoOfferPostImageResponse(response);
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

  const generateOfferImage = useCallback(
    async (params: GenerateOfferImageParams): Promise<LocalGenerateOfferImageResult> => {
      return generateOfferImageMutation.mutateAsync(params);
    },
    [generateOfferImageMutation]
  );

  const generateOfferPostImage = useCallback(
    async (params: GenerateOfferPostImageParams): Promise<LocalGenerateOfferPostImageResult> => {
      return generateOfferPostImageMutation.mutateAsync(params);
    },
    [generateOfferPostImageMutation]
  );

  return {
    generateOfferImage,
    generateOfferPostImage,
    isGeneratingOfferImage: generateOfferImageMutation.isPending,
    isGeneratingOfferPostImage: generateOfferPostImageMutation.isPending,
  };
}
