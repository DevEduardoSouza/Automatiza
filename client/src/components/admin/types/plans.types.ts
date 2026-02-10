/**
 * Types for the PlansTab component and usePlansTab hook.
 * Manages subscription plans via ConnectRPC AdminService.
 */

import type { Plan as ProtoPlan } from '@/gen/admin/v1/admin_pb';

/**
 * Local Plan type derived from proto, with snake_case for UI compatibility.
 */
export interface Plan {
  id: string;
  name: string;
  slug: string;
  description: string | null;
  price_monthly: number | null;
  price_semester: number | null;
  price_annual: number | null;
  group_limit: number;
  storage_limit: number;
  features: string[];
  kiwify_product_id_monthly: string | null;
  kiwify_product_id_semester: string | null;
  kiwify_product_id_annual: string | null;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

/**
 * Converts proto Plan to local Plan format.
 */
export function fromProtoPlan(proto: ProtoPlan): Plan {
  return {
    id: proto.id,
    name: proto.name,
    slug: proto.slug,
    description: proto.description ?? null,
    price_monthly: proto.priceMonthly ?? null,
    price_semester: proto.priceSemester ?? null,
    price_annual: proto.priceAnnual ?? null,
    group_limit: proto.groupLimit,
    storage_limit: Number(proto.storageLimit),
    features: proto.features,
    kiwify_product_id_monthly: proto.kiwifyProductIdMonthly ?? null,
    kiwify_product_id_semester: proto.kiwifyProductIdSemester ?? null,
    kiwify_product_id_annual: proto.kiwifyProductIdAnnual ?? null,
    is_active: proto.isActive,
    created_at: proto.createdAt,
    updated_at: proto.updatedAt,
  };
}

export interface PlanFormData {
  name: string;
  slug: string;
  description: string;
  price_monthly: string;
  price_semester: string;
  price_annual: string;
  group_limit: string;
  storage_limit: string;
  features: string;
  kiwify_product_id_monthly: string;
  kiwify_product_id_semester: string;
  kiwify_product_id_annual: string;
}

export const INITIAL_PLAN_FORM_DATA: PlanFormData = {
  name: '',
  slug: '',
  description: '',
  price_monthly: '',
  price_semester: '',
  price_annual: '',
  group_limit: '1',
  storage_limit: '1000000000',
  features: '',
  kiwify_product_id_monthly: '',
  kiwify_product_id_semester: '',
  kiwify_product_id_annual: '',
};

export interface UsePlansTabReturn {
  plans: Plan[];
  loading: boolean;
  saving: boolean;
  editingPlan: Plan | null;
  isDialogOpen: boolean;
  setIsDialogOpen: (open: boolean) => void;
  formData: PlanFormData;
  setFormData: (data: PlanFormData) => void;
  handleOpenDialog: (plan?: Plan) => void;
  handleSavePlan: () => Promise<void>;
  handleDeletePlan: (id: string) => Promise<void>;
  formatStorage: (bytes: number) => string;
}
