import { useAuth } from '@/contexts';
import {
  CouponsService,
  CreateCouponRuleRequestSchema,
  DeleteCouponRuleRequestSchema,
  DiscountType,
  ListCouponRulesRequestSchema,
  UpdateCouponRuleRequestSchema,
  type CouponRule as ProtoCouponRule,
} from '@/gen/coupons/v1/coupons_pb';
import { useToast } from '@/hooks/use-toast';
import { useTransport } from '@connectrpc/connect-query';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';

/**
 * UI-friendly representation of a coupon rule. snake_case mirrors the
 * pattern used by the rest of the dashboard (e.g. use-coupons), even
 * though the proto is camelCase, so existing form components can plug
 * in without conversion.
 *
 * As of migration 048 the rule is standalone — it carries its own
 * code/message/discount fields. coupon_id remains as a legacy pointer
 * for existing rows; new rules leave it empty.
 */
export interface LocalCouponRule {
  id: string;
  user_id: string;
  coupon_id: string;
  platform: string;
  period_start: string | null;
  period_end: string | null;
  min_price: number;
  priority: number;
  is_active: boolean;
  official_only: boolean;
  code: string | null;
  message: string | null;
  discount_type: 'fixed' | 'percent' | null;
  discount_value: number | null;
  max_discount: number | null;
  /**
   * IDs oficiais de categoria da Shopee. Vazio = regra genérica (vale para
   * qualquer categoria). Só existe em platform === 'shopee'.
   */
  product_category_ids: number[];
  created_at: string;
  updated_at: string;
}

export interface CreateCouponRuleParams {
  platform: string;
  period_start?: string | null;
  period_end?: string | null;
  min_price?: number;
  priority?: number;
  is_active?: boolean;
  official_only?: boolean;
  code?: string | null;
  message?: string | null;
  discount_type?: 'fixed' | 'percent' | null;
  discount_value?: number | null;
  max_discount?: number | null;
  product_category_ids?: number[];
}

export interface UpdateCouponRuleParams {
  id: string;
  platform?: string;
  // empty string explicitly clears (writes NULL); undefined leaves alone
  period_start?: string;
  period_end?: string;
  min_price?: number;
  priority?: number;
  is_active?: boolean;
  official_only?: boolean;
  code?: string;
  message?: string;
  discount_type?: 'fixed' | 'percent' | null;
  discount_value?: number;
  max_discount?: number;
  /**
   * Lista não vazia substitui as categorias. Lista vazia/undefined SEM
   * clear_product_category_ids não toca na coluna — para limpar é preciso
   * mandar clear_product_category_ids=true (e a lista vazia).
   */
  product_category_ids?: number[];
  clear_product_category_ids?: boolean;
}

/** Teto do Int32 da Shopee — mesmo limite validado no backend. */
const MAX_SHOPEE_CATEGORY_ID = 2147483647;
/** Máximo de categorias por regra — espelha maxProductCategoryIDs no backend. */
const MAX_CATEGORY_IDS = 512;

/**
 * Sanitiza os ids antes de virarem bigint no wire: descarta valor não
 * inteiro, fora de faixa ou duplicado, ordena e corta no teto. Sem isto um
 * estado corrompido viraria InvalidArgument do servidor sem explicação.
 */
function toProtoCategoryIds(ids: number[] | undefined): bigint[] {
  if (!ids || ids.length === 0) return [];
  const unique = Array.from(new Set(ids))
    .filter((id) => Number.isSafeInteger(id) && id > 0 && id <= MAX_SHOPEE_CATEGORY_ID)
    .sort((a, b) => a - b)
    .slice(0, MAX_CATEGORY_IDS);
  return unique.map((id) => BigInt(id));
}

function discountTypeToProto(
  v: 'fixed' | 'percent' | null | undefined,
): DiscountType | undefined {
  if (v === 'fixed') return DiscountType.FIXED;
  if (v === 'percent') return DiscountType.PERCENT;
  return undefined;
}

function discountTypeFromProto(v: DiscountType | undefined): 'fixed' | 'percent' | null {
  if (v === DiscountType.FIXED) return 'fixed';
  if (v === DiscountType.PERCENT) return 'percent';
  return null;
}

