import { useAuth } from '@/contexts';
import {
  CheckStatusRequestSchema,
  type CheckStatusResponse,
  CreateInstanceRequestSchema,
  type CreateInstanceResponse,
  DeleteInstanceRequestSchema,
  type DeleteInstanceResponse,
  FetchGroupParticipantsRequestSchema,
  type FetchGroupParticipantsResponse,
  FetchGroupsRequestSchema,
  type FetchGroupsResponse,
  type GroupParticipant,
  GroupSettingsAction,
  InstanceState,
  ReconnectInstanceRequestSchema,
  type ReconnectInstanceResponse,
  RequestPairingCodeRequestSchema,
  type RequestPairingCodeResponse,
  SendAudioRequestSchema,
  type SendAudioResponse,
  SendMessageRequestSchema,
  type SendMessageResponse,
  SendPollRequestSchema,
  type SendPollResponse,
  SyncGroupsRequestSchema,
  type SyncGroupsResponse,
  SyncGroupsStatusRequestSchema,
  type SyncGroupsStatusResponse,
  SyncJobState,
  UpdateGroupDescriptionRequestSchema,
  type UpdateGroupDescriptionResponse,
  UpdateGroupPictureRequestSchema,
  type UpdateGroupPictureResponse,
  UpdateGroupSettingsRequestSchema,
  type UpdateGroupSettingsResponse,
  UpdateGroupSubjectRequestSchema,
  type UpdateGroupSubjectResponse,
  type WhatsAppGroup,
  WhatsAppService,
} from '@/gen/whatsapp/v1/whatsapp_pb';
import { getErrorInfo } from '@/lib/connect-transport';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useMutation } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useToast } from './use-toast';

// ============================================================
// Local Types - Instance Management
// ============================================================

/**
 * Result of creating a WhatsApp instance.
 */
export interface LocalCreateInstanceResult {
  success: boolean;
  instanceName?: string;
  friendlyName?: string;
  qrcodeBase64?: string;
  error?: string;
}

/**
 * Result of deleting a WhatsApp instance.
 */
export interface LocalDeleteInstanceResult {
  success: boolean;
  message?: string;
}

/**
 * Result of reconnecting a WhatsApp instance.
 */
export interface LocalReconnectInstanceResult {
  success: boolean;
  qrcodeBase64?: string;
  error?: string;
}

/**
 * Instance connection state (local enum mapping).
 */
export type LocalInstanceState = 'unspecified' | 'open' | 'close' | 'connecting';

/**
 * Result of checking instance status.
 */
export interface LocalCheckStatusResult {
  success: boolean;
  state: LocalInstanceState;
  qrcodeBase64?: string;
}

/**
 * Result of requesting a phone pairing code (alternative to QR).
 */
export interface LocalRequestPairingCodeResult {
  success: boolean;
  code?: string;
  error?: string;
}

// ============================================================
// Local Types - Group Management
// ============================================================

/**
 * Local WhatsApp group interface.
 */
export interface LocalWhatsAppGroup {
  id: string;
  subject?: string;
  description?: string;
  size?: number;
  restrict?: boolean;
  announce?: boolean;
}

/**
 * Result of fetching groups.
 */
export interface LocalFetchGroupsResult {
  success: boolean;
  groups: LocalWhatsAppGroup[];
}

/**
 * Local group participant interface.
 */
export interface LocalGroupParticipant {
  id: string;
  name?: string;
  isAdmin?: boolean;
}

/**
 * Result of fetching group participants.
 */
export interface LocalFetchGroupParticipantsResult {
  success: boolean;
  participants: LocalGroupParticipant[];
}

/**
 * Result of starting group sync.
 */
export interface LocalSyncGroupsResult {
  success: boolean;
  jobId: string;
}

/**
 * Sync job state (local enum mapping).
 */
export type LocalSyncJobState = 'unspecified' | 'pending' | 'in_progress' | 'completed' | 'failed';

/**
 * Result of checking sync job status.
 */
export interface LocalSyncGroupsStatusResult {
  state: LocalSyncJobState;
  progress?: number;
  failedReason?: string;
}

// ============================================================
// Local Types - Messaging
// ============================================================

/**
 * Parameters for sending a message.
 */
export interface SendMessageParams {
  instanceName: string;
  groupId: string;
  message: string;
  mentions?: string[];
  images?: string[];
}

/**
 * Result of sending a message.
 */
export interface LocalSendMessageResult {
  success: boolean;
  messageId?: string;
  error?: string;
}

