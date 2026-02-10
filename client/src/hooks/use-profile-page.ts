import { useAuth } from '@/contexts';
import {
  ChangePasswordRequestSchema,
  type ChangePasswordResponse,
  GetProfileRequestSchema,
  type GetProfileResponse,
  GetUserLimitsRequestSchema,
  type GetUserLimitsResponse,
  type Profile,
  ProfilesService,
  UpdateProfileRequestSchema,
  type UpdateProfileResponse,
  type UserLimits,
} from '@/gen/profiles/v1/profiles_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

// ============================================================
// Cache Policies (per CLAUDE.md)
// ============================================================

const CACHE_PROFILE = 5 * 60 * 1000; // 5 minutes
const CACHE_LIMITS = 30 * 1000; // 30 seconds

// ============================================================
// Local Types
// ============================================================

interface LocalProfile {
  id: string;
  email: string | null;
  plan: string | null;
  phone_number: string | null;
  max_automations: number | null;
  billing_next_payment_at: string | null;
  plan_access_until: string | null;
  is_blocked: boolean;
  created_at: string;
  updated_at: string;
}

interface LocalUserLimits {
  automations_count: number;
  max_automations: number | null;
  connected_instances_count: number;
}

interface UseProfilePageReturn {
  loading: boolean;
  profile: LocalProfile | null;
  automationsCount: number;
  connectedInstancesCount: number;
  phoneDraft: string;
  savingPhone: boolean;
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
  savingPassword: boolean;
  maxAutomationsEffective: number | null;
  automationsLimitText: string;
  setPhoneDraft: (value: string) => void;
  setCurrentPassword: (value: string) => void;
  setNewPassword: (value: string) => void;
  setConfirmPassword: (value: string) => void;
  savePhone: () => Promise<void>;
  savePassword: () => Promise<void>;
  formatDateBR: (iso?: string | null) => string;
}

// ============================================================
// Converters
// ============================================================

function fromProtoProfile(profile: Profile): LocalProfile {
  return {
    id: profile.id,
    email: profile.email ?? null,
    plan: profile.plan ?? null,
    phone_number: profile.phoneNumber ?? null,
    max_automations: profile.maxAutomations ?? null,
    billing_next_payment_at: profile.billingNextPaymentAt ?? null,
    plan_access_until: profile.planAccessUntil ?? null,
    is_blocked: profile.isBlocked,
    created_at: profile.createdAt,
    updated_at: profile.updatedAt,
  };
}

function fromProtoUserLimits(limits: UserLimits): LocalUserLimits {
  return {
    automations_count: limits.automationsCount,
    max_automations: limits.maxAutomations ?? null,
    connected_instances_count: limits.connectedInstancesCount,
  };
}

// ============================================================
// Hook Implementation
// ============================================================

/**
 * Hook for profile page logic.
 * Uses gRPC/ConnectRPC for all server communication.
 * Manages user profile, phone number, and password updates.
 */
