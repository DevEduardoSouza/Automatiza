import {
  AdminService,
  CreateCategoryRequestSchema,
  CreateSubcategoryRequestSchema,
  DeleteCategoryRequestSchema,
  DeleteSubcategoryRequestSchema,
  ListCategoriesRequestSchema,
  type Category as ProtoCategory,
  type Subcategory as ProtoSubcategory,
  UpdateCategoryRequestSchema,
  UpdateSubcategoryRequestSchema,
} from '@/gen/admin/v1/admin_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

// ============================================================
// Local Types
// ============================================================

export interface LocalSubcategory {
  id: string;
  categoryId: string;
  slug: string;
  name: string;
  displayKey: string;
  groupName?: string;
  displayOrder: number;
  isActive: boolean;
  isSystem: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface LocalCategory {
  id: string;
  slug: string;
  name: string;
  icon?: string;
  displayOrder: number;
  isActive: boolean;
  hasRefinements: boolean;
  isSystem: boolean;
  createdAt: string;
  updatedAt: string;
  subcategories: LocalSubcategory[];
}

export interface CreateCategoryInput {
  slug: string;
  name: string;
  icon?: string;
  displayOrder?: number;
  hasRefinements: boolean;
}

export interface UpdateCategoryInput {
  id: string;
  name?: string;
  icon?: string;
  displayOrder?: number;
  isActive?: boolean;
  hasRefinements?: boolean;
}

export interface CreateSubcategoryInput {
  categoryId: string;
  slug: string;
  name: string;
  displayKey: string;
  groupName?: string;
  displayOrder?: number;
}

export interface UpdateSubcategoryInput {
  id: string;
  name?: string;
  displayKey?: string;
  groupName?: string;
  displayOrder?: number;
  isActive?: boolean;
}

export interface UseCategoriesOptions {
  includeInactive?: boolean;
  includeSubcategories?: boolean;
  enabled?: boolean;
}

export interface UseCategoriesReturn {
  categories: LocalCategory[];
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
  refetch: () => void;
  // Mutations
  createCategory: (input: CreateCategoryInput) => Promise<LocalCategory>;
  updateCategory: (input: UpdateCategoryInput) => Promise<LocalCategory>;
  deleteCategory: (id: string) => Promise<boolean>;
  createSubcategory: (input: CreateSubcategoryInput) => Promise<LocalSubcategory>;
  updateSubcategory: (input: UpdateSubcategoryInput) => Promise<LocalSubcategory>;
  deleteSubcategory: (id: string) => Promise<boolean>;
  // Mutation states
  isCreatingCategory: boolean;
  isUpdatingCategory: boolean;
  isDeletingCategory: boolean;
  isCreatingSubcategory: boolean;
  isUpdatingSubcategory: boolean;
  isDeletingSubcategory: boolean;
}

// ============================================================
// Conversion Functions
// ============================================================

function fromProtoSubcategory(proto: ProtoSubcategory): LocalSubcategory {
  return {
    id: proto.id,
    categoryId: proto.categoryId,
    slug: proto.slug,
    name: proto.name,
    displayKey: proto.displayKey,
    groupName: proto.groupName,
    displayOrder: proto.displayOrder,
    isActive: proto.isActive,
    isSystem: proto.isSystem,
    createdAt: proto.createdAt,
    updatedAt: proto.updatedAt,
  };
}

function fromProtoCategory(proto: ProtoCategory): LocalCategory {
  return {
    id: proto.id,
    slug: proto.slug,
    name: proto.name,
    icon: proto.icon,
    displayOrder: proto.displayOrder,
    isActive: proto.isActive,
    hasRefinements: proto.hasRefinements,
    isSystem: proto.isSystem,
    createdAt: proto.createdAt,
    updatedAt: proto.updatedAt,
    subcategories: proto.subcategories.map(fromProtoSubcategory),
  };
}

// ============================================================
// Query Keys
// ============================================================

export const categoriesQueryKey = ['categories'] as const;

// ============================================================
// Hook
// ============================================================

/**
 * Hook for fetching and managing categories and subcategories.
 *
 * This hook provides:
 * - Fetching categories with optional subcategories
 * - CRUD operations for categories
 * - CRUD operations for subcategories
 * - Automatic cache invalidation on mutations
 *
 * @example
 * ```tsx
 * // Basic usage - fetch active categories with subcategories
 * const { categories, isLoading } = useCategories();
 *
 * // Admin usage - include inactive for management
 * const { categories, createCategory, deleteCategory } = useCategories({
 *   includeInactive: true,
 *   includeSubcategories: true,
 * });
 * ```
 */
export function useCategories(options: UseCategoriesOptions = {}): UseCategoriesReturn {
  const { includeInactive = false, includeSubcategories = true, enabled = true } = options;
  const { t } = useTranslation();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const transport = useTransport();

  // Query for fetching categories
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: [...categoriesQueryKey, { includeInactive, includeSubcategories }],
    queryFn: async () => {
      const client = createClient(AdminService, transport);
      const response = await client.listCategories(
        create(ListCategoriesRequestSchema, {
          includeInactive,
          includeSubcategories,
        })
      );
      return response.categories.map(fromProtoCategory);
    },
    enabled,
    staleTime: 5 * 60 * 1000, // 5 minutes
  });

  // Create category mutation
  const createCategoryMutation = useMutation({
    mutationFn: async (input: CreateCategoryInput): Promise<LocalCategory> => {
      const client = createClient(AdminService, transport);
      const response = await client.createCategory(
        create(CreateCategoryRequestSchema, {
          slug: input.slug,
          name: input.name,
          icon: input.icon,
          displayOrder: input.displayOrder,
          hasRefinements: input.hasRefinements,
        })
      );
      if (!response.category) {
        throw new Error('Category not returned from server');
      }
      return fromProtoCategory(response.category);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: categoriesQueryKey });
      toast({
        title: t('admin.categories.createSuccess', 'Categoria criada com sucesso'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('admin.categories.createError', 'Erro ao criar categoria'),
        description: errorInfo.userMessage,
      });
    },
  });

  // Update category mutation
  const updateCategoryMutation = useMutation({
    mutationFn: async (input: UpdateCategoryInput): Promise<LocalCategory> => {
      const client = createClient(AdminService, transport);
      const response = await client.updateCategory(
        create(UpdateCategoryRequestSchema, {
          id: input.id,
          name: input.name,
          icon: input.icon,
          displayOrder: input.displayOrder,
          isActive: input.isActive,
          hasRefinements: input.hasRefinements,
        })
      );
      if (!response.category) {
        throw new Error('Category not returned from server');
      }
      return fromProtoCategory(response.category);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: categoriesQueryKey });
      toast({
        title: t('admin.categories.updateSuccess', 'Categoria atualizada com sucesso'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('admin.categories.updateError', 'Erro ao atualizar categoria'),
        description: errorInfo.userMessage,
      });
    },
  });

  // Delete category mutation
  const deleteCategoryMutation = useMutation({
    mutationFn: async (id: string): Promise<boolean> => {
      const client = createClient(AdminService, transport);
      const response = await client.deleteCategory(create(DeleteCategoryRequestSchema, { id }));
      return response.success;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: categoriesQueryKey });
      toast({
        title: t('admin.categories.deleteSuccess', 'Categoria excluída com sucesso'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('admin.categories.deleteError', 'Erro ao excluir categoria'),
        description: errorInfo.userMessage,
      });
    },
  });

  // Create subcategory mutation
  const createSubcategoryMutation = useMutation({
    mutationFn: async (input: CreateSubcategoryInput): Promise<LocalSubcategory> => {
      const client = createClient(AdminService, transport);
      const response = await client.createSubcategory(
        create(CreateSubcategoryRequestSchema, {
          categoryId: input.categoryId,
          slug: input.slug,
          name: input.name,
          displayKey: input.displayKey,
          groupName: input.groupName,
          displayOrder: input.displayOrder,
        })
      );
      if (!response.subcategory) {
        throw new Error('Subcategory not returned from server');
      }
      return fromProtoSubcategory(response.subcategory);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: categoriesQueryKey });
      toast({
        title: t('admin.categories.createSubcategorySuccess', 'Subcategoria criada com sucesso'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('admin.categories.createSubcategoryError', 'Erro ao criar subcategoria'),
        description: errorInfo.userMessage,
      });
    },
  });

  // Update subcategory mutation
  const updateSubcategoryMutation = useMutation({
    mutationFn: async (input: UpdateSubcategoryInput): Promise<LocalSubcategory> => {
      const client = createClient(AdminService, transport);
      const response = await client.updateSubcategory(
        create(UpdateSubcategoryRequestSchema, {
          id: input.id,
          name: input.name,
          displayKey: input.displayKey,
          groupName: input.groupName,
          displayOrder: input.displayOrder,
          isActive: input.isActive,
        })
      );
      if (!response.subcategory) {
        throw new Error('Subcategory not returned from server');
      }
      return fromProtoSubcategory(response.subcategory);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: categoriesQueryKey });
      toast({
        title: t(
          'admin.categories.updateSubcategorySuccess',
          'Subcategoria atualizada com sucesso'
        ),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('admin.categories.updateSubcategoryError', 'Erro ao atualizar subcategoria'),
        description: errorInfo.userMessage,
      });
    },
  });

  // Delete subcategory mutation
  const deleteSubcategoryMutation = useMutation({
    mutationFn: async (id: string): Promise<boolean> => {
      const client = createClient(AdminService, transport);
      const response = await client.deleteSubcategory(
        create(DeleteSubcategoryRequestSchema, { id })
      );
      return response.success;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: categoriesQueryKey });
      toast({
        title: t('admin.categories.deleteSubcategorySuccess', 'Subcategoria excluída com sucesso'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('admin.categories.deleteSubcategoryError', 'Erro ao excluir subcategoria'),
        description: errorInfo.userMessage,
      });
    },
  });

  return {
    categories: data ?? [],
    isLoading,
    isError,
    error: error as Error | null,
    refetch,
    // Category mutations
    createCategory: createCategoryMutation.mutateAsync,
    updateCategory: updateCategoryMutation.mutateAsync,
    deleteCategory: deleteCategoryMutation.mutateAsync,
    // Subcategory mutations
    createSubcategory: createSubcategoryMutation.mutateAsync,
    updateSubcategory: updateSubcategoryMutation.mutateAsync,
    deleteSubcategory: deleteSubcategoryMutation.mutateAsync,
    // Mutation states
    isCreatingCategory: createCategoryMutation.isPending,
    isUpdatingCategory: updateCategoryMutation.isPending,
    isDeletingCategory: deleteCategoryMutation.isPending,
    isCreatingSubcategory: createSubcategoryMutation.isPending,
    isUpdatingSubcategory: updateSubcategoryMutation.isPending,
    isDeletingSubcategory: deleteSubcategoryMutation.isPending,
  };
}
