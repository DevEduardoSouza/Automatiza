import { useAuth } from '@/contexts';
import {
  AiToolsService,
  AnalyzeEngagementRequestSchema,
  type AnalyzeEngagementResponse,
  AnalyzeTrendsRequestSchema,
  type AnalyzeTrendsResponse,
  type EngagementMetrics,
  EnhanceMessageRequestSchema,
  type EnhanceMessageResponse,
  type EnhancedMessage,
  SuggestOfferTextRequestSchema,
  type SuggestOfferTextResponse,
  type TrendItem,
} from '@/gen/ai/v1/ai_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useMutation } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

/**
 * Local engagement metrics interface.
 */
export interface LocalEngagementMetrics {
  totalMessages: number;
  activeMembers: number;
  engagementRate: number;
  bestTimeToPost?: string;
}

/**
 * Local engagement analysis result.
 */
export interface LocalEngagementAnalysis {
  metrics: LocalEngagementMetrics;
  summary?: string;
}

/**
 * Local trend item interface.
 */
export interface LocalTrendItem {
  name: string;
  score: number;
  description?: string;
}

/**
 * Local trends analysis result.
 */
export interface LocalTrendsAnalysis {
  trends: LocalTrendItem[];
  analysis?: string;
}

/**
 * Parameters for analyzing trends.
 */
export interface AnalyzeTrendsParams {
  platform: string;
  category: string;
}

/**
 * Parameters for enhancing messages.
 */
export interface EnhanceMessageParams {
  productLinks: string;
  copyStyle?: string;
  affiliateLink?: string;
}

/**
 * Parameters for suggesting offer text.
 */
export interface SuggestOfferTextParams {
  category?: string;
  store?: string;
  mode?: string;
}

interface UseAiToolsConnectReturn {
  /** Analyze engagement for a WhatsApp group */
  analyzeEngagement: (groupId: string) => Promise<LocalEngagementAnalysis | null>;
  /** Analyze product trends on a platform */
  analyzeTrends: (params: AnalyzeTrendsParams) => Promise<LocalTrendsAnalysis | null>;
  /** Enhance promotional messages using AI */
  enhanceMessage: (params: EnhanceMessageParams) => Promise<string[]>;
  /** Suggest offer text based on category and store */
  suggestOfferText: (params: SuggestOfferTextParams) => Promise<string[]>;
  /** Whether engagement analysis is in progress */
  isAnalyzingEngagement: boolean;
  /** Whether trends analysis is in progress */
  isAnalyzingTrends: boolean;
  /** Whether message enhancement is in progress */
  isEnhancing: boolean;
  /** Whether offer text suggestion is in progress */
  isSuggesting: boolean;
}

/**
 * Converts proto EngagementMetrics to local format.
 */
function fromProtoMetrics(metrics: EngagementMetrics | undefined): LocalEngagementMetrics | null {
  if (!metrics) {
    return null;
  }
  return {
    totalMessages: metrics.totalMessages,
    activeMembers: metrics.activeMembers,
    engagementRate: metrics.engagementRate,
    bestTimeToPost: metrics.bestTimeToPost,
  };
}

/**
 * Converts proto AnalyzeEngagementResponse to local format.
 */
function fromProtoEngagementResponse(
  response: AnalyzeEngagementResponse | undefined
): LocalEngagementAnalysis | null {
  if (!response) {
    return null;
  }
  const metrics = fromProtoMetrics(response.metrics);
  if (!metrics) {
    return null;
  }
  return {
    metrics,
    summary: response.summary,
  };
}

/**
 * Converts proto TrendItem to local format.
 */
function fromProtoTrendItem(item: TrendItem): LocalTrendItem {
  return {
    name: item.name,
    score: item.score,
    description: item.description,
  };
}

/**
 * Converts proto AnalyzeTrendsResponse to local format.
 */
function fromProtoTrendsResponse(
  response: AnalyzeTrendsResponse | undefined
): LocalTrendsAnalysis | null {
  if (!response) {
    return null;
  }
  return {
    trends: response.trends.map(fromProtoTrendItem),
    analysis: response.analysis,
  };
}

