import { useAuth } from '@/contexts';
import {
  createPlan,
  deletePlan,
  listPlans,
  updatePlan,
} from '@/gen/admin/v1/admin-AdminService_connectquery';
import {
  CreatePlanRequestSchema,
  DeletePlanRequestSchema,
  ListPlansRequestSchema,
  UpdatePlanRequestSchema,
} from '@/gen/admin/v1/admin_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { useMutation, useQuery } from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import {
  INITIAL_PLAN_FORM_DATA,
  type Plan,
  type PlanFormData,
  type UsePlansTabReturn,
  fromProtoPlan,
} from '../types/plans.types';

/**
 * Hook that manages plans tab logic via ConnectRPC.
 * Allows creating, editing, and deleting subscription plans.
 */
export function usePlansTab(): UsePlansTabReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const queryClient = useQueryClient();

  const [editingPlan, setEditingPlan] = useState<Plan | null>(null);
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [formData, setFormData] = useState<PlanFormData>(INITIAL_PLAN_FORM_DATA);

  // Query plans via ConnectRPC
  const { data: plansResponse, isLoading: loading } = useQuery(
    listPlans,
    create(ListPlansRequestSchema, { includeInactive: true }),
    { enabled: !!user }
  );

  // Transform proto plans to local format
  const plans: Plan[] = useMemo(
    () => (plansResponse?.plans ?? []).map(fromProtoPlan),
    [plansResponse]
  );

  // Create plan mutation
  const createPlanMutation = useMutation(createPlan, {
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['connectRpc', 'admin.v1.AdminService'] });
      toast.success(t('admin.plans.success.created'));
      setIsDialogOpen(false);
      setFormData(INITIAL_PLAN_FORM_DATA);
    },
    onError: (error) => {
      const errInfo = getErrorInfo(error);
      toast.error(errInfo.userMessage || t('admin.plans.errors.createError'));
    },
  });

  // Update plan mutation
  const updatePlanMutation = useMutation(updatePlan, {
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['connectRpc', 'admin.v1.AdminService'] });
      toast.success(t('admin.plans.success.updated'));
      setIsDialogOpen(false);
      setFormData(INITIAL_PLAN_FORM_DATA);
      setEditingPlan(null);
    },
    onError: (error) => {
      const errInfo = getErrorInfo(error);
      toast.error(errInfo.userMessage || t('admin.plans.errors.updateError'));
    },
  });

  // Delete plan mutation
  const deletePlanMutation = useMutation(deletePlan, {
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['connectRpc', 'admin.v1.AdminService'] });
      toast.success(t('admin.plans.success.deleted'));
    },
    onError: (error) => {
      const errInfo = getErrorInfo(error);
      toast.error(errInfo.userMessage || t('admin.plans.errors.deleteError'));
    },
  });

  const saving = createPlanMutation.isPending || updatePlanMutation.isPending;

  const handleOpenDialog = useCallback((plan?: Plan) => {
    if (plan) {
      setEditingPlan(plan);
      setFormData({
        name: plan.name,
        slug: plan.slug,
        description: plan.description || '',
        price_monthly: plan.price_monthly ? plan.price_monthly.toString() : '',
        price_semester: plan.price_semester ? plan.price_semester.toString() : '',
        price_annual: plan.price_annual ? plan.price_annual.toString() : '',
        group_limit: plan.group_limit.toString(),
        storage_limit: plan.storage_limit.toString(),
        features: plan.features.join('\n'),
        kiwify_product_id_monthly: plan.kiwify_product_id_monthly || '',
        kiwify_product_id_semester: plan.kiwify_product_id_semester || '',
        kiwify_product_id_annual: plan.kiwify_product_id_annual || '',
      });
    } else {
      setEditingPlan(null);
      setFormData(INITIAL_PLAN_FORM_DATA);
    }
    setIsDialogOpen(true);
  }, []);

  const handleSavePlan = useCallback(async () => {
    if (!formData.name.trim()) {
      toast.error(t('admin.plans.errors.nameRequired'));
      return;
    }

    if (!formData.slug.trim()) {
      toast.error(t('admin.plans.errors.slugRequired'));
      return;
    }

    const parsePrice = (value: string): number | undefined => {
      const parsed = Number.parseFloat(value);
      return Number.isNaN(parsed) || parsed === 0 ? undefined : parsed;
    };

    const priceMonthly = parsePrice(formData.price_monthly);
    const priceSemester = parsePrice(formData.price_semester);
    const priceAnnual = parsePrice(formData.price_annual);

    if (!priceMonthly && !priceSemester && !priceAnnual) {
      toast.error(t('admin.plans.errors.atLeastOnePrice'));
      return;
    }

    const hasMonthlyLink = !!formData.kiwify_product_id_monthly.trim();
    const hasSemesterLink = !!formData.kiwify_product_id_semester.trim();
    const hasAnnualLink = !!formData.kiwify_product_id_annual.trim();

    if (priceMonthly && !hasMonthlyLink) {
      toast.error(
        t('admin.plans.errors.kiwifyIdRequired', {
          period: t('admin.plans.periods.monthly'),
        })
      );
      return;
    }

    if (priceSemester && !hasSemesterLink) {
      toast.error(
        t('admin.plans.errors.kiwifyIdRequired', {
          period: t('admin.plans.periods.semester'),
        })
      );
      return;
    }

    if (priceAnnual && !hasAnnualLink) {
      toast.error(
        t('admin.plans.errors.kiwifyIdRequired', {
          period: t('admin.plans.periods.annual'),
        })
      );
      return;
    }

    const features = formData.features
      .split('\n')
      .map((f) => f.trim())
      .filter((f) => f.length > 0);

    if (editingPlan) {
      // Update existing plan
      await updatePlanMutation.mutateAsync(
        create(UpdatePlanRequestSchema, {
          id: editingPlan.id,
          slug: formData.slug,
          name: formData.name,
          isActive: true,
          description: formData.description || undefined,
          priceMonthly,
          priceSemester,
          priceAnnual,
          groupLimit: Number.parseInt(formData.group_limit, 10) || 1,
          storageLimit: BigInt(formData.storage_limit || '1000000000'),
          features,
          kiwifyProductIdMonthly: formData.kiwify_product_id_monthly || undefined,
          kiwifyProductIdSemester: formData.kiwify_product_id_semester || undefined,
          kiwifyProductIdAnnual: formData.kiwify_product_id_annual || undefined,
        })
      );
    } else {
      // Create new plan
      await createPlanMutation.mutateAsync(
        create(CreatePlanRequestSchema, {
          slug: formData.slug,
          name: formData.name,
          isActive: true,
          description: formData.description || undefined,
          priceMonthly,
          priceSemester,
          priceAnnual,
          groupLimit: Number.parseInt(formData.group_limit, 10) || 1,
          storageLimit: BigInt(formData.storage_limit || '1000000000'),
          features,
          kiwifyProductIdMonthly: formData.kiwify_product_id_monthly || undefined,
          kiwifyProductIdSemester: formData.kiwify_product_id_semester || undefined,
          kiwifyProductIdAnnual: formData.kiwify_product_id_annual || undefined,
        })
      );
    }
  }, [formData, editingPlan, createPlanMutation, updatePlanMutation, t]);

  const handleDeletePlan = useCallback(
    async (id: string) => {
      if (!confirm(t('admin.plans.confirm.delete'))) return;
      await deletePlanMutation.mutateAsync(create(DeletePlanRequestSchema, { id }));
    },
    [deletePlanMutation, t]
  );

  const formatStorage = useCallback((bytes: number) => {
    return `${(bytes / 1000000000).toFixed(0)}GB`;
  }, []);

  return {
    plans,
    loading,
    saving,
    editingPlan,
    isDialogOpen,
    setIsDialogOpen,
    formData,
    setFormData,
    handleOpenDialog,
    handleSavePlan,
    handleDeletePlan,
    formatStorage,
  };
}
