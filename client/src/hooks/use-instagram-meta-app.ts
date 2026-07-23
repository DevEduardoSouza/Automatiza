import {
  DeleteMetaAppRequestSchema,
  GetMetaAppRequestSchema,
  InstagramService,
  type MetaApp,
  SaveMetaAppRequestSchema,
} from '@/gen/instagram/v1/instagram_pb';
import { captureHookError } from '@/lib/sentry';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

const META_APP_QUERY_KEY = ['instagram-meta-app'];
const STATUS_QUERY_KEY = ['instagram-status'];

/**
 * Hook do aplicativo da Meta que o PRÓPRIO cliente registra.
 *
 * O segredo do aplicativo é write-only: o servidor nunca o devolve, então a
 * tela só sabe que existe um (`hasSecret`). Salvar com o campo de segredo vazio
 * mantém o que já está guardado — sem isso, trocar só o ID do aplicativo
 * obrigaria o cliente a buscar o segredo na Meta de novo.
 */
export function useInstagramMetaApp() {
  const transport = useTransport();
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  const { toast } = useToast();

  const appQuery = useQuery<MetaApp | undefined>({
    queryKey: META_APP_QUERY_KEY,
    queryFn: async () => {
      const client = createClient(InstagramService, transport);
      const res = await client.getMetaApp(create(GetMetaAppRequestSchema, {}));
      return res.app;
    },
    staleTime: 30_000,
  });

  const saveMutation = useMutation({
    mutationFn: async (input: { appId: string; appSecret: string }) => {
      const client = createClient(InstagramService, transport);
      return client.saveMetaApp(create(SaveMetaAppRequestSchema, input));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: META_APP_QUERY_KEY });
      // O status carrega hasMetaApp, que muda o passo mostrado na aba Conta.
      queryClient.invalidateQueries({ queryKey: STATUS_QUERY_KEY });
      toast({ title: t('pages.instagram.app.saved') });
    },
    onError: (error: unknown) => {
      captureHookError(error, 'useInstagramMetaApp', 'save');
      toast({ title: t('pages.instagram.app.saveFailed'), variant: 'destructive' });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      const client = createClient(InstagramService, transport);
      await client.deleteMetaApp(create(DeleteMetaAppRequestSchema, {}));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: META_APP_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: STATUS_QUERY_KEY });
      toast({ title: t('pages.instagram.app.removed') });
    },
    onError: (error: unknown) => {
      captureHookError(error, 'useInstagramMetaApp', 'delete');
      toast({ title: t('pages.instagram.app.removeFailed'), variant: 'destructive' });
    },
  });

  return {
    app: appQuery.data,
    loading: appQuery.isLoading,
    save: saveMutation.mutate,
    saving: saveMutation.isPending,
    remove: deleteMutation.mutate,
    removing: deleteMutation.isPending,
  };
}
