import { useAuth } from '@/contexts';
import { listCoupons } from '@/gen/coupons/v1/coupons-CouponsService_connectquery';
import {
  CouponFilterSchema,
  CouponsService,
  CreateCouponRequestSchema,
  DeleteCouponRequestSchema,
  DiscountType,
  type Coupon as ProtoCoupon,
  Store,
  UpdateCouponRequestSchema,
} from '@/gen/coupons/v1/coupons_pb';
import { useToast } from '@/hooks/use-toast';
import { getErrorInfo } from '@/lib/connect-transport';
import { safeClipboard } from '@/lib/sentry';
import { create } from '@bufbuild/protobuf';
import { Code, createClient } from '@connectrpc/connect';
import { createConnectQueryKey, useQuery, useTransport } from '@connectrpc/connect-query';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useAffiliateCredentialsConnect } from './use-affiliate-credentials-connect';

/**
 * Maps gRPC error codes to i18n translation keys for coupon operations.
 */
function getCouponErrorMessage(code: Code, t: (key: string) => string): string | null {
  switch (code) {
    case Code.AlreadyExists:
      return t('pages.coupons.errors.duplicateCode');
    case Code.NotFound:
      return t('pages.coupons.errors.notFound');
    case Code.InvalidArgument:
      return t('pages.coupons.errors.invalidData');
    default:
      return null;
  }
}

/**
 * Store key type for form data (snake_case for UI compatibility).
 */
type StoreKey = 'shopee' | 'amazon' | 'magalu' | 'ml' | 'shein' | 'aliexpress' | 'awin';

/**
 * Discount type for form data (snake_case for UI compatibility).
 */
type LocalDiscountType = 'fixed' | 'percent';

const STORE_KEYS: StoreKey[] = ['shopee', 'amazon', 'magalu', 'ml', 'shein', 'aliexpress', 'awin'];

const CATEGORY_KEYS = [
  'generalOffers',
  'electronics',
  'fashion',
  'men',
  'homeDecor',
  'beauty',
  'sports',
  'books',
  'toys',
  'children',
  'maternity',
  'food',
  'automotive',
  'pets',
] as const;

/**
 * Local coupon interface matching UI expectations (snake_case).
 */
interface LocalCoupon {
  id: string;
  user_id: string;
  code: string;
  title: string;
  store: string;
  affiliate_link: string | null;
  expires_at: string | null;
  is_active: boolean;
  category: string | null;
  discount_type: LocalDiscountType | null;
  discount_value: number | null;
  max_discount: number | null;
  min_product_price: number | null;
  coupon_text: string | null;
  created_at: string;
  updated_at: string;
}

/**
 * Create coupon form data interface.
 */
interface CreateCouponData {
  code: string;
  title: string;
  store: StoreKey;
  affiliate_link?: string | null;
  expires_at?: string | null;
  category?: string | null;
  discount_type?: LocalDiscountType | null;
  discount_value?: number | null;
  max_discount?: number | null;
  min_product_price?: number | null;
  coupon_text?: string | null;
}

interface StoreOption {
  value: string;
  label: string;
  disabled?: boolean;
}

interface CategoryOption {
  value: string;
  label: string;
}

/**
 * Maps local store key to proto Store enum.
 */
function toProtoStore(store: string): Store {
  switch (store) {
    case 'shopee':
      return Store.SHOPEE;
    case 'amazon':
      return Store.AMAZON;
    case 'magalu':
      return Store.MAGALU;
    case 'ml':
      return Store.MERCADO_LIVRE;
    case 'shein':
      return Store.SHEIN;
    case 'aliexpress':
      return Store.ALIEXPRESS;
    case 'awin':
      return Store.AWIN;
    default:
      return Store.UNSPECIFIED;
  }
}

/**
 * Maps proto Store enum to local store key.
 */
function fromProtoStore(store: Store): string {
  switch (store) {
    case Store.SHOPEE:
      return 'shopee';
    case Store.AMAZON:
      return 'amazon';
    case Store.MAGALU:
      return 'magalu';
    case Store.MERCADO_LIVRE:
      return 'ml';
    case Store.SHEIN:
      return 'shein';
    case Store.ALIEXPRESS:
      return 'aliexpress';
    case Store.AWIN:
      return 'awin';
    default:
      return 'shopee';
  }
}

/**
 * Maps local discount type to proto DiscountType enum.
 */
function toProtoDiscountType(type: LocalDiscountType | null | undefined): DiscountType {
  if (type === 'percent') {
    return DiscountType.PERCENT;
  }
  return DiscountType.FIXED;
}

/**
 * Maps proto DiscountType enum to local discount type.
 */
