import { ErrorBoundary } from '@/components/error-boundary';
import {
  GroupBulkActions,
  GroupConnectionStatus,
  GroupInviteCodeDialog,
  GroupListTable,
  GroupMembersDialog,
  GroupReconnectDialog,
} from '@/components/groups';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useGroups } from '@/hooks/use-groups';
import { cn } from '@/lib/utils';
import { AlertCircle, RefreshCw, Search, Users } from 'lucide-react';
import { useTranslation } from 'react-i18next';

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
  const {
    groups,
    loading,
    searchQuery,
    selectedGroups,
    message,
    description,
    groupName,
    isEnhancing,
    mediaFile,
    groupPhoto,
    mentionOpen,
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
  } = useGroups();

  if (loading) {
    return (
      <div className="groups-page flex flex-col w-full h-full">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="flex items-center justify-center min-h-[400px]">
            <div className="text-center space-y-4">
              <div
                className="h-12 w-12 animate-spin rounded-full border-4 border-t-transparent mx-auto"
                style={{ borderColor: `${colors.wine} transparent transparent transparent` }}
              />
              <p style={{ color: colors.wineTextMuted }}>{t('pages.groups.loading')}</p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <ErrorBoundary>
      <div className="groups-page flex flex-col w-full h-full">
        <div
          className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8"
          style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
        >
          <div className="space-y-4 xs:space-y-5 sm:space-y-6 md:space-y-8">
            {/* Header */}
            <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
              <div className="flex items-center gap-2 xs:gap-3">
                <div
                  className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
                  style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
                >
                  <Users className="w-5 h-5 xs:w-6 xs:h-6" style={{ color: colors.wine }} />
                </div>
                <div>
                  <h1
                    className="text-xl xs:text-2xl md:text-3xl font-semibold tracking-tight"
                    style={{ color: '#18181b' }}
                  >
                    {t('pages.groups.title')}
                  </h1>
                  <p
                    className="text-xs xs:text-sm md:text-base tracking-tight"
                    style={{ color: colors.wineTextMuted }}
                  >
                    {t('pages.groups.subtitle', { count: groups.length })}
                  </p>
                </div>
              </div>
              <Button
                onClick={handleSyncGroups}
                disabled={syncing || connectionStatus !== 'connected'}
                className="bg-[#8D2D53] text-white hover:bg-[#7a2747]"
              >
                <RefreshCw
                  className={cn('mr-2 h-4 w-4', syncing && 'animate-spin')}
                  aria-hidden="true"
                />
                {syncing ? t('pages.groups.sync.syncing') : t('pages.groups.sync.syncGroups')}
              </Button>
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
              className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6"
              role="alert"
              style={{
                backgroundColor: 'rgba(180, 120, 60, 0.08)',
                borderColor: 'rgba(180, 120, 60, 0.25)',
                backdropFilter: 'blur(12px) saturate(180%)',
                WebkitBackdropFilter: 'blur(12px) saturate(180%)',
              }}
            >
              <div className="flex items-center gap-2 xs:gap-3 mb-2 xs:mb-3">
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
                  className="text-sm xs:text-base font-semibold tracking-tight"
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

            {/* Search */}
            <div className="relative max-w-md">
              <Search
                className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4"
                style={{ color: colors.wineTextMuted }}
                aria-hidden="true"
              />
              <Input
                type="text"
                placeholder={t('pages.groups.search.placeholder')}
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-10"
                aria-label={t('pages.groups.search.label')}
              />
            </div>

            {/* Groups Table */}
            <GroupListTable
              groups={groups}
              filteredGroups={filteredGroups}
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
