import { useAuth } from '@/contexts';
import { listPlans } from '@/gen/plans/v1/plans-PlansService_connectquery';
import {
  CreatePlanRequestSchema,
  DeletePlanRequestSchema,
  GetPlanRequestSchema,
  PlansService,
  type Plan as ProtoPlan,
  UpdatePlanRequestSchema,
} from '@/gen/plans/v1/plans_pb';
import { useToast } from '@/hooks/use-toast';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { Code, createClient } from '@connectrpc/connect';
import { createConnectQueryKey, useQuery, useTransport } from '@connectrpc/connect-query';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

// ============================================================
// Local Types (snake_case for UI compatibility)
// ============================================================

/**
 * Local plan interface matching UI expectations.
 */
interface LocalPlan {
  id: string;
  slug: string;
  name: string;
  description: string | null;
  max_automations: number;
  max_groups: number;
  max_instances: number;
  price_monthly: number | null;
  price_semester: number | null;
  price_annual: number | null;
  features: string[];
  is_active: boolean;
  group_limit: number;
  storage_limit: number;
  kiwify_product_id_monthly: string | null;
  kiwify_product_id_semester: string | null;
  kiwify_product_id_annual: string | null;
  created_at: string;
  updated_at: string;
}

/**
 * Create plan form data interface.
 */
interface CreatePlanData {
  slug: string;
  name: string;
  description?: string | null;
  max_automations?: number;
  max_groups?: number;
  max_instances?: number;
  price_monthly?: number | null;
  price_semester?: number | null;
  price_annual?: number | null;
  features?: string[];
  is_active: boolean;
  group_limit?: number;
  storage_limit?: number;
  kiwify_product_id_monthly?: string | null;
  kiwify_product_id_semester?: string | null;
  kiwify_product_id_annual?: string | null;
}

/**
 * Update plan form data interface (all fields optional except id).
 */
interface UpdatePlanData {
  slug?: string;
  name?: string;
  description?: string | null;
  max_automations?: number;
  max_groups?: number;
  max_instances?: number;
  price_monthly?: number | null;
  price_semester?: number | null;
  price_annual?: number | null;
  features?: string[];
  is_active?: boolean;
  group_limit?: number;
  storage_limit?: number;
  kiwify_product_id_monthly?: string | null;
  kiwify_product_id_semester?: string | null;
  kiwify_product_id_annual?: string | null;
}

// ============================================================
// Error Mapping
// ============================================================

/**
 * Maps gRPC error codes to i18n translation keys for plan operations.
 */
function getPlanErrorMessage(code: Code, t: (key: string) => string): string | null {
  switch (code) {
    case Code.AlreadyExists:
      return t('admin.plans.errors.duplicateSlug');
    case Code.NotFound:
      return t('admin.plans.errors.notFound');
    case Code.InvalidArgument:
      return t('admin.plans.errors.invalidData');
    case Code.PermissionDenied:
      return t('admin.plans.errors.permissionDenied');
    default:
      return null;
  }
}

// ============================================================
// Conversion Functions
// ============================================================

/**
 * Converts proto Plan to local format.
 */
function fromProtoPlan(plan: ProtoPlan): LocalPlan {
  return {
    id: plan.id,
    slug: plan.slug,
    name: plan.name,
    description: plan.description ?? null,
    max_automations: plan.maxAutomations,
    max_groups: plan.maxGroups,
    max_instances: plan.maxInstances,
    price_monthly: plan.priceMonthly ?? null,
    price_semester: plan.priceSemester ?? null,
    price_annual: plan.priceAnnual ?? null,
    features: plan.features ?? [],
    is_active: plan.isActive,
    group_limit: plan.groupLimit,
    storage_limit: Number(plan.storageLimit),
    kiwify_product_id_monthly: plan.kiwifyProductIdMonthly ?? null,
    kiwify_product_id_semester: plan.kiwifyProductIdSemester ?? null,
    kiwify_product_id_annual: plan.kiwifyProductIdAnnual ?? null,
    created_at: plan.createdAt,
    updated_at: plan.updatedAt,
  };
}

// ============================================================
// Initial Form Data
// ============================================================

const initialFormData: CreatePlanData = {
  slug: '',
  name: '',
  description: null,
  max_automations: 0,
  max_groups: 0,
  max_instances: 0,
  price_monthly: null,
  price_semester: null,
  price_annual: null,
  features: [],
  is_active: true,
  group_limit: 0,
  storage_limit: 0,
  kiwify_product_id_monthly: null,
  kiwify_product_id_semester: null,
  kiwify_product_id_annual: null,
};

// ============================================================
// Hook Return Type
// ============================================================