function fromProto(r: ProtoCouponRule): LocalCouponRule {
  return {
    id: r.id,
    user_id: r.userId,
    coupon_id: r.couponId,
    platform: r.platform,
    period_start: r.periodStart ?? null,
    period_end: r.periodEnd ?? null,
    min_price: r.minPrice,
    priority: r.priority,
    is_active: r.isActive,
    official_only: r.officialOnly,
    code: r.code ?? null,
    message: r.message ?? null,
    discount_type: discountTypeFromProto(r.discountType),
    discount_value: r.discountValue ?? null,
    max_discount: r.maxDiscount ?? null,
    // proto int64 chega como bigint; ids da Shopee cabem em Int32.
    product_category_ids: (r.productCategoryIds ?? []).map((id) => Number(id)),
    created_at: r.createdAt,
    updated_at: r.updatedAt,
  };
}

const RULES_QUERY_KEY = ['coupon-rules'];

/**
 * Hook returning the list of coupon rules for the authenticated user
 * plus mutation helpers (create / update / delete). Uses the same
 * Tanstack Query cache key as the page so optimistic invalidation
 * keeps the table fresh after a write.
 */
export function useCouponRules() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const { user } = useAuth();
  const transport = useTransport();
  const queryClient = useQueryClient();

  const client = createClient(CouponsService, transport);

  const listQuery = useQuery({
    queryKey: RULES_QUERY_KEY,
    enabled: !!user?.id,
    queryFn: async () => {
      const resp = await client.listCouponRules(create(ListCouponRulesRequestSchema, {}));
      return resp.rules.map(fromProto);
    },
  });

  const invalidate = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: RULES_QUERY_KEY });
  }, [queryClient]);

  const createMutation = useMutation({
    mutationFn: async (params: CreateCouponRuleParams) => {
      const resp = await client.createCouponRule(
        create(CreateCouponRuleRequestSchema, {
          platform: params.platform,
          periodStart: params.period_start ?? undefined,
          periodEnd: params.period_end ?? undefined,
          minPrice: params.min_price,
          priority: params.priority,
          isActive: params.is_active,
          officialOnly: params.official_only,
          code: params.code ?? undefined,
          message: params.message ?? undefined,
          discountType: discountTypeToProto(params.discount_type),
          discountValue: params.discount_value ?? undefined,
          maxDiscount: params.max_discount ?? undefined,
          productCategoryIds: toProtoCategoryIds(params.product_category_ids),
        }),
      );
      return resp.rule ? fromProto(resp.rule) : null;
    },
    onSuccess: () => {
      invalidate();
      toast({ title: t('pages.couponRules.success.created', { defaultValue: 'Regra criada' }) });
    },
    onError: () => {
      toast({
        variant: 'destructive',
        title: t('pages.couponRules.errors.create', { defaultValue: 'Falha ao criar regra' }),
      });
    },
  });

  const updateMutation = useMutation({
    mutationFn: async (params: UpdateCouponRuleParams) => {
      const resp = await client.updateCouponRule(
        create(UpdateCouponRuleRequestSchema, {
          id: params.id,
          platform: params.platform,
          periodStart: params.period_start,
          periodEnd: params.period_end,
          minPrice: params.min_price,
          priority: params.priority,
          isActive: params.is_active,
          officialOnly: params.official_only,
          code: params.code,
          message: params.message,
          discountType: discountTypeToProto(params.discount_type),
          discountValue: params.discount_value,
          maxDiscount: params.max_discount,
          productCategoryIds: toProtoCategoryIds(params.product_category_ids),
          clearProductCategoryIds: params.clear_product_category_ids,
        }),
      );
      return resp.rule ? fromProto(resp.rule) : null;
    },
    onSuccess: () => {
      invalidate();
      toast({ title: t('pages.couponRules.success.updated', { defaultValue: 'Regra atualizada' }) });
    },
    onError: () => {
      toast({
        variant: 'destructive',
        title: t('pages.couponRules.errors.update', { defaultValue: 'Falha ao atualizar regra' }),
      });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async (id: string) => {
      await client.deleteCouponRule(create(DeleteCouponRuleRequestSchema, { id }));
    },
    onSuccess: () => {
      invalidate();
      toast({ title: t('pages.couponRules.success.deleted', { defaultValue: 'Regra removida' }) });
    },
    onError: () => {
      toast({
        variant: 'destructive',
        title: t('pages.couponRules.errors.delete', { defaultValue: 'Falha ao remover regra' }),
      });
    },
  });

  return {
    rules: listQuery.data ?? [],
    isLoading: listQuery.isLoading,
    isError: listQuery.isError,
    refetch: listQuery.refetch,
    createRule: createMutation.mutateAsync,
    updateRule: updateMutation.mutateAsync,
    deleteRule: deleteMutation.mutateAsync,
    isMutating:
      createMutation.isPending || updateMutation.isPending || deleteMutation.isPending,
  };
}
