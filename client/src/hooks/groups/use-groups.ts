import type { Group } from '@/types/group';
import { useCallback, useState } from 'react';
import { useNavigate } from 'react-router';
import type { UseGroupsReturn } from './types';
import { useConnectionManager } from './use-connection-manager';
import { useGroupsAudio } from './use-groups-audio';
import { useGroupsBulkActions } from './use-groups-bulk-actions';
import { useGroupsData } from './use-groups-data';
import { useGroupsEnhance } from './use-groups-enhance';
import { useGroupsMembers } from './use-groups-members';
import { useGroupsMessaging } from './use-groups-messaging';
import { useGroupsPoll } from './use-groups-poll';
import { useGroupsRealtime } from './use-groups-realtime';
import { useGroupsSelection } from './use-groups-selection';
import { useGroupsUI } from './use-groups-ui';

interface UseGroupsOptions {
  // Forwarded to useGroupsData; filters the server query and scopes
  // pagination to the chip. null/undefined = no chip filter.
  instanceFilterId?: string | null;
}

/**
 * Composite hook for groups page logic
 * Manages WhatsApp groups, messages, polls, and bulk actions
 */
export function useGroups(options: UseGroupsOptions = {}): UseGroupsReturn {
  const navigate = useNavigate();

  // Internal state for groups (needed for setGroups in UI hook)
  const [internalGroups, setInternalGroups] = useState<Group[]>([]);

  // Data management
  const data = useGroupsData({ instanceFilterId: options.instanceFilterId });

  // Sync internal groups with data groups
  const groups = data.groups.length > 0 ? data.groups : internalGroups;

  // Selection management
  const selection = useGroupsSelection();

  // Connection management
  const connection = useConnectionManager(data.loadGroups);

  // Messaging
  const messaging = useGroupsMessaging({
    selectedGroups: selection.selectedGroups,
    groups,
    clearSelection: selection.clearSelection,
  });

  // Audio
  const audio = useGroupsAudio({
    selectedGroups: selection.selectedGroups,
    groups,
    clearSelection: selection.clearSelection,
  });

  // Loading state for bulk actions
  const [bulkLoading, setBulkLoading] = useState(false);

  // Bulk actions
  const bulk = useGroupsBulkActions({
    selectedGroups: selection.selectedGroups,
    groups,
    instanceName: connection.instanceName,
    loadGroups: data.loadGroups,
    clearSelection: selection.clearSelection,
    setLoading: setBulkLoading,
  });

  // Poll
  const poll = useGroupsPoll({
    selectedGroups: selection.selectedGroups,
    groups,
    clearSelection: selection.clearSelection,
  });

  // UI helpers
  const ui = useGroupsUI({
    loadGroups: data.loadGroups,
    updateGroupOptimistic: data.updateGroupOptimistic,
  });

  // Members management
  const members = useGroupsMembers();

  // Real-time SSE updates (auto-refetches groups on change events)
  useGroupsRealtime();

  // AI enhance
  const enhance = useGroupsEnhance();

  // Wrapper for toggleAll that passes current groups
  const toggleAll = useCallback(() => {
    selection.toggleAll(groups);
  }, [selection, groups]);

  // Wrapper for enhance that uses messaging's message state
  const handleEnhanceMessage = useCallback(async () => {
    await enhance.handleEnhanceMessage(messaging.message, messaging.setMessage);
  }, [enhance, messaging.message, messaging.setMessage]);

  // Combined loading state
  const loading = data.loading || bulkLoading;

  return {
    // Data
    groups,
    loading,
    error: data.error,
    searchQuery: data.searchQuery,
    filteredGroups: data.filteredGroups,
    syncing: data.syncing,
    setSearchQuery: data.setSearchQuery,
    loadGroups: data.loadGroups,
    handleSyncGroups: data.handleSyncGroups,

    // Pagination
    usePagination: data.usePagination,
    currentPage: data.currentPage,
    pageSize: data.pageSize,
    totalCount: data.totalCount,
    totalPages: data.totalPages,
    hasMore: data.hasMore,
    enablePagination: data.enablePagination,
    disablePagination: data.disablePagination,
    setCurrentPage: data.setCurrentPage,
    setPageSize: data.setPageSize,
    goToNextPage: data.goToNextPage,
    goToPreviousPage: data.goToPreviousPage,

    // Selection
    selectedGroups: selection.selectedGroups,
    toggleGroup: selection.toggleGroup,
    toggleAll,

    // Connection
    connectionStatus: connection.connectionStatus,
    instanceName: connection.instanceName,
    reconnecting: connection.reconnecting,
    reconnectQrCode: connection.reconnectQrCode,
    showReconnectDialog: connection.showReconnectDialog,
    setShowReconnectDialog: connection.setShowReconnectDialog,
    handleReconnect: connection.handleReconnect,

    // Messaging
    message: messaging.message,
    mediaFile: messaging.mediaFile,
    mentionOpen: messaging.mentionOpen,
    mentionParticipants: messaging.mentionParticipants,
    loadingMentionParticipants: messaging.loadingMentionParticipants,
    sendMessageDialogOpen: messaging.sendMessageDialogOpen,
    setMessage: messaging.setMessage,
    setMentionOpen: messaging.setMentionOpen,
    setSendMessageDialogOpen: messaging.setSendMessageDialogOpen,
    insertMention: messaging.insertMention,
    handleSendMessage: messaging.handleSendMessage,
    handleMediaUpload: messaging.handleMediaUpload,
    removeMediaFile: messaging.removeMediaFile,

    // Audio
    audioFile: audio.audioFile,
    audioPtt: audio.audioPtt,
    audioCaption: audio.audioCaption,
    sendingAudio: audio.sendingAudio,
    sendAudioDialogOpen: audio.sendAudioDialogOpen,
    setAudioPtt: audio.setAudioPtt,
    setAudioCaption: audio.setAudioCaption,
    setSendAudioDialogOpen: audio.setSendAudioDialogOpen,
    handleAudioUpload: audio.handleAudioUpload,
    removeAudioFile: audio.removeAudioFile,
    handleSendAudio: audio.handleSendAudio,

    // Bulk actions
    groupName: bulk.groupName,
    groupPhoto: bulk.groupPhoto,
    description: bulk.description,
    autoNumberGroups: bulk.autoNumberGroups,
    changeNameDialogOpen: bulk.changeNameDialogOpen,
    changePhotoDialogOpen: bulk.changePhotoDialogOpen,
    changeDescriptionDialogOpen: bulk.changeDescriptionDialogOpen,
    setGroupName: bulk.setGroupName,
    setDescription: bulk.setDescription,
    setAutoNumberGroups: bulk.setAutoNumberGroups,
    setChangeNameDialogOpen: bulk.setChangeNameDialogOpen,
    setChangePhotoDialogOpen: bulk.setChangePhotoDialogOpen,
    setChangeDescriptionDialogOpen: bulk.setChangeDescriptionDialogOpen,
    handlePhotoUpload: bulk.handlePhotoUpload,
    handleBulkAction: bulk.handleBulkAction,

    // Poll
    pollDialogOpen: poll.pollDialogOpen,
    pollQuestion: poll.pollQuestion,
    pollOptions: poll.pollOptions,
    sendingPoll: poll.sendingPoll,
    setPollDialogOpen: poll.setPollDialogOpen,
    setPollQuestion: poll.setPollQuestion,
    setPollOptions: poll.setPollOptions,
    addPollOption: poll.addPollOption,
    removePollOption: poll.removePollOption,
    updatePollOption: poll.updatePollOption,
    handleSendPoll: poll.handleSendPoll,

    // UI
    editInviteCodeDialogOpen: ui.editInviteCodeDialogOpen,
    selectedGroupForInvite: ui.selectedGroupForInvite,
    inviteCodeInput: ui.inviteCodeInput,
    setEditInviteCodeDialogOpen: ui.setEditInviteCodeDialogOpen,
    setSelectedGroupForInvite: ui.setSelectedGroupForInvite,
    setInviteCodeInput: ui.setInviteCodeInput,
    handleSaveInviteCode: ui.handleSaveInviteCode,
    toggleFavorite: ui.toggleFavorite,
    getStatusBadge: ui.getStatusBadge,

    // Enhance
    isEnhancing: enhance.isEnhancing,
    handleEnhanceMessage,

    // Members management
    members,

    // Navigation
    navigate,
  };
}
