import { useAuth } from '@/contexts';
import {
  ProcessKiwifyWebhookRequestSchema,
  type ProcessKiwifyWebhookResponse,
  SimulateKiwifyWebhookRequestSchema,
  type SimulateKiwifyWebhookResponse,
  WebhooksService,
} from '@/gen/webhooks/v1/webhooks_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create, fromJson } from '@bufbuild/protobuf';
import { StructSchema } from '@bufbuild/protobuf/wkt';
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
 * Result of processing a Kiwify webhook.
 */
export interface LocalProcessKiwifyWebhookResult {
  success: boolean;
  message?: string;
  eventType?: string;
  error?: string;
}

/**
 * Parameters for simulating a Kiwify webhook.
 */
export interface SimulateKiwifyWebhookParams {
  email: string;
  event: string;
  product?: string;
  token?: string;
}

/**
 * Result of simulating a Kiwify webhook.
 */
export interface LocalSimulateKiwifyWebhookResult {
  success: boolean;
  message?: string;
  eventDetails?: string;
  error?: string;
}

// ============================================================
// Hook Return Type
// ============================================================

interface UseWebhooksConnectReturn {
  processKiwifyWebhook: (
    webhookData: Record<string, unknown>
  ) => Promise<LocalProcessKiwifyWebhookResult>;
  simulateKiwifyWebhook: (
    params: SimulateKiwifyWebhookParams
  ) => Promise<LocalSimulateKiwifyWebhookResult>;
  isProcessingKiwifyWebhook: boolean;
  isSimulatingKiwifyWebhook: boolean;
}

// ============================================================
// Converters
// ============================================================

function fromProtoProcessKiwifyWebhookResponse(
  response: ProcessKiwifyWebhookResponse
): LocalProcessKiwifyWebhookResult {
  return {
    success: response.success,
    message: response.message,
    eventType: response.eventType,
    error: response.error,
  };
}

function fromProtoSimulateKiwifyWebhookResponse(
  response: SimulateKiwifyWebhookResponse
): LocalSimulateKiwifyWebhookResult {
  return {
    success: response.success,
    message: response.message,
    eventDetails: response.eventDetails,
    error: response.error,
  };
}

// ============================================================
// Hook Implementation
// ============================================================

/**
 * Hook for webhook operations via ConnectRPC.
 */
export function useWebhooksConnect(): UseWebhooksConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();

  // ============================================================
  // Process Kiwify Webhook
  // ============================================================

  const processKiwifyWebhookMutation = useMutation({
    mutationFn: async (
      webhookData: Record<string, unknown>
    ): Promise<LocalProcessKiwifyWebhookResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WebhooksService, transport);

      // Convert Record to google.protobuf.Struct
      const webhookStruct = fromJson(StructSchema, webhookData);

      const response: ProcessKiwifyWebhookResponse = await client.processKiwifyWebhook(
        create(ProcessKiwifyWebhookRequestSchema, {
          webhookData: webhookStruct,
        })
      );

      return fromProtoProcessKiwifyWebhookResponse(response);
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
  // Simulate Kiwify Webhook
  // ============================================================

  const simulateKiwifyWebhookMutation = useMutation({
    mutationFn: async (
      params: SimulateKiwifyWebhookParams
    ): Promise<LocalSimulateKiwifyWebhookResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WebhooksService, transport);
      const response: SimulateKiwifyWebhookResponse = await client.simulateKiwifyWebhook(
        create(SimulateKiwifyWebhookRequestSchema, {
          email: params.email,
          event: params.event,
          product: params.product,
          token: params.token,
        })
      );

      return fromProtoSimulateKiwifyWebhookResponse(response);
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

  const processKiwifyWebhook = useCallback(
    async (webhookData: Record<string, unknown>): Promise<LocalProcessKiwifyWebhookResult> => {
      return processKiwifyWebhookMutation.mutateAsync(webhookData);
    },
    [processKiwifyWebhookMutation]
  );

  const simulateKiwifyWebhook = useCallback(
    async (params: SimulateKiwifyWebhookParams): Promise<LocalSimulateKiwifyWebhookResult> => {
      return simulateKiwifyWebhookMutation.mutateAsync(params);
    },
    [simulateKiwifyWebhookMutation]
  );

  return {
    processKiwifyWebhook,
    simulateKiwifyWebhook,
    isProcessingKiwifyWebhook: processKiwifyWebhookMutation.isPending,
    isSimulatingKiwifyWebhook: simulateKiwifyWebhookMutation.isPending,
  };
}