interface UsePlansConnectReturn {
  /** List of plans */
  plans: LocalPlan[];
  /** Whether plans are loading */
  loading: boolean;
  /** Dialog open state */
  isDialogOpen: boolean;
  /** Plan being edited (null for create) */
  editingPlan: LocalPlan | null;
  /** Filter by active status */
  filterActive: string;
  /** Form data for create/edit */
  formData: CreatePlanData;
  /** Filtered plans based on active filter */
  filteredPlans: LocalPlan[];
  /** Whether any mutation is pending */
  isPending: boolean;
  /** Include inactive plans in query */
  includeInactive: boolean;
  /** Set filter active value */
  setFilterActive: (value: string) => void;
  /** Set form data */
  setFormData: React.Dispatch<React.SetStateAction<CreatePlanData>>;
  /** Set dialog open state */
  setIsDialogOpen: (value: boolean) => void;
  /** Set include inactive flag */
  setIncludeInactive: (value: boolean) => void;
  /** Open dialog for create or edit */
  handleOpenDialog: (plan?: LocalPlan) => void;
  /** Close dialog and reset form */
  handleCloseDialog: () => void;
  /** Submit form (create or update) */
  handleSubmit: (e: React.FormEvent) => Promise<void>;
  /** Delete a plan */
  handleDelete: (id: string) => Promise<void>;
  /** Toggle plan active status */
  handleToggleActive: (plan: LocalPlan) => Promise<void>;
  /** Refetch plans */
  refetchPlans: () => void;
  /** Get a single plan by ID */
  getPlan: (id: string) => Promise<LocalPlan | null>;
}

// ============================================================
// Hook Implementation
// ============================================================

/**
 * Hook for plans management using ConnectRPC.
 * Provides CRUD operations for subscription plans (admin only).
 */
