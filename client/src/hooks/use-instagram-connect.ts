import {
  DisconnectRequestSchema,
  GetAuthorizeUrlRequestSchema,
  GetStatusRequestSchema,
  type GetStatusResponse,
  InstagramService,
} from '@/gen/instagram/v1/instagram_pb';
import { captureHookError } from '@/lib/sentry';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

const STATUS_QUERY_KEY = ['instagram-status'];

/**
 * Hook para a integração de Story do Instagram: status da conexão, iniciar o
 * OAuth (redirect full-page) e desconectar. Espelha o padrão de
 * use-affiliate-credentials-connect.
 */
export function useInstagramConnect() {
  const transport = useTransport();
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  const { toast } = useToast();

  const statusQuery = useQuery<GetStatusResponse>({
    queryKey: STATUS_QUERY_KEY,
    queryFn: async () => {
      const client = createClient(InstagramService, transport);
      return client.getStatus(create(GetStatusRequestSchema, {}));
    },
    staleTime: 30_000,
  });

  const connect = useCallback(async () => {
    try {
      const client = createClient(InstagramService, transport);
      const res = await client.getAuthorizeUrl(create(GetAuthorizeUrlRequestSchema, {}));
      if (res.url) {
        window.location.href = res.url;
      }
    } catch (error: unknown) {
      captureHookError(error, 'useInstagramConnect', 'connect');
      toast({
        title: t('pages.instagram.errors.connectFailed'),
        variant: 'destructive',
      });
    }
  }, [transport, t, toast]);

  const disconnectMutation = useMutation({
    mutationFn: async () => {
      const client = createClient(InstagramService, transport);
      await client.disconnect(create(DisconnectRequestSchema, {}));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: STATUS_QUERY_KEY });
      toast({ title: t('pages.instagram.success.disconnected') });
    },
    onError: (error: unknown) => {
      captureHookError(error, 'useInstagramConnect', 'disconnect');
      toast({
        title: t('pages.instagram.errors.disconnectFailed'),
        variant: 'destructive',
      });
    },
  });

  return {
    status: statusQuery.data,
    loading: statusQuery.isLoading,
    connect,
    disconnect: disconnectMutation.mutate,
    disconnecting: disconnectMutation.isPending,
  };
}
