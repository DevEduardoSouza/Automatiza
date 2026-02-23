import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// ============================================================
// Mock Modules
// ============================================================

vi.mock('@/contexts', () => ({
  useAuth: () => ({ user: { id: 'user-1' } }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

vi.mock('@/lib/connect-transport', () => ({
  getErrorInfo: (e: unknown) => ({
    userMessage: e instanceof Error ? e.message : 'error',
    code: 'INTERNAL',
  }),
}));

vi.mock('@/lib/sentry', () => ({
  captureHookError: vi.fn(),
}));

vi.mock('@/gen/admin/v1/admin_pb', () => ({
  CreatePlanRequestSchema: 'CreatePlanRequestSchema',
  UpdatePlanRequestSchema: 'UpdatePlanRequestSchema',
  DeletePlanRequestSchema: 'DeletePlanRequestSchema',
  ListPlansRequestSchema: 'ListPlansRequestSchema',
}));

vi.mock('@/gen/admin/v1/admin-AdminService_connectquery', () => ({
  listPlans: 'listPlans',
  createPlan: 'createPlan',
  updatePlan: 'updatePlan',
  deletePlan: 'deletePlan',
}));

vi.mock('@bufbuild/protobuf', () => ({
  create: (_schema: unknown, data: unknown) => data,
}));

const mockProtoPlan = {
  id: 'plan-1',
  name: 'Pro',
  slug: 'pro',
  description: 'Pro plan',
  priceMonthly: 29.9,
  priceSemester: undefined,
  priceAnnual: 299.0,
  groupLimit: 10,
  storageLimit: BigInt(5000000000),
  features: ['feature-a', 'feature-b'],
  kiwifyProductIdMonthly: 'kw-monthly',
  kiwifyProductIdSemester: undefined,
  kiwifyProductIdAnnual: 'kw-annual',
  isActive: true,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

let mockPlansResponse: { plans: (typeof mockProtoPlan)[] } | undefined = {
  plans: [mockProtoPlan],
};

const mockCreateMutateAsync = vi.fn();
const mockUpdateMutateAsync = vi.fn();
const mockDeleteMutateAsync = vi.fn();

let createOnSuccess: (() => void) | undefined;
let createOnError: ((error: unknown) => void) | undefined;
let updateOnSuccess: (() => void) | undefined;
let updateOnError: ((error: unknown) => void) | undefined;
let deleteOnSuccess: (() => void) | undefined;
let deleteOnError: ((error: unknown) => void) | undefined;

vi.mock('@connectrpc/connect-query', () => ({
  useQuery: () => ({ data: mockPlansResponse, isLoading: false }),
  useMutation: (_mutationFn: unknown, opts?: Record<string, unknown>) => {
    if (_mutationFn === 'createPlan') {
      createOnSuccess = opts?.onSuccess as (() => void) | undefined;
      createOnError = opts?.onError as ((error: unknown) => void) | undefined;
      return { mutateAsync: mockCreateMutateAsync, isPending: false };
    }
    if (_mutationFn === 'updatePlan') {
      updateOnSuccess = opts?.onSuccess as (() => void) | undefined;
      updateOnError = opts?.onError as ((error: unknown) => void) | undefined;
      return { mutateAsync: mockUpdateMutateAsync, isPending: false };
    }
    if (_mutationFn === 'deletePlan') {
      deleteOnSuccess = opts?.onSuccess as (() => void) | undefined;
      deleteOnError = opts?.onError as ((error: unknown) => void) | undefined;
      return { mutateAsync: mockDeleteMutateAsync, isPending: false };
    }
    return { mutateAsync: vi.fn(), isPending: false };
  },
}));

const mockInvalidateQueries = vi.fn();
vi.mock('@tanstack/react-query', () => ({
  useQueryClient: () => ({ invalidateQueries: mockInvalidateQueries }),
}));

// ============================================================
// Import after mocks
// ============================================================

import { captureHookError } from '@/lib/sentry';
import { toast } from 'sonner';
import { INITIAL_PLAN_FORM_DATA } from '../../types/plans.types';
import type { Plan } from '../../types/plans.types';
import { usePlansTab } from '../use-plans-tab';

// ============================================================
// Test Helpers
// ============================================================

const samplePlan: Plan = {
  id: 'plan-1',
  name: 'Pro',
  slug: 'pro',
  description: 'Pro plan',
  price_monthly: 29.9,
  price_semester: null,
  price_annual: 299.0,
  group_limit: 10,
  storage_limit: 5000000000,
  features: ['feature-a', 'feature-b'],
  kiwify_product_id_monthly: 'kw-monthly',
  kiwify_product_id_semester: null,
  kiwify_product_id_annual: 'kw-annual',
  is_active: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

function renderAndGet() {
  const { result } = renderHook(() => usePlansTab());
  return result;
}

// ============================================================
// Tests
// ============================================================

describe('usePlansTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPlansResponse = { plans: [mockProtoPlan] };
    mockCreateMutateAsync.mockResolvedValue({});
    mockUpdateMutateAsync.mockResolvedValue({});
    mockDeleteMutateAsync.mockResolvedValue({});
    vi.spyOn(globalThis, 'confirm').mockReturnValue(true);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── Initial state ───────────────────────────────────────────

  describe('initial state', () => {
    it('should have editingPlan = null', () => {
      const result = renderAndGet();
      expect(result.current.editingPlan).toBeNull();
    });

    it('should have isDialogOpen = false', () => {
      const result = renderAndGet();
      expect(result.current.isDialogOpen).toBe(false);
    });

    it('should have formData = INITIAL_PLAN_FORM_DATA', () => {
      const result = renderAndGet();
      expect(result.current.formData).toEqual(INITIAL_PLAN_FORM_DATA);
    });

    it('should have saving = false', () => {
      const result = renderAndGet();
      expect(result.current.saving).toBe(false);
    });
  });

  // ── Plans list ──────────────────────────────────────────────

  describe('plans list', () => {
    it('should map proto plans to local format', () => {
      const result = renderAndGet();
      expect(result.current.plans).toHaveLength(1);
      expect(result.current.plans[0].id).toBe('plan-1');
      expect(result.current.plans[0].name).toBe('Pro');
      expect(result.current.plans[0].slug).toBe('pro');
      expect(result.current.plans[0].price_monthly).toBe(29.9);
      expect(result.current.plans[0].price_semester).toBeNull();
      expect(result.current.plans[0].features).toEqual(['feature-a', 'feature-b']);
    });

    it('should return empty array when no plans response', () => {
      mockPlansResponse = undefined;
      const result = renderAndGet();
      expect(result.current.plans).toEqual([]);
    });
  });

  // ── handleOpenDialog ────────────────────────────────────────

  describe('handleOpenDialog', () => {
    it('should reset form and open dialog when no plan provided', () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenDialog());

      expect(result.current.editingPlan).toBeNull();
      expect(result.current.formData).toEqual(INITIAL_PLAN_FORM_DATA);
      expect(result.current.isDialogOpen).toBe(true);
    });

    it('should populate form with plan data when plan provided', () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenDialog(samplePlan));

      expect(result.current.editingPlan).toEqual(samplePlan);
      expect(result.current.formData.name).toBe('Pro');
      expect(result.current.formData.slug).toBe('pro');
      expect(result.current.formData.price_monthly).toBe('29.9');
      expect(result.current.formData.price_annual).toBe('299');
      expect(result.current.formData.features).toBe('feature-a\nfeature-b');
      expect(result.current.formData.kiwify_product_id_monthly).toBe('kw-monthly');
      expect(result.current.isDialogOpen).toBe(true);
    });
  });

  // ── handleSavePlan validation ───────────────────────────────

  describe('handleSavePlan validation', () => {
    it('should toast error when name is empty', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenDialog());
      act(() =>
        result.current.setFormData({
          ...INITIAL_PLAN_FORM_DATA,
          name: '',
          slug: 'test',
          price_monthly: '10',
          kiwify_product_id_monthly: 'kw-1',
        })
      );

      await act(async () => {
        await result.current.handleSavePlan();
      });

      expect(toast.error).toHaveBeenCalledWith('admin.plans.errors.nameRequired');
      expect(mockCreateMutateAsync).not.toHaveBeenCalled();
    });

    it('should toast error when slug is empty', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenDialog());
      act(() =>
        result.current.setFormData({
          ...INITIAL_PLAN_FORM_DATA,
          name: 'Test',
          slug: '',
          price_monthly: '10',
          kiwify_product_id_monthly: 'kw-1',
        })
      );

      await act(async () => {
        await result.current.handleSavePlan();
      });

      expect(toast.error).toHaveBeenCalledWith('admin.plans.errors.slugRequired');
      expect(mockCreateMutateAsync).not.toHaveBeenCalled();
    });

    it('should toast error when no prices are set', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenDialog());
      act(() =>
        result.current.setFormData({
          ...INITIAL_PLAN_FORM_DATA,
          name: 'Test',
          slug: 'test',
          price_monthly: '',
          price_semester: '',
          price_annual: '',
        })
      );

      await act(async () => {
        await result.current.handleSavePlan();
      });

      expect(toast.error).toHaveBeenCalledWith('admin.plans.errors.atLeastOnePrice');
      expect(mockCreateMutateAsync).not.toHaveBeenCalled();
    });

    it('should toast error when price has no kiwify id', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenDialog());
      act(() =>
        result.current.setFormData({
          ...INITIAL_PLAN_FORM_DATA,
          name: 'Test',
          slug: 'test',
          price_monthly: '29.9',
          kiwify_product_id_monthly: '',
        })
      );

      await act(async () => {
        await result.current.handleSavePlan();
      });

      expect(toast.error).toHaveBeenCalledWith('admin.plans.errors.kiwifyIdRequired');
      expect(mockCreateMutateAsync).not.toHaveBeenCalled();
    });
  });

  // ── handleSavePlan create ───────────────────────────────────

  describe('handleSavePlan create', () => {
    it('should parse features and create plan', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenDialog());
      act(() =>
        result.current.setFormData({
          ...INITIAL_PLAN_FORM_DATA,
          name: 'New Plan',
          slug: 'new-plan',
          price_monthly: '19.9',
          features: 'feat 1\nfeat 2\n\nfeat 3',
          kiwify_product_id_monthly: 'kw-new',
        })
      );

      await act(async () => {
        await result.current.handleSavePlan();
      });

      expect(mockCreateMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          slug: 'new-plan',
          name: 'New Plan',
          features: ['feat 1', 'feat 2', 'feat 3'],
          kiwifyProductIdMonthly: 'kw-new',
        })
      );
    });
  });

  // ── handleSavePlan update ───────────────────────────────────

  describe('handleSavePlan update', () => {
    it('should update with plan ID when editing', async () => {
      const result = renderAndGet();

      act(() => result.current.handleOpenDialog(samplePlan));

      await act(async () => {
        await result.current.handleSavePlan();
      });

      expect(mockUpdateMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'plan-1',
          name: 'Pro',
          slug: 'pro',
        })
      );
      expect(mockCreateMutateAsync).not.toHaveBeenCalled();
    });

    it('should capture error on save failure', async () => {
      const error = new Error('Save failed');
      mockUpdateMutateAsync.mockRejectedValue(error);

      const result = renderAndGet();
      act(() => result.current.handleOpenDialog(samplePlan));

      await act(async () => {
        await result.current.handleSavePlan();
      });

      expect(captureHookError).toHaveBeenCalledWith(error, 'usePlansTab', 'handleSavePlan');
    });
  });

  // ── handleDeletePlan ────────────────────────────────────────

  describe('handleDeletePlan', () => {
    it('should delete plan when confirmed', async () => {
      const result = renderAndGet();

      await act(async () => {
        await result.current.handleDeletePlan('plan-1');
      });

      expect(mockDeleteMutateAsync).toHaveBeenCalledWith(expect.objectContaining({ id: 'plan-1' }));
    });

    it('should not delete when user cancels confirm', async () => {
      vi.spyOn(globalThis, 'confirm').mockReturnValue(false);

      const result = renderAndGet();

      await act(async () => {
        await result.current.handleDeletePlan('plan-1');
      });

      expect(mockDeleteMutateAsync).not.toHaveBeenCalled();
    });

    it('should capture error on delete failure', async () => {
      const error = new Error('Delete failed');
      mockDeleteMutateAsync.mockRejectedValue(error);

      const result = renderAndGet();

      await act(async () => {
        await result.current.handleDeletePlan('plan-1');
      });

      expect(captureHookError).toHaveBeenCalledWith(error, 'usePlansTab', 'handleDeletePlan');
    });
  });

  // ── formatStorage ───────────────────────────────────────────

  describe('formatStorage', () => {
    it('should convert bytes to GB string', () => {
      const result = renderAndGet();
      expect(result.current.formatStorage(5000000000)).toBe('5GB');
      expect(result.current.formatStorage(1000000000)).toBe('1GB');
      expect(result.current.formatStorage(500000000)).toBe('1GB');
    });
  });

  // ── Mutation callbacks ──────────────────────────────────────

  describe('mutation callbacks', () => {
    it('should invalidate queries and toast on create success', () => {
      renderAndGet();
      createOnSuccess?.();

      expect(mockInvalidateQueries).toHaveBeenCalledWith({
        queryKey: ['connectRpc', 'admin.v1.AdminService'],
      });
      expect(toast.success).toHaveBeenCalledWith('admin.plans.success.created');
    });

    it('should toast error on create failure', () => {
      renderAndGet();
      createOnError?.(new Error('Create failed'));

      expect(toast.error).toHaveBeenCalledWith('Create failed');
    });

    it('should invalidate queries and toast on update success', () => {
      renderAndGet();
      updateOnSuccess?.();

      expect(mockInvalidateQueries).toHaveBeenCalledWith({
        queryKey: ['connectRpc', 'admin.v1.AdminService'],
      });
      expect(toast.success).toHaveBeenCalledWith('admin.plans.success.updated');
    });

    it('should toast error on update failure', () => {
      renderAndGet();
      updateOnError?.(new Error('Update failed'));

      expect(toast.error).toHaveBeenCalledWith('Update failed');
    });

    it('should invalidate queries and toast on delete success', () => {
      renderAndGet();
      deleteOnSuccess?.();

      expect(mockInvalidateQueries).toHaveBeenCalledWith({
        queryKey: ['connectRpc', 'admin.v1.AdminService'],
      });
      expect(toast.success).toHaveBeenCalledWith('admin.plans.success.deleted');
    });

    it('should toast error on delete failure', () => {
      renderAndGet();
      deleteOnError?.(new Error('Delete failed'));

      expect(toast.error).toHaveBeenCalledWith('Delete failed');
    });
  });
});
