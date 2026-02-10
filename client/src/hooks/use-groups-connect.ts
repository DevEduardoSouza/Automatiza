import { useAuth } from '@/contexts';
import {
  AddGroupMemberRequestSchema,
  type AddGroupMemberResponse,
  BulkUpdateGroupsRequestSchema,
  type BulkUpdateGroupsResponse,
  CreateGroupRequestSchema,
  type CreateGroupResponse,
  DeleteGroupRequestSchema,
  DemoteMemberRequestSchema,
  type DemoteMemberResponse,
  GetGroupInviteCodeRequestSchema,
  type GetGroupInviteCodeResponse,
  GetGroupParticipantsRequestSchema,
  type GetGroupParticipantsResponse,
  GetGroupRequestSchema,
  type GetGroupResponse,
  GetGroupsByIdsRequestSchema,
  type GetGroupsByIdsResponse,
  GetSyncStatusRequestSchema,
  type GetSyncStatusResponse,
  type Group,
  GroupStatus,
  GroupsService,
  LeaveGroupRequestSchema,
  type LeaveGroupResponse,
  ListGroupsRequestSchema,
  type ListGroupsResponse,
  PromoteMemberRequestSchema,
  type PromoteMemberResponse,
  type GroupParticipant as ProtoGroupParticipant,
  RemoveGroupMemberRequestSchema,
  type RemoveGroupMemberResponse,
  SendGroupAudioRequestSchema,
  type SendGroupAudioResponse,
  SendGroupImageRequestSchema,
  type SendGroupImageResponse,
  SendGroupMessageRequestSchema,
  type SendGroupMessageResponse,
  SendGroupPollRequestSchema,
  type SendGroupPollResponse,
  SyncGroupsRequestSchema,
  type SyncGroupsResponse,
  ToggleGroupFavoriteRequestSchema,
  type ToggleGroupFavoriteResponse,
  UpdateGroupDescriptionRequestSchema,
  type UpdateGroupDescriptionResponse,
  UpdateGroupPictureRequestSchema,
  type UpdateGroupPictureResponse,
  UpdateGroupRequestSchema,
  type UpdateGroupResponse,
  UpdateGroupSettingsRequestSchema,
  type UpdateGroupSettingsResponse,
  UpdateGroupSubjectRequestSchema,
  type UpdateGroupSubjectResponse,
} from '@/gen/groups/v1/groups_pb';
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

export type LocalGroupStatus = 'open' | 'closed' | 'full';