function fromProtoDiscountType(type: DiscountType): LocalDiscountType | null {
  switch (type) {
    case DiscountType.FIXED:
      return 'fixed';
    case DiscountType.PERCENT:
      return 'percent';
    default:
      return null;
  }
}

/**
 * Converts proto Coupon to local format.
 */
function fromProtoCoupon(coupon: ProtoCoupon): LocalCoupon {
  return {
    id: coupon.id,
    user_id: coupon.userId,
    code: coupon.code,
    title: coupon.title,
    store: fromProtoStore(coupon.store),
    affiliate_link: coupon.affiliateLink ?? null,
    expires_at: coupon.expiresAt ?? null,
    is_active: coupon.isActive,
    category: coupon.category ?? null,
    discount_type: fromProtoDiscountType(coupon.discountType),
    discount_value: coupon.discountValue ?? null,
    max_discount: coupon.maxDiscount ?? null,
    min_product_price: coupon.minProductPrice ?? null,
    coupon_text: coupon.couponText ?? null,
    created_at: coupon.createdAt,
    updated_at: coupon.updatedAt,
  };
}

const initialFormData: CreateCouponData = {
  code: '',
  title: '',
  store: 'shopee',
  affiliate_link: '',
  expires_at: null,
  category: null,
  discount_type: 'fixed',
  discount_value: null,
  max_discount: null,
  min_product_price: null,
  coupon_text: null,
};

interface UseCouponsConnectReturn {
  coupons: LocalCoupon[];
  loading: boolean;
  isDialogOpen: boolean;
  editingCoupon: LocalCoupon | null;
  filterStore: string;
  filterStatus: string;
  searchQuery: string;
  formData: CreateCouponData;
  stores: StoreOption[];
  categories: CategoryOption[];
  filteredCoupons: LocalCoupon[];
  isPending: boolean;
  setFilterStore: (value: string) => void;
  setFilterStatus: (value: string) => void;
  setSearchQuery: (value: string) => void;
  setFormData: React.Dispatch<React.SetStateAction<CreateCouponData>>;
  setIsDialogOpen: (value: boolean) => void;
  handleOpenDialog: (coupon?: LocalCoupon) => void;
  handleCloseDialog: () => void;
  handleSubmit: (e: React.FormEvent) => Promise<void>;
  handleDelete: (id: string) => Promise<void>;
  handleToggleActive: (coupon: LocalCoupon) => Promise<void>;
  handleCopyCode: (code: string) => void;
  isExpired: (coupon: LocalCoupon) => boolean;
}

/**
 * Hook for coupons page logic using ConnectRPC.
 * Manages coupon CRUD operations with React Query.
 */
