import { useAuth } from '@/contexts';
import { ListCategoriesRequestSchema, ShopeeService } from '@/gen/affiliates/v1/shopee_pb';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useQuery } from '@tanstack/react-query';

/**
 * Grupo curado de categorias oficiais da Shopee.
 *
 * ATENÇÃO: o MESMO id pode aparecer em mais de um grupo — o catálogo é um
 * mapeamento curado, não uma partição. Por isso a união de ids selecionados
 * NÃO reconstrói quais grupos o operador marcou, e a UI nunca tenta
 * mostrar nomes a partir dos ids persistidos.
 */
export interface ShopeeCategoryGroup {
  name: string;
  ids: number[];
}

const SHOPEE_CATEGORIES_QUERY_KEY = ['shopee-categories'];

/**
 * Catálogo de grupos de categoria da Shopee, usado para montar regras de
 * cupom restritas a categoria. O RPC é autenticado; a query só roda com
 * usuário logado.
 *
 * Falha aqui NÃO pode derrubar a página de regras: quem consome trata
 * `isError` mostrando um retry isolado e continua permitindo salvar uma
 * regra genérica.
 */
export function useShopeeCategories() {
  const { user } = useAuth();
  const transport = useTransport();

  const query = useQuery({
    queryKey: SHOPEE_CATEGORIES_QUERY_KEY,
    enabled: !!user?.id,
    // Catálogo estático no backend — não precisa refetch agressivo.
    staleTime: 60 * 60 * 1000,
    queryFn: async (): Promise<ShopeeCategoryGroup[]> => {
      const client = createClient(ShopeeService, transport);
      const resp = await client.listCategories(create(ListCategoriesRequestSchema, {}));
      return resp.groups.map((g) => ({
        name: g.name,
        // proto int64 chega como bigint; os ids da Shopee cabem em Int32,
        // então Number() é seguro (ver normalizeProductCategoryIDs no backend).
        ids: g.ids.map((id) => Number(id)),
      }));
    },
  });

  return {
    groups: query.data ?? [],
    isLoading: query.isLoading,
    isError: query.isError,
    refetch: query.refetch,
  };
}
