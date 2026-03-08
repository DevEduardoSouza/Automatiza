import { useAuth } from '@/contexts';
import {
  FilterType,
  GenerateAffiliateLinkRequestSchema,
  type GenerateAffiliateLinkResponse,
  SearchOffersRequestSchema,
  type SearchOffersResponse,
  type ShopeeOffer,
  ShopeeService,
  SortBy,
} from '@/gen/affiliates/v1/shopee_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useMutation } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

/**
 * Local offer interface matching UI expectations.
 */
export interface LocalShopeeOffer {
  productUrl: string;
  title: string;
  originalPrice: number;
  finalPrice: number;
  discountPercent: number;
  imageUrl?: string;
  storeName?: string;
  commissionRate?: number;
}

/**
 * Search parameters for Shopee offers.
 */
export interface ShopeeSearchParams {
  keyword?: string;
  categories?: string[];
  minPrice?: number;
  maxPrice?: number;
  minDiscount?: number;
  sortBy?: 'commission' | 'price_low' | 'price_high' | 'sales' | 'discount' | 'saving';
  limit?: number;
  filterType?: 'light' | 'heavy';
  useCategoryIds?: boolean;
}

interface UseShopeeConnectReturn {
  /** Search for Shopee offers */
  searchOffers: (params: ShopeeSearchParams) => Promise<LocalShopeeOffer[]>;
  /** Generate affiliate link for a product */
  generateAffiliateLink: (productUrl: string) => Promise<string>;
  /** Whether search is in progress */
  isSearching: boolean;
  /** Whether link generation is in progress */
  isGenerating: boolean;
}

/**
 * Maps string sortBy to proto SortBy enum.
 */
function mapSortBy(sortBy?: string): SortBy {
  switch (sortBy) {
    case 'commission':
      return SortBy.COMMISSION;
    case 'price_low':
      return SortBy.PRICE_LOW;
    case 'price_high':
      return SortBy.PRICE_HIGH;
    case 'sales':
      return SortBy.SALES;
    case 'discount':
      return SortBy.DISCOUNT;
    case 'saving':
      return SortBy.SAVING;
    default:
      return SortBy.UNSPECIFIED;
  }
}

/**
 * Maps string filterType to proto FilterType enum.
 */
function mapFilterType(filterType?: string): FilterType {
  switch (filterType) {
    case 'light':
      return FilterType.LIGHT;
    case 'heavy':
      return FilterType.HEAVY;
    default:
      return FilterType.UNSPECIFIED;
  }
}

/**
 * Converts proto ShopeeOffer to local format.
 */
function fromProtoOffer(offer: ShopeeOffer): LocalShopeeOffer {
  return {
    productUrl: offer.productUrl,
    title: offer.title,
    originalPrice: offer.originalPrice,
    finalPrice: offer.finalPrice,
    discountPercent: offer.discountPercent,
    imageUrl: offer.imageUrl,
    storeName: offer.storeName,
    commissionRate: offer.commissionRate,
  };
}

/**
 * Converts proto SearchOffersResponse to local offers array.
 */
function fromProtoSearchResponse(response: SearchOffersResponse | undefined): LocalShopeeOffer[] {
  if (!response?.offers) {
    return [];
  }
  return response.offers.map(fromProtoOffer);
}

/**
 * Hook for Shopee affiliate operations via ConnectRPC.
 * Provides search offers and affiliate link generation.
 */
export function useShopeeConnect(): UseShopeeConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();

  // Mutation for searching offers
  const searchMutation = useMutation({
    mutationFn: async (params: ShopeeSearchParams): Promise<LocalShopeeOffer[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(ShopeeService, transport);
      const response = await client.searchOffers(
        create(SearchOffersRequestSchema, {
          userId: user.id,
          keyword: params.keyword,
          categories: params.categories ?? [],
          minPrice: params.minPrice,
          maxPrice: params.maxPrice,
          minDiscount: params.minDiscount,
          sortBy: mapSortBy(params.sortBy),
          limit: params.limit,
          filterType: mapFilterType(params.filterType),
          useCategoryIds: params.useCategoryIds,
        })
      );

      return fromProtoSearchResponse(response);
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

  // Mutation for generating affiliate link
  const generateLinkMutation = useMutation({
    mutationFn: async (productUrl: string): Promise<GenerateAffiliateLinkResponse> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(ShopeeService, transport);
      const response: GenerateAffiliateLinkResponse = await client.generateAffiliateLink(
        create(GenerateAffiliateLinkRequestSchema, {
          productUrl,
          userId: user.id,
        })
      );

      return response;
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

  const searchOffers = useCallback(
    async (params: ShopeeSearchParams): Promise<LocalShopeeOffer[]> => {
      return searchMutation.mutateAsync(params);
    },
    [searchMutation]
  );

  const generateAffiliateLink = useCallback(
    async (productUrl: string): Promise<string> => {
      const resp = await generateLinkMutation.mutateAsync(productUrl);
      return resp.affiliateUrl;
    },
    [generateLinkMutation]
  );

  const getProductPreview = useCallback(
    async (productUrl: string) => {
      const resp = await generateLinkMutation.mutateAsync(productUrl);
      return {
        title: resp.productTitle ?? null,
        price: resp.productPrice ?? null,
        imageUrl: resp.productImageUrl ?? null,
      };
    },
    [generateLinkMutation]
  );

  return {
    searchOffers,
    generateAffiliateLink,
    getProductPreview,
    isSearching: searchMutation.isPending,
    isGenerating: generateLinkMutation.isPending,
  };
}