export interface LocalGroup {
  id: string;
  user_id: string;
  instance_id?: string;
  wa_group_id: string;
  name: string;
  description?: string;
  members_count: number;
  member_limit: number;
  status: LocalGroupStatus;
  invite_code?: string;
  is_admin: boolean;
  is_favorite: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateGroupParams {
  wa_group_id: string;
  name: string;
  description?: string;
  instance_id?: string;
  members_count?: number;
  member_limit?: number;
  status?: LocalGroupStatus;
  invite_code?: string;
  is_admin?: boolean;
}

export interface UpdateGroupParams {
  id: string;
  name?: string;
  description?: string;
  members_count?: number;
  member_limit?: number;
  status?: LocalGroupStatus;
  invite_code?: string;
  is_admin?: boolean;
  is_favorite?: boolean;
}

export interface BulkUpdateGroupsParams {
  ids: string[];
  status?: LocalGroupStatus;
  name?: string;
  is_favorite?: boolean;
}

export interface SyncGroupsResult {
  success: boolean;
  job_id?: string;
  message?: string;
}

export interface SyncStatusResult {
  status: string;
  progress: number;
  error?: string;
  groups_synced: number;
  total_groups?: number;
}

export interface SendMessageParams {
  instance_name: string;
  group_id: string;
  message: string;
  mentions?: string[];
  images?: string[];
  reply_to?: string;
}

export interface SendMessageResult {
  message_id: string;
  action_id: string;
}

export interface SendAudioParams {
  instance_name: string;
  group_id: string;
  audio: string;
  mimetype?: string;
  ptt: boolean;
  caption?: string;
}

export interface SendAudioResult {
  message_id: string;
  action_id: string;
}

export interface SendImageParams {
  instance_name: string;
  group_id: string;
  image: string;
  mimetype?: string;
  caption?: string;
  reply_to?: string;
}

export interface SendImageResult {
  message_id: string;
  action_id: string;
}

export interface SendPollParams {
  instance_name: string;
  group_id: string;
  name: string;
  options: string[];
  multiple_answers?: boolean;
}

export interface SendPollResult {
  message_id: string;
  action_id: string;
}

export interface UpdateSubjectParams {
  instance_name: string;
  group_id: string;
  subject: string;
}

export interface UpdateDescriptionParams {
  instance_name: string;
  group_id: string;
  description: string;
}

export interface UpdatePictureParams {
  instance_name: string;
  group_id: string;
  url?: string;
  base64_data?: string;
  mimetype?: string;
}

export interface UpdateSettingsParams {
  instance_name: string;
  group_id: string;
  messages_admin_only: boolean;
}

export interface GetInviteCodeResult {
  invite_code: string;
  invite_url: string;
}

export interface GroupParticipant {
  id: string;
  is_admin: boolean;
  is_super_admin: boolean;
  phone_number: string;
}

export interface GetGroupParticipantsParams {
  instance_name: string;
  group_id: string;
}

export interface AddGroupMemberParams {
  instance_name: string;
  group_id: string;
  phone_numbers: string[];
}

export interface AddGroupMemberResult {
  added_count: number;
  failed_numbers: string[];
}

export interface RemoveGroupMemberParams {
  instance_name: string;
  group_id: string;
  phone_numbers: string[];
}

export interface RemoveGroupMemberResult {
  removed_count: number;
  failed_numbers: string[];
}

export interface LeaveGroupParams {
  instance_name: string;
  group_id: string;
}

export interface PromoteMemberParams {
  instance_name: string;
  group_id: string;
  phone_number: string;
}

export interface DemoteMemberParams {
  instance_name: string;
  group_id: string;
  phone_number: string;
}

// ============================================================
// Pagination Types
// ============================================================

export interface ListGroupsParams {
  adminOnly?: boolean;
  page?: number;
  pageSize?: number;
  search?: string;
}

export interface PaginatedGroupsResult {
  groups: LocalGroup[];
  totalCount: number;
  page: number;
  pageSize: number;
  totalPages: number;
  hasMore: boolean;
}

// ============================================================
// Hook Return Type
// ============================================================

interface UseGroupsConnectReturn {
  groups: LocalGroup[];
  loading: boolean;
  refetch: () => Promise<void>;

  // CRUD
  listGroups: (adminOnly?: boolean) => Promise<LocalGroup[]>;
  listGroupsPaginated: (params: ListGroupsParams) => Promise<PaginatedGroupsResult>;
  getGroup: (id: string) => Promise<LocalGroup>;
  getGroupsByIds: (ids: string[]) => Promise<LocalGroup[]>;
  createGroup: (params: CreateGroupParams) => Promise<LocalGroup>;
  updateGroup: (params: UpdateGroupParams) => Promise<LocalGroup>;
  toggleGroupFavorite: (id: string, isFavorite: boolean) => Promise<LocalGroup>;
  syncGroups: (instanceName: string) => Promise<SyncGroupsResult>;
  getSyncStatus: (jobId: string) => Promise<SyncStatusResult>;
  deleteGroup: (id: string) => Promise<void>;
  bulkUpdateGroups: (params: BulkUpdateGroupsParams) => Promise<number>;

  // Messaging
  sendGroupMessage: (params: SendMessageParams) => Promise<SendMessageResult>;
  sendGroupAudio: (params: SendAudioParams) => Promise<SendAudioResult>;
  sendGroupImage: (params: SendImageParams) => Promise<SendImageResult>;
  sendGroupPoll: (params: SendPollParams) => Promise<SendPollResult>;

  // Settings
  updateGroupSubject: (params: UpdateSubjectParams) => Promise<boolean>;
  updateGroupDescription: (params: UpdateDescriptionParams) => Promise<boolean>;
  updateGroupPicture: (params: UpdatePictureParams) => Promise<boolean>;
  updateGroupSettings: (params: UpdateSettingsParams) => Promise<boolean>;

  // Utilities
  getGroupInviteCode: (instanceName: string, groupId: string) => Promise<GetInviteCodeResult>;
  getGroupParticipants: (params: GetGroupParticipantsParams) => Promise<GroupParticipant[]>;