export function usePlansConnect(): UsePlansConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const transport = useTransport();

  // Local state
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [editingPlan, setEditingPlan] = useState<LocalPlan | null>(null);
  const [filterActive, setFilterActive] = useState<string>('all');
  const [formData, setFormData] = useState<CreatePlanData>(initialFormData);
  const [includeInactive, setIncludeInactive] = useState(true);

  // Query for fetching plans via ConnectRPC
  const {
    data: plansResponse,
    isLoading: loading,
    refetch: refetchPlans,
  } = useQuery(
    listPlans,
    { includeInactive },
    {
      enabled: !!user,
      staleTime: 30_000, // 30 seconds
    }
  );

  // Convert proto response to local format
  const plans: LocalPlan[] = useMemo(
    () => (plansResponse?.plans ?? []).map(fromProtoPlan),
    [plansResponse]
  );

  // Mutation for getting a single plan
  const getPlanMutation = useMutation({
    mutationFn: async (id: string) => {
      const client = createClient(PlansService, transport);
      const response = await client.getPlan(create(GetPlanRequestSchema, { id }));
      return response.plan ? fromProtoPlan(response.plan) : null;
    },
  });

  // Mutation for creating plan
  const createMutation = useMutation({
    mutationFn: async (data: CreatePlanData) => {
      const client = createClient(PlansService, transport);
      return client.createPlan(
        create(CreatePlanRequestSchema, {
          slug: data.slug,
          name: data.name,
          description: data.description || undefined,
          maxAutomations: data.max_automations ?? undefined,
          maxGroups: data.max_groups ?? undefined,
          maxInstances: data.max_instances ?? undefined,
          priceMonthly: data.price_monthly ?? undefined,
          priceSemester: data.price_semester ?? undefined,
          priceAnnual: data.price_annual ?? undefined,
          features: data.features ?? [],
          isActive: data.is_active,
          groupLimit: data.group_limit ?? undefined,
          storageLimit: data.storage_limit ? BigInt(data.storage_limit) : undefined,
          kiwifyProductIdMonthly: data.kiwify_product_id_monthly || undefined,
          kiwifyProductIdSemester: data.kiwify_product_id_semester || undefined,
          kiwifyProductIdAnnual: data.kiwify_product_id_annual || undefined,
        })
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listPlans, cardinality: 'finite' }),
      });
      toast({
        title: t('admin.plans.success.created'),
        description: t('admin.plans.success.createdDescription'),
      });
      handleCloseDialog();
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      const message = getPlanErrorMessage(errorInfo.code, t) ?? errorInfo.userMessage;
      toast({
        variant: 'destructive',
        title: t('admin.plans.errors.createError'),
        description: message,
      });
    },
  });

  // Mutation for updating plan
  const updateMutation = useMutation({
    mutationFn: async ({ id, data }: { id: string; data: UpdatePlanData }) => {
      const client = createClient(PlansService, transport);
      return client.updatePlan(
        create(UpdatePlanRequestSchema, {
          id,
          slug: data.slug,
          name: data.name,
          description: data.description || undefined,
          maxAutomations: data.max_automations ?? undefined,
          maxGroups: data.max_groups ?? undefined,
          maxInstances: data.max_instances ?? undefined,
          priceMonthly: data.price_monthly ?? undefined,
          priceSemester: data.price_semester ?? undefined,
          priceAnnual: data.price_annual ?? undefined,
          features: data.features,
          isActive: data.is_active,
          groupLimit: data.group_limit ?? undefined,
          storageLimit: data.storage_limit ? BigInt(data.storage_limit) : undefined,
          kiwifyProductIdMonthly: data.kiwify_product_id_monthly || undefined,
          kiwifyProductIdSemester: data.kiwify_product_id_semester || undefined,
          kiwifyProductIdAnnual: data.kiwify_product_id_annual || undefined,
        })
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listPlans, cardinality: 'finite' }),
      });
      toast({
        title: t('admin.plans.success.updated'),
        description: t('admin.plans.success.updatedDescription'),
      });
      handleCloseDialog();
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      const message = getPlanErrorMessage(errorInfo.code, t) ?? errorInfo.userMessage;
      toast({
        variant: 'destructive',
        title: t('admin.plans.errors.updateError'),
        description: message,
      });
    },
  });

  // Mutation for deleting plan
  const deleteMutation = useMutation({
    mutationFn: async (id: string) => {
      const client = createClient(PlansService, transport);
      return client.deletePlan(create(DeletePlanRequestSchema, { id }));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listPlans, cardinality: 'finite' }),
      });
      toast({
        title: t('admin.plans.success.deleted'),
        description: t('admin.plans.success.deletedDescription'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      const message = getPlanErrorMessage(errorInfo.code, t) ?? errorInfo.userMessage;
      toast({
        variant: 'destructive',
        title: t('admin.plans.errors.deleteError'),
        description: message,
      });
    },
  });

  // Mutation for toggling active status
  const toggleActiveMutation = useMutation({
    mutationFn: async ({ id, is_active }: { id: string; is_active: boolean }) => {
      const client = createClient(PlansService, transport);
      return client.updatePlan(
        create(UpdatePlanRequestSchema, {
          id,
          isActive: is_active,
        })
      );
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listPlans, cardinality: 'finite' }),
      });
      toast({
        title: variables.is_active ? t('common.activated') : t('common.deactivated'),
        description: t('admin.plans.success.updatedDescription'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      const message = getPlanErrorMessage(errorInfo.code, t) ?? errorInfo.userMessage;
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: message,
      });
    },
  });

  const isPending =
    createMutation.isPending ||
    updateMutation.isPending ||
    deleteMutation.isPending ||
    toggleActiveMutation.isPending ||
    getPlanMutation.isPending;

  // Handlers
  const handleOpenDialog = useCallback((plan?: LocalPlan) => {
    if (plan) {
      setEditingPlan(plan);
      setFormData({
        slug: plan.slug,
        name: plan.name,
        description: plan.description,
        max_automations: plan.max_automations,
        max_groups: plan.max_groups,
        max_instances: plan.max_instances,
        price_monthly: plan.price_monthly,
        price_semester: plan.price_semester,
        price_annual: plan.price_annual,
        features: plan.features,
        is_active: plan.is_active,
        group_limit: plan.group_limit,
        storage_limit: plan.storage_limit,
        kiwify_product_id_monthly: plan.kiwify_product_id_monthly,
        kiwify_product_id_semester: plan.kiwify_product_id_semester,
        kiwify_product_id_annual: plan.kiwify_product_id_annual,
      });
    } else {
      setEditingPlan(null);
      setFormData(initialFormData);
    }
    setIsDialogOpen(true);
  }, []);

  const handleCloseDialog = useCallback(() => {
    setIsDialogOpen(false);
    setEditingPlan(null);
    setFormData(initialFormData);
  }, []);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();

      if (editingPlan) {
        updateMutation.mutate({ id: editingPlan.id, data: formData });
      } else {
        createMutation.mutate(formData);
      }
    },
    [formData, editingPlan, updateMutation, createMutation]
  );

  const handleDelete = useCallback(
    async (id: string) => {
      if (!confirm(t('admin.plans.confirmDelete'))) return;
      deleteMutation.mutate(id);
    },
    [t, deleteMutation]
  );

  const handleToggleActive = useCallback(
    async (plan: LocalPlan) => {
      toggleActiveMutation.mutate({
        id: plan.id,
        is_active: !plan.is_active,
      });
    },
    [toggleActiveMutation]
  );

  const getPlan = useCallback(
    async (id: string): Promise<LocalPlan | null> => {
      try {
        return await getPlanMutation.mutateAsync(id);
      } catch {
        return null;
      }
    },
    [getPlanMutation]
  );

  // Filtered plans based on active filter
  const filteredPlans = useMemo(
    () =>
      plans.filter((plan) => {
        if (filterActive === 'all') return true;
        if (filterActive === 'active') return plan.is_active;
        if (filterActive === 'inactive') return !plan.is_active;
        return true;
      }),
    [plans, filterActive]
  );

  return {
    plans,
    loading,
    isDialogOpen,
    editingPlan,
    filterActive,
    formData,
    filteredPlans,
    isPending,
    includeInactive,
    setFilterActive,
    setFormData,
    setIsDialogOpen,
    setIncludeInactive,
    handleOpenDialog,
    handleCloseDialog,
    handleSubmit,
    handleDelete,
    handleToggleActive,
    refetchPlans: () => refetchPlans(),
    getPlan,
  };
}
