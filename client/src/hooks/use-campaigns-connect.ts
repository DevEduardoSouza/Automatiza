import { useAuth } from '@/contexts';
import {
  type Campaign,
  type CampaignMessage,
  CampaignsService,
  CreateCampaignRequestSchema,
  type CreateCampaignResponse,
} from '@/gen/campaigns/v1/campaigns_pb';
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
 * Local campaign message interface.
 */
export interface LocalCampaignMessage {
  text: string;
  emoji?: string;
  targetAudience?: string;
}

/**
 * Local campaign interface.
 */
export interface LocalCampaign {
  title: string;
  description?: string;
  messages: LocalCampaignMessage[];
  hashtags: string[];
  suggestedTimes: string[];
}

/**
 * Result of campaign creation.
 */
export interface LocalCreateCampaignResult {
  success: boolean;
  campaign?: LocalCampaign;
  error?: string;
}

// ============================================================
// Hook Return Type
// ============================================================

interface UseCampaignsConnectReturn {
  createCampaign: (prompt: string) => Promise<LocalCreateCampaignResult>;
  isCreatingCampaign: boolean;
}

// ============================================================
// Converters
// ============================================================

function fromProtoCampaignMessage(message: CampaignMessage): LocalCampaignMessage {
  return {
    text: message.text,
    emoji: message.emoji,
    targetAudience: message.targetAudience,
  };
}

function fromProtoCampaign(campaign: Campaign): LocalCampaign {
  return {
    title: campaign.title,
    description: campaign.description,
    messages: campaign.messages.map(fromProtoCampaignMessage),
    hashtags: [...campaign.hashtags],
    suggestedTimes: [...campaign.suggestedTimes],
  };
}

function fromProtoCreateCampaignResponse(
  response: CreateCampaignResponse
): LocalCreateCampaignResult {
  return {
    success: response.success,
    campaign: response.campaign ? fromProtoCampaign(response.campaign) : undefined,
    error: response.error,
  };
}

// ============================================================
// Hook Implementation
// ============================================================

/**
 * Hook for AI-powered campaign generation via ConnectRPC.
 */
export function useCampaignsConnect(): UseCampaignsConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();

  const createCampaignMutation = useMutation({
    mutationFn: async (prompt: string): Promise<LocalCreateCampaignResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(CampaignsService, transport);
      const response: CreateCampaignResponse = await client.createCampaign(
        create(CreateCampaignRequestSchema, { prompt })
      );

      return fromProtoCreateCampaignResponse(response);
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

  const createCampaign = useCallback(
    async (prompt: string): Promise<LocalCreateCampaignResult> => {
      return createCampaignMutation.mutateAsync(prompt);
    },
    [createCampaignMutation]
  );

  return {
    createCampaign,
    isCreatingCampaign: createCampaignMutation.isPending,
  };
}
