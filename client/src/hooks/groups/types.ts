import type { GroupParticipant } from '@/hooks/use-groups-connect';
import type { Group } from '@/types/group';
import type { ChangeEvent } from 'react';
import type { useNavigate } from 'react-router';
import type { UseGroupsMembersReturn } from './use-groups-members';

/**
 * Bulk action types for group operations
 */
export type BulkActionType = 'close' | 'open' | 'rename' | 'photo' | 'description';

/**
 * Connection status for WhatsApp instance
 */
export type ConnectionStatus = 'checking' | 'connected' | 'disconnected';

/**
 * Configuration for bulk actions - maps action type to history type and i18n key
 */
export const BULK_ACTION_CONFIG: Record<BulkActionType, { historyType: string; i18nKey: string }> =
  {
    close: {
      historyType: 'close_groups',
      i18nKey: 'pages.groups.bulkActions.closeGroups',
    },
    open: {
      historyType: 'open_groups',
      i18nKey: 'pages.groups.bulkActions.openGroups',
    },
    rename: {
      historyType: 'change_name',
      i18nKey: 'pages.groups.bulkActions.changeName',
    },
    photo: {
      historyType: 'change_photo',
      i18nKey: 'pages.groups.bulkActions.changePhoto',
    },
    description: {
      historyType: 'change_description',
      i18nKey: 'pages.groups.bulkActions.changeDescription',
    },
  };

/**
 * Maps legacy action strings to BulkActionType
 */
export const ACTION_STRING_MAP: Record<string, BulkActionType> = {
  'Fechar grupos': 'close',
  'Abrir grupos': 'open',
  'Alterar nome': 'rename',
  'Alterar foto': 'photo',
  'Alterar descrição': 'description',
};

/**
 * Return type for useGroupsData hook
 */
export interface UseGroupsDataReturn {
  groups: Group[];
  loading: boolean;
  error: Error | null;
  searchQuery: string;
  filteredGroups: Group[];
  syncing: boolean;
  setSearchQuery: (value: string) => void;
  loadGroups: () => Promise<void>;
  handleSyncGroups: () => Promise<void>;
  // Pagination state
  usePagination: boolean;
  currentPage: number;
  pageSize: number;
  totalCount: number;
  totalPages: number;
  hasMore: boolean;
  // Pagination actions
  enablePagination: () => void;
  disablePagination: () => void;
  setCurrentPage: (page: number) => void;
  setPageSize: (size: number) => void;
  goToNextPage: () => void;
  goToPreviousPage: () => void;
  // Optimistic updates
  updateGroupOptimistic: (groupId: string, updates: Partial<Group>) => void;
  clearOptimisticUpdate: (groupId: string) => void;
}

/**
 * Return type for useGroupsSelection hook
 */
export interface UseGroupsSelectionReturn {
  selectedGroups: string[];
  toggleGroup: (id: string) => void;
  toggleAll: (groups: Group[]) => void;
  clearSelection: () => void;
}

/**
 * Return type for useConnectionManager hook
 */
export interface UseConnectionManagerReturn {
  connectionStatus: ConnectionStatus;
  instanceName: string;
  reconnecting: boolean;
  reconnectQrCode: string | null;
  showReconnectDialog: boolean;
  setShowReconnectDialog: (value: boolean) => void;
  handleReconnect: () => Promise<void>;
  checkConnectionStatus: () => Promise<void>;
}

/**
 * Return type for useGroupsMessaging hook
 */
export interface UseGroupsMessagingReturn {
  message: string;
  mediaFile: File | null;
  mentionOpen: boolean;
  sendMessageDialogOpen: boolean;
  mentionParticipants: GroupParticipant[];
  loadingMentionParticipants: boolean;
  setMessage: (value: string) => void;
  setMentionOpen: (value: boolean) => void;
  setSendMessageDialogOpen: (value: boolean) => void;
  insertMention: (mention: string) => void;
  handleSendMessage: () => Promise<void>;
  handleMediaUpload: (e: React.ChangeEvent<HTMLInputElement>) => void;
  removeMediaFile: () => void;
}

/**
 * Return type for useGroupsAudio hook
 */
export interface UseGroupsAudioReturn {
  audioFile: File | null;
  audioPtt: boolean;
  audioCaption: string;
  sendingAudio: boolean;
  sendAudioDialogOpen: boolean;
  setAudioPtt: (value: boolean) => void;
  setAudioCaption: (value: string) => void;
  setSendAudioDialogOpen: (value: boolean) => void;
  handleAudioUpload: (e: ChangeEvent<HTMLInputElement>) => void;
  removeAudioFile: () => void;
  handleSendAudio: () => Promise<void>;
}

/**
 * Return type for useGroupsBulkActions hook
 */
export interface UseGroupsBulkActionsReturn {
  groupName: string;
  groupPhoto: File | null;
  description: string;
  autoNumberGroups: boolean;
  changeNameDialogOpen: boolean;
  changePhotoDialogOpen: boolean;
  changeDescriptionDialogOpen: boolean;
  setGroupName: (value: string) => void;
  setDescription: (value: string) => void;
  setAutoNumberGroups: (value: boolean) => void;
  setChangeNameDialogOpen: (value: boolean) => void;
  setChangePhotoDialogOpen: (value: boolean) => void;
  setChangeDescriptionDialogOpen: (value: boolean) => void;
  handlePhotoUpload: (e: React.ChangeEvent<HTMLInputElement>) => void;
  handleBulkAction: (action: string) => Promise<void>;
}

/**
 * Return type for useGroupsPoll hook
 */