  // Member Management
  addGroupMember: (params: AddGroupMemberParams) => Promise<AddGroupMemberResult>;
  removeGroupMember: (params: RemoveGroupMemberParams) => Promise<RemoveGroupMemberResult>;
  leaveGroup: (params: LeaveGroupParams) => Promise<boolean>;
  promoteMember: (params: PromoteMemberParams) => Promise<boolean>;
  demoteMember: (params: DemoteMemberParams) => Promise<boolean>;

  // Loading states - CRUD
  isListingGroups: boolean;
  isGettingGroup: boolean;
  isCreatingGroup: boolean;
  isUpdatingGroup: boolean;
  isTogglingFavorite: boolean;
  isSyncingGroups: boolean;
  isGettingSyncStatus: boolean;
  isDeletingGroup: boolean;
  isBulkUpdating: boolean;

  // Loading states - Messaging
  isSendingMessage: boolean;
  isSendingAudio: boolean;
  isSendingImage: boolean;
  isSendingPoll: boolean;

  // Loading states - Settings
  isUpdatingSubject: boolean;
  isUpdatingDescription: boolean;
  isUpdatingPicture: boolean;
  isUpdatingSettings: boolean;

  // Loading states - Utilities
  isGettingInviteCode: boolean;

  // Loading states - Member Management
  isAddingMember: boolean;
  isRemovingMember: boolean;
  isLeavingGroup: boolean;
  isPromotingMember: boolean;
  isDemotingMember: boolean;
}

// ============================================================
// Converters
// ============================================================

function toProtoStatus(status: LocalGroupStatus): GroupStatus {
  const map: Record<LocalGroupStatus, GroupStatus> = {
    open: GroupStatus.OPEN,
    closed: GroupStatus.CLOSED,
    full: GroupStatus.FULL,
  };
  return map[status] ?? GroupStatus.UNSPECIFIED;
}

function fromProtoStatus(status: GroupStatus): LocalGroupStatus {
  const map: Record<GroupStatus, LocalGroupStatus> = {
    [GroupStatus.UNSPECIFIED]: 'open',
    [GroupStatus.OPEN]: 'open',
    [GroupStatus.CLOSED]: 'closed',
    [GroupStatus.FULL]: 'full',
  };
  return map[status] ?? 'open';
}

function fromProtoGroup(group: Group): LocalGroup {
  return {
    id: group.id,
    user_id: group.userId,
    instance_id: group.instanceId,
    wa_group_id: group.waGroupId,
    name: group.name,
    description: group.description,
    members_count: group.membersCount,
    member_limit: group.memberLimit,
    status: fromProtoStatus(group.status),
    invite_code: group.inviteCode,
    is_admin: group.isAdmin,
    is_favorite: group.isFavorite,
    created_at: group.createdAt,
    updated_at: group.updatedAt,
  };
}

// ============================================================
// Hook Implementation
// ============================================================

/**
 * Hook for group operations via ConnectRPC.
 * Provides CRUD operations and sync functionality.
 */
export function useGroupsConnect(): UseGroupsConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();
  const queryClient = useQueryClient();

  // ============================================================
  // List Groups Query
  // ============================================================

  const {
    data: groupsResponse,
    isLoading: loading,
    refetch: refetchQuery,
  } = useQuery({
    queryKey: ['groups', 'list', user?.id],
    queryFn: async (): Promise<LocalGroup[]> => {
      if (!user?.id) return [];
      const client = createClient(GroupsService, transport);

      // Fetch all groups - admin validation is handled by WAHA when executing actions
      const response: ListGroupsResponse = await client.listGroups(
        create(ListGroupsRequestSchema, { adminOnly: false })
      );
      return response.groups.map(fromProtoGroup);
    },
    enabled: !!user?.id,
    staleTime: 30000,
    refetchInterval: 60000,
  });

  const groups = groupsResponse ?? [];

  const refetch = useCallback(async () => {
    await refetchQuery();
  }, [refetchQuery]);

  // ============================================================
  // List Groups Mutation (for explicit calls)
  // ============================================================

