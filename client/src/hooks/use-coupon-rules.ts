import { useAuth } from '@/contexts';
import {
  CouponsService,
  CreateCouponRuleRequestSchema,
  DeleteCouponRuleRequestSchema,
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
  created_at: string;
  updated_at: string;
}

export interface CreateCouponRuleParams {
  coupon_id: string;
  platform: string;
  period_start?: string | null;
  period_end?: string | null;
  min_price?: number;
  priority?: number;
  is_active?: boolean;
}

export interface UpdateCouponRuleParams {
  id: string;
  coupon_id?: string;
  platform?: string;
  // empty string explicitly clears (writes NULL); undefined leaves alone
  period_start?: string;
  period_end?: string;
  min_price?: number;
  priority?: number;
  is_active?: boolean;
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
          couponId: params.coupon_id,
          platform: params.platform,
          periodStart: params.period_start ?? undefined,
          periodEnd: params.period_end ?? undefined,
          minPrice: params.min_price,
          priority: params.priority,
          isActive: params.is_active,
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
          couponId: params.coupon_id,
          platform: params.platform,
          periodStart: params.period_start,
          periodEnd: params.period_end,
          minPrice: params.min_price,
          priority: params.priority,
          isActive: params.is_active,
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
