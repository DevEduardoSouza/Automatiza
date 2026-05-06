import { ErrorBoundary } from '@/components/error-boundary';
import {
  GroupBulkActions,
  GroupConnectionStatus,
  GroupInviteCodeDialog,
  GroupListTable,
  GroupMembersDialog,
  GroupReconnectDialog,
} from '@/components/groups';
import { Input } from '@/components/ui/input';
import { ChipSelector } from '@/components/whatsapp/chip-selector';
import { useGroups } from '@/hooks/use-groups';
import { useInstancesConnect } from '@/hooks/use-instances-connect';
import { cn } from '@/lib/utils';
import * as Sentry from '@sentry/react';
import { AlertCircle, RefreshCw, Search, WifiOff } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

const GroupsIcon = ({ className, style }: { className?: string; style?: React.CSSProperties }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    style={style}
  >
    <path d="M15.5 11C15.5 9.067 13.933 7.5 12 7.5C10.067 7.5 8.5 9.067 8.5 11C8.5 12.933 10.067 14.5 12 14.5C13.933 14.5 15.5 12.933 15.5 11Z" />
    <path d="M15.4827 11.3499C15.8047 11.4475 16.1462 11.5 16.5 11.5C18.433 11.5 20 9.933 20 8C20 6.067 18.433 4.5 16.5 4.5C14.6851 4.5 13.1928 5.8814 13.0173 7.65013" />
    <path d="M10.9827 7.65013C10.8072 5.8814 9.31492 4.5 7.5 4.5C5.567 4.5 4 6.067 4 8C4 9.933 5.567 11.5 7.5 11.5C7.85381 11.5 8.19535 11.4475 8.51727 11.3499" />
    <path d="M22 16.5C22 13.7386 19.5376 11.5 16.5 11.5" />
    <path d="M17.5 19.5C17.5 16.7386 15.0376 14.5 12 14.5C8.96243 14.5 6.5 16.7386 6.5 19.5" />
    <path d="M7.5 11.5C4.46243 11.5 2 13.7386 2 16.5" />
  </svg>
);

// Wine/burgundy color palette based on #8D2D53
const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
  iconBgHover: 'rgba(141, 45, 83, 0.15)',
};

/**
 * Página de gerenciamento de grupos do WhatsApp.
 * Permite visualizar, filtrar e executar ações em massa nos grupos.
 */