/**
 * Parameters for sending an audio message.
 */
export interface SendAudioParams {
  instanceName: string;
  groupId: string;
  audio: string;
  mimetype?: string;
  ptt?: boolean;
  caption?: string;
}

/**
 * Result of sending an audio message.
 */
export interface LocalSendAudioResult {
  success: boolean;
  messageId?: string;
  error?: string;
}

/**
 * Parameters for sending a poll.
 */
export interface SendPollParams {
  instanceName: string;
  groupId: string;
  name: string;
  options: string[];
  selectableCount?: number;
}

/**
 * Result of sending a poll.
 */
export interface LocalSendPollResult {
  success: boolean;
  messageId?: string;
  error?: string;
}

// ============================================================
// Local Types - Group Updates
// ============================================================

/**
 * Parameters for updating group subject.
 */
export interface UpdateGroupSubjectParams {
  instanceName: string;
  groupId: string;
  subject: string;
}

/**
 * Result of updating group subject.
 */
export interface LocalUpdateGroupResult {
  success: boolean;
  error?: string;
}

/**
 * Parameters for updating group description.
 */
export interface UpdateGroupDescriptionParams {
  instanceName: string;
  groupId: string;
  description: string;
}

/**
 * Parameters for updating group picture.
 */
export interface UpdateGroupPictureParams {
  instanceName: string;
  groupId: string;
  imageUrl: string;
}

/**
 * Group settings action (local type).
 */
export type LocalGroupSettingsAction = 'announcement' | 'not_announcement';

/**
 * Parameters for updating group settings.
 */
export interface UpdateGroupSettingsParams {
  instanceName: string;
  groupId: string;
  action: LocalGroupSettingsAction;
}

// ============================================================
// Hook Return Type
// ============================================================

interface UseWhatsAppConnectReturn {
  // Instance Management
  createInstance: (instanceName?: string) => Promise<LocalCreateInstanceResult>;
  deleteInstance: (instanceName: string) => Promise<LocalDeleteInstanceResult>;
  reconnectInstance: (instanceName: string) => Promise<LocalReconnectInstanceResult>;
  checkStatus: (instanceName: string, skipQr?: boolean) => Promise<LocalCheckStatusResult>;
  requestPairingCode: (params: {
    instanceName: string;
    phoneNumber: string;
  }) => Promise<LocalRequestPairingCodeResult>;

  // Group Management
  fetchGroups: (instanceName: string) => Promise<LocalFetchGroupsResult>;
  fetchGroupParticipants: (
    instanceName: string,
    groupId: string
  ) => Promise<LocalFetchGroupParticipantsResult>;
  syncGroups: (instanceName: string) => Promise<LocalSyncGroupsResult>;
  syncGroupsStatus: (jobId: string) => Promise<LocalSyncGroupsStatusResult>;

  // Messaging
  sendMessage: (params: SendMessageParams) => Promise<LocalSendMessageResult>;
  sendAudio: (params: SendAudioParams) => Promise<LocalSendAudioResult>;
  sendPoll: (params: SendPollParams) => Promise<LocalSendPollResult>;

  // Group Updates
  updateGroupSubject: (params: UpdateGroupSubjectParams) => Promise<LocalUpdateGroupResult>;
  updateGroupDescription: (params: UpdateGroupDescriptionParams) => Promise<LocalUpdateGroupResult>;
  updateGroupPicture: (params: UpdateGroupPictureParams) => Promise<LocalUpdateGroupResult>;
  updateGroupSettings: (params: UpdateGroupSettingsParams) => Promise<LocalUpdateGroupResult>;

  // Loading States
  isCreatingInstance: boolean;
  isDeletingInstance: boolean;
  isReconnectingInstance: boolean;
  isCheckingStatus: boolean;
  isFetchingGroups: boolean;
  isFetchingParticipants: boolean;
  isSyncingGroups: boolean;
  isCheckingSyncStatus: boolean;
  isSendingMessage: boolean;
  isSendingAudio: boolean;
  isSendingPoll: boolean;
  isUpdatingSubject: boolean;
  isUpdatingDescription: boolean;
  isUpdatingPicture: boolean;
  isUpdatingSettings: boolean;
}

// ============================================================
// Converters
// ============================================================

function fromProtoInstanceState(state: InstanceState): LocalInstanceState {
  switch (state) {
    case InstanceState.OPEN:
      return 'open';
    case InstanceState.CLOSE:
      return 'close';
    case InstanceState.CONNECTING:
      return 'connecting';
    default:
      return 'unspecified';
  }
}