/**
 * Converts proto EnhanceMessageResponse to string array.
 */
function fromProtoEnhanceResponse(response: EnhanceMessageResponse | undefined): string[] {
  if (!response?.messages) {
    return [];
  }
  return response.messages.map((msg: EnhancedMessage) => msg.message);
}

/**
 * Converts proto SuggestOfferTextResponse to string array.
 */
function fromProtoSuggestResponse(response: SuggestOfferTextResponse | undefined): string[] {
  if (!response?.texts) {
    return [];
  }
  return response.texts;
}

/**
 * Hook for AI tools operations via ConnectRPC.
 * Provides engagement analysis, trends analysis, message enhancement, and offer text suggestion.
 */
export function useAiToolsConnect(): UseAiToolsConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();

  // Mutation for analyzing engagement
  const analyzeEngagementMutation = useMutation({
    mutationFn: async (groupId: string): Promise<LocalEngagementAnalysis | null> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AiToolsService, transport);
      const response: AnalyzeEngagementResponse = await client.analyzeEngagement(
        create(AnalyzeEngagementRequestSchema, {
          groupId,
        })
      );

      return fromProtoEngagementResponse(response);
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

  // Mutation for analyzing trends
  const analyzeTrendsMutation = useMutation({
    mutationFn: async (params: AnalyzeTrendsParams): Promise<LocalTrendsAnalysis | null> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AiToolsService, transport);
      const response: AnalyzeTrendsResponse = await client.analyzeTrends(
        create(AnalyzeTrendsRequestSchema, {
          platform: params.platform,
          category: params.category,
        })
      );

      return fromProtoTrendsResponse(response);
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

  // Mutation for enhancing messages
  const enhanceMessageMutation = useMutation({
    mutationFn: async (params: EnhanceMessageParams): Promise<string[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AiToolsService, transport);
      const response: EnhanceMessageResponse = await client.enhanceMessage(
        create(EnhanceMessageRequestSchema, {
          productLinks: params.productLinks,
          copyStyle: params.copyStyle,
          affiliateLink: params.affiliateLink,
        })
      );

      return fromProtoEnhanceResponse(response);
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

  // Mutation for suggesting offer text
  const suggestOfferTextMutation = useMutation({
    mutationFn: async (params: SuggestOfferTextParams): Promise<string[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(AiToolsService, transport);
      const response: SuggestOfferTextResponse = await client.suggestOfferText(
        create(SuggestOfferTextRequestSchema, {
          category: params.category ?? undefined,
          store: params.store ?? undefined,
          mode: params.mode,
        })
      );

      return fromProtoSuggestResponse(response);
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

  const analyzeEngagement = useCallback(
    async (groupId: string): Promise<LocalEngagementAnalysis | null> => {
      return analyzeEngagementMutation.mutateAsync(groupId);
    },
    [analyzeEngagementMutation]
  );

  const analyzeTrends = useCallback(
    async (params: AnalyzeTrendsParams): Promise<LocalTrendsAnalysis | null> => {
      return analyzeTrendsMutation.mutateAsync(params);
    },
    [analyzeTrendsMutation]
  );

  const enhanceMessage = useCallback(
    async (params: EnhanceMessageParams): Promise<string[]> => {
      return enhanceMessageMutation.mutateAsync(params);
    },
    [enhanceMessageMutation]
  );

  const suggestOfferText = useCallback(
    async (params: SuggestOfferTextParams): Promise<string[]> => {
      return suggestOfferTextMutation.mutateAsync(params);
    },
    [suggestOfferTextMutation]
  );

  return {
    analyzeEngagement,
    analyzeTrends,
    enhanceMessage,
    suggestOfferText,
    isAnalyzingEngagement: analyzeEngagementMutation.isPending,
    isAnalyzingTrends: analyzeTrendsMutation.isPending,
    isEnhancing: enhanceMessageMutation.isPending,
    isSuggesting: suggestOfferTextMutation.isPending,
  };
}
