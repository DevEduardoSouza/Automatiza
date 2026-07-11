import { useAuth } from '@/contexts';
import {
  BlockUserRequestSchema,
  type BlockUserResponse,
  GetProfileRequestSchema,
  type GetProfileResponse,
  GetUserLimitsRequestSchema,
  type GetUserLimitsResponse,
  ListProfilesRequestSchema,
  type ListProfilesResponse,
  type Profile,
  ProfilesService,
  SearchProfilesRequestSchema,
  type SearchProfilesResponse,
  UnblockUserRequestSchema,
  type UnblockUserResponse,
  UpdateProfileRequestSchema,
  type UpdateProfileResponse,
  UpdateUserPlanRequestSchema,
  type UpdateUserPlanResponse,
  type UserLimits,
} from '@/gen/profiles/v1/profiles_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

// ============================================================
// Local Types
// ============================================================

export interface LocalProfile {
  id: string;
  email?: string;
  plan?: string;
  phone_number?: string;
  max_automations?: number;
  billing_next_payment_at?: string;
  plan_access_until?: string;
  is_blocked: boolean;
  created_at: string;
  updated_at: string;
}

export interface LocalUserLimits {
  automations_count: number;
  max_automations?: number;
  connected_instances_count: number;
  /** Max distinct destination groups the plan allows (0 = unlimited). */
  group_limit: number;
  /** Current distinct destination groups used across active automations. */
  groups_used: number;
  /** Max WhatsApp chips this account may connect — the same cap the server enforces. */
  max_instances: number;
}

export interface UpdateProfileParams {
  phone_number?: string;
}

export interface UpdateUserPlanParams {
  user_id: string;
  plan?: string;
  max_automations?: number;
  plan_access_until?: string;
}

// ============================================================
// Hook Return Type
// ============================================================

interface UseProfilesConnectReturn {
  profile: LocalProfile | null;
  limits: LocalUserLimits | null;
  loading: boolean;
  refetch: () => Promise<void>;

  getProfile: () => Promise<LocalProfile>;
  updateProfile: (params: UpdateProfileParams) => Promise<LocalProfile>;
  getUserLimits: () => Promise<LocalUserLimits>;

  // Admin operations
  listProfiles: (
    cursor?: string,
    limit?: number
  ) => Promise<{ profiles: LocalProfile[]; nextCursor: string; hasMore: boolean }>;
  searchProfiles: (
    query: string,
    cursor?: string,
    limit?: number
  ) => Promise<{ profiles: LocalProfile[]; nextCursor: string; hasMore: boolean }>;
  blockUser: (userId: string) => Promise<LocalProfile>;
  unblockUser: (userId: string) => Promise<LocalProfile>;
  updateUserPlan: (params: UpdateUserPlanParams) => Promise<LocalProfile>;

  isGettingProfile: boolean;
  isUpdatingProfile: boolean;
  isGettingLimits: boolean;
  isListingProfiles: boolean;
  isSearchingProfiles: boolean;
  isBlockingUser: boolean;
  isUnblockingUser: boolean;
  isUpdatingPlan: boolean;
}

// ============================================================
// Converters
// ============================================================

function fromProtoProfile(profile: Profile): LocalProfile {
  return {
    id: profile.id,
    email: profile.email,
    plan: profile.plan,
    phone_number: profile.phoneNumber,
    max_automations: profile.maxAutomations,
    billing_next_payment_at: profile.billingNextPaymentAt,
    plan_access_until: profile.planAccessUntil,
    is_blocked: profile.isBlocked,
    created_at: profile.createdAt,
    updated_at: profile.updatedAt,
  };
}

function fromProtoUserLimits(limits: UserLimits): LocalUserLimits {
  return {
    automations_count: limits.automationsCount,
    max_automations: limits.maxAutomations,
    connected_instances_count: limits.connectedInstancesCount,
    group_limit: limits.groupLimit,
    groups_used: limits.groupsUsed,
    max_instances: limits.maxInstances,
  };
}

// ============================================================
// Hook Implementation
// ============================================================

/**
 * Hook for profile operations via ConnectRPC.
 * Provides profile management and admin operations.
 */