export function useProfilePage(): UseProfilePageReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();
  const queryClient = useQueryClient();

  // ============================================================
  // Local State
  // ============================================================

  const [phoneDraft, setPhoneDraft] = useState('');
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');

  // ============================================================
  // Date Formatting
  // ============================================================

  const formatDateBR = useCallback((iso?: string | null): string => {
    if (!iso) return '-';
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return '-';
    return d.toLocaleString('pt-BR');
  }, []);

  // ============================================================
  // Get Profile Query
  // ============================================================

  const {
    data: profileData,
    isLoading: loadingProfile,
    isError: profileError,
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
    staleTime: CACHE_PROFILE,
  });

  // Show error toast when profile fails to load
  useEffect(() => {
    if (profileError) {
      toast({
        variant: 'destructive',
        title: t('pages.profile.errors.loadProfile'),
        description: t('pages.profile.errors.loadProfileMessage'),
      });
    }
  }, [profileError, toast, t]);

  // ============================================================
  // Get User Limits Query
  // ============================================================

  const {
    data: limitsData,
    isLoading: loadingLimits,
    isError: limitsError,
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
    staleTime: CACHE_LIMITS,
  });

  // Show error toast when limits fails to load
  useEffect(() => {
    if (limitsError) {
      toast({
        variant: 'destructive',
        title: t('pages.profile.errors.loadProfile'),
        description: t('pages.profile.errors.loadProfileMessage'),
      });
    }
  }, [limitsError, toast, t]);

  // ============================================================
  // Derived Values
  // ============================================================

  const profile = profileData ?? null;
  const limits = limitsData ?? null;
  const loading = loadingProfile || loadingLimits;
  const automationsCount = limits?.automations_count ?? 0;
  const connectedInstancesCount = limits?.connected_instances_count ?? 0;

  // Sync phone draft with profile
  useEffect(() => {
    if (profile?.phone_number !== undefined) {
      setPhoneDraft(String(profile.phone_number ?? ''));
    }
  }, [profile?.phone_number]);

  const maxAutomationsEffective = useMemo(() => {
    const v = Number(profile?.max_automations);
    if (Number.isFinite(v) && v > 0) return Math.floor(v);
    return null;
  }, [profile?.max_automations]);

  const automationsLimitText = useMemo(() => {
    if (!maxAutomationsEffective) {
      return `${automationsCount} ${t('pages.profile.accountInfo.automations')}`;
    }
    return `${automationsCount}/${maxAutomationsEffective} ${t('pages.profile.accountInfo.automations')}`;
  }, [automationsCount, maxAutomationsEffective, t]);

  // ============================================================
  // Update Phone Mutation
  // ============================================================

  const updatePhoneMutation = useMutation({
    mutationFn: async (phoneNumber: string): Promise<LocalProfile> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(ProfilesService, transport);
      const response: UpdateProfileResponse = await client.updateProfile(
        create(UpdateProfileRequestSchema, {
          phoneNumber: phoneNumber || undefined,
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
        title: t('pages.profile.success.phoneUpdated'),
        description: t('pages.profile.success.phoneUpdatedMessage'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('pages.profile.errors.savePhone'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Change Password Mutation
  // ============================================================

  const changePasswordMutation = useMutation({
    mutationFn: async (params: {
      currentPassword: string;
      newPassword: string;
    }): Promise<boolean> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(ProfilesService, transport);
      const response: ChangePasswordResponse = await client.changePassword(
        create(ChangePasswordRequestSchema, {
          currentPassword: params.currentPassword,
          newPassword: params.newPassword,
        })
      );

      return response.success;
    },
    onSuccess: () => {
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
      toast({
        title: t('pages.profile.success.passwordUpdated'),
        description: t('pages.profile.success.passwordUpdatedMessage'),
      });
    },
    onError: (error: unknown) => {
      const errorInfo = getErrorInfo(error);
      toast({
        variant: 'destructive',
        title: t('pages.profile.errors.updatePassword'),
        description: errorInfo.userMessage,
      });
    },
  });

  // ============================================================
  // Callback Wrappers
  // ============================================================

  const savePhone = useCallback(async (): Promise<void> => {
    const cleaned = phoneDraft.trim();
    await updatePhoneMutation.mutateAsync(cleaned);
  }, [phoneDraft, updatePhoneMutation]);

  const savePassword = useCallback(async (): Promise<void> => {
    // Validate current password
    if (!currentPassword) {
      toast({
        title: t('pages.profile.errors.invalidPassword'),
        description: t('pages.profile.errors.currentPasswordRequired'),
        variant: 'destructive',
      });
      return;
    }

    // Validate new password length
    if (!newPassword || newPassword.length < 8) {
      toast({
        title: t('pages.profile.errors.invalidPassword'),
        description: t('pages.profile.errors.invalidPasswordMessage'),
        variant: 'destructive',
      });
      return;
    }

    // Validate password confirmation
    if (newPassword !== confirmPassword) {
      toast({
        title: t('pages.profile.errors.passwordMismatch'),
        description: t('pages.profile.errors.passwordMismatchMessage'),
        variant: 'destructive',
      });
      return;
    }

    await changePasswordMutation.mutateAsync({
      currentPassword,
      newPassword,
    });
  }, [currentPassword, newPassword, confirmPassword, changePasswordMutation, toast, t]);

  return {
    loading,
    profile,
    automationsCount,
    connectedInstancesCount,
    phoneDraft,
    savingPhone: updatePhoneMutation.isPending,
    currentPassword,
    newPassword,
    confirmPassword,
    savingPassword: changePasswordMutation.isPending,
    maxAutomationsEffective,
    automationsLimitText,
    setPhoneDraft,
    setCurrentPassword,
    setNewPassword,
    setConfirmPassword,
    savePhone,
    savePassword,
    formatDateBR,
  };
}
