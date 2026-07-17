import { useAuth } from '@/contexts';
import {
  AdminService,
  AdminSpreadAutomationScheduleRequestSchema,
  type User as AdminUser,
  type AdminUserAutomation,
  CreateUserRequestSchema,
  GetUserInvestigationRequestSchema,
  GrantAdminRoleRequestSchema,
  ListAdminUsersRequestSchema,
  RevokeAdminRoleRequestSchema,
} from '@/gen/admin/v1/admin_pb';
import {
  listProfiles,
  searchProfiles,
} from '@/gen/profiles/v1/profiles-ProfilesService_connectquery';
import {
  type Profile,
  ProfilesService,
  UnblockUserRequestSchema,
  UpdateUserPlanRequestSchema,
} from '@/gen/profiles/v1/profiles_pb';
import { useAdminConnect } from '@/hooks/use-admin-connect';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { createConnectQueryKey, useQuery, useTransport } from '@connectrpc/connect-query';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { z } from 'zod';
import type {
  CreateUserFormData,
  User,
  UserAutomationRow,
  UserInvestigation,
} from '../users/types';

const createUserSchema = (t: (key: string) => string) =>
  z.object({
    email: z
      .string()
      .trim()
      .email({ message: t('admin.users.validation.invalidEmail') })
      .max(255),
    password: z
      .string()
      .min(6, { message: t('admin.users.validation.passwordMinLength') })
      .max(72),
    phone_number: z.string().trim().max(20).optional().or(z.literal('')),
    plan: z.enum(['free', 'pro', 'elite']),
  });

const ITEMS_PER_PAGE = 10;

export interface UseUsersTabReturn {
  filteredUsers: User[];
  setFilteredUsers: (users: User[]) => void;
  adminIds: Set<string>;
  searchTerm: string;
  setSearchTerm: (term: string) => void;
  statusFilter: 'all' | 'active' | 'inactive';
  setStatusFilter: (filter: 'all' | 'active' | 'inactive') => void;
  loading: boolean;
  isDialogOpen: boolean;
  setIsDialogOpen: (open: boolean) => void;
  creating: boolean;
  currentPage: number;
  setCurrentPage: (page: number) => void;
  totalUsers: number;
  totalPages: number;
  formData: CreateUserFormData;
  setFormData: (data: CreateUserFormData) => void;
  detailsOpen: boolean;
  setDetailsOpen: (open: boolean) => void;
  detailsLoading: boolean;
  details: UserInvestigation | null;
  bulkActionLoading: boolean;
  spreadMinutes: number;
  setSpreadMinutes: (minutes: number) => void;
  handleWhatsAppCall: (phoneNumber: string | null) => void;
  handleCreateUser: () => Promise<void>;
  isAdmin: (userId: string) => boolean;
  makeAdmin: (userId: string) => Promise<void>;
  revokeAdmin: (userId: string) => Promise<void>;
  updateMaxAutomations: (userId: string, value: number | null) => Promise<void>;
  openUserDetails: (user: User) => Promise<void>;
  unblockUserAccount: () => Promise<void>;
  handleResetPassword: () => Promise<void>;
  spreadNextRunAt: () => Promise<void>;
  handleRefreshDetails: () => void;
}

/**
 * Converts a Proto Profile to a local User type.
 */
function fromProtoProfile(profile: Profile): User {
  return {
    id: profile.id,
    email: profile.email ?? '',
    plan: profile.plan ?? 'free',
    phone_number: profile.phoneNumber ?? null,
    max_automations: profile.maxAutomations ?? null,
    billing_status: profile.isBlocked ? 'suspended' : null,
    billing_suspended_at: null, // Not available in proto
    billing_suspension_reason: null, // Not available in proto
    billing_suspension_kind: null, // Not available in proto
    billing_next_payment_at: profile.billingNextPaymentAt ?? null,
    plan_access_until: profile.planAccessUntil ?? null,
    created_at: profile.createdAt,
    updated_at: profile.updatedAt,
  };
}

/**
 * Converts an AdminUser proto to a local User type.
 */
function fromAdminUser(user: AdminUser): User {
  return {
    id: user.id,
    email: user.email ?? '',
    plan: user.plan ?? 'free',
    phone_number: user.phoneNumber ?? null,
    max_automations: null,
    billing_status: user.billingStatus ?? null,
    billing_suspended_at: null,
    billing_suspension_reason: null,
    billing_suspension_kind: null,
    billing_next_payment_at: null,
    plan_access_until: null,
    created_at: user.createdAt,
    updated_at: user.updatedAt,
  };
}