export function useProfilesConnect(): UseProfilesConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();
  const queryClient = useQueryClient();

  // ============================================================
  // Get Profile Query
  // ============================================================

  const {
    data: profileData,
    isLoading: loadingProfile,
    refetch: refetchProfile,
  } = useQuery({
    queryKey: ['profile', user?.id],
    queryFn: async (): Promise<LocalProfile | null> => {
      if (!user?.id) return null;
      const client = createClient(ProfilesService, transport);
      const response: GetProfileResponse = await client.getProfile(
        create(GetProfileRequestSchema, {})
      );
      return response.profile ? fromProtoProfile(response.profile) : null;
    },
    enabled: !!user?.id,
    staleTime: 60000,
  });

  // ============================================================
  // Get User Limits Query
  // ============================================================

  const {
    data: limitsData,
    isLoading: loadingLimits,
    refetch: refetchLimits,
  } = useQuery({
    queryKey: ['profile', 'limits', user?.id],
    queryFn: async (): Promise<LocalUserLimits | null> => {
      if (!user?.id) return null;
      const client = createClient(ProfilesService, transport);
      const response: GetUserLimitsResponse = await client.getUserLimits(
        create(GetUserLimitsRequestSchema, {})
      );
      return response.limits ? fromProtoUserLimits(response.limits) : null;
    },
    enabled: !!user?.id,
    staleTime: 30000,
  });

  const profile = profileData ?? null;
  const limits = limitsData ?? null;
  const loading = loadingProfile || loadingLimits;

  const refetch = useCallback(async () => {
    await Promise.all([refetchProfile(), refetchLimits()]);
  }, [refetchProfile, refetchLimits]);

  // ============================================================
  // Get Profile Mutation
  // ============================================================

  const getProfileMutation = useMutation({
    mutationFn: async (): Promise<LocalProfile> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(ProfilesService, transport);
      const response: GetProfileResponse = await client.getProfile(
        create(GetProfileRequestSchema, {})
      );

      if (!response.profile) {
        throw new Error(t('pages.profile.errors.notFound'));
      }

      return fromProtoProfile(response.profile);
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Update Profile
  // ============================================================

  const updateProfileMutation = useMutation({
    mutationFn: async (params: UpdateProfileParams): Promise<LocalProfile> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(ProfilesService, transport);
      const response: UpdateProfileResponse = await client.updateProfile(
        create(UpdateProfileRequestSchema, {
          phoneNumber: params.phone_number,
        })
      );

      if (!response.profile) {
        throw new Error(t('pages.profile.errors.updateFailed'));
      }

      return fromProtoProfile(response.profile);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['profile'] });
      toast({
        title: t('pages.profile.success.updated'),
        description: t('pages.profile.success.updatedMessage'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Get User Limits Mutation
  // ============================================================

  const getUserLimitsMutation = useMutation({
    mutationFn: async (): Promise<LocalUserLimits> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(ProfilesService, transport);
      const response: GetUserLimitsResponse = await client.getUserLimits(
        create(GetUserLimitsRequestSchema, {})
      );

      if (!response.limits) {
        throw new Error(t('pages.profile.errors.limitsNotFound'));
      }

      return fromProtoUserLimits(response.limits);
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Admin: List Profiles
  // ============================================================

  const listProfilesMutation = useMutation({
    mutationFn: async (params: {
      cursor?: string;
      limit?: number;
    }): Promise<{ profiles: LocalProfile[]; nextCursor: string; hasMore: boolean }> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(ProfilesService, transport);
      const response: ListProfilesResponse = await client.listProfiles(
        create(ListProfilesRequestSchema, {
          cursor: params.cursor,
          limit: params.limit,
        })
      );

      return {
        profiles: response.profiles.map(fromProtoProfile),
        nextCursor: response.nextCursor ?? '',
        hasMore: response.hasMore,
      };
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Admin: Search Profiles
  // ============================================================

  const searchProfilesMutation = useMutation({
    mutationFn: async (params: {
      query: string;
      cursor?: string;
      limit?: number;
    }): Promise<{ profiles: LocalProfile[]; nextCursor: string; hasMore: boolean }> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(ProfilesService, transport);
      const response: SearchProfilesResponse = await client.searchProfiles(
        create(SearchProfilesRequestSchema, {
          query: params.query,
          cursor: params.cursor,
          limit: params.limit,
        })
      );

      return {
        profiles: response.profiles.map(fromProtoProfile),
        nextCursor: response.nextCursor ?? '',
        hasMore: response.hasMore,
      };
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Admin: Block User
  // ============================================================

  const blockUserMutation = useMutation({
    mutationFn: async (userId: string): Promise<LocalProfile> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(ProfilesService, transport);
      const response: BlockUserResponse = await client.blockUser(
        create(BlockUserRequestSchema, { userId })
      );

      if (!response.profile) {
        throw new Error(t('admin.errors.blockFailed'));
      }

      return fromProtoProfile(response.profile);
    },
    onSuccess: () => {
      toast({
        title: t('admin.success.userBlocked'),
        description: t('admin.success.userBlockedMessage'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Admin: Unblock User
  // ============================================================

  const unblockUserMutation = useMutation({
    mutationFn: async (userId: string): Promise<LocalProfile> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(ProfilesService, transport);
      const response: UnblockUserResponse = await client.unblockUser(
        create(UnblockUserRequestSchema, { userId })
      );

      if (!response.profile) {
        throw new Error(t('admin.errors.unblockFailed'));
      }

      return fromProtoProfile(response.profile);
    },
    onSuccess: () => {
      toast({
        title: t('admin.success.userUnblocked'),
        description: t('admin.success.userUnblockedMessage'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Admin: Update User Plan
  // ============================================================

  const updateUserPlanMutation = useMutation({
    mutationFn: async (params: UpdateUserPlanParams): Promise<LocalProfile> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(ProfilesService, transport);
      const response: UpdateUserPlanResponse = await client.updateUserPlan(
        create(UpdateUserPlanRequestSchema, {
          userId: params.user_id,
          plan: params.plan,
          maxAutomations: params.max_automations,
          planAccessUntil: params.plan_access_until,
        })
      );

      if (!response.profile) {
        throw new Error(t('admin.errors.updatePlanFailed'));
      }

      return fromProtoProfile(response.profile);
    },
    onSuccess: () => {
      toast({
        title: t('admin.success.planUpdated'),
        description: t('admin.success.planUpdatedMessage'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('common.error'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Callback Wrappers
  // ============================================================

  const getProfile = useCallback(async (): Promise<LocalProfile> => {
    return getProfileMutation.mutateAsync();
  }, [getProfileMutation]);

  const updateProfile = useCallback(
    async (params: UpdateProfileParams): Promise<LocalProfile> => {
      return updateProfileMutation.mutateAsync(params);
    },
    [updateProfileMutation]
  );

  const getUserLimits = useCallback(async (): Promise<LocalUserLimits> => {
    return getUserLimitsMutation.mutateAsync();
  }, [getUserLimitsMutation]);

  const listProfiles = useCallback(
    async (
      cursor?: string,
      limit?: number
    ): Promise<{ profiles: LocalProfile[]; nextCursor: string; hasMore: boolean }> => {
      return listProfilesMutation.mutateAsync({ cursor, limit });
    },
    [listProfilesMutation]
  );

  const searchProfiles = useCallback(
    async (
      query: string,
      cursor?: string,
      limit?: number
    ): Promise<{ profiles: LocalProfile[]; nextCursor: string; hasMore: boolean }> => {
      return searchProfilesMutation.mutateAsync({ query, cursor, limit });
    },
    [searchProfilesMutation]
  );

  const blockUser = useCallback(
    async (userId: string): Promise<LocalProfile> => {
      return blockUserMutation.mutateAsync(userId);
    },
    [blockUserMutation]
  );

  const unblockUser = useCallback(
    async (userId: string): Promise<LocalProfile> => {
      return unblockUserMutation.mutateAsync(userId);
    },
    [unblockUserMutation]
  );

  const updateUserPlan = useCallback(
    async (params: UpdateUserPlanParams): Promise<LocalProfile> => {
      return updateUserPlanMutation.mutateAsync(params);
    },
    [updateUserPlanMutation]
  );

  return {
    profile,
    limits,
    loading,
    refetch,

    getProfile,
    updateProfile,
    getUserLimits,
    listProfiles,
    searchProfiles,
    blockUser,
    unblockUser,
    updateUserPlan,

    isGettingProfile: getProfileMutation.isPending,
    isUpdatingProfile: updateProfileMutation.isPending,
    isGettingLimits: getUserLimitsMutation.isPending,
    isListingProfiles: listProfilesMutation.isPending,
    isSearchingProfiles: searchProfilesMutation.isPending,
    isBlockingUser: blockUserMutation.isPending,
    isUnblockingUser: unblockUserMutation.isPending,
    isUpdatingPlan: updateUserPlanMutation.isPending,
  };
}