export function useCoupons(): UseCouponsConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const transport = useTransport();

  const { activeStores } = useAffiliateCredentialsConnect();

  const stores = useMemo<StoreOption[]>(
    () =>
      STORE_KEYS.map((key) => {
        const activeStore = activeStores.find((s) => s.store === key);
        const isConfigured = activeStore?.is_configured ?? false;
        return {
          value: key,
          label: isConfigured
            ? t(`pages.coupons.stores.${key}`)
            : `${t(`pages.coupons.stores.${key}`)} (${t('pages.coupons.stores.notConfigured')})`,
          disabled: !isConfigured,
        };
      }),
    [t, activeStores]
  );

  const categories = useMemo<CategoryOption[]>(
    () =>
      CATEGORY_KEYS.map((key) => ({
        value: key,
        label: t(`pages.coupons.categories.${key}`),
      })),
    [t]
  );

  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [editingCoupon, setEditingCoupon] = useState<LocalCoupon | null>(null);
  const [filterStore, setFilterStore] = useState<string>('all');
  const [filterStatus, setFilterStatus] = useState<string>('all');
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [formData, setFormData] = useState<CreateCouponData>(initialFormData);

  // Build filter for query
  const queryFilter = useMemo(() => {
    if (filterStore === 'all' && filterStatus === 'all') {
      return {};
    }

    const filter = create(CouponFilterSchema, {});

    if (filterStore !== 'all') {
      filter.store = toProtoStore(filterStore);
    }

    if (filterStatus !== 'all') {
      filter.isActive = filterStatus === 'active';
    }

    return { filter };
  }, [filterStore, filterStatus]);

  // Query for fetching coupons via ConnectRPC
  const { data: couponsResponse, isLoading: loading } = useQuery(listCoupons, queryFilter, {
    enabled: !!user,
  });

  const coupons: LocalCoupon[] = useMemo(
    () => (couponsResponse?.coupons ?? []).map(fromProtoCoupon),
    [couponsResponse]
  );

  // Mutation for creating coupon
  const createMutation = useMutation({
    mutationFn: async (data: CreateCouponData) => {
      const client = createClient(CouponsService, transport);
      return client.createCoupon(
        create(CreateCouponRequestSchema, {
          code: data.code,
          title: data.title,
          store: toProtoStore(data.store),
          affiliateLink: data.affiliate_link || undefined,
          expiresAt: data.expires_at || undefined,
          category: data.category || undefined,
          discountType: toProtoDiscountType(data.discount_type),
          discountValue: data.discount_value ?? undefined,
          maxDiscount: data.max_discount ?? undefined,
          minProductPrice: data.min_product_price ?? undefined,
          couponText: data.coupon_text || undefined,
        })
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listCoupons, cardinality: 'finite' }),
      });
      queryClient.invalidateQueries({ queryKey: ['coupons'] });
      toast({
        title: t('pages.coupons.success.created'),
        description: t('pages.coupons.success.created'),
      });
      handleCloseDialog();
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      const message = getCouponErrorMessage(errorInfo.code, t) ?? errorInfo.userMessage;
      toast({
        variant: 'destructive',
        title: t('pages.coupons.errors.save'),
        description: message,
      });
    },
  });

  // Mutation for updating coupon
  const updateMutation = useMutation({
    mutationFn: async ({ id, data }: { id: string; data: Partial<CreateCouponData> }) => {
      const client = createClient(CouponsService, transport);
      return client.updateCoupon(
        create(UpdateCouponRequestSchema, {
          id,
          code: data.code,
          title: data.title,
          store: data.store ? toProtoStore(data.store) : undefined,
          affiliateLink: data.affiliate_link || undefined,
          expiresAt: data.expires_at || undefined,
          category: data.category || undefined,
          discountType: data.discount_type ? toProtoDiscountType(data.discount_type) : undefined,
          discountValue: data.discount_value ?? undefined,
          maxDiscount: data.max_discount ?? undefined,
          minProductPrice: data.min_product_price ?? undefined,
          couponText: data.coupon_text || undefined,
        })
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listCoupons, cardinality: 'finite' }),
      });
      queryClient.invalidateQueries({ queryKey: ['coupons'] });
      toast({
        title: t('pages.coupons.success.updated'),
        description: t('pages.coupons.success.updated'),
      });
      handleCloseDialog();
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      const message = getCouponErrorMessage(errorInfo.code, t) ?? errorInfo.userMessage;
      toast({
        variant: 'destructive',
        title: t('pages.coupons.errors.save'),
        description: message,
      });
    },
  });

  // Mutation for deleting coupon
  const deleteMutation = useMutation({
    mutationFn: async (id: string) => {
      const client = createClient(CouponsService, transport);
      return client.deleteCoupon(create(DeleteCouponRequestSchema, { id }));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listCoupons, cardinality: 'finite' }),
      });
      queryClient.invalidateQueries({ queryKey: ['coupons'] });
      toast({
        title: t('pages.coupons.success.deleted'),
        description: t('pages.coupons.success.deleted'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      const message = getCouponErrorMessage(errorInfo.code, t) ?? errorInfo.userMessage;
      toast({
        variant: 'destructive',
        title: t('pages.coupons.errors.delete'),
        description: message,
      });
    },
  });

  // Mutation for toggling active status
  const toggleActiveMutation = useMutation({
    mutationFn: async ({ id, is_active }: { id: string; is_active: boolean }) => {
      const client = createClient(CouponsService, transport);
      return client.updateCoupon(
        create(UpdateCouponRequestSchema, {
          id,
          isActive: is_active,
        })
      );
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listCoupons, cardinality: 'finite' }),
      });
      queryClient.invalidateQueries({ queryKey: ['coupons'] });
      toast({
        title: variables.is_active ? t('common.active') : t('common.inactive'),
        description: t('pages.coupons.success.updated'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      const message = getCouponErrorMessage(errorInfo.code, t) ?? errorInfo.userMessage;
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
    toggleActiveMutation.isPending;

  const handleOpenDialog = useCallback((coupon?: LocalCoupon) => {
    if (coupon) {
      setEditingCoupon(coupon);
      setFormData({
        code: coupon.code,
        title: coupon.title,
        store: coupon.store as StoreKey,
        affiliate_link: coupon.affiliate_link || '',
        expires_at: coupon.expires_at ? new Date(coupon.expires_at).toISOString() : null,
        category: coupon.category,
        discount_type: coupon.discount_type || 'fixed',
        discount_value: coupon.discount_value ?? null,
        max_discount: coupon.max_discount ?? null,
        min_product_price: coupon.min_product_price ?? null,
        coupon_text: coupon.coupon_text ?? null,
      });
    } else {
      setEditingCoupon(null);
      setFormData(initialFormData);
    }
    setIsDialogOpen(true);
  }, []);

  const handleCloseDialog = useCallback(() => {
    setIsDialogOpen(false);
    setEditingCoupon(null);
    setFormData(initialFormData);
  }, []);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();

      const payload: CreateCouponData = {
        ...formData,
        affiliate_link: formData.affiliate_link?.trim() ? formData.affiliate_link.trim() : null,
        coupon_text:
          formData.coupon_text && String(formData.coupon_text).trim()
            ? String(formData.coupon_text).trim()
            : null,
      };

      if (editingCoupon) {
        updateMutation.mutate({ id: editingCoupon.id, data: payload });
      } else {
        createMutation.mutate(payload);
      }
    },
    [formData, editingCoupon, updateMutation, createMutation]
  );

  const handleDelete = useCallback(
    async (id: string) => {
      if (!confirm(t('pages.coupons.confirmDelete'))) return;
      deleteMutation.mutate(id);
    },
    [t, deleteMutation]
  );

  const handleToggleActive = useCallback(
    async (coupon: LocalCoupon) => {
      const confirmMessage = coupon.is_active
        ? t('pages.coupons.confirmDeactivate')
        : t('pages.coupons.confirmActivate');
      if (!confirm(confirmMessage)) return;
      toggleActiveMutation.mutate({
        id: coupon.id,
        is_active: !coupon.is_active,
      });
    },
    [toggleActiveMutation, t]
  );

  const handleCopyCode = useCallback(
    async (code: string) => {
      const success = await safeClipboard(code);
      if (success) {
        toast({
          title: t('pages.coupons.success.copied'),
          description: code,
        });
      } else {
        toast({
          variant: 'destructive',
          title: t('common.error'),
          description: t('common.clipboardUnavailable'),
        });
      }
    },
    [toast, t]
  );

  const buildDiscountTag = useCallback((coupon: LocalCoupon): string | null => {
    if (!coupon.discount_type || !coupon.discount_value) return null;
    const code = coupon.code;
    if (coupon.discount_type === 'percent') {
      const base = coupon.max_discount
        ? `desc[${coupon.discount_value}%|${coupon.max_discount}:${code}]`
        : `desc[${coupon.discount_value}%:${code}]`;
      return base;
    }
    return `desc[${coupon.discount_value}:${code}]`;
  }, []);

  const handleCopyTag = useCallback(
    async (coupon: LocalCoupon) => {
      const tag = buildDiscountTag(coupon);
      if (!tag) return;
      const success = await safeClipboard(tag);
      if (success) {
        toast({
          title: t('pages.coupons.success.tagCopied'),
          description: tag,
        });
      } else {
        toast({
          variant: 'destructive',
          title: t('common.error'),
          description: t('common.clipboardUnavailable'),
        });
      }
    },
    [buildDiscountTag, toast, t]
  );

  const isExpired = useCallback((coupon: LocalCoupon): boolean => {
    if (!coupon.expires_at) return false;
    return new Date(coupon.expires_at) < new Date();
  }, []);

  const filteredCoupons = useMemo(
    () =>
      coupons.filter((coupon) => {
        // Filter by search query (code or title)
        if (searchQuery.trim()) {
          const query = searchQuery.toLowerCase().trim();
          const matchesCode = coupon.code.toLowerCase().includes(query);
          const matchesTitle = coupon.title.toLowerCase().includes(query);
          if (!matchesCode && !matchesTitle) return false;
        }
        if (filterStore !== 'all' && coupon.store !== filterStore) return false;
        if (filterStatus === 'active' && (!coupon.is_active || isExpired(coupon))) return false;
        if (filterStatus === 'inactive' && coupon.is_active && !isExpired(coupon)) return false;
        if (filterStatus === 'expired' && !isExpired(coupon)) return false;
        return true;
      }),
    [coupons, filterStore, filterStatus, searchQuery, isExpired]
  );

  return {
    coupons,
    loading,
    isDialogOpen,
    editingCoupon,
    filterStore,
    filterStatus,
    searchQuery,
    formData,
    stores,
    categories,
    filteredCoupons,
    isPending,
    setFilterStore,
    setFilterStatus,
    setSearchQuery,
    setFormData,
    setIsDialogOpen,
    handleOpenDialog,
    handleCloseDialog,
    handleSubmit,
    handleDelete,
    handleToggleActive,
    handleCopyCode,
    handleCopyTag,
    buildDiscountTag,
    isExpired,
  };
}