/**
 * Converts an AdminUserAutomation proto to a local UserAutomationRow.
 */
function fromProtoAutomation(auto: AdminUserAutomation): UserAutomationRow {
  return {
    id: auto.id,
    name: auto.name,
    status: (auto.status as 'active' | 'paused' | 'inactive') ?? 'paused',
    mode: auto.mode ?? null,
    interval_minutes: auto.intervalMinutes ?? null,
    delay_seconds: auto.delaySeconds ?? null,
    start_time: auto.startTime ?? null,
    end_time: auto.endTime ?? null,
    stores: auto.stores ?? null,
    send_groups: auto.sendGroups ?? null,
    categories: auto.categories ?? null,
    last_run_at: auto.lastRunAt ?? null,
    next_run_at: auto.nextRunAt ?? null,
    created_at: auto.createdAt ?? null,
  };
}

/**
 * Hook que gerencia a logica da aba de usuarios.
 * Permite listar, criar, editar e gerenciar usuarios.
 */
export function useUsersTab(): UseUsersTabReturn {
  const { t } = useTranslation();
  const { user: authUser } = useAuth();
  const { resetUserPassword } = useAdminConnect();
  const transport = useTransport();
  const queryClient = useQueryClient();

  const [filteredUsers, setFilteredUsers] = useState<User[]>([]);
  const [adminIds, setAdminIds] = useState<Set<string>>(new Set());
  const [searchTerm, setSearchTerm] = useState('');
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'inactive'>('all');
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalUsers, setTotalUsers] = useState(0);
  const [formData, setFormData] = useState<CreateUserFormData>({
    email: '',
    password: '',
    phone_number: '',
    plan: 'free',
  });
  const [cursor, setCursor] = useState<string | undefined>(undefined);

  const [detailsOpen, setDetailsOpen] = useState(false);
  const [detailsLoading, setDetailsLoading] = useState(false);
  const [details, setDetails] = useState<UserInvestigation | null>(null);
  const [bulkActionLoading, setBulkActionLoading] = useState(false);
  const [spreadMinutes, setSpreadMinutes] = useState<number>(2);

  // Use ConnectRPC query for listing profiles
  const {
    data: listResponse,
    isLoading: listLoading,
    refetch: refetchList,
  } = useQuery(
    listProfiles,
    { limit: ITEMS_PER_PAGE, cursor },
    {
      enabled: !searchTerm && !!authUser,
      staleTime: 30_000,
    }
  );

  // Use ConnectRPC query for searching profiles
  const {
    data: searchResponse,
    isLoading: searchLoading,
    refetch: refetchSearch,
  } = useQuery(
    searchProfiles,
    { query: searchTerm, limit: ITEMS_PER_PAGE },
    {
      enabled: !!searchTerm && !!authUser,
      staleTime: 30_000,
    }
  );

  const loading = searchTerm ? searchLoading : listLoading;

  // Unblock user mutation
  const unblockUserMutation = useMutation({
    mutationFn: async (userId: string) => {
      const client = createClient(ProfilesService, transport);
      return client.unblockUser(create(UnblockUserRequestSchema, { userId }));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listProfiles, cardinality: 'finite' }),
      });
    },
  });

  // Update user plan mutation (for max_automations)
  const updateUserPlanMutation = useMutation({
    mutationFn: async ({ userId, maxAutomations }: { userId: string; maxAutomations?: number }) => {
      const client = createClient(ProfilesService, transport);
      return client.updateUserPlan(create(UpdateUserPlanRequestSchema, { userId, maxAutomations }));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({ schema: listProfiles, cardinality: 'finite' }),
      });
    },
  });

  // Process responses from queries
  useEffect(() => {
    if (searchTerm && searchResponse) {
      const users = searchResponse.profiles.map(fromProtoProfile);
      setFilteredUsers(users);
      setTotalUsers(users.length);
    } else if (!searchTerm && listResponse) {
      const users = listResponse.profiles.map(fromProtoProfile);
      setFilteredUsers(users);
      // Note: The proto doesn't include total count, we approximate
      // In a real scenario, we'd need a count endpoint or the backend to return total
      if (listResponse.hasMore) {
        setTotalUsers(currentPage * ITEMS_PER_PAGE + 1);
      } else {
        setTotalUsers((currentPage - 1) * ITEMS_PER_PAGE + users.length);
      }
    }
  }, [searchResponse, listResponse, searchTerm, currentPage]);

  // Fetch admin IDs on mount
  useEffect(() => {
    const loadAdminIds = async () => {
      try {
        const client = createClient(AdminService, transport);
        const response = await client.listAdminUsers(create(ListAdminUsersRequestSchema, {}));
        setAdminIds(new Set(response.admins.map((a) => a.userId)));
      } catch (e) {
        console.error('Failed to load admin IDs:', e);
      }
    };

    if (authUser) {
      loadAdminIds();
    }
  }, [authUser, transport]);

  const totalPages = Math.ceil(totalUsers / ITEMS_PER_PAGE);

  const handleWhatsAppCall = (phoneNumber: string | null) => {
    if (!phoneNumber) {
      toast.error(t('admin.users.errors.noWhatsApp'));
      return;
    }
    const cleanNumber = phoneNumber.replace(/\D/g, '');
    window.open(`https://wa.me/${cleanNumber}`, '_blank');
  };

  /**
   * Creates a new user via AdminService.CreateUser.
   */
  const handleCreateUser = async () => {
    try {
      const validatedData = createUserSchema(t).parse(formData);
      setCreating(true);

      const client = createClient(AdminService, transport);
      const response = await client.createUser(
        create(CreateUserRequestSchema, {
          email: validatedData.email,
          plan: validatedData.plan,
          phoneNumber: validatedData.phone_number || undefined,
          sendWelcomeEmail: false,
        })
      );

      toast.success(t('admin.users.success.created'));

      if (response.temporaryPassword) {
        toast.info(
          t('admin.users.success.tempPassword', { password: response.temporaryPassword }),
          {
            duration: 15000,
          }
        );
      }

      setIsDialogOpen(false);
      setFormData({
        email: '',
        password: '',
        phone_number: '',
        plan: 'free',
      });
      refetchList();
    } catch (error: unknown) {
      if (error instanceof z.ZodError) {
        const firstError = error.errors[0];
        toast.error(firstError.message);
      } else {
        const errInfo = getErrorInfo(error);
        toast.error(errInfo.userMessage || t('admin.users.errors.createError'));
      }
    } finally {
      setCreating(false);
    }
  };

  const isAdmin = (userId: string) => adminIds.has(userId);

  /**
   * Grants admin role to a user.
   */
  const makeAdmin = async (userId: string) => {
    try {
      const client = createClient(AdminService, transport);
      await client.grantAdminRole(create(GrantAdminRoleRequestSchema, { userId }));

      const updated = new Set(adminIds);
      updated.add(userId);
      setAdminIds(updated);
      toast.success(t('admin.users.success.adminGranted'));
    } catch (e: unknown) {
      const errInfo = getErrorInfo(e);
      toast.error(errInfo.userMessage || t('admin.users.errors.promoteError'));
    }
  };

  /**
   * Revokes admin role from a user.
   */
  const revokeAdmin = async (userId: string) => {
    if (authUser?.id === userId) {
      toast.error(t('admin.users.errors.cannotRemoveOwnAdmin'));
      return;
    }
    try {
      const client = createClient(AdminService, transport);
      await client.revokeAdminRole(create(RevokeAdminRoleRequestSchema, { userId }));

      const updated = new Set(adminIds);
      updated.delete(userId);
      setAdminIds(updated);
      toast.success(t('admin.users.success.adminRevoked'));
    } catch (e: unknown) {
      const errInfo = getErrorInfo(e);
      toast.error(errInfo.userMessage || t('admin.users.errors.revokeError'));
    }
  };

  const updateMaxAutomations = async (userId: string, value: number | null) => {
    try {
      await updateUserPlanMutation.mutateAsync({
        userId,
        maxAutomations: value ?? undefined,
      });
      toast.success(t('admin.users.success.limitUpdated'));
      refetchList();
    } catch (e: unknown) {
      const errInfo = getErrorInfo(e);
      toast.error(errInfo.userMessage || t('admin.users.errors.updateLimitError'));
    }
  };

  /**
   * Opens user details dialog with full investigation data.
   */
  const openUserDetails = async (u: User) => {
    setDetailsOpen(true);
    setDetailsLoading(true);
    setDetails(null);
    try {
      const client = createClient(AdminService, transport);
      const response = await client.getUserInvestigation(
        create(GetUserInvestigationRequestSchema, { userId: u.id })
      );

      setDetails({
        profile: response.user ? fromAdminUser(response.user) : u,
        isAdmin: response.isAdmin,
        instances: response.instances.map((i) => ({
          instance_id: i.instanceId,
          status: i.status,
          updated_at: i.updatedAt ?? null,
        })),
        automationStats: {
          total: response.automationsTotal,
          active: response.automationsActive,
          paused: response.automationsPaused,
        },
        automations: response.automations.map(fromProtoAutomation),
      });
    } catch (e: unknown) {
      const errInfo = getErrorInfo(e);
      toast.error(errInfo.userMessage || t('admin.users.errors.loadDetailsError'));
      setDetailsOpen(false);
    } finally {
      setDetailsLoading(false);
    }
  };

  const unblockUserAccount = async () => {
    if (!details) return;
    const ok = window.confirm(t('admin.users.confirm.unblock', { email: details.profile.email }));
    if (!ok) return;

    setBulkActionLoading(true);
    try {
      await unblockUserMutation.mutateAsync(details.profile.id);

      toast.success(t('admin.users.success.unblocked'));
      await openUserDetails(details.profile);
      refetchList();
    } catch (e: unknown) {
      const errInfo = getErrorInfo(e);
      toast.error(errInfo.userMessage || t('admin.users.errors.unblockError'));
    } finally {
      setBulkActionLoading(false);
    }
  };

  const handleResetPassword = async () => {
    if (!details) return;
    if (authUser?.id === details.profile.id) {
      toast.error(t('admin.users.errors.cannotResetOwnPassword'));
      return;
    }
    const ok = window.confirm(
      t('admin.users.confirm.resetPassword', { email: details.profile.email })
    );
    if (!ok) return;

    setBulkActionLoading(true);
    try {
      const result = await resetUserPassword(details.profile.id);

      if (result.success) {
        toast.success(t('admin.users.success.passwordReset', { email: result.email }), {
          duration: 5000,
        });
        if (result.newPassword) {
          toast.info(
            t('admin.users.success.tempPassword', {
              password: result.newPassword,
            }),
            {
              duration: 15000,
              description: t('admin.users.success.tempPasswordHint'),
            }
          );
        }
      }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : t('admin.users.errors.resetPasswordError');
      toast.error(msg);
    } finally {
      setBulkActionLoading(false);
    }
  };

  /**
   * Spreads next_run_at across user's active automations.
   */
  const spreadNextRunAt = async () => {
    if (!details) return;
    const mins = Number(spreadMinutes);
    const spacing = Number.isFinite(mins) && mins > 0 ? Math.floor(mins) : 2;

    setBulkActionLoading(true);
    try {
      const client = createClient(AdminService, transport);
      const response = await client.adminSpreadAutomationSchedule(
        create(AdminSpreadAutomationScheduleRequestSchema, {
          userId: details.profile.id,
          spreadMinutes: spacing,
          activeOnly: true,
        })
      );

      if (response.updatedCount === 0) {
        toast.message(t('admin.users.details.actions.noActiveAutomations'));
      } else {
        toast.success(
          t('admin.users.success.scheduleSpread', {
            count: response.updatedCount,
            minutes: spacing,
          })
        );
      }
      await openUserDetails(details.profile);
    } catch (e: unknown) {
      const errInfo = getErrorInfo(e);
      toast.error(errInfo.userMessage || t('admin.users.errors.rescheduleError'));
    } finally {
      setBulkActionLoading(false);
    }
  };

  const handleRefreshDetails = () => {
    if (details) {
      openUserDetails(details.profile);
    }
  };

  return {
    filteredUsers,
    setFilteredUsers,
    adminIds,
    searchTerm,
    setSearchTerm,
    statusFilter,
    setStatusFilter,
    loading,
    isDialogOpen,
    setIsDialogOpen,
    creating,
    currentPage,
    setCurrentPage,
    totalUsers,
    totalPages,
    formData,
    setFormData,
    detailsOpen,
    setDetailsOpen,
    detailsLoading,
    details,
    bulkActionLoading,
    spreadMinutes,
    setSpreadMinutes,
    handleWhatsAppCall,
    handleCreateUser,
    isAdmin,
    makeAdmin,
    revokeAdmin,
    updateMaxAutomations,
    openUserDetails,
    unblockUserAccount,
    handleResetPassword,
    spreadNextRunAt,
    handleRefreshDetails,
  };
}
