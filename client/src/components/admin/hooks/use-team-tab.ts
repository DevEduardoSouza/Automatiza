import { useAuth } from '@/contexts';
import {
  AdminService,
  GrantAdminRoleRequestSchema,
  ListAdminUsersRequestSchema,
  RevokeAdminRoleRequestSchema,
} from '@/gen/admin/v1/admin_pb';
import { ProfilesService, SearchProfilesRequestSchema } from '@/gen/profiles/v1/profiles_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import type { TeamMember, UseTeamTabReturn } from '../types/team.types';

/**
 * Hook que gerencia a logica da aba de equipe administrativa.
 * Permite listar, adicionar e remover administradores do sistema.
 */
export function useTeamTab(): UseTeamTabReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const transport = useTransport();
  const [teamMembers, setTeamMembers] = useState<TeamMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [newAdminEmail, setNewAdminEmail] = useState('');

  const fetchTeamMembers = useCallback(async () => {
    if (!user) {
      setLoading(false);
      return;
    }

    setLoading(true);
    try {
      const client = createClient(AdminService, transport);
      const response = await client.listAdminUsers(create(ListAdminUsersRequestSchema, {}));
      setTeamMembers(
        response.admins.map((admin) => ({
          id: admin.userId,
          user_id: admin.userId,
          role: admin.role,
          created_at: admin.grantedAt,
          email: admin.email,
        }))
      );
    } catch (error) {
      const errInfo = getErrorInfo(error);
      toast.error(errInfo.userMessage || t('admin.team.errors.loadError'));
    } finally {
      setLoading(false);
    }
  }, [t, transport, user]);

  useEffect(() => {
    fetchTeamMembers();
  }, [fetchTeamMembers]);

  const handleAddAdmin = async () => {
    if (!newAdminEmail) {
      toast.error(t('admin.team.errors.emailRequired'));
      return;
    }

    try {
      const profilesClient = createClient(ProfilesService, transport);
      const searchResponse = await profilesClient.searchProfiles(
        create(SearchProfilesRequestSchema, { query: newAdminEmail, limit: 1 })
      );

      if (!searchResponse.profiles.length) {
        toast.error(t('admin.team.errors.userNotFound'));
        return;
      }

      const foundProfile = searchResponse.profiles[0];

      const adminClient = createClient(AdminService, transport);
      await adminClient.grantAdminRole(
        create(GrantAdminRoleRequestSchema, { userId: foundProfile.id })
      );

      toast.success(t('admin.team.success.added'));
      setIsDialogOpen(false);
      setNewAdminEmail('');
      fetchTeamMembers();
    } catch (error) {
      const errInfo = getErrorInfo(error);
      if (errInfo.code === 'ALREADY_EXISTS') {
        toast.error(t('admin.team.errors.alreadyAdmin'));
      } else {
        toast.error(errInfo.userMessage || t('admin.team.errors.addError'));
      }
    }
  };

  const handleRemoveAdmin = async (id: string, email: string) => {
    if (!confirm(t('admin.team.confirm.remove', { email }))) return;

    try {
      const client = createClient(AdminService, transport);
      await client.revokeAdminRole(create(RevokeAdminRoleRequestSchema, { userId: id }));

      toast.success(t('admin.team.success.removed'));
      fetchTeamMembers();
    } catch (error) {
      const errInfo = getErrorInfo(error);
      toast.error(errInfo.userMessage || t('admin.team.errors.removeError'));
    }
  };

  return {
    teamMembers,
    loading,
    isDialogOpen,
    setIsDialogOpen,
    newAdminEmail,
    setNewAdminEmail,
    handleAddAdmin,
    handleRemoveAdmin,
  };
}