export interface UseGroupsPollReturn {
  pollDialogOpen: boolean;
  pollQuestion: string;
  pollOptions: string[];
  sendingPoll: boolean;
  setPollDialogOpen: (value: boolean) => void;
  setPollQuestion: (value: string) => void;
  setPollOptions: (options: string[]) => void;
  addPollOption: () => void;
  removePollOption: (index: number) => void;
  updatePollOption: (index: number, value: string) => void;
  handleSendPoll: () => Promise<void>;
}

/**
 * Return type for useGroupsUI hook
 */
export interface UseGroupsUIReturn {
  editInviteCodeDialogOpen: boolean;
  selectedGroupForInvite: Group | null;
  inviteCodeInput: string;
  setEditInviteCodeDialogOpen: (value: boolean) => void;
  setSelectedGroupForInvite: (group: Group | null) => void;
  setInviteCodeInput: (value: string) => void;
  handleSaveInviteCode: () => Promise<void>;
  toggleFavorite: (groupId: string, currentFavorite: boolean) => Promise<void>;
  getStatusBadge: (status: string) => { label: string; className: string } | undefined;
}

/**
 * Return type for useGroupsEnhance hook
 */
export interface UseGroupsEnhanceReturn {
  isEnhancing: boolean;
  handleEnhanceMessage: (message: string, setMessage: (msg: string) => void) => Promise<void>;
}

/**
 * Combined return type for useGroups hook
 */
export interface UseGroupsReturn {
  groups: Group[];
  loading: boolean;
  error: Error | null;
  searchQuery: string;
  selectedGroups: string[];
  message: string;
  description: string;
  groupName: string;
  isEnhancing: boolean;
  mediaFile: File | null;
  groupPhoto: File | null;
  mentionOpen: boolean;
  mentionParticipants: GroupParticipant[];
  loadingMentionParticipants: boolean;
  connectionStatus: ConnectionStatus;
  instanceName: string;
  reconnecting: boolean;
  reconnectQrCode: string | null;
  showReconnectDialog: boolean;
  syncing: boolean;
  sendMessageDialogOpen: boolean;
  sendAudioDialogOpen: boolean;
  audioFile: File | null;
  audioPtt: boolean;
  audioCaption: string;
  sendingAudio: boolean;
  changeNameDialogOpen: boolean;
  changePhotoDialogOpen: boolean;
  changeDescriptionDialogOpen: boolean;
  autoNumberGroups: boolean;
  editInviteCodeDialogOpen: boolean;
  selectedGroupForInvite: Group | null;
  inviteCodeInput: string;
  pollDialogOpen: boolean;
  pollQuestion: string;
  pollOptions: string[];
  sendingPoll: boolean;
  filteredGroups: Group[];
  // Pagination state
  usePagination: boolean;
  currentPage: number;
  pageSize: number;
  totalCount: number;
  totalPages: number;
  hasMore: boolean;
  // Pagination actions
  enablePagination: () => void;
  disablePagination: () => void;
  setCurrentPage: (page: number) => void;
  setPageSize: (size: number) => void;
  goToNextPage: () => void;
  goToPreviousPage: () => void;
  setSearchQuery: (value: string) => void;
  setMessage: (value: string) => void;
  setDescription: (value: string) => void;
  setGroupName: (value: string) => void;
  setMentionOpen: (value: boolean) => void;
  setShowReconnectDialog: (value: boolean) => void;
  setSendMessageDialogOpen: (value: boolean) => void;
  setSendAudioDialogOpen: (value: boolean) => void;
  setAudioPtt: (value: boolean) => void;
  setAudioCaption: (value: string) => void;
  setChangeNameDialogOpen: (value: boolean) => void;
  setChangePhotoDialogOpen: (value: boolean) => void;
  setChangeDescriptionDialogOpen: (value: boolean) => void;
  setAutoNumberGroups: (value: boolean) => void;
  setEditInviteCodeDialogOpen: (value: boolean) => void;
  setSelectedGroupForInvite: (group: Group | null) => void;
  setInviteCodeInput: (value: string) => void;
  setPollDialogOpen: (value: boolean) => void;
  setPollQuestion: (value: string) => void;
  setPollOptions: (options: string[]) => void;
  handleReconnect: () => Promise<void>;
  handleSyncGroups: () => Promise<void>;
  handleSaveInviteCode: () => Promise<void>;
  toggleFavorite: (groupId: string, currentFavorite: boolean) => Promise<void>;
  insertMention: (mention: string) => void;
  toggleGroup: (id: string) => void;
  toggleAll: () => void;
  handleSendMessage: () => Promise<void>;
  handleAudioUpload: (e: ChangeEvent<HTMLInputElement>) => void;
  removeAudioFile: () => void;
  handleSendAudio: () => Promise<void>;
  handleBulkAction: (action: string) => Promise<void>;
  handleEnhanceMessage: () => Promise<void>;
  handleMediaUpload: (e: React.ChangeEvent<HTMLInputElement>) => void;
  handlePhotoUpload: (e: React.ChangeEvent<HTMLInputElement>) => void;
  removeMediaFile: () => void;
  addPollOption: () => void;
  removePollOption: (index: number) => void;
  updatePollOption: (index: number, value: string) => void;
  handleSendPoll: () => Promise<void>;
  getStatusBadge: (status: string) => { label: string; className: string } | undefined;
  loadGroups: () => Promise<void>;
  navigate: ReturnType<typeof useNavigate>;

  // Members management
  members: UseGroupsMembersReturn;
}
