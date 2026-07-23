import {
  type AIConfig,
  GetAIConfigRequestSchema,
  GetActivityRequestSchema,
  InstagramService,
  SaveAIConfigRequestSchema,
} from '@/gen/instagram/v1/instagram_pb';
import { captureHookError } from '@/lib/sentry';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

const AI_QUERY_KEY = ['instagram-ai-config'];
const ACTIVITY_QUERY_KEY = ['instagram-activity'];

/** Configuração da IA que responde no Instagram, e o histórico do que chegou. */
export function useInstagramAI() {
  const transport = useTransport();
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  const { toast } = useToast();

  const configQuery = useQuery({
    queryKey: AI_QUERY_KEY,
    queryFn: async () => {
      const client = createClient(InstagramService, transport);
      return client.getAIConfig(create(GetAIConfigRequestSchema, {}));
    },
    staleTime: 30_000,
  });

  const activityQuery = useQuery({
    queryKey: ACTIVITY_QUERY_KEY,
    queryFn: async () => {
      const client = createClient(InstagramService, transport);
      const res = await client.getActivity(create(GetActivityRequestSchema, { limit: 30 }));
      return res.items;
    },
    staleTime: 15_000,
  });

  const saveMutation = useMutation({
    mutationFn: async (config: Partial<AIConfig>) => {
      const client = createClient(InstagramService, transport);
      return client.saveAIConfig(create(SaveAIConfigRequestSchema, { config: config as AIConfig }));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: AI_QUERY_KEY });
      toast({ title: t('pages.instagram.ai.saved') });
    },
    onError: (error: unknown) => {
      captureHookError(error, 'useInstagramAI', 'save');
      toast({ title: t('pages.instagram.ai.saveFailed'), variant: 'destructive' });
    },
  });

  return {
    config: configQuery.data?.config,
    aiAvailable: configQuery.data?.aiAvailable ?? false,
    dailyCap: configQuery.data?.dailyCap ?? 0,
    loading: configQuery.isLoading,
    activity: activityQuery.data ?? [],
    activityLoading: activityQuery.isLoading,
    save: saveMutation.mutate,
    saving: saveMutation.isPending,
  };
}