  const listGroupsMutation = useMutation({
    mutationFn: async (adminOnly = false): Promise<LocalGroup[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: ListGroupsResponse = await client.listGroups(
        create(ListGroupsRequestSchema, { adminOnly })
      );

      return response.groups.map(fromProtoGroup);
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
  // List Groups Paginated
  // ============================================================

  const listGroupsPaginatedMutation = useMutation({
    mutationFn: async (params: ListGroupsParams): Promise<PaginatedGroupsResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: ListGroupsResponse = await client.listGroups(
        create(ListGroupsRequestSchema, {
          adminOnly: params.adminOnly,
          page: params.page,
          pageSize: params.pageSize,
          search: params.search,
        })
      );

      return {
        groups: response.groups.map(fromProtoGroup),
        totalCount: response.totalCount,
        page: response.page,
        pageSize: response.pageSize,
        totalPages: response.totalPages,
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
  // Get Group
  // ============================================================

  const getGroupMutation = useMutation({
    mutationFn: async (id: string): Promise<LocalGroup> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: GetGroupResponse = await client.getGroup(
        create(GetGroupRequestSchema, { id })
      );

      if (!response.group) {
        throw new Error(t('pages.groups.errors.notFound'));
      }

      return fromProtoGroup(response.group);
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
  // Get Groups By IDs
  // ============================================================

  const getGroupsByIdsMutation = useMutation({
    mutationFn: async (ids: string[]): Promise<LocalGroup[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: GetGroupsByIdsResponse = await client.getGroupsByIds(
        create(GetGroupsByIdsRequestSchema, { ids })
      );

      return response.groups.map(fromProtoGroup);
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
  // Create Group
  // ============================================================

  const createGroupMutation = useMutation({
    mutationFn: async (params: CreateGroupParams): Promise<LocalGroup> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: CreateGroupResponse = await client.createGroup(
        create(CreateGroupRequestSchema, {
          waGroupId: params.wa_group_id,
          name: params.name,
          description: params.description,
          instanceId: params.instance_id,
          membersCount: params.members_count,
          memberLimit: params.member_limit,
          status: params.status ? toProtoStatus(params.status) : undefined,
          inviteCode: params.invite_code,
          isAdmin: params.is_admin,
        })
      );

      if (!response.group) {
        throw new Error(t('pages.groups.errors.createFailed'));
      }

      return fromProtoGroup(response.group);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
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
  // Update Group
  // ============================================================

  const updateGroupMutation = useMutation({
    mutationFn: async (params: UpdateGroupParams): Promise<LocalGroup> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: UpdateGroupResponse = await client.updateGroup(
        create(UpdateGroupRequestSchema, {
          id: params.id,
          name: params.name,
          description: params.description,
          membersCount: params.members_count,
          memberLimit: params.member_limit,
          status: params.status ? toProtoStatus(params.status) : undefined,
          inviteCode: params.invite_code,
          isAdmin: params.is_admin,
          isFavorite: params.is_favorite,
        })
      );

      if (!response.group) {
        throw new Error(t('pages.groups.errors.updateFailed'));
      }

      return fromProtoGroup(response.group);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
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
  // Toggle Group Favorite
  // ============================================================

  const toggleFavoriteMutation = useMutation({
    mutationFn: async (params: {
      id: string;
      isFavorite: boolean;
    }): Promise<LocalGroup> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: ToggleGroupFavoriteResponse = await client.toggleGroupFavorite(
        create(ToggleGroupFavoriteRequestSchema, {
          id: params.id,
          isFavorite: params.isFavorite,
        })
      );

      if (!response.group) {
        throw new Error(t('pages.groups.errors.updateFailed'));
      }

      return fromProtoGroup(response.group);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
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
  // Sync Groups
  // ============================================================

  const syncGroupsMutation = useMutation({
    mutationFn: async (instanceName: string): Promise<SyncGroupsResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: SyncGroupsResponse = await client.syncGroups(
        create(SyncGroupsRequestSchema, { instanceName })
      );

      return {
        success: response.success,
        job_id: response.jobId,
        message: response.message,
      };
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
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
  // Delete Group
  // ============================================================

  const deleteGroupMutation = useMutation({
    mutationFn: async (id: string): Promise<void> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      await client.deleteGroup(create(DeleteGroupRequestSchema, { id }));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
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
  // Bulk Update Groups
  // ============================================================

  const bulkUpdateMutation = useMutation({
    mutationFn: async (params: BulkUpdateGroupsParams): Promise<number> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: BulkUpdateGroupsResponse = await client.bulkUpdateGroups(
        create(BulkUpdateGroupsRequestSchema, {
          ids: params.ids,
          status: params.status ? toProtoStatus(params.status) : undefined,
          name: params.name,
          isFavorite: params.is_favorite,
        })
      );

      return response.updatedCount;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
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
  // Get Sync Status
  // ============================================================

  const getSyncStatusMutation = useMutation({
    mutationFn: async (jobId: string): Promise<SyncStatusResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: GetSyncStatusResponse = await client.getSyncStatus(
        create(GetSyncStatusRequestSchema, { jobId })
      );

      return {
        status: response.status,
        progress: response.progress,
        error: response.error,
        groups_synced: response.groupsSynced,
        total_groups: response.totalGroups,
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
  // Messaging - Send Group Message
  // ============================================================

  const sendGroupMessageMutation = useMutation({
    mutationFn: async (params: SendMessageParams): Promise<SendMessageResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: SendGroupMessageResponse = await client.sendGroupMessage(
        create(SendGroupMessageRequestSchema, {
          instanceName: params.instance_name,
          groupId: params.group_id,
          message: params.message,
          mentions: params.mentions ?? [],
          images: params.images ?? [],
          replyTo: params.reply_to,
        })
      );

      return {
        message_id: response.messageId,
        action_id: response.actionId,
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
  // Messaging - Send Group Audio
  // ============================================================

  const sendGroupAudioMutation = useMutation({
    mutationFn: async (params: SendAudioParams): Promise<SendAudioResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: SendGroupAudioResponse = await client.sendGroupAudio(
        create(SendGroupAudioRequestSchema, {
          instanceName: params.instance_name,
          groupId: params.group_id,
          audio: params.audio,
          mimetype: params.mimetype,
          ptt: params.ptt,
          caption: params.caption,
        })
      );

      return {
        message_id: response.messageId,
        action_id: response.actionId,
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
  // Messaging - Send Group Image
  // ============================================================

  const sendGroupImageMutation = useMutation({
    mutationFn: async (params: SendImageParams): Promise<SendImageResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: SendGroupImageResponse = await client.sendGroupImage(
        create(SendGroupImageRequestSchema, {
          instanceName: params.instance_name,
          groupId: params.group_id,
          image: params.image,
          mimetype: params.mimetype,
          caption: params.caption,
          replyTo: params.reply_to,
        })
      );

      return {
        message_id: response.messageId,
        action_id: response.actionId,
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
  // Messaging - Send Group Poll
  // ============================================================

  const sendGroupPollMutation = useMutation({
    mutationFn: async (params: SendPollParams): Promise<SendPollResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: SendGroupPollResponse = await client.sendGroupPoll(
        create(SendGroupPollRequestSchema, {
          instanceName: params.instance_name,
          groupId: params.group_id,
          name: params.name,
          options: params.options,
          multipleAnswers: params.multiple_answers ?? false,
        })
      );

      return {
        message_id: response.messageId,
        action_id: response.actionId,
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
  // Settings - Update Group Subject
  // ============================================================

  const updateGroupSubjectMutation = useMutation({
    mutationFn: async (params: UpdateSubjectParams): Promise<boolean> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: UpdateGroupSubjectResponse = await client.updateGroupSubject(
        create(UpdateGroupSubjectRequestSchema, {
          instanceName: params.instance_name,
          groupId: params.group_id,
          subject: params.subject,
        })
      );

      return response.success;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
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
  // Settings - Update Group Description
  // ============================================================

  const updateGroupDescriptionMutation = useMutation({
    mutationFn: async (params: UpdateDescriptionParams): Promise<boolean> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: UpdateGroupDescriptionResponse = await client.updateGroupDescription(
        create(UpdateGroupDescriptionRequestSchema, {
          instanceName: params.instance_name,
          groupId: params.group_id,
          description: params.description,
        })
      );

      return response.success;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
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
  // Settings - Update Group Picture
  // ============================================================

  const updateGroupPictureMutation = useMutation({
    mutationFn: async (params: UpdatePictureParams): Promise<boolean> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: UpdateGroupPictureResponse = await client.updateGroupPicture(
        create(UpdateGroupPictureRequestSchema, {
          instanceName: params.instance_name,
          groupId: params.group_id,
          picture: params.base64_data
            ? { case: 'base64Data', value: params.base64_data }
            : params.url
              ? { case: 'url', value: params.url }
              : { case: undefined, value: undefined },
          mimetype: params.mimetype,
        })
      );

      return response.success;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
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
  // Settings - Update Group Settings
  // ============================================================

  const updateGroupSettingsMutation = useMutation({
    mutationFn: async (params: UpdateSettingsParams): Promise<boolean> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: UpdateGroupSettingsResponse = await client.updateGroupSettings(
        create(UpdateGroupSettingsRequestSchema, {
          instanceName: params.instance_name,
          groupId: params.group_id,
          messagesAdminOnly: params.messages_admin_only,
        })
      );

      return response.success;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
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
  // Utilities - Get Group Invite Code
  // ============================================================

  const getGroupInviteCodeMutation = useMutation({
    mutationFn: async (params: {
      instanceName: string;
      groupId: string;
    }): Promise<GetInviteCodeResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: GetGroupInviteCodeResponse = await client.getGroupInviteCode(
        create(GetGroupInviteCodeRequestSchema, {
          instanceName: params.instanceName,
          groupId: params.groupId,
        })
      );

      return {
        invite_code: response.inviteCode,
        invite_url: response.inviteUrl,
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
  // Get Group Participants
  // ============================================================

  const getGroupParticipantsMutation = useMutation({
    mutationFn: async (params: GetGroupParticipantsParams): Promise<GroupParticipant[]> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: GetGroupParticipantsResponse = await client.getGroupParticipants(
        create(GetGroupParticipantsRequestSchema, {
          instanceName: params.instance_name,
          groupId: params.group_id,
        })
      );

      return response.participants.map((p: ProtoGroupParticipant) => ({
        id: p.id,
        is_admin: p.isAdmin,
        is_super_admin: p.isSuperAdmin,
        phone_number: p.phoneNumber,
      }));
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
  // Member Management Mutations
  // ============================================================

  const addGroupMemberMutation = useMutation({
    mutationFn: async (params: AddGroupMemberParams): Promise<AddGroupMemberResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: AddGroupMemberResponse = await client.addGroupMember(
        create(AddGroupMemberRequestSchema, {
          instanceName: params.instance_name,
          groupId: params.group_id,
          phoneNumbers: params.phone_numbers,
        })
      );

      return {
        added_count: response.addedCount,
        failed_numbers: [...response.failedNumbers],
      };
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
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

  const removeGroupMemberMutation = useMutation({
    mutationFn: async (params: RemoveGroupMemberParams): Promise<RemoveGroupMemberResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: RemoveGroupMemberResponse = await client.removeGroupMember(
        create(RemoveGroupMemberRequestSchema, {
          instanceName: params.instance_name,
          groupId: params.group_id,
          phoneNumbers: params.phone_numbers,
        })
      );

      return {
        removed_count: response.removedCount,
        failed_numbers: [...response.failedNumbers],
      };
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
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

  const leaveGroupMutation = useMutation({
    mutationFn: async (params: LeaveGroupParams): Promise<boolean> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: LeaveGroupResponse = await client.leaveGroup(
        create(LeaveGroupRequestSchema, {
          instanceName: params.instance_name,
          groupId: params.group_id,
        })
      );

      return response.success;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
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

  const promoteMemberMutation = useMutation({
    mutationFn: async (params: PromoteMemberParams): Promise<boolean> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: PromoteMemberResponse = await client.promoteMember(
        create(PromoteMemberRequestSchema, {
          instanceName: params.instance_name,
          groupId: params.group_id,
          phoneNumber: params.phone_number,
        })
      );

      return response.success;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
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

  const demoteMemberMutation = useMutation({
    mutationFn: async (params: DemoteMemberParams): Promise<boolean> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(GroupsService, transport);
      const response: DemoteMemberResponse = await client.demoteMember(
        create(DemoteMemberRequestSchema, {
          instanceName: params.instance_name,
          groupId: params.group_id,
          phoneNumber: params.phone_number,
        })
      );

      return response.success;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
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

  const listGroups = useCallback(
    async (adminOnly = false): Promise<LocalGroup[]> => {
      return listGroupsMutation.mutateAsync(adminOnly);
    },
    [listGroupsMutation.mutateAsync]
  );

  const listGroupsPaginated = useCallback(
    async (params: ListGroupsParams): Promise<PaginatedGroupsResult> => {
      return listGroupsPaginatedMutation.mutateAsync(params);
    },
    [listGroupsPaginatedMutation.mutateAsync]
  );

  const getGroup = useCallback(
    async (id: string): Promise<LocalGroup> => {
      return getGroupMutation.mutateAsync(id);
    },
    [getGroupMutation.mutateAsync]
  );

  const getGroupsByIds = useCallback(
    async (ids: string[]): Promise<LocalGroup[]> => {
      return getGroupsByIdsMutation.mutateAsync(ids);
    },
    [getGroupsByIdsMutation.mutateAsync]
  );

  const createGroup = useCallback(
    async (params: CreateGroupParams): Promise<LocalGroup> => {
      return createGroupMutation.mutateAsync(params);
    },
    [createGroupMutation.mutateAsync]
  );

  const updateGroup = useCallback(
    async (params: UpdateGroupParams): Promise<LocalGroup> => {
      return updateGroupMutation.mutateAsync(params);
    },
    [updateGroupMutation.mutateAsync]
  );

  const toggleGroupFavorite = useCallback(
    async (id: string, isFavorite: boolean): Promise<LocalGroup> => {
      return toggleFavoriteMutation.mutateAsync({ id, isFavorite });
    },
    [toggleFavoriteMutation.mutateAsync]
  );

  const syncGroups = useCallback(
    async (instanceName: string): Promise<SyncGroupsResult> => {
      return syncGroupsMutation.mutateAsync(instanceName);
    },
    [syncGroupsMutation.mutateAsync]
  );

  const getSyncStatus = useCallback(
    async (jobId: string): Promise<SyncStatusResult> => {
      return getSyncStatusMutation.mutateAsync(jobId);
    },
    [getSyncStatusMutation.mutateAsync]
  );

  const deleteGroup = useCallback(
    async (id: string): Promise<void> => {
      return deleteGroupMutation.mutateAsync(id);
    },
    [deleteGroupMutation.mutateAsync]
  );

  const bulkUpdateGroups = useCallback(
    async (params: BulkUpdateGroupsParams): Promise<number> => {
      return bulkUpdateMutation.mutateAsync(params);
    },
    [bulkUpdateMutation.mutateAsync]
  );

  // Messaging callbacks
  const sendGroupMessage = useCallback(
    async (params: SendMessageParams): Promise<SendMessageResult> => {
      return sendGroupMessageMutation.mutateAsync(params);
    },
    [sendGroupMessageMutation.mutateAsync]
  );

  const sendGroupAudio = useCallback(
    async (params: SendAudioParams): Promise<SendAudioResult> => {
      return sendGroupAudioMutation.mutateAsync(params);
    },
    [sendGroupAudioMutation.mutateAsync]
  );

  const sendGroupImage = useCallback(
    async (params: SendImageParams): Promise<SendImageResult> => {
      return sendGroupImageMutation.mutateAsync(params);
    },
    [sendGroupImageMutation.mutateAsync]
  );

  const sendGroupPoll = useCallback(
    async (params: SendPollParams): Promise<SendPollResult> => {
      return sendGroupPollMutation.mutateAsync(params);
    },
    [sendGroupPollMutation.mutateAsync]
  );

  // Settings callbacks
  const updateGroupSubject = useCallback(
    async (params: UpdateSubjectParams): Promise<boolean> => {
      return updateGroupSubjectMutation.mutateAsync(params);
    },
    [updateGroupSubjectMutation.mutateAsync]
  );

  const updateGroupDescription = useCallback(
    async (params: UpdateDescriptionParams): Promise<boolean> => {
      return updateGroupDescriptionMutation.mutateAsync(params);
    },
    [updateGroupDescriptionMutation.mutateAsync]
  );

  const updateGroupPicture = useCallback(
    async (params: UpdatePictureParams): Promise<boolean> => {
      return updateGroupPictureMutation.mutateAsync(params);
    },
    [updateGroupPictureMutation.mutateAsync]
  );

  const updateGroupSettings = useCallback(
    async (params: UpdateSettingsParams): Promise<boolean> => {
      return updateGroupSettingsMutation.mutateAsync(params);
    },
    [updateGroupSettingsMutation.mutateAsync]
  );

  // Utility callbacks
  const getGroupInviteCode = useCallback(
    async (instanceName: string, groupId: string): Promise<GetInviteCodeResult> => {
      return getGroupInviteCodeMutation.mutateAsync({ instanceName, groupId });
    },
    [getGroupInviteCodeMutation.mutateAsync]
  );

  const getGroupParticipants = useCallback(
    async (params: GetGroupParticipantsParams): Promise<GroupParticipant[]> => {
      return getGroupParticipantsMutation.mutateAsync(params);
    },
    [getGroupParticipantsMutation.mutateAsync]
  );

  // Member Management callbacks
  const addGroupMember = useCallback(
    async (params: AddGroupMemberParams): Promise<AddGroupMemberResult> => {
      return addGroupMemberMutation.mutateAsync(params);
    },
    [addGroupMemberMutation.mutateAsync]
  );

  const removeGroupMember = useCallback(
    async (params: RemoveGroupMemberParams): Promise<RemoveGroupMemberResult> => {
      return removeGroupMemberMutation.mutateAsync(params);
    },
    [removeGroupMemberMutation.mutateAsync]
  );

  const leaveGroup = useCallback(
    async (params: LeaveGroupParams): Promise<boolean> => {
      return leaveGroupMutation.mutateAsync(params);
    },
    [leaveGroupMutation.mutateAsync]
  );

  const promoteMember = useCallback(
    async (params: PromoteMemberParams): Promise<boolean> => {
      return promoteMemberMutation.mutateAsync(params);
    },
    [promoteMemberMutation.mutateAsync]
  );

  const demoteMember = useCallback(
    async (params: DemoteMemberParams): Promise<boolean> => {
      return demoteMemberMutation.mutateAsync(params);
    },
    [demoteMemberMutation.mutateAsync]
  );

  return {
    groups,
    loading,
    refetch,

    // CRUD
    listGroups,
    listGroupsPaginated,
    getGroup,
    getGroupsByIds,
    createGroup,
    updateGroup,
    toggleGroupFavorite,
    syncGroups,
    getSyncStatus,
    deleteGroup,
    bulkUpdateGroups,

    // Messaging
    sendGroupMessage,
    sendGroupAudio,
    sendGroupImage,
    sendGroupPoll,

    // Settings
    updateGroupSubject,
    updateGroupDescription,
    updateGroupPicture,
    updateGroupSettings,

    // Utilities
    getGroupInviteCode,
    getGroupParticipants,

    // Member Management
    addGroupMember,
    removeGroupMember,
    leaveGroup,
    promoteMember,
    demoteMember,

    // Loading states - CRUD
    isListingGroups: listGroupsMutation.isPending,
    isGettingGroup: getGroupMutation.isPending,
    isCreatingGroup: createGroupMutation.isPending,
    isUpdatingGroup: updateGroupMutation.isPending,
    isTogglingFavorite: toggleFavoriteMutation.isPending,
    isSyncingGroups: syncGroupsMutation.isPending,
    isGettingSyncStatus: getSyncStatusMutation.isPending,
    isDeletingGroup: deleteGroupMutation.isPending,
    isBulkUpdating: bulkUpdateMutation.isPending,

    // Loading states - Messaging
    isSendingMessage: sendGroupMessageMutation.isPending,
    isSendingAudio: sendGroupAudioMutation.isPending,
    isSendingImage: sendGroupImageMutation.isPending,
    isSendingPoll: sendGroupPollMutation.isPending,

    // Loading states - Settings
    isUpdatingSubject: updateGroupSubjectMutation.isPending,
    isUpdatingDescription: updateGroupDescriptionMutation.isPending,
    isUpdatingPicture: updateGroupPictureMutation.isPending,
    isUpdatingSettings: updateGroupSettingsMutation.isPending,

    // Loading states - Utilities
    isGettingInviteCode: getGroupInviteCodeMutation.isPending,

    // Loading states - Member Management
    isAddingMember: addGroupMemberMutation.isPending,
    isRemovingMember: removeGroupMemberMutation.isPending,
    isLeavingGroup: leaveGroupMutation.isPending,
    isPromotingMember: promoteMemberMutation.isPending,
    isDemotingMember: demoteMemberMutation.isPending,
  };
}