function fromProtoSyncJobState(state: SyncJobState): LocalSyncJobState {
  switch (state) {
    case SyncJobState.PENDING:
      return 'pending';
    case SyncJobState.IN_PROGRESS:
      return 'in_progress';
    case SyncJobState.COMPLETED:
      return 'completed';
    case SyncJobState.FAILED:
      return 'failed';
    default:
      return 'unspecified';
  }
}

function toProtoGroupSettingsAction(action: LocalGroupSettingsAction): GroupSettingsAction {
  switch (action) {
    case 'announcement':
      return GroupSettingsAction.ANNOUNCEMENT;
    case 'not_announcement':
      return GroupSettingsAction.NOT_ANNOUNCEMENT;
    default:
      return GroupSettingsAction.UNSPECIFIED;
  }
}

function fromProtoGroup(group: WhatsAppGroup): LocalWhatsAppGroup {
  return {
    id: group.id,
    subject: group.subject,
    description: group.description,
    size: group.size,
    restrict: group.restrict,
    announce: group.announce,
  };
}

function fromProtoParticipant(participant: GroupParticipant): LocalGroupParticipant {
  return {
    id: participant.id,
    name: participant.name,
    isAdmin: participant.isAdmin,
  };
}

function fromProtoCreateInstanceResponse(
  response: CreateInstanceResponse
): LocalCreateInstanceResult {
  return {
    success: response.success,
    instanceName: response.instanceName,
    friendlyName: response.friendlyName,
    qrcodeBase64: response.qrcodeBase64,
    error: response.error,
  };
}

function fromProtoDeleteInstanceResponse(
  response: DeleteInstanceResponse
): LocalDeleteInstanceResult {
  return {
    success: response.success,
    message: response.message,
  };
}

function fromProtoReconnectInstanceResponse(
  response: ReconnectInstanceResponse
): LocalReconnectInstanceResult {
  return {
    success: response.success,
    qrcodeBase64: response.qrcodeBase64,
    error: response.error,
  };
}

function fromProtoCheckStatusResponse(response: CheckStatusResponse): LocalCheckStatusResult {
  return {
    success: response.success,
    state: fromProtoInstanceState(response.state),
    qrcodeBase64: response.qrcodeBase64,
  };
}

function fromProtoFetchGroupsResponse(response: FetchGroupsResponse): LocalFetchGroupsResult {
  return {
    success: response.success,
    groups: response.groups.map(fromProtoGroup),
  };
}

function fromProtoFetchGroupParticipantsResponse(
  response: FetchGroupParticipantsResponse
): LocalFetchGroupParticipantsResult {
  return {
    success: response.success,
    participants: response.participants.map(fromProtoParticipant),
  };
}

function fromProtoSyncGroupsResponse(response: SyncGroupsResponse): LocalSyncGroupsResult {
  return {
    success: response.success,
    jobId: response.jobId,
  };
}

function fromProtoSyncGroupsStatusResponse(
  response: SyncGroupsStatusResponse
): LocalSyncGroupsStatusResult {
  return {
    state: fromProtoSyncJobState(response.state),
    progress: response.progress,
    failedReason: response.failedReason,
  };
}

function fromProtoSendMessageResponse(response: SendMessageResponse): LocalSendMessageResult {
  return {
    success: response.success,
    messageId: response.messageId,
    error: response.error,
  };
}

function fromProtoSendAudioResponse(response: SendAudioResponse): LocalSendAudioResult {
  return {
    success: response.success,
    messageId: response.messageId,
    error: response.error,
  };
}

function fromProtoSendPollResponse(response: SendPollResponse): LocalSendPollResult {
  return {
    success: response.success,
    messageId: response.messageId,
    error: response.error,
  };
}

function fromProtoUpdateGroupSubjectResponse(
  response: UpdateGroupSubjectResponse
): LocalUpdateGroupResult {
  return {
    success: response.success,
    error: response.error,
  };
}

function fromProtoUpdateGroupDescriptionResponse(
  response: UpdateGroupDescriptionResponse
): LocalUpdateGroupResult {
  return {
    success: response.success,
    error: response.error,
  };
}

function fromProtoUpdateGroupPictureResponse(
  response: UpdateGroupPictureResponse
): LocalUpdateGroupResult {
  return {
    success: response.success,
    error: response.error,
  };
}