export default function Groups() {
  const { t } = useTranslation();

  // Chip filter (multi-instance): null = "Todos os números". Lifted above
  // useGroups so it can flow into the data hook and become a server-side
  // filter — this is what makes pagination and total count scope to the
  // selected chip instead of leaving the user staring at half-empty pages.
  const { instances } = useInstancesConnect();
  const [selectedChipId, setSelectedChipId] = useState<string | null>(null);

  const {
    groups,
    loading,
    error,
    searchQuery,
    selectedGroups,
    message,
    description,
    groupName,
    isEnhancing,
    mediaFile,
    groupPhoto,
    mentionOpen,
    mentionParticipants,
    loadingMentionParticipants,
    connectionStatus,
    instanceName,
    reconnecting,
    reconnectQrCode,
    showReconnectDialog,
    syncing,
    sendMessageDialogOpen,
    sendAudioDialogOpen,
    audioFile,
    audioPtt,
    audioCaption,
    sendingAudio,
    changeNameDialogOpen,
    changePhotoDialogOpen,
    changeDescriptionDialogOpen,
    autoNumberGroups,
    editInviteCodeDialogOpen,
    selectedGroupForInvite,
    inviteCodeInput,
    pollDialogOpen,
    pollQuestion,
    pollOptions,
    sendingPoll,
    filteredGroups,
    // Pagination state
    usePagination,
    currentPage,
    pageSize,
    totalCount,
    totalPages,
    hasMore,
    // Pagination actions
    setCurrentPage,
    setPageSize,
    setSearchQuery,
    setMessage,
    setDescription,
    setGroupName,
    setMentionOpen,
    setShowReconnectDialog,
    setSendMessageDialogOpen,
    setSendAudioDialogOpen,
    setAudioPtt,
    setAudioCaption,
    setChangeNameDialogOpen,
    setChangePhotoDialogOpen,
    setChangeDescriptionDialogOpen,
    setAutoNumberGroups,
    setEditInviteCodeDialogOpen,
    setSelectedGroupForInvite,
    setInviteCodeInput,
    setPollDialogOpen,
    setPollQuestion,
    setPollOptions,
    handleReconnect,
    handleSyncGroups,
    loadGroups,
    handleSaveInviteCode,
    toggleFavorite,
    insertMention,
    toggleGroup,
    toggleAll,
    handleSendMessage,
    handleAudioUpload,
    removeAudioFile,
    handleSendAudio,
    handleBulkAction,
    handleEnhanceMessage,
    handleMediaUpload,
    handlePhotoUpload,
    removeMediaFile,
    addPollOption,
    removePollOption,
    updatePollOption,
    handleSendPoll,
    members,
  } = useGroups({ instanceFilterId: selectedChipId });

  // Filter is server-side (via useGroups({ instanceFilterId })), so what
  // the data hook returns is already scoped to the selected chip. No
  // client-side filtering layer here on top.
  const displayedGroups = filteredGroups;

  useEffect(() => {
    if (error) {
      Sentry.captureException(error, {
        tags: { module: 'groups', flow: 'load-groups' },
      });
    }
  }, [error]);

  if (loading) {
    return (
      <div className="groups-page flex flex-col w-full h-full xl:max-w-[1400px] 2xl:max-w-[1600px] xl:mx-auto 2xl:mx-auto">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl lg:rounded-3xl p-4 xs:p-5 sm:p-6 md:p-8 lg:p-10 xl:p-12"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="flex items-center justify-center min-h-[300px] xs:min-h-[350px] sm:min-h-[400px]">
            <div className="text-center space-y-4">
              <div
                className="h-8 w-8 xs:h-10 xs:w-10 sm:h-12 sm:w-12 animate-spin rounded-full border-[3px] xs:border-4 border-t-transparent mx-auto"
                style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
              />
              <p className="text-xs xs:text-sm" style={{ color: colors.wineTextMuted }}>
                {t('pages.groups.loading')}
              </p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="groups-page flex flex-col w-full h-full xl:max-w-[1400px] 2xl:max-w-[1600px] xl:mx-auto 2xl:mx-auto">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl lg:rounded-3xl p-4 xs:p-5 sm:p-6 md:p-8 lg:p-10 xl:p-12"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="flex items-center justify-center min-h-[300px] xs:min-h-[350px] sm:min-h-[400px]">
            <div className="text-center space-y-4 max-w-md">
              <div
                className="p-3 rounded-full mx-auto w-fit"
                style={{ backgroundColor: 'rgba(239, 68, 68, 0.1)' }}
              >
                <WifiOff className="h-8 w-8 xs:h-10 xs:w-10" style={{ color: 'rgb(220, 50, 50)' }} />
              </div>
              <h3 className="text-base xs:text-lg font-semibold" style={{ color: '#18181b' }}>
                {t('pages.groups.errors.loadFailed')}
              </h3>
              <p className="text-xs xs:text-sm" style={{ color: colors.wineTextMuted }}>
                {t('pages.groups.errors.loadFailedMessage')}
              </p>
              <button
                type="button"
                onClick={() => loadGroups()}
                className="auth-btn-glow relative inline-flex cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm font-semibold text-white transition-all"
              >
                <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
                  <div className="auth-btn-glow__glow absolute inset-0" />
                </div>
                <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
                  <RefreshCw className="h-4 w-4" aria-hidden="true" />
                  {t('pages.groups.errors.retry')}
                </span>
              </button>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <ErrorBoundary>
      <div className="groups-page flex flex-col w-full h-full xl:max-w-[1400px] 2xl:max-w-[1600px] xl:mx-auto 2xl:mx-auto">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl lg:rounded-3xl p-4 xs:p-5 sm:p-6 md:p-8 lg:p-10 xl:p-12"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8 lg:space-y-10 xl:space-y-12">
            {/* Header */}
            <div className="flex flex-col gap-3 xs:gap-4 md:flex-row md:items-center md:justify-between">
              <div className="flex items-center gap-2 xs:gap-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                  style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
                >
                  <GroupsIcon className="w-5 h-5 xs:w-6 xs:h-6" style={{ color: colors.wine }} />
                </div>
                <div>
                  <h2
                    className="text-xl xs:text-2xl md:text-3xl xl:text-4xl font-semibold tracking-tight"
                    style={{ color: '#18181b' }}
                  >
                    {t('pages.groups.title')}
                  </h2>
                  <p
                    className="text-xs xs:text-sm tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.groups.subtitle', { count: totalCount })}
                  </p>
                </div>
              </div>
              <button
                type="button"
                onClick={handleSyncGroups}
                disabled={syncing || connectionStatus !== 'connected'}
                className="auth-btn-glow relative inline-flex w-full sm:w-auto cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
              >
                <div className="auth-btn-glow__loader absolute inset-0 z-[1] bg-transparent">
                  <div className="auth-btn-glow__glow absolute inset-0" />
                </div>
                <span className="relative z-[2] flex items-center gap-2 font-semibold select-none">
                  <RefreshCw
                    className={cn('h-4 w-4', syncing && 'animate-spin')}
                    aria-hidden="true"
                  />
                  {syncing ? t('pages.groups.sync.syncing') : t('pages.groups.sync.syncGroups')}
                </span>
              </button>
            </div>

            {/* Connection Status Alerts */}
            <GroupConnectionStatus
              status={connectionStatus}
              instanceName={instanceName}
              reconnecting={reconnecting}
              onReconnect={handleReconnect}
            />

            {/* QR Code Reconnect Dialog */}
            <GroupReconnectDialog
              open={showReconnectDialog}
              onOpenChange={setShowReconnectDialog}
              instanceName={instanceName}
              qrCode={reconnectQrCode}
            />

            {/* Admin Actions Disclaimer - Glassmorphism */}
            <div
              className="rounded-xl xs:rounded-2xl lg:rounded-3xl border p-4 xs:p-5 sm:p-6 lg:p-8 xl:p-10"
              role="alert"
              style={{
                backgroundColor: 'rgba(180, 120, 60, 0.08)',
                borderColor: 'rgba(180, 120, 60, 0.25)',
                backdropFilter: 'blur(12px) saturate(180%)',
                WebkitBackdropFilter: 'blur(12px) saturate(180%)',
              }}
            >
              <div className="flex items-center gap-2 xs:gap-3 md:gap-4 mb-2 xs:mb-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg"
                  style={{ backgroundColor: 'rgba(180, 120, 60, 0.12)' }}
                >
                  <AlertCircle
                    className="w-4 h-4 xs:w-5 xs:h-5"
                    style={{ color: 'rgb(160, 100, 40)' }}
                    aria-hidden="true"
                  />
                </div>
                <span
                  className="text-sm xs:text-base md:text-lg font-semibold tracking-tight"
                  style={{ color: 'rgb(120, 75, 30)' }}
                >
                  {t('pages.groups.warnings.adminOnly')}
                </span>
              </div>
              <p
                className="text-xs xs:text-sm leading-relaxed"
                style={{ color: 'rgba(120, 75, 30, 0.8)' }}
              >
                {t('pages.groups.warnings.adminOnlyDescription')}
              </p>
            </div>

            {/* Bulk Actions */}
            <GroupBulkActions
              selectedGroupsCount={selectedGroups.length}
              onCloseGroups={() => handleBulkAction('Fechar grupos')}
              onOpenGroups={() => handleBulkAction('Abrir grupos')}
              changeNameDialogOpen={changeNameDialogOpen}
              onChangeNameDialogOpenChange={setChangeNameDialogOpen}
              groupName={groupName}
              onGroupNameChange={setGroupName}
              autoNumberGroups={autoNumberGroups}
              onAutoNumberGroupsChange={setAutoNumberGroups}
              onChangeNameSubmit={() => handleBulkAction('Alterar nome')}
              changePhotoDialogOpen={changePhotoDialogOpen}
              onChangePhotoDialogOpenChange={setChangePhotoDialogOpen}
              groupPhoto={groupPhoto}
              onPhotoUpload={handlePhotoUpload}
              onChangePhotoSubmit={() => handleBulkAction('Alterar foto')}
              changeDescriptionDialogOpen={changeDescriptionDialogOpen}
              onChangeDescriptionDialogOpenChange={setChangeDescriptionDialogOpen}
              description={description}
              onDescriptionChange={setDescription}
              onChangeDescriptionSubmit={() => handleBulkAction('Alterar descrição')}
              sendMessageDialogOpen={sendMessageDialogOpen}
              onSendMessageDialogOpenChange={setSendMessageDialogOpen}
              message={message}
              onMessageChange={setMessage}
              mentionOpen={mentionOpen}
              onMentionOpenChange={setMentionOpen}
              onInsertMention={insertMention}
              mentionParticipants={mentionParticipants}
              loadingMentionParticipants={loadingMentionParticipants}
              isEnhancing={isEnhancing}
              onEnhanceMessage={handleEnhanceMessage}
              mediaFile={mediaFile}
              onMediaUpload={handleMediaUpload}
              onRemoveMedia={removeMediaFile}
              onSendMessageSubmit={handleSendMessage}
              sendAudioDialogOpen={sendAudioDialogOpen}
              onSendAudioDialogOpenChange={setSendAudioDialogOpen}
              audioFile={audioFile}
              onAudioUpload={handleAudioUpload}
              onRemoveAudio={removeAudioFile}
              audioPtt={audioPtt}
              onAudioPttChange={setAudioPtt}
              audioCaption={audioCaption}
              onAudioCaptionChange={setAudioCaption}
              sendingAudio={sendingAudio}
              onSendAudioSubmit={handleSendAudio}
              pollDialogOpen={pollDialogOpen}
              onPollDialogOpenChange={setPollDialogOpen}
              pollQuestion={pollQuestion}
              onPollQuestionChange={setPollQuestion}
              pollOptions={pollOptions}
              onAddPollOption={addPollOption}
              onRemovePollOption={removePollOption}
              onUpdatePollOption={updatePollOption}
              sendingPoll={sendingPoll}
              onSendPollSubmit={handleSendPoll}
            />

            {/* Search + chip filter */}
            <div className="flex flex-col sm:flex-row sm:items-center gap-3">
              <div className="relative flex-1 max-w-full sm:max-w-md lg:max-w-lg xl:max-w-xl">
                <Search
                  className="absolute left-2.5 xs:left-3 top-1/2 transform -translate-y-1/2 h-3.5 w-3.5 xs:h-4 xs:w-4"
                  style={{ color: colors.wineTextMuted }}
                  aria-hidden="true"
                />
                <Input
                  type="text"
                  placeholder={t('pages.groups.search.placeholder')}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-8 xs:pl-10"
                  aria-label={t('pages.groups.search.label')}
                />
              </div>
              {instances.length > 1 && (
                <ChipSelector
                  value={selectedChipId}
                  onChange={setSelectedChipId}
                  instances={instances}
                  allowAll
                  placeholder={t(
                    'pages.groups.chipFilter.placeholder',
                    'Filtrar por número'
                  )}
                />
              )}
            </div>

            {/* Groups Table */}
            <ErrorBoundary>
            <GroupListTable
              groups={groups}
              filteredGroups={displayedGroups}
              selectedGroups={selectedGroups}
              searchQuery={searchQuery}
              onToggleGroup={toggleGroup}
              onToggleAll={toggleAll}
              onToggleFavorite={toggleFavorite}
              onEditInviteCode={(group) => {
                setSelectedGroupForInvite(group);
                setInviteCodeInput(group.invite_code || '');
                setEditInviteCodeDialogOpen(true);
              }}
              onManageMembers={members.openMembersDialog}
              instances={instances}
              // Pagination props
              usePagination={usePagination}
              currentPage={currentPage}
              pageSize={pageSize}
              totalCount={totalCount}
              totalPages={totalPages}
              hasMore={hasMore}
              onPageChange={setCurrentPage}
              onPageSizeChange={setPageSize}
            />
            </ErrorBoundary>

            {/* Invite Code Dialog */}
            <GroupInviteCodeDialog
              open={editInviteCodeDialogOpen}
              onOpenChange={setEditInviteCodeDialogOpen}
              selectedGroup={selectedGroupForInvite}
              inviteCode={inviteCodeInput}
              onInviteCodeChange={setInviteCodeInput}
              onSubmit={handleSaveInviteCode}
            />

            {/* Members Management Dialog */}
            <GroupMembersDialog
              open={members.membersDialogOpen}
              onOpenChange={(open) => {
                if (!open) members.closeMembersDialog();
              }}
              groupName={members.selectedGroupForMembers?.name ?? ''}
              participants={members.participants}
              filteredParticipants={members.filteredParticipants}
              loading={members.loadingParticipants}
              phoneNumberInput={members.phoneNumberInput}
              searchQuery={members.searchQuery}
              onPhoneNumberChange={members.setPhoneNumberInput}
              onSearchChange={members.setSearchQuery}
              onAddMember={members.handleAddMember}
              onRemoveMember={members.handleRemoveMember}
              onLeaveGroup={members.handleLeaveGroup}
              onPromoteMember={members.handlePromoteMember}
              onDemoteMember={members.handleDemoteMember}
              isAddingMember={members.isAddingMember}
              isRemovingMember={members.isRemovingMember}
              isLeavingGroup={members.isLeavingGroup}
              isPromotingMember={members.isPromotingMember}
              isDemotingMember={members.isDemotingMember}
            />
          </div>
        </div>
      </div>
    </ErrorBoundary>
  );
}