function fromProtoUpdateGroupSettingsResponse(
  response: UpdateGroupSettingsResponse
): LocalUpdateGroupResult {
  return {
    success: response.success,
    error: response.error,
  };
}

// ============================================================
// Hook Implementation
// ============================================================

/**
 * Hook for WhatsApp operations via ConnectRPC.
 * Provides instance management, group management, messaging, and group updates.
 *
 * Canonical hook for all WhatsApp operations.
 */
export function useWhatsAppConnect(): UseWhatsAppConnectReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const transport = useTransport();

  // ============================================================
  // Instance Management Mutations
  // ============================================================

  const createInstanceMutation = useMutation({
    mutationFn: async (instanceName?: string): Promise<LocalCreateInstanceResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: CreateInstanceResponse = await client.createInstance(
        create(CreateInstanceRequestSchema, {
          instanceName,
        })
      );

      return fromProtoCreateInstanceResponse(response);
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

  const deleteInstanceMutation = useMutation({
    mutationFn: async (instanceName: string): Promise<LocalDeleteInstanceResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: DeleteInstanceResponse = await client.deleteInstance(
        create(DeleteInstanceRequestSchema, {
          instanceName,
        })
      );

      return fromProtoDeleteInstanceResponse(response);
    },
    // No onError toast — all callers handle errors in their own try/catch
    // (disconnect, QR refresh, handleConnect all ignore delete failures).
  });

  const reconnectInstanceMutation = useMutation({
    mutationFn: async (instanceName: string): Promise<LocalReconnectInstanceResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: ReconnectInstanceResponse = await client.reconnectInstance(
        create(ReconnectInstanceRequestSchema, {
          instanceName,
        })
      );

      return fromProtoReconnectInstanceResponse(response);
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

  const checkStatusMutation = useMutation({
    mutationFn: async ({
      instanceName,
      skipQr,
    }: {
      instanceName: string;
      skipQr?: boolean;
    }): Promise<LocalCheckStatusResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: CheckStatusResponse = await client.checkStatus(
        create(CheckStatusRequestSchema, {
          instanceName,
          skipQr,
        })
      );

      return fromProtoCheckStatusResponse(response);
    },
    // No onError toast — callers (polling, verify, loadConnectionState) all
    // handle errors silently. Showing a toast here causes spurious error
    // notifications when the session is temporarily unavailable during
    // disconnect or QR refresh.
  });

  const requestPairingCodeMutation = useMutation({
    mutationFn: async ({
      instanceName,
      phoneNumber,
    }: {
      instanceName: string;
      phoneNumber: string;
    }): Promise<LocalRequestPairingCodeResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: RequestPairingCodeResponse = await client.requestPairingCode(
        create(RequestPairingCodeRequestSchema, {
          instanceName,
          phoneNumber,
        })
      );

      return { success: response.success, code: response.code, error: response.error };
    },
    // No onError toast — the AddChipModal surfaces errors inline in the code panel.
  });

  // ============================================================
  // Group Management Mutations
  // ============================================================

  const fetchGroupsMutation = useMutation({
    mutationFn: async (instanceName: string): Promise<LocalFetchGroupsResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: FetchGroupsResponse = await client.fetchGroups(
        create(FetchGroupsRequestSchema, {
          instanceName,
        })
      );

      return fromProtoFetchGroupsResponse(response);
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

  const fetchGroupParticipantsMutation = useMutation({
    mutationFn: async (params: {
      instanceName: string;
      groupId: string;
    }): Promise<LocalFetchGroupParticipantsResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: FetchGroupParticipantsResponse = await client.fetchGroupParticipants(
        create(FetchGroupParticipantsRequestSchema, {
          instanceName: params.instanceName,
          groupId: params.groupId,
        })
      );

      return fromProtoFetchGroupParticipantsResponse(response);
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

  const syncGroupsMutation = useMutation({
    mutationFn: async (instanceName: string): Promise<LocalSyncGroupsResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: SyncGroupsResponse = await client.syncGroups(
        create(SyncGroupsRequestSchema, {
          instanceName,
        })
      );

      return fromProtoSyncGroupsResponse(response);
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

  const syncGroupsStatusMutation = useMutation({
    mutationFn: async (jobId: string): Promise<LocalSyncGroupsStatusResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: SyncGroupsStatusResponse = await client.syncGroupsStatus(
        create(SyncGroupsStatusRequestSchema, {
          jobId,
        })
      );

      return fromProtoSyncGroupsStatusResponse(response);
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
  // Messaging Mutations
  // ============================================================

  const sendMessageMutation = useMutation({
    mutationFn: async (params: SendMessageParams): Promise<LocalSendMessageResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: SendMessageResponse = await client.sendMessage(
        create(SendMessageRequestSchema, {
          instanceName: params.instanceName,
          groupId: params.groupId,
          message: params.message,
          mentions: params.mentions ?? [],
          images: params.images ?? [],
        })
      );

      return fromProtoSendMessageResponse(response);
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

  const sendAudioMutation = useMutation({
    mutationFn: async (params: SendAudioParams): Promise<LocalSendAudioResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: SendAudioResponse = await client.sendAudio(
        create(SendAudioRequestSchema, {
          instanceName: params.instanceName,
          groupId: params.groupId,
          audio: params.audio,
          mimetype: params.mimetype,
          ptt: params.ptt ?? true,
          caption: params.caption,
        })
      );

      return fromProtoSendAudioResponse(response);
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

  const sendPollMutation = useMutation({
    mutationFn: async (params: SendPollParams): Promise<LocalSendPollResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: SendPollResponse = await client.sendPoll(
        create(SendPollRequestSchema, {
          instanceName: params.instanceName,
          groupId: params.groupId,
          name: params.name,
          options: params.options,
          selectableCount: params.selectableCount,
        })
      );

      return fromProtoSendPollResponse(response);
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
  // Group Update Mutations
  // ============================================================

  const updateGroupSubjectMutation = useMutation({
    mutationFn: async (params: UpdateGroupSubjectParams): Promise<LocalUpdateGroupResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: UpdateGroupSubjectResponse = await client.updateGroupSubject(
        create(UpdateGroupSubjectRequestSchema, {
          instanceName: params.instanceName,
          groupId: params.groupId,
          subject: params.subject,
        })
      );

      return fromProtoUpdateGroupSubjectResponse(response);
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

  const updateGroupDescriptionMutation = useMutation({
    mutationFn: async (params: UpdateGroupDescriptionParams): Promise<LocalUpdateGroupResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: UpdateGroupDescriptionResponse = await client.updateGroupDescription(
        create(UpdateGroupDescriptionRequestSchema, {
          instanceName: params.instanceName,
          groupId: params.groupId,
          description: params.description,
        })
      );

      return fromProtoUpdateGroupDescriptionResponse(response);
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

  const updateGroupPictureMutation = useMutation({
    mutationFn: async (params: UpdateGroupPictureParams): Promise<LocalUpdateGroupResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: UpdateGroupPictureResponse = await client.updateGroupPicture(
        create(UpdateGroupPictureRequestSchema, {
          instanceName: params.instanceName,
          groupId: params.groupId,
          imageUrl: params.imageUrl,
        })
      );

      return fromProtoUpdateGroupPictureResponse(response);
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

  const updateGroupSettingsMutation = useMutation({
    mutationFn: async (params: UpdateGroupSettingsParams): Promise<LocalUpdateGroupResult> => {
      if (!user?.id) {
        throw new Error(t('common.errors.notAuthenticated'));
      }

      const client = createClient(WhatsAppService, transport);
      const response: UpdateGroupSettingsResponse = await client.updateGroupSettings(
        create(UpdateGroupSettingsRequestSchema, {
          instanceName: params.instanceName,
          groupId: params.groupId,
          action: toProtoGroupSettingsAction(params.action),
        })
      );

      return fromProtoUpdateGroupSettingsResponse(response);
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

  const createInstance = useCallback(
    async (instanceName?: string): Promise<LocalCreateInstanceResult> => {
      return createInstanceMutation.mutateAsync(instanceName);
    },
    [createInstanceMutation]
  );

  const deleteInstance = useCallback(
    async (instanceName: string): Promise<LocalDeleteInstanceResult> => {
      return deleteInstanceMutation.mutateAsync(instanceName);
    },
    [deleteInstanceMutation]
  );

  const reconnectInstance = useCallback(
    async (instanceName: string): Promise<LocalReconnectInstanceResult> => {
      return reconnectInstanceMutation.mutateAsync(instanceName);
    },
    [reconnectInstanceMutation]
  );

  const checkStatus = useCallback(
    async (instanceName: string, skipQr?: boolean): Promise<LocalCheckStatusResult> => {
      return checkStatusMutation.mutateAsync({ instanceName, skipQr });
    },
    [checkStatusMutation]
  );

  const requestPairingCode = useCallback(
    async (params: {
      instanceName: string;
      phoneNumber: string;
    }): Promise<LocalRequestPairingCodeResult> => {
      return requestPairingCodeMutation.mutateAsync(params);
    },
    [requestPairingCodeMutation]
  );

  const fetchGroups = useCallback(
    async (instanceName: string): Promise<LocalFetchGroupsResult> => {
      return fetchGroupsMutation.mutateAsync(instanceName);
    },
    [fetchGroupsMutation]
  );

  const fetchGroupParticipants = useCallback(
    async (instanceName: string, groupId: string): Promise<LocalFetchGroupParticipantsResult> => {
      return fetchGroupParticipantsMutation.mutateAsync({ instanceName, groupId });
    },
    [fetchGroupParticipantsMutation]
  );

  const syncGroups = useCallback(
    async (instanceName: string): Promise<LocalSyncGroupsResult> => {
      return syncGroupsMutation.mutateAsync(instanceName);
    },
    [syncGroupsMutation]
  );

  const syncGroupsStatus = useCallback(
    async (jobId: string): Promise<LocalSyncGroupsStatusResult> => {
      return syncGroupsStatusMutation.mutateAsync(jobId);
    },
    [syncGroupsStatusMutation]
  );

  const sendMessage = useCallback(
    async (params: SendMessageParams): Promise<LocalSendMessageResult> => {
      return sendMessageMutation.mutateAsync(params);
    },
    [sendMessageMutation]
  );

  const sendAudio = useCallback(
    async (params: SendAudioParams): Promise<LocalSendAudioResult> => {
      return sendAudioMutation.mutateAsync(params);
    },
    [sendAudioMutation]
  );

  const sendPoll = useCallback(
    async (params: SendPollParams): Promise<LocalSendPollResult> => {
      return sendPollMutation.mutateAsync(params);
    },
    [sendPollMutation]
  );

  const updateGroupSubject = useCallback(
    async (params: UpdateGroupSubjectParams): Promise<LocalUpdateGroupResult> => {
      return updateGroupSubjectMutation.mutateAsync(params);
    },
    [updateGroupSubjectMutation]
  );

  const updateGroupDescription = useCallback(
    async (params: UpdateGroupDescriptionParams): Promise<LocalUpdateGroupResult> => {
      return updateGroupDescriptionMutation.mutateAsync(params);
    },
    [updateGroupDescriptionMutation]
  );

  const updateGroupPicture = useCallback(
    async (params: UpdateGroupPictureParams): Promise<LocalUpdateGroupResult> => {
      return updateGroupPictureMutation.mutateAsync(params);
    },
    [updateGroupPictureMutation]
  );

  const updateGroupSettings = useCallback(
    async (params: UpdateGroupSettingsParams): Promise<LocalUpdateGroupResult> => {
      return updateGroupSettingsMutation.mutateAsync(params);
    },
    [updateGroupSettingsMutation]
  );

  return {
    // Instance Management
    createInstance,
    deleteInstance,
    reconnectInstance,
    checkStatus,
    requestPairingCode,

    // Group Management
    fetchGroups,
    fetchGroupParticipants,
    syncGroups,
    syncGroupsStatus,

    // Messaging
    sendMessage,
    sendAudio,
    sendPoll,

    // Group Updates
    updateGroupSubject,
    updateGroupDescription,
    updateGroupPicture,
    updateGroupSettings,

    // Loading States
    isCreatingInstance: createInstanceMutation.isPending,
    isDeletingInstance: deleteInstanceMutation.isPending,
    isReconnectingInstance: reconnectInstanceMutation.isPending,
    isCheckingStatus: checkStatusMutation.isPending,
    isFetchingGroups: fetchGroupsMutation.isPending,
    isFetchingParticipants: fetchGroupParticipantsMutation.isPending,
    isSyncingGroups: syncGroupsMutation.isPending,
    isCheckingSyncStatus: syncGroupsStatusMutation.isPending,
    isSendingMessage: sendMessageMutation.isPending,
    isSendingAudio: sendAudioMutation.isPending,
    isSendingPoll: sendPollMutation.isPending,
    isUpdatingSubject: updateGroupSubjectMutation.isPending,
    isUpdatingDescription: updateGroupDescriptionMutation.isPending,
    isUpdatingPicture: updateGroupPictureMutation.isPending,
    isUpdatingSettings: updateGroupSettingsMutation.isPending,
  };
}
